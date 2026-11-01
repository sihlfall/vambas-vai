package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCompileManifestAndLegacySource(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Demo.vbp"), "Name=\"Demo\"\nModule=Entry; Main.bas\n")
	write(t, filepath.Join(dir, "Main.bas"), "' Legacy comment: caf\xe9\r\nPublic Function Run() As Long\r\n Run = 42\r\nEnd Function\r\n")
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"-project", filepath.Join(dir, "Demo.vbp"), "-entry", "entry.run"}, &stdout, &stderr); err != nil {
		t.Fatal(err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "define i32 @main()") || !strings.Contains(stdout.String(), "Entry.Run") || stderr.Len() != 0 {
		t.Fatal(stdout.String(), stderr.String())
	}
	output := filepath.Join(dir, "output.ll")
	stdout.Reset()
	if err := run(context.Background(), []string{"-root", dir, "-o", output}, &stdout, &stderr); err != nil {
		t.Fatal(err, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(data), "define internal i32 @vba.p1") || strings.Contains(string(data), "define i32 @main()") || stdout.Len() != 0 {
		t.Fatal(err, string(data))
	}
}

func TestCompileClassExample(t *testing.T) {
	for _, root := range []string{"../../examples/classes", "../../examples/object-contract"} {
		t.Run(root, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run(context.Background(), []string{"-root", root, "-entry", "Main.Main"}, &stdout, &stderr); err != nil {
				t.Fatal(err, stderr.String())
			}
			for _, fragment := range []string{"@vba.class.type1", "@vba_class_new", "@vba_class_check", "define i32 @main()"} {
				if !strings.Contains(stdout.String(), fragment) {
					t.Fatalf("missing %s", fragment)
				}
			}
			if stderr.Len() != 0 {
				t.Fatal(stderr.String())
			}
		})
	}
}

func TestCompilationFailurePreservesOutput(t *testing.T) {
	for _, test := range []struct{ name, text string }{
		{"parse", "Sub Run()\n x =\nEnd Sub\n"},
		{"bind", "Option Explicit\nSub Run()\n missing = 1\nEnd Sub\n"},
		{"unsupported", "Sub Run()\n Dim value As LongLong\nEnd Sub\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, "Main.bas"), test.text)
			output := filepath.Join(dir, "output.ll")
			write(t, output, "previous output")
			var stdout, stderr bytes.Buffer
			if err := run(context.Background(), []string{"-root", dir, "-o", output}, &stdout, &stderr); err == nil {
				t.Fatal("compilation succeeded")
			}
			data, err := os.ReadFile(output)
			if err != nil || string(data) != "previous output" || stdout.Len() != 0 || !strings.Contains(stderr.String(), "Main.bas:") {
				t.Fatal(err, string(data), stdout.String(), stderr.String())
			}
		})
	}
}

func TestEntryAndOutputErrors(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "Main.bas")
	source := "Sub Run(ByVal value As Long)\nEnd Sub\n"
	write(t, input, source)
	for _, args := range [][]string{{"-entry", "Main.Missing"}, {"-entry", "Main.Run"}, {"-o", input}, {"unexpected.bas"}} {
		var stdout, stderr bytes.Buffer
		if err := run(context.Background(), append([]string{"-root", dir}, args...), &stdout, &stderr); err == nil || stdout.Len() != 0 {
			t.Fatal("invalid command emitted code", args)
		}
	}
	data, err := os.ReadFile(input)
	if err != nil || string(data) != source {
		t.Fatal("source was overwritten", err)
	}
}

func TestSSAOutputWithoutLLVM(t *testing.T) {
	// An empty PATH proves that the compiler itself constructs SSA, including
	// loop phis, rather than invoking opt or another external tool.
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Main.bas"), `Function Run(ByVal limit As Long) As Long
 Dim n As Long
 While n < limit
  n = n + 1
 Wend
 Run = n
End Function`)
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"-root", dir}, &stdout, &stderr); err != nil {
		t.Fatal(err, stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "phi i32") || strings.Contains(text, "alloca") || strings.Contains(text, "vba.variant") || !strings.Contains(text, "; Main.Run") {
		t.Fatalf("expected direct scalar SSA with source comments:\n%s", text)
	}
	output := filepath.Join(dir, "ssa.ll")
	stdout.Reset()
	if err := run(context.Background(), []string{"-root", dir, "-o", output}, &stdout, &stderr); err != nil {
		t.Fatal(err, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != text || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("file and stdout SSA differ", err, stdout.String(), stderr.String())
	}
	// Frontend failure must preserve the existing SSA file.
	write(t, filepath.Join(dir, "Main.bas"), "Sub Run()\n missing =\nEnd Sub")
	if err := run(context.Background(), []string{"-root", dir, "-o", output}, &stdout, &stderr); err == nil {
		t.Fatal("invalid program compiled")
	}
	data, err = os.ReadFile(output)
	if err != nil || string(data) != text || stdout.Len() != 0 {
		t.Fatal("failed compilation changed output", err)
	}
}
