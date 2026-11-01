package checker

import "sihlfall.ch/vambas-vai/ast"

func (c *Checker) checkControlStatement(node *ast.Node) {
	var condition *ast.Node
	if variable := controlVariable(node); variable != nil && variable.Kind == ast.KindIdentifier {
		for parent := node.Parent; parent != nil && !ast.IsProcedureDeclaration(parent); parent = parent.Parent {
			if other := controlVariable(parent); other != nil && other.Kind == ast.KindIdentifier && c.symbols[other] != nil && c.symbols[other] == c.symbols[variable] {
				c.report(DiagnosticInvalidOperand, variable, "For control variable is already in use by an enclosing loop")
				break
			}
		}
	}
	switch node.Kind {
	case ast.KindOnGotoStatement, ast.KindOnGosubStatement:
		n := node.AsOnGotoStatement()
		c.checkConversion(n.Expression, n.Expression, c.intrinsic[TypeInteger], false)
	case ast.KindSelectStatement:
		n := node.AsSelectStatement()
		c.checkCaseOperand(n.Expression)
		for _, clause := range n.CaseBlock.AsCaseBlock().Clauses.Nodes {
			if clause.Kind == ast.KindDefaultClause {
				continue
			}
			for _, item := range clause.AsCaseClause().Expressions.Nodes {
				switch item.Kind {
				case ast.KindCaseRangeClause:
					c.checkCaseOperand(item.AsCaseRangeClause().LowerBound)
					c.checkCaseOperand(item.AsCaseRangeClause().UpperBound)
				case ast.KindCaseComparisonClause:
					c.checkCaseOperand(item.AsCaseComparisonClause().Expression)
				default:
					c.checkCaseOperand(item)
				}
			}
		}
	case ast.KindIfStatement:
		condition = node.AsIfStatement().Expression
	case ast.KindWhileStatement:
		condition = node.AsWhileStatement().Expression
	case ast.KindDoStatement:
		condition = node.AsDoStatement().Expression
	case ast.KindForStatement:
		n := node.AsForStatement()
		variable := n.Initializer.AsLetStatement().LExpression
		t := c.typeOfNode(variable)
		if t.Kind != TypeVariant && (t.Kind < TypeByte || t.Kind > TypeDate) && t.Kind != TypeError && t.Kind != TypeUnknown {
			c.report(DiagnosticInvalidOperand, variable, "A For control variable must be numeric or Variant")
		}
		for _, expr := range []*ast.Node{n.Initializer.AsLetStatement().RExpression, n.Limit, n.Step} {
			if expr != nil {
				c.checkConversion(expr, expr, c.intrinsic[TypeDouble], false)
			}
		}
	case ast.KindForEachStatement:
		n := node.AsForEachStatement()
		variable := c.typeOfNode(n.Initializer)
		collection := c.typeOfNode(n.Expression)
		if collection.Kind != TypeArray && collection.Kind != TypeVariant && !object(collection.Kind) && collection.Kind != TypeUnknown && collection.Kind != TypeError {
			c.report(DiagnosticInvalidOperand, n.Expression, "For Each requires an array or an enumerable object")
		}
		if collection.Kind == TypeArray && variable.Kind != TypeVariant && variable.Kind != TypeError {
			c.report(DiagnosticInvalidOperand, n.Initializer, "A For Each array control variable must be Variant")
		}
		if s := c.storageOf(n.Initializer); !s.Writable && s.Kind != StorageDynamic && variable.Kind != TypeError {
			c.report(DiagnosticInvalidStorage, n.Initializer, "For Each requires writable control storage")
		}
	}
	if condition != nil {
		c.checkConversion(condition, condition, c.intrinsic[TypeBoolean], false)
	}
}

func (c *Checker) checkCaseOperand(node *ast.Node) {
	switch c.simpleType(node).Kind {
	case TypeArray, TypeVoid, TypeModule, TypeStructure, TypeProcedure:
		c.report(DiagnosticInvalidOperand, node, "Select Case requires scalar comparison operands")
	}
}

func controlVariable(node *ast.Node) *ast.Node {
	if node.Kind == ast.KindForStatement {
		return node.AsForStatement().Initializer.AsLetStatement().LExpression
	}
	if node.Kind == ast.KindForEachStatement {
		return node.AsForEachStatement().Initializer
	}
	return nil
}
