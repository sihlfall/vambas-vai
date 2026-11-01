// vambas-test discovers comment annotations without execution, or compiles a native
// test worker and supervises one fresh process per selected test module.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sihlfall.ch/vambas-vai/project"
	"sihlfall.ch/vambas-vai/testdiscovery"
	"sihlfall.ch/vambas-vai/testrunner"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	code, err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
func run(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet("vambas-test", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "project directory")
	projectFile := flags.String("project", "", "VB project manifest (.vbp)")
	encoding := flags.String("source-encoding", "auto", "source encoding: auto, utf8, windows1252")
	list := flags.Bool("list", false, "list discovered tests as JSON without compiling or executing VBA")
	filter := flags.String("filter", "", "case-insensitive glob against module.procedure or full test ID")
	category := flags.String("category", "", "select annotation category (case-insensitive)")
	clang := flags.String("clang", "clang-20", "LLVM Clang executable")
	runtimeDir := flags.String("runtime", testrunner.DefaultRuntimeDir(), "directory containing native runtime sources")
	optimization := flags.String("opt", "O0", "LLVM optimization level: O0 or O2")
	timeout := flags.Duration("timeout", 30*time.Second, "maximum duration for each module worker")
	jsonOutput := flags.String("json", "", "write JSON report to file, or - for stdout")
	junitOutput := flags.String("junit", "", "write JUnit XML report to file")
	if e := flags.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0, nil
		}
		return 2, e
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("select inputs with -root or -project")
	}
	if *timeout <= 0 {
		return 2, fmt.Errorf("timeout must be positive")
	}
	if *root == "" && *projectFile == "" {
		*root = "."
	}
	config, e := project.LoadConfigWithOptions(ctx, project.LoadOptions{Root: *root, ProjectFile: *projectFile, SourceEncoding: *encoding})
	if e != nil {
		return 2, e
	}
	if len(config.Sources) == 0 {
		return 2, fmt.Errorf("project contains no VBA source files")
	}
	for _, output := range []string{*jsonOutput, *junitOutput} {
		if output == "" || output == "-" {
			continue
		}
		path, e := filepath.Abs(output)
		if e != nil {
			return 2, e
		}
		inputs := []string{config.ProjectFile}
		for _, s := range config.Sources {
			inputs = append(inputs, s.FileName)
		}
		for _, input := range inputs {
			if sameFile(path, input) {
				return 2, fmt.Errorf("report would overwrite input %s", input)
			}
		}
	}
	if *junitOutput == "-" {
		return 2, fmt.Errorf("use a file for -junit")
	}
	if *jsonOutput != "" && *junitOutput != "" && sameFile(*jsonOutput, *junitOutput) {
		return 2, fmt.Errorf("JSON and JUnit reports require separate paths")
	}
	snapshot := project.NewSession(config).Snapshot()
	p, e := snapshot.Program(ctx)
	if e != nil {
		return 2, e
	}
	manifest, ds := (testdiscovery.Rubberduck{}).Discover(p)
	if len(ds) > 0 {
		for _, d := range ds {
			fmt.Fprintln(stderr, testrunner.Diagnostic(d))
		}
		return 2, fmt.Errorf("test discovery failed with %d diagnostic(s)", len(ds))
	}
	options := testrunner.Options{Clang: *clang, RuntimeDir: *runtimeDir, Optimization: *optimization, Timeout: *timeout, Filter: *filter, Category: *category}
	if *list {
		for i, s := range manifest.Suites {
			tests := []testdiscovery.TestCase{}
			for _, t := range s.Tests {
				ok, e := testrunner.Match(t, options)
				if e != nil {
					return 2, e
				}
				if ok {
					tests = append(tests, t)
				}
			}
			manifest.Suites[i].Tests = tests
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return 0, encoder.Encode(manifest)
	}
	report, e := testrunner.Run(ctx, p, manifest, options)
	if e != nil {
		return 2, e
	}
	if *jsonOutput != "" {
		data, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			return 2, e
		}
		data = append(data, '\n')
		if *jsonOutput == "-" {
			_, e = stdout.Write(data)
		} else {
			e = os.WriteFile(*jsonOutput, data, 0644)
		}
		if e != nil {
			return 2, e
		}
	}
	if *junitOutput != "" {
		data, e := report.JUnit()
		if e != nil {
			return 2, e
		}
		if e = os.WriteFile(*junitOutput, data, 0644); e != nil {
			return 2, e
		}
	}
	human := stdout
	if *jsonOutput == "-" {
		human = stderr
	}
	total, passed, skipped := 0, 0, 0
	for _, s := range report.Suites {
		for _, t := range s.Tests {
			total++
			if t.Status == "passed" {
				passed++
			}
			if t.Status == "skipped" {
				skipped++
			}
			fmt.Fprintf(human, "%s %s (%d assertions)\n", t.Status, t.Test.Name, t.Assertions)
			for _, issue := range t.Issues {
				fmt.Fprintf(human, "  %s:%d:%d [%s] %s: %s\n", issue.Source.File, issue.Source.Line, issue.Source.Column, issue.Phase, issue.Method, issue.Message)
			}
		}
		for _, issue := range s.Issues {
			fmt.Fprintf(human, "%s [%s]: %s\n", s.Name, issue.Phase, issue.Message)
		}
	}
	for _, d := range report.CompileDiagnostics {
		fmt.Fprintln(stderr, d)
	}
	if len(report.CompileDiagnostics) > 0 {
		return 2, nil
	}
	fmt.Fprintf(human, "%d tests: %d passed, %d skipped, %d other outcomes\n", total, passed, skipped, total-passed-skipped)
	if report.Failed() {
		return 1, nil
	}
	return 0, nil
}
func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, e := filepath.Abs(a)
	if e != nil {
		return false
	}
	bb, e := filepath.Abs(b)
	if e != nil {
		return false
	}
	if aa == bb {
		return true
	}
	sa, e := os.Stat(aa)
	if e != nil {
		return false
	}
	sb, e := os.Stat(bb)
	return e == nil && os.SameFile(sa, sb)
}
