// vambas-compile loads the same project snapshot as the language server and emits
// verified scalar LLVM IR. Native linking remains a separate LLVM tool invocation.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/llvm"
	"sihlfall.ch/vambas-vai/lowering"
	"sihlfall.ch/vambas-vai/project"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("vambas-compile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "project directory (defaults to current directory when -project is omitted)")
	manifest := flags.String("project", "", "VB project manifest (.vbp)")
	encoding := flags.String("source-encoding", "auto", "source encoding: auto, utf8, windows1252")
	entry := flags.String("entry", "", "qualified parameterless procedure for native main, e.g. Main.Main (omit for a private module)")
	output := flags.String("o", "-", "LLVM output filename, or - for stdout")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments; select inputs with -root or -project")
	}
	if *root == "" && *manifest == "" {
		*root = "."
	}
	config, err := project.LoadConfigWithOptions(ctx, project.LoadOptions{Root: *root, ProjectFile: *manifest, SourceEncoding: *encoding})
	if err != nil {
		return err
	}
	if len(config.Sources) == 0 {
		return fmt.Errorf("project contains no VBA source files")
	}
	// Reuse import decoding and designer/BOM masking from the editor snapshot.
	snapshot := project.NewSession(config).Snapshot()
	p, err := snapshot.Program(ctx)
	if err != nil {
		return err
	}
	semantic, diagnostics := lowering.Lower(p)
	if len(diagnostics) != 0 {
		for _, d := range diagnostics {
			diagnostic(stderr, d.File, d.Loc.Pos(), string(d.Code), d.Message)
		}
		return fmt.Errorf("compilation failed with %d diagnostic(s)", len(diagnostics))
	}
	options := llvm.Options{}
	if *entry != "" {
		for _, procedure := range semantic.Procedures {
			if strings.EqualFold(procedure.Name, *entry) {
				options.Entry = procedure.ID
				break
			}
		}
		if options.Entry == 0 {
			return fmt.Errorf("entry procedure %q not found; use a qualified module.procedure name", *entry)
		}
	}
	text, backendDiagnostics := llvm.Emit(semantic, options)
	if len(backendDiagnostics) != 0 {
		for _, d := range backendDiagnostics {
			diagnostic(stderr, ast.SourceFileOf(d.Source.Node), d.Source.Range.Pos(), "llvm", d.Message)
		}
		return fmt.Errorf("LLVM emission failed with %d diagnostic(s)", len(backendDiagnostics))
	}
	if *output == "-" {
		_, err := io.WriteString(stdout, text)
		return err
	}
	path, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	// Output selection must never accidentally truncate a loaded source/manifest.
	for _, input := range config.Sources {
		if sameFile(path, input.FileName) {
			return fmt.Errorf("output would overwrite source %s", path)
		}
	}
	if sameFile(path, config.ProjectFile) {
		return fmt.Errorf("output would overwrite project manifest %s", path)
	}
	return os.WriteFile(path, []byte(text), 0644)
}

func sameFile(output, input string) bool {
	if output == input {
		return true
	}
	a, err := os.Stat(output)
	if err != nil {
		return false
	}
	b, err := os.Stat(input)
	return err == nil && os.SameFile(a, b)
}

func diagnostic(out io.Writer, file *ast.SourceFile, pos int, code, message string) {
	if file == nil {
		fmt.Fprintf(out, "%s: %s\n", code, message)
		return
	}
	pos = max(0, min(pos, len(file.Text)))
	prefix := file.Text[:pos]
	line := strings.Count(prefix, "\n") + 1
	column := pos - strings.LastIndex(prefix, "\n")
	fmt.Fprintf(out, "%s:%d:%d: %s: %s\n", file.FileName, line, column, code, message)
}
