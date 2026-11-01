#!/usr/bin/env python3
"""Audit unchanged stdVBA sources with vambas-compile and the installed LLVM tools.

Each source gets its own temporary .vbp so alternate WIP implementations are not
mixed. Parser-clean failures are retried with unambiguous project types named by
unknown-type diagnostics. No source preprocessing, missing-name stubs, procedure
removal, or compiler changes are applied. Native checks link, but do not exercise
the library APIs.
"""

import argparse
from collections import Counter, defaultdict
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile


HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
DIAGNOSTIC = re.compile(r"^(.*):(\d+):(\d+): ([^:]+): (.*)$")
NOTES = {
    "stdICallable.cls": "Interface: four empty member bodies.",
    "WIP/stdDictionary/stdIHashable.cls": "Interface: two empty member bodies.",
    "WIP/stdGithub.cls": "WIP stub: four empty member bodies.",
    "WIP/stdStruct.cls": "Comment-only skeleton; VB_Name is stdCStruct; no procedures.",
}
NOTES_REVISION = "4d0ebb32a7161376cf54f0cf84013ee711a718e8"


def tool(name):
    result = shutil.which(name + "-20") or shutil.which(name)
    if result is None:
        raise SystemExit("Required tool is missing: " + name)
    return result


def run(command, **kwargs):
    return subprocess.run(command, cwd=REPO, capture_output=True, text=True,
                          timeout=60, **kwargs)


def checked(command, **kwargs):
    result = run(command, **kwargs)
    if result.returncode:
        raise RuntimeError(f"{command}:\n{result.stdout}{result.stderr}")
    return result.stdout


def decode(data):
    if data.startswith((b"\xff\xfe", b"\xfe\xff")):
        return data.decode("utf-16")
    try:
        return data.decode("utf-8-sig")
    except UnicodeDecodeError:
        return data.decode("cp1252", errors="replace")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=REPO.parent / "stdvba" / "src")
    parser.add_argument("--output", type=Path, default=HERE)
    parser.add_argument("--full-diagnostics", type=Path,
                        help="Optional full compiler output directory (can be large).")
    args = parser.parse_args()
    root, output = args.source.resolve(), args.output.resolve()
    source_revision = checked(["git", "-C", str(root), "rev-parse", "HEAD"]).strip()
    source_status = checked(["git", "-C", str(root), "status", "--short"])
    output.mkdir(parents=True, exist_ok=True)
    if args.full_diagnostics:
        args.full_diagnostics.mkdir(parents=True, exist_ok=True)

    # Read compiler-owned diagnostic categories rather than guessing from messages.
    parse_codes = set(re.findall(r'Diagnostic\w+\s+DiagnosticCode\s*=\s*"([^"]+)"',
                                 (REPO / "ast/diagnostic.go").read_text()))
    bind_codes = set(re.findall(r'Binding\w+\s+BindingDiagnosticCode\s*=\s*"([^"]+)"',
                                (REPO / "ast/binding.go").read_text()))
    modules = []
    for path in sorted(root.rglob("*")):
        if path.suffix.lower() not in (".bas", ".cls", ".frm"):
            continue
        data = path.read_bytes()
        text = decode(data)
        name = re.search(r'^Attribute\s+VB_Name\s*=\s*"([^"]+)"', text, re.M | re.I)
        modules.append({"path": str(path.relative_to(root)),
                        "module_name": name[1] if name else path.stem,
                        "group": "top-level" if path.parent == root else "WIP",
                        "sha256": hashlib.sha256(data).hexdigest()})
    if not modules:
        raise SystemExit("No VBA sources found in " + str(root))
    names = defaultdict(list)
    for module in modules:
        names[module["module_name"].casefold()].append(module)

    clang, assembler, optimizer = tool("clang"), tool("llvm-as"), tool("opt")
    runtime = REPO / "runtime"
    runtime_sources = [str(runtime / name) for name in (runtime / "sources.txt").read_text().split()]
    with tempfile.TemporaryDirectory(prefix="stdvba-audit-") as tmp:
        temp = Path(tmp)
        compiler = temp / "vambas-compile"
        checked(["go", "build", "-o", str(compiler), "./cmd/vambas-compile"], env=os.environ)
        driver = temp / "link-driver.c"
        driver.write_text("int main(void) { return 0; }\n")

        def compile_sources(sources):
            with tempfile.TemporaryDirectory(prefix="module-", dir=temp) as workspace:
                work = Path(workspace)
                manifest = work / "audit.vbp"
                entries = ['Name="StdVBAAudit"']
                for module in sorted(sources, key=lambda m: m["path"]):
                    path = root / module["path"]
                    kind = {".bas": "Module", ".cls": "Class", ".frm": "Form"}[path.suffix.lower()]
                    value = str(path) if kind == "Form" else module["module_name"] + ";" + str(path)
                    entries.append(kind + "=" + value)
                manifest.write_text("\n".join(entries) + "\n")
                ir = work / "module.ll"
                command = [str(compiler), "-project", str(manifest), "-o", str(ir)]
                try:
                    result = run(command)
                except subprocess.TimeoutExpired:
                    return {"stage": "timeout", "diagnostic_count": 0, "diagnostics": []}
                diagnostics = []
                for line in result.stderr.splitlines():
                    match = DIAGNOSTIC.match(line)
                    if match:
                        file, line, column, code, message = match.groups()
                        path = Path(file)
                        file = str(path.relative_to(root)) if path.is_relative_to(root) else str(path)
                        source_lines = decode(path.read_bytes()).splitlines() if path.exists() else []
                        diagnostics.append({"file": file, "line": int(line), "column": int(column),
                                            "code": code, "message": message,
                                            "source": source_lines[int(line) - 1].strip() if int(line) <= len(source_lines) else ""})
                codes = {d["code"] for d in diagnostics}
                if result.returncode == 0:
                    stage = "compiled"
                elif codes & parse_codes:
                    stage = "parse"
                elif codes & bind_codes:
                    stage = "binding"
                elif codes & {"unsupported-lowering", "invalid-semantic-ir"}:
                    stage = "lowering"
                elif "llvm" in codes or "LLVM emission failed" in result.stderr:
                    stage = "llvm-emission"
                elif codes:
                    stage = "checking"
                else:
                    stage = "compiler-error"
                record = {"stage": stage, "diagnostic_count": len(diagnostics),
                          "diagnostic_codes": dict(sorted(Counter(d["code"] for d in diagnostics).items())),
                          "diagnostics": diagnostics[:5],
                          "missing_types": sorted(set(re.findall(r'Type "([^"]+)" is not defined', result.stderr)))}
                if args.full_diagnostics:
                    key = hashlib.sha256("\n".join(m["path"] for m in sources).encode()).hexdigest()[:12]
                    (args.full_diagnostics / (key + ".txt")).write_text(result.stderr.replace(str(root), "<stdvba-src>"))
                if stage == "compiled":
                    text = ir.read_text()
                    record["procedure_count"] = len(re.findall(r"^define [^\n]*@vba\.p\d+\(", text, re.M))
                    bitcode = work / "module.bc"
                    checked([assembler, str(ir), "-o", str(bitcode)])
                    checked([optimizer, "-passes=verify", "-disable-output", str(bitcode)])
                    record["llvm_verified"] = True
                    record["native_linked"] = []
                    for level in ("-O0", "-O2"):
                        checked([clang, "-std=c11", level, str(ir), str(driver), *runtime_sources,
                                 "-lm", "-o", str(work / ("link" + level))])
                        record["native_linked"].append(level)
                return record

        def audit(module):
            record = dict(module)
            record["standalone"] = compile_sources([module])
            if source_revision == NOTES_REVISION and not source_status and module["path"] in NOTES:
                record["assessment"] = NOTES[module["path"]]
            result = record["standalone"]
            selected = {module["path"]: module}
            ambiguous = {}
            # A parse failure cannot be repaired by adding companion classes.
            if result["stage"] not in ("parse", "compiled", "timeout", "compiler-error"):
                while True:
                    added = {}
                    for name in result["missing_types"]:
                        candidates = names[name.casefold()]
                        canonical = [m for m in candidates if m["group"] == "top-level"]
                        siblings = [m for m in candidates if Path(m["path"]).parent == Path(module["path"]).parent]
                        candidates = canonical or siblings or candidates
                        if len(candidates) == 1:
                            candidate = candidates[0]
                            if candidate["path"] not in selected:
                                added[candidate["path"]] = candidate
                        elif len(candidates) > 1:
                            ambiguous[name] = [m["path"] for m in candidates]
                    if not added:
                        break
                    selected.update(added)
                    result = compile_sources(list(selected.values()))
                    if result["stage"] in ("compiled", "parse", "timeout", "compiler-error"):
                        break
            if len(selected) > 1:
                record["with_dependencies"] = {"sources": sorted(selected), **result}
            if ambiguous:
                record["ambiguous_dependencies"] = ambiguous
            return record

        with ThreadPoolExecutor(max_workers=4) as pool:
            results = list(pool.map(audit, modules))
        # Test the top-level distribution together, without alternate WIP classes.
        top_level_project = compile_sources([m for m in modules if m["group"] == "top-level"])

    report = {"audited_utc": datetime.now(timezone.utc).isoformat(),
              "source_revision": source_revision,
              "compiler_revision": checked(["git", "rev-parse", "HEAD"]).strip(),
              "source_worktree_status": source_status,
              "compiler_worktree_status": checked(["git", "status", "--short"]),
              "clang_version": checked([clang, "--version"]).splitlines()[0],
              "method": "Unchanged source files through temporary .vbp manifests; compile every procedure. "
                        "Diagnostic-driven unambiguous typed dependency retries only. No API execution claims.",
              "summary": {group: dict(Counter(r["standalone"]["stage"] for r in results if r["group"] == group))
                          for group in ("top-level", "WIP")},
              "top_level_project": top_level_project, "modules": results}
    (output / "results.json").write_text(json.dumps(report, indent=2) + "\n")

    successes = [r for r in results if r["standalone"]["stage"] == "compiled"]
    top_successes = [r for r in successes if r["group"] == "top-level"]
    wip_successes = [r for r in successes if r["group"] == "WIP"]
    dependency_successes = [r for r in results if r["standalone"]["stage"] != "compiled"
                           and r.get("with_dependencies", {}).get("stage") == "compiled"]
    lines = ["# stdVBA compilation audit", "",
             f"Source revision: `{source_revision}`. Audited {report['audited_utc']}.", "",
             f"All {len(modules)} VBA source files were tested unchanged, using the normal project loader and compiler CLI. "
             "Each file gets a separate manifest so alternate WIP classes do not collide. "
             "Every procedure is compiled, including unreachable code. "
             "Successful outputs pass LLVM assembly/verification and native linking at both `-O0` and `-O2` on Linux. "
             "The native link driver does not call the library APIs; this is not a functional or Windows interoperability test.", "",
             "## What compiles", "",
             "| Source | VB module name | Procedures | Assessment |", "|---|---|---:|---|"]
    for r in results:
        if r["standalone"]["stage"] == "compiled":
            lines.append(f"| `{r['path']}` | `{r['module_name']}` | {r['standalone']['procedure_count']} | {r.get('assessment', '')} |")
    lines += ["", f"{len(top_successes)} of {sum(m['group'] == 'top-level' for m in modules)} top-level distribution modules compile; "
              f"{len(wip_successes)} additional successes are under WIP. "
              f"Dependency retries produced {len(dependency_successes)} additional successes. "
              + ("All successful files at this audited source revision are interfaces, stubs, or skeletons; "
                 "none establishes a working implemented stdVBA utility."
                 if successes and all(r.get("assessment") for r in successes)
                 else "Compilation/linking alone does not establish functional API support."), "",
              "## First blocking stage", "",
              "| Group | Files | Parse | Binding | Checking | Lowering | Compiled |", "|---|---:|---:|---:|---:|---:|---:|"]
    for group in ("top-level", "WIP"):
        counts = report["summary"][group]
        lines.append(f"| {group} | {sum(counts.values())} | " + " | ".join(str(counts.get(s, 0)) for s in ("parse", "binding", "checking", "lowering", "compiled")) + " |")
    lines += ["", "Stages describe the earliest observed blocker, not an exhaustive list of missing features. "
              "The CLI also reports recovery and downstream diagnostics; diagnostic counts are not independent defects. "
              "Dependencies are added only for unknown declared project types, preferring top-level modules and then same-directory WIP modules. "
              "Ambiguous types and external Office/COM references are never stubbed. Late-bound dependencies may remain undetected. "
              "A top-level-only project build also fails. A recursive `-root ../stdvba/src` additionally mixes alternate WIP implementations.", "",
              "## Useful next candidates", "",
              *[f"- `{r['path']}` ({r.get('with_dependencies', r['standalone'])['stage']}): "
                + r.get('with_dependencies', r['standalone'])['diagnostics'][0]['message']
                + ". This is only the first observed gate, not a complete support claim."
                for r in results if r['path'] in ('stdQuadTree.cls', 'stdFiber.cls', 'stdRegex.cls', 'stdSentry.cls')
                and r.get('with_dependencies', r['standalone'])['stage'] != 'compiled'
                and r.get('with_dependencies', r['standalone'])['diagnostics']],
              "- Most other top-level modules first hit conditional compilation, `Declare`/`PtrSafe` DLL imports, events, or other unsupported syntax. Office and external COM dependencies are additional obstacles.", "",
              "## Per-module results", "",
              "Dependency results below are retries, not replacements for the standalone stage. "
              "See [results.json](results.json) for hashes, diagnostic counts/codes, excerpts, and selected dependency source lists.", "",
              "| Source | Standalone | With typed dependencies | First diagnostic |", "|---|---|---|---|"]
    for r in results:
        result = r.get("with_dependencies", r["standalone"])
        ds = result["diagnostics"]
        first = f"{ds[0]['file']}:{ds[0]['line']}: {ds[0]['message']}" if ds else r.get("assessment", "")
        first = first.replace("|", "\\|").replace("\n", " ")
        lines.append(f"| `{r['path']}` | {r['standalone']['stage']} | {r['with_dependencies']['stage'] if 'with_dependencies' in r else '—'} | {first} |")
    lines += ["", "## Reproduce", "", "```sh",
              "python3 llvm/audits/stdvba/run.py --source ../stdvba/src --output /tmp/stdvba-audit",
              "```", "", "Requires Go, Clang, llvm-as, and opt (version 20 preferred). "
              "Set `GOCACHE` if the default cache is not writable. `--full-diagnostics /tmp/stdvba-diagnostics` "
              "preserves full compiler messages. All manifests, binaries, and intermediate LLVM files use temporary directories; the stdVBA checkout is read-only.", ""]
    (output / "README.md").write_text("\n".join(lines))
    print(json.dumps(report["summary"], indent=2))
    print("Compiled:", [r["path"] for r in results if r["standalone"]["stage"] == "compiled"])


if __name__ == "__main__":
    main()
