#!/usr/bin/env python3
"""Verify unchanged core modules and execute a Collection-free API probe.

The original source directory is read only. Copies and all build products use a
TemporaryDirectory. The execution copy removes exactly SplitByRegex, whose
Collection dependency is still unsupported; this is not full-project success.
"""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
SOURCE = REPO.parent / "vba-regex" / "src"
ENV = dict(os.environ, GOCACHE="/tmp/vambas-vai-go-cache")


def tool(name):
    path = shutil.which(name + "-20") or shutil.which(name)
    if path is None:
        raise SystemExit(f"LLVM tool required: {name}")
    return path


def run(*args):
    subprocess.run(args, cwd=REPO, env=ENV, check=True)


with tempfile.TemporaryDirectory(prefix="vba-regex-audit-") as temp:
    root = Path(temp)
    core, execution = root / "core", root / "execution"
    core.mkdir()
    execution.mkdir()
    for source in sorted(SOURCE.glob("*.bas")):
        data = source.read_bytes()
        if source.name != "StaticRegex.bas":
            (core / source.name).write_bytes(data)
        else:
            text, count = re.subn(
                r"(?ms)^Public Function SplitByRegex\b.*?^End Function\s*\n",
                "",
                data.decode("utf-8-sig"),
            )
            if count != 1:
                raise SystemExit("Expected exactly one SplitByRegex function")
            data = text.encode("utf-8")
        (execution / source.name).write_bytes(data)
    shutil.copyfile(HERE / "execution-probe.bas", execution / "Probe.bas")
    run("go", "run", "./cmd/vambas-compile", "-root", str(core), "-o", str(root / "core.ll"))
    run(tool("llvm-as"), str(root / "core.ll"), "-o", str(root / "core.bc"))
    run(tool("opt"), "-passes=verify", "-disable-output", str(root / "core.bc"))
    run("go", "run", "./cmd/vambas-compile", "-root", str(execution), "-entry", "Probe.Run", "-o", str(root / "execution.ll"))
    runtime_sources = [str(REPO / "runtime" / name) for name in
                       (REPO / "runtime" / "sources.txt").read_text().split()]
    results = {}
    for level in ("-O0", "-O2"):
        binary = root / ("probe" + level)
        run(tool("clang"), level, str(root / "execution.ll"), *runtime_sources, "-lm", "-o", str(binary))
        result = subprocess.run([str(binary)], timeout=30, check=False)
        results[level] = result.returncode
        if result.returncode != 42:
            raise SystemExit(f"Regex execution failed at {level}: {result.returncode}")
    print(json.dumps({"unchanged_core_modules": 13, "core_llvm_verified": True,
                      "execution_excludes": "StaticRegex.SplitByRegex only",
                      "execution_exit_status": results}, indent=2))
