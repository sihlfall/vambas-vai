package testrunner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/testdiscovery"
	"testing"
	"time"
)

func TestWorkerCrashPreservesCompletedCases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process fixture")
	}
	binary := filepath.Join(t.TempDir(), "worker")
	script := `#!/bin/sh
printf '%s\n' '{"event":"test_start","index":0}' '{"event":"test_end","index":0,"status":"passed","assertions":0}' '{"event":"test_start","index":1}'
kill -TERM $$
`
	if e := os.WriteFile(binary, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	suite := testdiscovery.Suite{ID: "suite", Name: "Suite", Tests: []testdiscovery.TestCase{{ID: "one"}, {ID: "two"}}}
	r := runSuite(context.Background(), binary, 0, suite, suite.Tests, &ir.Program{}, time.Second)
	if len(r.Issues) != 1 || r.Issues[0].Kind != "crashed" || r.Tests[0].Status != "passed" || r.Tests[1].Status != "crashed" {
		t.Fatal(r)
	}
}
