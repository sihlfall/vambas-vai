package checker

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
)

func (c *Checker) checkAssignment(node *ast.Node) {
	assignment := node.AsLetStatement()
	left, right := assignment.LExpression, assignment.RExpression
	to := c.typeOfNode(left)
	operation := c.calls[left]
	resultTarget := operation != nil && (operation.Kind == CallProcedure || operation.Kind == CallPropertyGet)
	if to.Kind == TypeClass && (node.Kind == ast.KindLetStatement && operation == nil || resultTarget) || to.Kind == TypeObject && node.Kind == ast.KindLetStatement && operation == nil || resultTarget && (to.Kind == TypeObject || to.Kind == TypeVariant) {
		delete(c.calls, left)
		delete(c.resolved, left)
		to = c.application(left, left, to, nil)
		c.calls[left].Prefix = operation
	}
	storage := c.storageOf(left)
	if to.Kind == TypeArray {
		symbol := storage.Symbol
		if symbol == nil && storage.Resolution != nil && storage.Resolution.Kind == CallArrayReference {
			symbol = storage.Resolution.Target.Symbol
		}
		if symbol != nil && symbol.ValueDeclaration != nil {
			if dims := declaredDimensions(symbol.ValueDeclaration); dims != nil && len(dims.Nodes) > 0 {
				c.report(DiagnosticInvalidStorage, left, "A fixed-size array cannot be assigned as a whole")
			}
		}
	}
	if !storage.Writable && storage.Kind != StorageDynamic && to.Kind != TypeError && !c.hasBoundStorageError(left) {
		c.report(DiagnosticInvalidStorage, left, "The assignment target is not writable storage")
	}
	c.checkConversion(node, right, to, node.Kind == ast.KindSetStatement)
}

func (c *Checker) hasBoundStorageError(node *ast.Node) bool {
	if c.boundErrors[node] {
		return true
	}
	if node.Kind == ast.KindPropertyAccessExpression {
		return c.boundErrors[node.AsPropertyAccessExpression().Name]
	}
	return false
}

func (c *Checker) checkConversion(owner, expression *ast.Node, to *Type, set bool) {
	from := c.typeOfNode(expression)
	if !set && !object(to.Kind) && from.Kind == TypeClass {
		from = c.simpleType(expression)
	}
	kind := conversionKind(from, to, set)
	c.conversions[owner] = &Conversion{Kind: kind, From: from, To: to}
	if kind == ConversionInvalid && from.Kind != TypeError && to.Kind != TypeError {
		c.report(DiagnosticTypeMismatch, expression, fmt.Sprintf("Cannot %s-assign %s to %s", assignmentName(set), from, to))
	}
}

func assignmentName(set bool) string {
	if set {
		return "Set"
	}
	return "Let"
}

func conversionKind(from, to *Type, set bool) ConversionKind {
	f, t := from.Kind, to.Kind
	if f == TypeError || t == TypeError {
		return ConversionInvalid
	}
	if f == TypeUnknown || t == TypeUnknown {
		return ConversionDeferred
	}
	if set {
		if (!object(f) && f != TypeVariant) || (!object(t) && t != TypeVariant) {
			return ConversionInvalid
		}
		return ConversionSet
	}
	if f == TypeStructure || t == TypeStructure {
		if f == TypeStructure && t == TypeStructure && from.Symbol == to.Symbol {
			return ConversionIdentity
		}
		return ConversionInvalid
	}
	if f == TypeArray || t == TypeArray {
		if f == TypeArray && t == TypeArray {
			if sameType(from, to) {
				return ConversionIdentity
			}
			return ConversionInvalid
		}
		if f == TypeString && t == TypeArray && to.ElementType.Kind == TypeByte || f == TypeArray && from.ElementType.Kind == TypeByte && t == TypeString {
			return ConversionLet
		}
		if f == TypeVariant || t == TypeVariant {
			return ConversionLet
		}
		return ConversionInvalid
	}
	if object(f) || object(t) {
		return ConversionDeferred // Let may invoke a default value member.
	}
	if f == TypeVoid || t == TypeVoid || f == TypeModule || t == TypeModule || f == TypeProcedure || t == TypeProcedure {
		return ConversionInvalid
	}
	if f == t {
		return ConversionIdentity
	}
	if scalar(f) && scalar(t) || f == TypeVariant || t == TypeVariant {
		return ConversionLet
	}
	return ConversionInvalid
}
