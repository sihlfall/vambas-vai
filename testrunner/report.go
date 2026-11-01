// Package testrunner supervises native test workers. It is deliberately separate
// from compiler snapshots and the language server: discovery never runs VBA.
package testrunner

import (
	"encoding/xml"
	"fmt"
	"sihlfall.ch/vambas-vai/testdiscovery"
	"time"
)

type Issue struct {
	Kind    string                 `json:"kind"`
	Phase   string                 `json:"phase"`
	Method  string                 `json:"method,omitempty"`
	Message string                 `json:"message"`
	Number  int                    `json:"number,omitempty"`
	Source  testdiscovery.Location `json:"source"`
}
type Result struct {
	Test       testdiscovery.TestCase `json:"test"`
	Status     string                 `json:"status"`
	Assertions int                    `json:"assertions"`
	Duration   time.Duration          `json:"durationNs"`
	Issues     []Issue                `json:"issues,omitempty"`
}
type SuiteResult struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Tests    []Result      `json:"tests"`
	Issues   []Issue       `json:"issues,omitempty"`
	Duration time.Duration `json:"durationNs"`
}
type Report struct {
	Project            string        `json:"project"`
	Suites             []SuiteResult `json:"suites"`
	CompileDiagnostics []string      `json:"compileDiagnostics,omitempty"`
}

func (r Report) Failed() bool {
	if len(r.CompileDiagnostics) > 0 {
		return true
	}
	for _, s := range r.Suites {
		if len(s.Issues) > 0 {
			return true
		}
		for _, t := range s.Tests {
			if t.Status != "passed" && t.Status != "skipped" {
				return true
			}
		}
	}
	return false
}

// JUnit maps assertion failures to failure, runtime/process/compile errors to
// error, and ignored/inconclusive cases to skipped. JSON retains all distinctions.
func (r Report) JUnit() ([]byte, error) {
	type detail struct {
		Type    string `xml:"type,attr"`
		Message string `xml:"message,attr"`
		Text    string `xml:",chardata"`
	}
	type testcase struct {
		Name    string  `xml:"name,attr"`
		Class   string  `xml:"classname,attr"`
		Time    string  `xml:"time,attr"`
		Failure *detail `xml:"failure,omitempty"`
		Error   *detail `xml:"error,omitempty"`
		Skipped *detail `xml:"skipped,omitempty"`
	}
	type suite struct {
		Name     string     `xml:"name,attr"`
		Tests    int        `xml:"tests,attr"`
		Failures int        `xml:"failures,attr"`
		Errors   int        `xml:"errors,attr"`
		Skipped  int        `xml:"skipped,attr"`
		Cases    []testcase `xml:"testcase"`
	}
	doc := struct {
		XMLName xml.Name `xml:"testsuites"`
		Suites  []suite  `xml:"testsuite"`
	}{}
	for _, s := range r.Suites {
		js := suite{Name: s.Name}
		add := func(t testcase) {
			js.Tests++
			if t.Failure != nil {
				js.Failures++
			}
			if t.Error != nil {
				js.Errors++
			}
			if t.Skipped != nil {
				js.Skipped++
			}
			js.Cases = append(js.Cases, t)
		}
		for _, t := range s.Tests {
			tc := testcase{Name: t.Test.Name, Class: s.Name, Time: fmt.Sprintf("%.6f", t.Duration.Seconds())}
			d := &detail{Type: t.Status, Message: t.Status}
			for _, i := range t.Issues {
				d.Text += fmt.Sprintf("%s:%d:%d [%s] %s: %s\n", i.Source.File, i.Source.Line, i.Source.Column, i.Phase, i.Method, i.Message)
			}
			switch t.Status {
			case "passed":
			case "skipped", "inconclusive":
				tc.Skipped = d
			case "failed":
				tc.Failure = d
			default:
				tc.Error = d
			}
			add(tc)
		}
		for _, i := range s.Issues {
			add(testcase{Name: i.Phase, Class: s.Name, Error: &detail{Type: i.Kind, Message: i.Message}})
		}
		doc.Suites = append(doc.Suites, js)
	}
	if len(r.CompileDiagnostics) > 0 {
		js := suite{Name: r.Project, Tests: 1, Errors: 1}
		text := ""
		for _, d := range r.CompileDiagnostics {
			text += d + "\n"
		}
		js.Cases = []testcase{{Name: "compile", Error: &detail{Type: "compile_error", Message: "Compilation failed", Text: text}}}
		doc.Suites = append(doc.Suites, js)
	}
	data, e := xml.MarshalIndent(doc, "", "  ")
	return append([]byte(xml.Header), data...), e
}
