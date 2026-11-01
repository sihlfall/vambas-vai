package ir

// InterfaceCallEffects includes every implementation of this nominal contract.
// Interface implementations can recurse through other interfaces, so lowering
// computes the same fixed point as for ordinary calls.
func InterfaceCallEffects(program *Program, id ProcedureID) Effects {
	effects := MayRaise
	if id == 0 || int(id) > len(program.Procedures) || program.Procedures[id-1] == nil {
		return effects
	}
	effects |= program.Procedures[id-1].Effects
	for _, class := range program.Classes {
		for _, iface := range class.Interfaces {
			for _, method := range iface.Methods {
				if method.Declaration == id && method.Implementation != 0 && int(method.Implementation) <= len(program.Procedures) && program.Procedures[method.Implementation-1] != nil {
					effects |= program.Procedures[method.Implementation-1].Effects
				}
			}
		}
	}
	return effects
}

func verifyInterfaces(program *Program, report func(Source, string, ...any)) map[ProcedureID]bool {
	members := make(map[ProcedureID]bool)
	procedure := func(id ProcedureID) *Procedure {
		if id == 0 || int(id) > len(program.Procedures) {
			return nil
		}
		return program.Procedures[id-1]
	}
	for _, class := range program.Classes {
		seen := make(map[ClassID]bool)
		for _, iface := range class.Interfaces {
			if iface.Class == 0 || int(iface.Class) > len(program.Classes) || iface.Class == class.ID || seen[iface.Class] {
				report(class.Source, "invalid or duplicate implemented interface")
				continue
			}
			seen[iface.Class] = true
			methods := make(map[ProcedureID]bool)
			for _, method := range iface.Methods {
				a, b := procedure(method.Declaration), procedure(method.Implementation)
				if a == nil || b == nil || a.Class != iface.Class || b.Class != class.ID || methods[method.Declaration] {
					report(class.Source, "invalid or duplicate interface method mapping")
					continue
				}
				methods[method.Declaration], members[method.Declaration] = true, true
				if a.ReturnType != b.ReturnType || len(a.Parameters) != len(b.Parameters) {
					report(class.Source, "interface implementation signature differs")
					continue
				}
				for i := 1; i < len(a.Parameters); i++ {
					x, y := a.Parameters[i], b.Parameters[i]
					if x == 0 || y == 0 || int(x) > len(a.Slots) || int(y) > len(b.Slots) {
						report(class.Source, "invalid interface parameter")
						continue
					}
					p, q := a.Slots[x-1], b.Slots[y-1]
					if p.Type != q.Type || p.ByRef != q.ByRef || p.ParamArray != q.ParamArray {
						report(class.Source, "interface implementation parameter differs")
					}
				}
			}
			for _, id := range program.Classes[iface.Class-1].Members {
				if p := procedure(id); p != nil && p.Invocation != nil && !methods[id] {
					report(class.Source, "incomplete interface implementation")
				}
			}
		}
	}
	return members
}
