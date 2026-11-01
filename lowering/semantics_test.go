package lowering_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ir"
)

// Small independent test oracle for the scalar semantic IR, not a native backend
// or VBA runtime. Observable aliasing, overflow, and commit behavior are tested
// here without prescribing LLVM layouts or instruction selection.
type argument struct {
	value  int64
	cell   *int64
	failed bool
}

func execute(t *testing.T, program *ir.Program, id ir.ProcedureID, arguments []argument) (int64, bool) {
	t.Helper()
	budget := 10000
	var run func(ir.ProcedureID, []argument) (int64, bool)
	run = func(id ir.ProcedureID, arguments []argument) (int64, bool) {
		p := program.Procedures[id-1]
		cells := make([]*int64, len(p.Slots)+1)
		for i := range p.Slots {
			cells[i+1] = new(int64)
		}
		for i, slot := range p.Parameters {
			if p.Slots[slot-1].ByRef {
				cells[slot] = arguments[i].cell
			} else {
				*cells[slot] = arguments[i].value
			}
		}
		for current := ir.StatementID(1); current != 0; {
			statement := p.Statements[current-1]
			budget--
			if budget <= 0 {
				t.Fatal("test execution did not terminate")
			}
			values := make(map[ir.ValueID]argument)
			for _, in := range statement.Instructions {
				budget--
				if budget <= 0 {
					t.Fatal("test execution did not terminate")
				}
				args := make([]argument, len(in.Args))
				for i, id := range in.Args {
					args[i] = values[id]
				}
				var result argument
				switch in.Op {
				case ir.OpConstant:
					result.value = in.Integer
				case ir.OpLoad:
					result.value = *cells[in.Slot]
				case ir.OpStore:
					*cells[in.Slot] = args[0].value
				case ir.OpAddress:
					result.cell = cells[in.Slot]
				case ir.OpConvert:
					result.value = args[0].value
				case ir.OpAdd:
					result.value = args[0].value + args[1].value
				case ir.OpSubtract:
					result.value = args[0].value - args[1].value
				case ir.OpMultiply:
					result.value = args[0].value * args[1].value
				case ir.OpNegate:
					result.value = -args[0].value
				case ir.OpNot:
					result.value = ^args[0].value
				case ir.OpCondition:
					if args[0].value != 0 {
						result.value = -1
					}
				case ir.OpCheckInitialized:
					result.failed = args[0].value == 0
				case ir.OpCheckDispatch:
					result.failed = args[0].value < 0 || args[0].value > 255
				case ir.OpAnd:
					result.value = args[0].value & args[1].value
				case ir.OpOr:
					result.value = args[0].value | args[1].value
				case ir.OpXor:
					result.value = args[0].value ^ args[1].value
				case ir.OpEqual, ir.OpNotEqual, ir.OpLess, ir.OpLessEqual, ir.OpGreater, ir.OpGreaterEqual:
					a, b := args[0].value, args[1].value
					truth := false
					switch in.Op {
					case ir.OpEqual:
						truth = a == b
					case ir.OpNotEqual:
						truth = a != b
					case ir.OpLess:
						truth = a < b
					case ir.OpLessEqual:
						truth = a <= b
					case ir.OpGreater:
						truth = a > b
					case ir.OpGreaterEqual:
						truth = a >= b
					}
					if truth {
						result.value = -1
					}
				case ir.OpCall:
					result.value, result.failed = run(in.Callee, args)
				case ir.OpTry:
					if args[0].failed {
						return 0, true
					}
					result.value = args[0].value
				default:
					t.Fatalf("test oracle cannot execute %s", in.Op)
				}
				if in.Type.Form == ir.Result {
					switch in.Type.Scalar {
					case ir.Integer:
						result.failed = result.failed || result.value < -32768 || result.value > 32767
					case ir.Long:
						result.failed = result.failed || result.value < -2147483648 || result.value > 2147483647
					}
				}
				if in.ID != 0 {
					values[in.ID] = result
				}
			}
			if statement.End == ir.Return {
				if p.ResultSlot != 0 {
					return *cells[p.ResultSlot], false
				}
				return 0, false
			}
			switch statement.End {
			case ir.Continue:
				current = statement.Next
			case ir.Jump:
				current = statement.Target
			case ir.Branch:
				current = statement.FalseTarget
				if values[statement.Condition].value != 0 {
					current = statement.Target
				}
			case ir.Switch:
				current = statement.Target
				for _, item := range statement.Cases {
					if values[statement.Selector].value == item.Value {
						current = item.Target
						break
					}
				}
			default:
				t.Fatal("unsupported oracle terminator")
			}
		}
		t.Fatal("procedure fell through without a return")
		return 0, true
	}
	return run(id, arguments)
}
