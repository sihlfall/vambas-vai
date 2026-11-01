#!/usr/bin/env python3
"""Run the complete original regex test project unchanged at O0 and O2.

No source or manifest is copied, rewritten, or filtered. Requires the sibling
vba-regex checkout and LLVM/Clang 20.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile

REPO = Path(__file__).resolve().parents[3]
REGEX = REPO.parent / "vba-regex"
ENV = dict(os.environ, GOCACHE="/tmp/vambas-vai-go-cache")
with tempfile.TemporaryDirectory(prefix="vba-regex-tests-") as temporary:
    directory = Path(temporary)
    manifest = REGEX / "test" / "TestVbaRegex.vbp"
    if not manifest.is_file():
        raise SystemExit(f"Original test project not found: {manifest}")
    for level in ("O0", "O2"):
        report = directory / (level + ".json")
        subprocess.run([
            "go", "run", "./cmd/vambas-test", "-project", str(manifest),
            "-opt", level, "-json", str(report),
        ], cwd=REPO, env=ENV, check=True)
        result = json.loads(report.read_text(encoding="utf-8"))
        tests = [test for suite in result["suites"] for test in suite["tests"]]
        if len(tests) != 316 or any(test["status"] != "passed" for test in tests):
            raise SystemExit("Unexpected full-project result; review test changes")
        print(f"{level}: all 316 original tests passed")
