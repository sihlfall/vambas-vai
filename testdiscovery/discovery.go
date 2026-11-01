// Package testdiscovery provides an editor-independent test manifest. The
// Rubberduck adapter interprets comment metadata produced by the VBA frontend.
package testdiscovery

import (
	"strings"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
)

type Host interface {
	Name() string
	SourceFiles() []*ast.SourceFile
	GetTypeChecker() *checker.Checker
}
type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}
type Invocation struct {
	Name        string      `json:"name"`
	Source      Location    `json:"source"`
	Declaration *ast.Node   `json:"-"`
	Symbol      *ast.Symbol `json:"-"`
}
type TestCase struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Category string     `json:"category"`
	Source   Location   `json:"source"`
	Entry    Invocation `json:"-"`
	Ignored  bool       `json:"ignored,omitempty"`
}
type Suite struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Source           Location    `json:"source"`
	Tests            []TestCase  `json:"tests"`
	ModuleInitialize *Invocation `json:"moduleInitialize,omitempty"`
	ModuleCleanup    *Invocation `json:"moduleCleanup,omitempty"`
	TestInitialize   *Invocation `json:"testInitialize,omitempty"`
	TestCleanup      *Invocation `json:"testCleanup,omitempty"`
}
type Manifest struct {
	Framework string  `json:"framework"`
	Project   string  `json:"project"`
	Suites    []Suite `json:"suites"`
}
type Adapter interface {
	Discover(Host) (Manifest, []ast.Diagnostic)
}
type Rubberduck struct{}

func location(file *ast.SourceFile, n *ast.Node) Location {
	pos := n.Pos()
	prefix := file.Text[:pos]
	lineStart := strings.LastIndex(prefix, "\n") + 1
	return Location{file.FileName, strings.Count(prefix, "\n") + 1, len(utf16.Encode([]rune(prefix[lineStart:]))) + 1, pos, n.End()}
}
func (Rubberduck) Discover(host Host) (Manifest, []ast.Diagnostic) {
	result := Manifest{Framework: "rubberduck", Project: host.Name(), Suites: []Suite{}}
	var ds []ast.Diagnostic
	checker := host.GetTypeChecker()
	for _, file := range host.SourceFiles() {
		module := false
		for _, a := range file.Annotations {
			if strings.EqualFold(a.Name, "TestModule") && a.Target == file.AsNode() {
				module = true
			}
		}
		if !module {
			continue
		}
		report := func(a ast.Annotation, message string) {
			ds = append(ds, ast.Diagnostic{File: file, Loc: a.Range, Code: "test-discovery", Message: message})
		}
		suite := Suite{ID: strings.ToLower(host.Name() + "." + file.ModuleName), Name: file.ModuleName, Source: location(file, file.AsNode()), Tests: []TestCase{}}
		if file.ModuleKind != ast.ModuleKindStandard {
			ds = append(ds, ast.Diagnostic{File: file, Loc: file.Loc, Code: "test-discovery", Message: "TestModule requires a standard module"})
			continue
		}
		moduleAnnotations := 0
		seen := map[*ast.Node]string{}
		ignored := map[*ast.Node]bool{}
		for _, a := range file.Annotations {
			if strings.EqualFold(a.Name, "IgnoreTest") && a.Target != nil {
				if a.Error != "" || len(a.Arguments) > 0 {
					report(a, "IgnoreTest does not take arguments")
				}
				ignored[a.Target] = true
			}
		}
		// Old test bodies can be commented out below an annotation. Preserve all
		// metadata in the AST; the closest repeated role on a declaration wins.
		lastRole := map[*ast.Node]map[string]int{}
		for i, a := range file.Annotations {
			if a.Target != nil {
				if lastRole[a.Target] == nil {
					lastRole[a.Target] = map[string]int{}
				}
				lastRole[a.Target][strings.ToLower(a.Name)] = i
			}
		}
		for index, a := range file.Annotations {
			name := strings.ToLower(a.Name)
			if a.Target != nil && name != "testmodule" && lastRole[a.Target][name] != index {
				continue
			}
			if name == "testmodule" {
				moduleAnnotations++
				if moduleAnnotations > 1 {
					report(a, "Duplicate TestModule annotation")
				}
				if a.Target != file.AsNode() {
					report(a, "TestModule requires a standalone module comment")
				}
				if a.Error != "" || len(a.Arguments) > 0 {
					report(a, "TestModule does not take arguments")
				}
				continue
			}
			if name != "testmethod" && name != "moduleinitialize" && name != "modulecleanup" && name != "testinitialize" && name != "testcleanup" {
				continue
			}
			if a.Error != "" {
				report(a, a.Error)
				continue
			}
			if name != "testmethod" && len(a.Arguments) > 0 || len(a.Arguments) > 1 {
				report(a, "Invalid test annotation arguments")
				continue
			}
			node := a.Target
			if node == nil {
				report(a, "Test annotation must immediately precede a procedure")
				continue
			}
			sig := checker.GetSignatureFromDeclaration(node)
			if node.Kind != ast.KindSubDeclaration || sig == nil || len(sig.Parameters) != 0 {
				report(a, "Tests and lifecycle hooks must be parameterless Sub procedures")
				continue
			}
			if seen[node] != "" {
				report(a, "A procedure cannot have multiple test/lifecycle roles")
				continue
			}
			seen[node] = name
			invocation := Invocation{Name: file.ModuleName + "." + node.AsFunctionDeclaration().Name().AsIdentifier().Text, Declaration: node, Symbol: node.Symbol, Source: location(file, node)}
			if name == "testmethod" {
				category := "Uncategorized"
				if len(a.Arguments) == 1 {
					category = a.Arguments[0]
				}
				suite.Tests = append(suite.Tests, TestCase{ID: strings.ToLower(host.Name() + "." + invocation.Name), Name: invocation.Name, Category: category, Source: location(file, node), Entry: invocation, Ignored: ignored[node]})
				continue
			}
			var slot **Invocation
			switch name {
			case "moduleinitialize":
				slot = &suite.ModuleInitialize
			case "modulecleanup":
				slot = &suite.ModuleCleanup
			case "testinitialize":
				slot = &suite.TestInitialize
			case "testcleanup":
				slot = &suite.TestCleanup
			}
			if *slot != nil {
				report(a, "Duplicate lifecycle hook")
			} else {
				*slot = &invocation
			}
		}
		result.Suites = append(result.Suites, suite)
	}
	return result, ds
}
