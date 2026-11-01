package ir_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
	"sihlfall.ch/vambas-vai/stdlib"
)

func libraryFixture(t *testing.T) (*ir.Program, *ir.Instruction) {
	t.Helper()
	p, diagnostics := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Sub Run()\nDim value As Variant\nvalue=Left(\"abc\",1)\nEnd Sub\n"}}}))
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	for _, statement := range p.Procedures[0].Statements {
		for i := range statement.Instructions {
			in := &statement.Instructions[i]
			if in.Op == ir.OpLibraryCall {
				return p, in
			}
		}
	}
	t.Fatal("missing generic library call")
	return nil, nil
}

func TestLibraryCallCatalogContracts(t *testing.T) {
	for _, fn := range stdlib.Functions() {
		signature, ok := ir.LookupLibraryCall(fn.ID)
		if ok != (fn.Call == stdlib.SnapshotCall) {
			t.Fatalf("%s has inconsistent execution metadata", fn.Name)
		}
		if ok && (signature.Arity != len(fn.Parameters) || signature.Name != "VBA."+fn.Module+"."+fn.Name || signature.Effects&ir.MayRaise == 0) {
			t.Fatalf("%s lost its declaration contract", fn.Name)
		}
	}
	p, call := libraryFixture(t)
	if call.Library != stdlib.Left || !strings.Contains(ir.Format(p), `library_call member=4 "VBA.Strings.Left"`) {
		t.Fatal("formatted IR lost its catalog identity", ir.Format(p))
	}
}

func TestVerifierRejectsMalformedLibraryCalls(t *testing.T) {
	for _, test := range []struct {
		name, diagnostic string
		mutate           func(*ir.Program, *ir.Instruction)
	}{
		{"missing member", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.None }},
		{"unknown member", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = 65535 }},
		{"specialized member", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.Len }},
		{"class identity", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.Collection }},
		{"receiver-aware member", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.CollectionItem }},
		{"Err state", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.ErrNumber }},
		{"Err method", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.ErrRaise }},
		{"object factory", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.CreateObject }},
		{"constant member", "unknown or unsupported library member", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.NullString }},
		{"wrong dollar form", "invalid library result type", func(_ *ir.Program, in *ir.Instruction) { in.Library = stdlib.LeftString }},
		{"missing arguments", "invalid library argument count", func(_ *ir.Program, in *ir.Instruction) { in.Args = nil }},
		{"lost effects", "incorrect effects or result form", func(_ *ir.Program, in *ir.Instruction) { in.Effects = 0 }},
		{"legacy opcode", "unsupported operation", func(_ *ir.Program, in *ir.Instruction) { in.Op, in.Library = ir.OpLeft, stdlib.None }},
		{"unexpected payload", "unexpected library member payload", func(p *ir.Program, _ *ir.Instruction) {
			for _, s := range p.Procedures[0].Statements {
				for i := range s.Instructions {
					if s.Instructions[i].Op == ir.OpTry {
						s.Instructions[i].Library = stdlib.Left
						return
					}
				}
			}
		}},
		{"reference argument", "library arguments require Variant snapshots", func(p *ir.Program, call *ir.Instruction) {
			for _, s := range p.Procedures[0].Statements {
				for i := range s.Instructions {
					if &s.Instructions[i] == call {
						call.Args[0] = 10000
						address := ir.Instruction{ID: 10000, Op: ir.OpAddress, Type: ir.Type{Scalar: ir.Variant, Form: ir.Reference}, Slot: p.Procedures[0].Slots[0].ID, Source: call.Source}
						s.Instructions = append(s.Instructions[:i:i], append([]ir.Instruction{address}, s.Instructions[i:]...)...)
						return
					}
				}
			}
		}},
		{"missing try", "fallible operation needs Result and an immediate try continuation", func(p *ir.Program, call *ir.Instruction) {
			for _, s := range p.Procedures[0].Statements {
				for i := range s.Instructions {
					if &s.Instructions[i] == call {
						s.Instructions = append(s.Instructions[:i+1], s.Instructions[i+2:]...)
						return
					}
				}
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, call := libraryFixture(t)
			test.mutate(p, call)
			var messages strings.Builder
			for _, d := range ir.Verify(p) {
				messages.WriteString(d.Message + "\n")
			}
			if !strings.Contains(messages.String(), test.diagnostic) {
				t.Fatalf("missing %q diagnostic:\n%s", test.diagnostic, messages.String())
			}
		})
	}
}
