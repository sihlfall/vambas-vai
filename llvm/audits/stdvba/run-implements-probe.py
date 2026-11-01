#!/usr/bin/env python3
"""Execute all four stdICallable contracts against its unchanged stdVBA source."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]


def tool(name):
    result = shutil.which(name + "-20") or shutil.which(name)
    if not result:
        raise SystemExit("Missing tool: " + name)
    return result


def run(command):
    return subprocess.run(command, cwd=REPO, capture_output=True, text=True,
                          check=True, timeout=60)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=REPO.parent / "stdvba" / "src")
    args = parser.parse_args()
    interface = (args.source / "stdICallable.cls").resolve()
    digest = hashlib.sha256(interface.read_bytes()).hexdigest()
    with tempfile.TemporaryDirectory(prefix="vba-implements-") as directory:
        work = Path(directory)
        manifest = work / "probe.vbp"
        manifest.write_text(
            'Type=Exe\nName="InterfaceProbe"\n'
            f'Module=ImplementsProbe; {HERE / "implements-probe.bas"}\n'
            f'Class=ImplementsProbeWorker; {HERE / "implements-probe-worker.cls"}\n'
            f'Class=stdICallable; {interface}\n')
        output = work / "probe.ll"
        run(["go", "run", "./cmd/vambas-compile", "-project", str(manifest),
             "-entry", "ImplementsProbe.Run", "-o", str(output)])
        bitcode = work / "probe.bc"
        run([tool("llvm-as"), str(output), "-o", str(bitcode)])
        run([tool("opt"), "-passes=verify", "-disable-output", str(bitcode)])
        sources = [str(REPO / "runtime" / line.strip())
                   for line in (REPO / "runtime/sources.txt").read_text().splitlines()
                   if line.strip() and not line.lstrip().startswith("#")]
        exits = {}
        for level in ("-O0", "-O2"):
            executable = work / ("probe" + level)
            run([tool("clang"), "-std=c11", level, str(output), *sources,
                 "-lm", "-o", str(executable)])
            result = subprocess.run([str(executable)], capture_output=True,
                                    text=True, timeout=30)
            if result.returncode != 42:
                raise SystemExit(f"{level}: expected 42, got {result.returncode}: {result.stderr}")
            exits[level] = result.returncode
    if hashlib.sha256(interface.read_bytes()).hexdigest() != digest:
        raise SystemExit("Interface source changed during probe")
    # Record the path relative to the checkouts' parent, not this machine.
    print(json.dumps({"interface": interface.relative_to(interface.parents[2]).as_posix(), "sha256": digest,
                      "source_revision": run(["git", "-C", str(interface.parent), "rev-parse", "HEAD"]).stdout.strip(),
                      "native_exits": exits}, indent=2))


if __name__ == "__main__":
    main()
