// Package compiler owns immutable VBA project snapshots. A Program collects
// source modules and binds them against a complete project declaration scope.
package compiler

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/parser"
)

type SourceFileInput struct {
	FileName   string
	Text       string
	ModuleName string         // Overrides VB_Name; otherwise the filename supplies a fallback.
	ModuleKind ast.ModuleKind // Unspecified infers .cls/.frm, otherwise standard.
}

type ProgramOptions struct {
	ProjectName string
	Sources     []SourceFileInput
}

type Program struct {
	name        string
	files       []*ast.SourceFile
	filesByName map[string]*ast.SourceFile
	symbol      *ast.Symbol
	checkerOnce sync.Once
	checker     *checker.Checker
}

// NewProgram parses fresh trees, declares every module, then binds bodies. Like
// typescript-go's Program it owns the file set and shared name scope. VBA has no
// imports: module kind and host identity are supplied by the caller. Rebuild after
// edits; neither syntax nor the program's symbol tables may be mutated by callers.
func NewProgram(options ProgramOptions) *Program {
	p := &Program{name: options.ProjectName, filesByName: make(map[string]*ast.SourceFile)}
	if p.name == "" {
		p.name = "Project"
	}
	for i, input := range options.Sources {
		file := parser.ParseSourceFile(input.Text).AsSourceFile()
		file.FileName = input.FileName
		if file.FileName == "" {
			file.FileName = fmt.Sprintf("Module%d.bas", i+1)
		}
		file.ModuleKind = input.ModuleKind
		if file.ModuleKind == ast.ModuleKindUnspecified {
			switch strings.ToLower(path.Ext(fileKey(file.FileName))) {
			case ".cls":
				file.ModuleKind = ast.ModuleKindClass
			case ".frm":
				file.ModuleKind = ast.ModuleKindUserForm
			default:
				file.ModuleKind = ast.ModuleKindStandard
			}
		}
		if input.ModuleName != "" {
			file.ModuleName = input.ModuleName
		}
		if file.ModuleName == "" {
			base := path.Base(strings.ReplaceAll(file.FileName, `\`, "/"))
			file.ModuleName = strings.TrimSuffix(base, path.Ext(base))
		}
		if file.ModuleKind == ast.ModuleKindDocument || file.ModuleKind == ast.ModuleKindUserForm {
			file.PredeclaredID = true // Host components have a default instance.
		}
		key := fileKey(file.FileName)
		if previous := p.filesByName[key]; previous != nil {
			file.BindDiagnostics = append(file.BindDiagnostics, ast.BindingDiagnostic{
				File: file, RelatedFile: previous, Node: file.AsNode(), RelatedNode: previous.AsNode(),
				Loc: file.Loc, RelatedLoc: previous.Loc, Code: ast.BindingDuplicateFile,
				Message: fmt.Sprintf("Duplicate source filename %q", file.FileName),
			})
		} else {
			p.filesByName[key] = file
		}
		p.files = append(p.files, file)
	}
	// Every input is parsed into a new, unpublished tree, satisfying BindProject's
	// ownership contract even when two input texts or filenames are identical.
	var err error
	p.symbol, err = binder.BindProject(p.name, p.files)
	if err != nil {
		panic(err) // Internal lifecycle violation, not a user-source diagnostic.
	}
	return p
}

func fileKey(name string) string {
	return strings.ToLower(path.Clean(strings.ReplaceAll(name, `\`, "/")))
}

func (p *Program) Name() string        { return p.name }
func (p *Program) Symbol() *ast.Symbol { return p.symbol }
func (p *Program) SourceFiles() []*ast.SourceFile {
	return append([]*ast.SourceFile(nil), p.files...)
}
func (p *Program) SourceFile(fileName string) *ast.SourceFile {
	return p.filesByName[fileKey(fileName)]
}
func (p *Program) Module(name string) *ast.SourceFile {
	if symbol := p.symbol.Exports.Lookup(name); symbol != nil {
		return ast.SourceFileOf(symbol.ValueDeclaration)
	}
	return nil
}
func (p *Program) ParseDiagnostics() []*ast.Diagnostic {
	var result []*ast.Diagnostic
	for _, file := range p.files {
		result = append(result, file.ParseDiagnostics...)
	}
	return result
}
func (p *Program) BindDiagnostics() []ast.BindingDiagnostic {
	var result []ast.BindingDiagnostic
	for _, file := range p.files {
		result = append(result, file.BindDiagnostics...)
	}
	return result
}

// GetTypeChecker lazily creates the semantic view of this immutable snapshot.
// The completed checker supports concurrent, read-only queries.
func (p *Program) GetTypeChecker() *checker.Checker {
	p.checkerOnce.Do(func() { p.checker = checker.NewChecker(p) })
	return p.checker
}

func (p *Program) GetSemanticDiagnostics(file *ast.SourceFile) []checker.Diagnostic {
	return p.GetTypeChecker().GetDiagnostics(file)
}
