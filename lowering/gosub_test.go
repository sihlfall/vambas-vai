package lowering_test

import (
	"sihlfall.ch/vambas-vai/ir"
	"testing"
)

func TestGosubHasExplicitFallibleStackAndContinuation(t *testing.T) {
	p := lower(t, module("Sub Run()\n GoSub Work\n Exit Sub\nWork: Return\nEnd Sub\n"))
	f := p.Procedures[0]
	var call, ret *ir.Statement
	for _, s := range f.Statements {
		if s.End == ir.Gosub {
			call = s
		}
		if s.End == ir.GosubReturn {
			ret = s
		}
	}
	if call == nil || ret == nil || call.Next == call.Target || ret.ReturnAddress == 0 {
		t.Fatal(ir.Format(p))
	}
	if call.Instructions[len(call.Instructions)-2].Op != ir.OpGosubPush || ret.Instructions[len(ret.Instructions)-2].Op != ir.OpGosubPop {
		t.Fatal(ir.Format(p))
	}
	original := call.Next
	call.Next = 0
	if len(ir.Verify(p)) == 0 {
		t.Fatal("GoSub without resumption site verified")
	}
	call.Next = original
	ret.ReturnAddress = 0
	if len(ir.Verify(p)) == 0 {
		t.Fatal("Return without popped address verified")
	}
}
