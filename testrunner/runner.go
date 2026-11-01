package testrunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/internal/runtimebuild"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
	"sihlfall.ch/vambas-vai/lowering"
	"sihlfall.ch/vambas-vai/testdiscovery"
)

type Options struct {
	Clang        string
	RuntimeDir   string
	Optimization string        // O0 or O2.
	Timeout      time.Duration // Per module, including all fixtures and selected tests.
	Filter       string        // Case-insensitive glob against qualified test ID/name.
	Category     string
}

func DefaultRuntimeDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "runtime")
}
func Match(test testdiscovery.TestCase, options Options) (bool, error) {
	if options.Category != "" && !strings.EqualFold(options.Category, test.Category) {
		return false, nil
	}
	if options.Filter == "" {
		return true, nil
	}
	pattern := strings.ToLower(options.Filter)
	match, err := filepath.Match(pattern, strings.ToLower(test.ID))
	if err != nil {
		return false, err
	}
	if !match {
		match, err = filepath.Match(pattern, strings.ToLower(test.Name))
	}
	return match, err
}
func Diagnostic(d ast.Diagnostic) string {
	if d.File == nil {
		return d.Message
	}
	pos := max(0, min(d.Loc.Pos(), len(d.File.Text)))
	prefix := d.File.Text[:pos]
	line := strings.Count(prefix, "\n") + 1
	col := len(utf16.Encode([]rune(prefix[strings.LastIndex(prefix, "\n")+1:]))) + 1
	return fmt.Sprintf("%s:%d:%d: %s: %s", d.File.FileName, line, col, d.Code, d.Message)
}
func Run(ctx context.Context, p *compiler.Program, manifest testdiscovery.Manifest, options Options) (Report, error) {
	report := Report{Project: p.Name(), Suites: []SuiteResult{}}
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	if options.Clang == "" {
		options.Clang = "clang-20"
	}
	if options.RuntimeDir == "" {
		options.RuntimeDir = DefaultRuntimeDir()
	}
	if options.Optimization == "" {
		options.Optimization = "O0"
	}
	if options.Optimization != "O0" && options.Optimization != "O2" {
		return report, fmt.Errorf("optimization must be O0 or O2")
	}
	selected := make([][]testdiscovery.TestCase, len(manifest.Suites))
	total := 0
	for i, s := range manifest.Suites {
		for _, t := range s.Tests {
			ok, e := Match(t, options)
			if e != nil {
				return report, e
			}
			if ok {
				selected[i] = append(selected[i], t)
				total++
			}
		}
	}
	if total == 0 {
		return report, fmt.Errorf("no tests matched")
	}
	semantic, ds := lowering.Lower(p)
	if len(ds) > 0 {
		for _, d := range ds {
			report.CompileDiagnostics = append(report.CompileDiagnostics, Diagnostic(d))
		}
		return report, nil
	}
	identities := map[*ast.Node]ir.ProcedureID{}
	for _, proc := range semantic.Procedures {
		identities[proc.Source.Node] = proc.ID
	}
	invocation := func(in *testdiscovery.Invocation) ir.ProcedureID {
		if in == nil {
			return 0
		}
		id := identities[in.Declaration]
		if id == 0 {
			report.CompileDiagnostics = append(report.CompileDiagnostics, "test manifest references a procedure outside this program: "+in.Name)
		}
		return id
	}
	reg := llvm.TestRegistration{}
	for _, s := range manifest.Suites {
		rs := llvm.TestSuite{ID: s.ID, Initialize: invocation(s.ModuleInitialize), Cleanup: invocation(s.ModuleCleanup), Before: invocation(s.TestInitialize), After: invocation(s.TestCleanup)}
		for _, t := range s.Tests {
			rs.Tests = append(rs.Tests, llvm.TestEntry{ID: t.ID, Procedure: invocation(&t.Entry), Ignored: t.Ignored})
		}
		reg.Suites = append(reg.Suites, rs)
	}
	if len(report.CompileDiagnostics) > 0 {
		return report, nil
	}
	text, backendDS := llvm.EmitTests(semantic, reg)
	if len(backendDS) > 0 {
		for _, d := range backendDS {
			report.CompileDiagnostics = append(report.CompileDiagnostics, d.Message)
		}
		return report, nil
	}
	temp, e := os.MkdirTemp("", "vambas-test-*")
	if e != nil {
		return report, e
	}
	defer os.RemoveAll(temp)
	input, binary := filepath.Join(temp, "tests.ll"), filepath.Join(temp, "tests")
	if e = os.WriteFile(input, []byte(text), 0600); e != nil {
		return report, e
	}
	runtimeSources, e := runtimebuild.Sources(options.RuntimeDir)
	if e != nil {
		report.CompileDiagnostics = append(report.CompileDiagnostics, e.Error())
		return report, nil
	}
	args := append([]string{"-" + options.Optimization, input}, runtimeSources...)
	args = append(args, "-lm", "-o", binary)
	command := exec.CommandContext(ctx, options.Clang, args...)
	output, e := command.CombinedOutput()
	if e != nil {
		report.CompileDiagnostics = append(report.CompileDiagnostics, fmt.Sprintf("native link: %v\n%s", e, output))
		return report, nil
	}
	for i, s := range manifest.Suites {
		if len(selected[i]) == 0 {
			continue
		}
		if ctx.Err() != nil {
			result := SuiteResult{ID: s.ID, Name: s.Name, Tests: []Result{}}
			issue := Issue{Kind: "cancelled", Phase: "worker", Message: ctx.Err().Error(), Source: s.Source}
			result.Issues = append(result.Issues, issue)
			for _, test := range selected[i] {
				result.Tests = append(result.Tests, Result{Test: test, Status: "cancelled", Issues: []Issue{issue}})
			}
			report.Suites = append(report.Suites, result)
			continue
		}
		result := runSuite(ctx, binary, i, s, selected[i], semantic, options.Timeout)
		report.Suites = append(report.Suites, result)
	}
	return report, nil
}

type event struct {
	Event        string        `json:"event"`
	Index        int           `json:"index"`
	Status       string        `json:"status"`
	Assertions   int           `json:"assertions"`
	Duration     time.Duration `json:"durationNs"`
	Phase        string        `json:"phase"`
	Method       string        `json:"method"`
	Message      string        `json:"message"`
	Inconclusive bool          `json:"inconclusive"`
	Procedure    int           `json:"procedure"`
	Statement    int           `json:"statement"`
	Offset       int           `json:"offset"`
	Number       int           `json:"number"`
}

func runSuite(ctx context.Context, binary string, index int, s testdiscovery.Suite, selected []testdiscovery.TestCase, p *ir.Program, timeout time.Duration) SuiteResult {
	result := SuiteResult{ID: s.ID, Name: s.Name, Tests: []Result{}}
	started := time.Now()

	args := []string{strconv.Itoa(index)}
	for _, t := range selected {
		args = append(args, t.ID)
	}
	workerCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(workerCtx, binary, args...)
	var stderr cappedBuffer
	command.Stderr = &stderr
	pipe, startError := command.StdoutPipe()
	if startError == nil {
		startError = command.Start()
	}

	current := -1
	complete := false
	protocolError := ""

	selectedByID := map[string]int{}
	for i, t := range selected {
		selectedByID[t.ID] = i
		result.Tests = append(result.Tests, Result{Test: t, Status: "pending"})
	}
	if startError != nil {
		result.Issues = append(result.Issues, Issue{Kind: "crashed", Phase: "worker", Message: startError.Error(), Source: s.Source})
		for i := range result.Tests {
			result.Tests[i].Status = "crashed"
		}
		return result
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	for scanner.Scan() {
		var ev event
		if e := json.Unmarshal(scanner.Bytes(), &ev); e != nil {
			protocolError = "invalid worker event: " + e.Error()
			break
		}
		switch ev.Event {
		case "test_start":
			if ev.Index < 0 || ev.Index >= len(s.Tests) {
				protocolError = "invalid worker test index"
				break
			}
			var ok bool
			current, ok = selectedByID[s.Tests[ev.Index].ID]
			if !ok {
				protocolError = "unselected worker test"
				current = -1
			}

		case "assertion", "error":
			kind := "assertion"
			if ev.Event == "error" {
				kind = "runtime_error"
			} else if ev.Inconclusive {
				kind = "inconclusive"
			}
			issue := Issue{Kind: kind, Phase: ev.Phase, Method: ev.Method, Message: ev.Message, Number: ev.Number, Source: sourceLocation(p, ev.Procedure, ev.Offset)}
			if current >= 0 {
				result.Tests[current].Issues = append(result.Tests[current].Issues, issue)
			} else {
				result.Issues = append(result.Issues, issue)
			}
		case "test_end":
			if current < 0 || ev.Index < 0 || ev.Index >= len(s.Tests) || s.Tests[ev.Index].ID != result.Tests[current].Test.ID {
				protocolError = "mismatched worker test completion"
				break
			}
			result.Tests[current].Status = ev.Status
			result.Tests[current].Assertions = ev.Assertions
			result.Tests[current].Duration = ev.Duration
			current = -1
		case "suite_end":
			complete = true
		default:
			protocolError = "unknown worker event: " + ev.Event
		}
		if protocolError != "" {
			break
		}
	}
	if e := scanner.Err(); e != nil {
		protocolError = e.Error()
	}
	if protocolError != "" {
		_ = command.Process.Kill()
	}
	err := command.Wait()
	unexpectedExit := false
	if err != nil {
		var exit *exec.ExitError
		unexpectedExit = !errors.As(err, &exit) || exit.ExitCode() != 1 || !resultFailed(result)
	}
	for _, test := range result.Tests {
		if test.Status == "pending" {
			complete = false
		}
	}
	if !complete || protocolError != "" || unexpectedExit {
		status, message := "crashed", "worker exited without completing the suite"
		if workerCtx.Err() != nil {
			status = "timed_out"
			message = "module exceeded " + timeout.String()
			if ctx.Err() != nil {
				status = "cancelled"
				message = ctx.Err().Error()
			}
		}
		if protocolError != "" {
			message = protocolError
		}
		if err != nil {
			message += ": " + err.Error()
		}
		if stderr.Len() > 0 {
			message += "\n" + stderr.String()
		}
		result.Issues = append(result.Issues, Issue{Kind: status, Phase: "worker", Message: message, Source: s.Source})
		for i := range result.Tests {
			if result.Tests[i].Status == "pending" {
				result.Tests[i].Status = status
				result.Tests[i].Issues = append(result.Tests[i].Issues, Issue{Kind: status, Phase: "worker", Message: message, Source: result.Tests[i].Test.Source})
			}
		}
	}
	result.Duration = time.Since(started)
	return result
}
func resultFailed(s SuiteResult) bool {
	if len(s.Issues) > 0 {
		return true
	}
	for _, t := range s.Tests {
		if t.Status != "passed" && t.Status != "skipped" && t.Status != "pending" {
			return true
		}
	}
	return false
}
func sourceLocation(p *ir.Program, procedure, offset int) testdiscovery.Location {
	if procedure < 1 || procedure > len(p.Procedures) {
		return testdiscovery.Location{}
	}
	src := p.Procedures[procedure-1].Source
	file := ast.SourceFileOf(src.Node)
	pos := max(0, min(offset, len(file.Text)))
	prefix := file.Text[:pos]
	return testdiscovery.Location{File: file.FileName, Start: pos, End: pos, Line: strings.Count(prefix, "\n") + 1, Column: len(utf16.Encode([]rune(prefix[strings.LastIndex(prefix, "\n")+1:]))) + 1}
}

// Bound diagnostic capture even if native code writes without a newline.
type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (64 << 10) - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}
