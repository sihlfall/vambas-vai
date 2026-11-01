package checker

import "sihlfall.ch/vambas-vai/ast"

type ArrayStaticBound struct{ Lower, Upper int32 }

// Bounds belong to the checked snapshot. Fixed Dim bounds are constants,
// unlike the executable expressions in a ReDim plan.
func (c *Checker) GetArrayBounds(declaration *ast.Node) []ArrayStaticBound {
	return c.arrayBounds[declaration]
}

func (c *Checker) checkArrayBounds(declaration *ast.Node) {
	dimensions := declaration.AsVariableDeclaration().Dimensions
	if dimensions == nil || len(dimensions.Nodes) == 0 {
		return
	}
	if len(dimensions.Nodes) > 60 {
		c.report(DiagnosticArgumentCount, declaration, "An array can have at most sixty dimensions")
		return
	}
	var bounds []ArrayStaticBound
	valid := true
	for _, node := range dimensions.Nodes {
		d := node.AsArrayDimension()
		low := int32(c.owners[declaration].OptionBase)
		if d.LowerBound != nil {
			value, constant := c.numericExpression(d.LowerBound)
			var fits bool
			low, fits = constantLong(value)
			if !constant || !fits {
				c.report(DiagnosticArgumentType, d.LowerBound, "A fixed array bound must be a constant coercible to Long")
				valid = false
			}
		}
		value, constant := c.numericExpression(d.UpperBound)
		high, fits := constantLong(value)
		if !constant || !fits {
			c.report(DiagnosticArgumentType, d.UpperBound, "A fixed array bound must be a constant coercible to Long")
			valid = false
		}
		if constant && fits && low > high {
			c.report(DiagnosticArgumentType, node, "An array lower bound cannot exceed its upper bound")
			valid = false
		}
		bounds = append(bounds, ArrayStaticBound{low, high})
	}
	if valid {
		c.arrayBounds[declaration] = bounds
	}
}
