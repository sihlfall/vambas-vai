package ir

import (
	"fmt"
	"strings"
)

// Format renders a verified program for inspection. This is a debug format, not
// a serialization format, native ABI, or input to LLVM.
func Format(program *Program) string {
	var out strings.Builder
	fmt.Fprintf(&out, "program %s\n", program.Name)
	for _, c := range program.Classes {
		fmt.Fprintf(&out, "class @c%d %s members=%v\n", c.ID, c.Name, c.Members)
		for _, iface := range c.Interfaces {
			fmt.Fprintf(&out, "  implements @c%d %v\n", iface.Class, iface.Methods)
		}
		if c.Initialize != 0 || c.Terminate != 0 {
			fmt.Fprintf(&out, "  initialize=@%d terminate=@%d\n", c.Initialize, c.Terminate)
		}
		for i, f := range c.Fields {
			fmt.Fprintf(&out, "  .%d %s : %s bounds=%v\n", i, f.Name, f.Type, f.Bounds)
			fmt.Fprintf(&out, "    public=%t class=@c%d builtin=%d autoclass=@c%d autonew=%d\n", f.Public, f.Class, f.Builtin, f.AutoClass, f.AutoNew)
		}
	}
	for _, r := range program.Records {
		fmt.Fprintf(&out, "udt @%d %s\n", r.ID, r.Name)
		for i, f := range r.Fields {
			fmt.Fprintf(&out, "  .%d %s : %s bounds=%v\n", i, f.Name, f.Type, f.Bounds)
		}
	}
	for _, g := range program.Globals {
		fmt.Fprintf(&out, "global @g%d %s : %s bounds=%v\n", g.ID, g.Name, g.Type, g.Bounds)
	}
	for _, g := range program.Globals {
		if g.AutoClass != 0 {
			fmt.Fprintf(&out, "global @g%d autoclass=@c%d\n", g.ID, g.AutoClass)
		}
		if g.AutoNew != 0 {
			fmt.Fprintf(&out, "global @g%d autonew=%d\n", g.ID, g.AutoNew)
		}
	}
	for _, p := range program.Procedures {
		fmt.Fprintf(&out, "proc @%d %s -> %s effects=%d result=$%d\n", p.ID, p.Name, p.ReturnType, p.Effects, p.ResultSlot)
		if p.Class != 0 {
			fmt.Fprintf(&out, "  class=@c%d receiver=$%d\n", p.Class, p.Receiver)
		}
		if signature := p.Invocation; signature != nil {
			fmt.Fprintf(&out, "  invoke %q kind=%d default=%t\n", signature.Name, signature.Kind, signature.Default)
			for _, a := range signature.Parameters {
				fmt.Fprintf(&out, "    arg %q %s byref=%t optional=%t paramarray=%t class=@c%d builtin=%d default=@%d\n", a.Name, a.Type, a.ByRef, a.Optional, a.ParamArray, a.Class, a.Builtin, a.Default)
			}
		}
		for _, s := range p.Slots {
			mode := "zero"
			if s.Kind == Parameter {
				mode = "byval"
				if s.ByRef {
					mode = "byref"
				}
			}
			if s.Kind == GlobalAlias {
				mode = fmt.Sprintf("global @g%d", s.Global)
			}
			if s.Kind == LoopState {
				mode = "loop_state zero"
			}
			if s.ParamArray {
				mode += " paramarray"
			}
			if s.AutoNew != 0 {
				mode += fmt.Sprintf(" autonew=%d", s.AutoNew)
			}
			if s.AutoClass != 0 {
				mode += fmt.Sprintf(" autoclass=@c%d", s.AutoClass)
			}
			fmt.Fprintf(&out, "  $%d %s : %s %s\n", s.ID, s.Name, s.Type, mode)
		}
		for _, scope := range p.Scopes {
			fmt.Fprintf(&out, "  scope %d parent=%d cleanup=%v\n", scope.ID, scope.Parent, scope.Cleanup)
		}
		for _, s := range p.Statements {
			fmt.Fprintf(&out, "  S%d retry=S%d next=S%d %s:%d..%d\n", s.ID, s.Retry, s.Next, s.Source.FileName, s.Source.Range.Pos(), s.Source.Range.End())
			if s.ErrorMode != 0 {
				fmt.Fprintf(&out, "    on_error mode=%d handler=S%d\n", s.ErrorMode, s.Handler)
			}
			if s.Scope != 0 {
				fmt.Fprintf(&out, "    scope %d\n", s.Scope)
			}
			for _, in := range s.Instructions {
				out.WriteString("    ")
				if in.ID != 0 {
					fmt.Fprintf(&out, "%%%d = ", in.ID)
				}
				fmt.Fprint(&out, in.Op)
				if in.Op == OpLibraryCall {
					fmt.Fprintf(&out, " member=%d", in.Library)
					if signature, ok := LookupLibraryCall(in.Library); ok {
						fmt.Fprintf(&out, " %q", signature.Name)
					}
				}
				if in.Op == OpTestAssert || in.Op == OpCollectionInvoke {
					fmt.Fprintf(&out, " method=%d names=%q", in.Integer, in.Names)
				}
				if in.Op == OpObjectInvoke {
					fmt.Fprintf(&out, " member=%q kind=%d names=%q", in.Member, in.Invocation, in.Names)
				}
				if in.Op == OpVariantRef && in.Integer != 0 {
					fmt.Fprintf(&out, " builtin=%d", in.Integer)
				}
				if in.Class != 0 {
					fmt.Fprintf(&out, " @c%d", in.Class)
				}
				if in.Op == OpFieldRef || in.Op == OpClassFieldRef {
					fmt.Fprintf(&out, " .%d", in.Field)
				}
				if in.Op == OpConstant {
					if Floating(in.Type) {
						fmt.Fprintf(&out, " %g", in.Float)
					} else {
						fmt.Fprintf(&out, " %d", in.Integer)
					}
				}
				if in.Op == OpStringLiteral {
					fmt.Fprintf(&out, " %v", in.Units)
				}
				if in.Slot != 0 {
					fmt.Fprintf(&out, " $%d", in.Slot)
				}
				if in.Callee != 0 {
					fmt.Fprintf(&out, " @%d", in.Callee)
				}
				for _, arg := range in.Args {
					fmt.Fprintf(&out, " %%%d", arg)
				}
				fmt.Fprintf(&out, " : %s", in.Type)
				if in.OnError == Propagate {
					out.WriteString(" success=next error=propagate")
				}
				if in.Ownership == Owned {
					out.WriteString(" owned")
				}
				out.WriteByte('\n')
			}
			if len(s.Cleanup) != 0 {
				fmt.Fprintf(&out, "    cleanup %v (reverse acquisition order, also on failure)\n", s.Cleanup)
			}
			if s.End == Gosub {
				fmt.Fprintf(&out, "    gosub S%d return=S%d\n", s.Target, s.Next)
			} else if s.End == GosubReturn {
				fmt.Fprintf(&out, "    gosub_return %%%d\n", s.ReturnAddress)
			} else if s.End == Resume {
				fmt.Fprintf(&out, "    resume mode=%d target=S%d\n", s.ResumeMode, s.Target)
			} else if s.End == Return {
				if p.ResultSlot == 0 {
					out.WriteString("    return Ok(Void)\n")
				} else {
					fmt.Fprintf(&out, "    return Ok(load $%d)\n", p.ResultSlot)
				}
			} else if s.End == Jump {
				fmt.Fprintf(&out, "    jump S%d\n", s.Target)
			} else if s.End == Branch {
				fmt.Fprintf(&out, "    branch %%%d true=S%d false=S%d\n", s.Condition, s.Target, s.FalseTarget)
			} else if s.End == Switch {
				fmt.Fprintf(&out, "    switch %%%d default=S%d", s.Selector, s.Target)
				for _, item := range s.Cases {
					fmt.Fprintf(&out, " %d:S%d", item.Value, item.Target)
				}
				out.WriteByte('\n')
			} else {
				fmt.Fprintf(&out, "    continue S%d\n", s.Next)
			}
		}
	}
	return out.String()
}
