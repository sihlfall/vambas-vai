package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDoesNotNeedNativeToolsOrExecutableBodies(t *testing.T) {
	root := t.TempDir()
	source := []byte("'@TestModule\n'@TestMethod(\"Future\")\nPrivate Sub Example()\n Dim c As New FutureWidget\nEnd Sub\n")
	if e := os.WriteFile(filepath.Join(root, "Cases.bas"), source, 0600); e != nil {
		t.Fatal(e)
	}
	var out, errors bytes.Buffer
	code, e := run(context.Background(), []string{"-root", root, "-list", "-clang", "no-such-clang", "-category", "future", "-filter", "cases.*"}, &out, &errors)
	if e != nil || code != 0 || !json.Valid(out.Bytes()) || !strings.Contains(out.String(), "Cases.Example") {
		t.Fatal(code, e, out.String(), errors.String())
	}
	out.Reset()
	code, e = run(context.Background(), []string{"-root", root, "-list", "-filter", "["}, &out, &errors)
	if e == nil || code != 2 {
		t.Fatal(code, e)
	}
	code, e = run(context.Background(), []string{"-root", root, "-json", filepath.Join(root, "Cases.bas")}, &out, &errors)
	if e == nil || code != 2 {
		t.Fatal("source overwrite accepted", code, e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "Cases.bas"))
	if !bytes.Equal(source, after) {
		t.Fatal("source overwritten")
	}
}
func TestCLIReportsNativeFailureAndExitStatus(t *testing.T) {
	if _, e := exec.LookPath("clang-20"); e != nil {
		t.Skip("requires LLVM 20")
	}
	root := t.TempDir()
	source := "'@TestModule\nPrivate a As New Rubberduck.AssertClass\n'@TestMethod\nPrivate Sub Example()\n a.Fail \"expected failure\"\nEnd Sub\n"
	if e := os.WriteFile(filepath.Join(root, "Cases.bas"), []byte(source), 0600); e != nil {
		t.Fatal(e)
	}
	var out, errors bytes.Buffer
	junit := filepath.Join(root, "results.xml")
	code, e := run(context.Background(), []string{"-root", root, "-json", "-", "-junit", junit}, &out, &errors)
	if e != nil || code != 1 || !json.Valid(out.Bytes()) || !strings.Contains(out.String(), "expected failure") {
		t.Fatal(code, e, out.String(), errors.String())
	}
	xml, e := os.ReadFile(junit)
	if e != nil || !strings.Contains(string(xml), "<failure") {
		t.Fatal(string(xml), e)
	}
}
