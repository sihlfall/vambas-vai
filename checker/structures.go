package checker

import "sihlfall.ch/vambas-vai/ast"

func (c *Checker) checkStructure(symbol *ast.Symbol) {
	if symbol == nil || c.structureStates[symbol] != 0 {
		return
	}
	c.structureStates[symbol] = 1
	members := symbol.ValueDeclaration.AsTypeDeclaration().Members
	if members == nil || len(members.Nodes) == 0 {
		c.report(DiagnosticInvalidOperand, symbol.ValueDeclaration, "A user-defined type must contain at least one field")
		c.structureStates[symbol] = 2
		return
	}
	for _, field := range members.Nodes {
		t := c.typeOfSymbol(field.Symbol)
		if t.Kind == TypeArray {
			if t.Rank == 0 {
				continue
			} // Dynamic array storage does not embed its elements.
			t = t.ElementType
		}
		if t.Kind != TypeStructure {
			continue
		}
		if c.structureStates[t.Symbol] == 1 {
			c.report(DiagnosticRecursiveType, field, "A user-defined type cannot embed itself, directly or through other fixed fields")
		} else {
			c.checkStructure(t.Symbol)
		}
	}
	c.structureStates[symbol] = 2
}
