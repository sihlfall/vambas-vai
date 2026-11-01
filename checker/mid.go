package checker

import "sihlfall.ch/vambas-vai/ast"

type MidAssignmentResolution struct {
	Storage                    *Storage
	Start, Length, Replacement *Conversion
	Valid, Deferred            bool
}

func (c *Checker) GetMidAssignmentResolution(node *ast.Node) *MidAssignmentResolution {
	return c.midAssignments[node]
}

func (c *Checker) checkMidAssignment(node *ast.Node) {
	n := node.AsMidAssignmentStatement()
	s := c.storageOf(n.Target)
	r := &MidAssignmentResolution{Storage: s, Valid: true}
	c.midAssignments[node] = r
	if !s.Writable || !s.Addressable {
		r.Valid = false
		if !c.hasBoundStorageError(n.Target) {
			c.report(DiagnosticInvalidStorage, n.Target, "Mid requires writable bound variable storage")
		}
	}
	if s.Type.Kind != TypeString && s.Type.Kind != TypeVariant && s.Type.Kind != TypeError {
		r.Valid = false
		c.report(DiagnosticTypeMismatch, n.Target, "Mid assignment requires a String or Variant variable")
	}
	r.Deferred = s.Type.Kind == TypeVariant || s.Type.Kind == TypeUnknown || s.Kind == StorageDynamic
	c.checkConversion(n.Start, n.Start, c.intrinsic[TypeLong], false)
	r.Start = c.conversions[n.Start]
	if n.Length != nil {
		c.checkConversion(n.Length, n.Length, c.intrinsic[TypeLong], false)
		r.Length = c.conversions[n.Length]
	}
	c.checkConversion(node, n.Expression, c.intrinsic[TypeString], false)
	r.Replacement = c.conversions[node]
	for _, conversion := range []*Conversion{r.Start, r.Length, r.Replacement} {
		if conversion == nil {
			continue
		}
		if conversion.Kind == ConversionInvalid {
			r.Valid = false
		}
		if conversion.Kind == ConversionDeferred {
			r.Deferred = true
		}
	}
}
