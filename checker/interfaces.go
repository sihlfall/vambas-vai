package checker

import (
	"fmt"
	"math"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
)

// InterfaceImplementation describes an explicitly implemented default interface.
// Implemented interfaces are not inherited, and their private handlers do not
// become members of the implementing class's default interface.
type InterfaceImplementation struct {
	Declaration *ast.Node
	Interface   *ast.Symbol
	Members     []InterfaceMember
}

type InterfaceMember struct {
	Declaration, Implementation *ast.Node
	Kind                        ast.Kind // Procedure/accessor kind; fields contribute accessor contracts.
}

func (c *Checker) GetImplementedInterfaces(class *ast.Symbol) []InterfaceImplementation {
	result := append([]InterfaceImplementation(nil), c.interfaces[class]...)
	for i := range result {
		result[i].Members = append([]InterfaceMember(nil), result[i].Members...)
	}
	return result
}

func (c *Checker) checkInterfaces() {
	for _, file := range c.files {
		seen := make(map[*ast.Symbol]bool)
		for _, node := range file.Statements.Nodes {
			if node.Kind != ast.KindImplementsStatement {
				continue
			}
			fail := func(message string) { c.report(DiagnosticImplements, node, message) }
			if !file.ModuleKind.IsClass() {
				fail("Implements requires a class module")
				continue
			}
			target := c.symbols[node.AsImplementsStatement().Interface]
			if target == nil {
				c.typeFromName(node.AsImplementsStatement().Interface)
				continue
			}
			if target.Flags&ast.SymbolFlagsClass == 0 || target.ValueDeclaration == nil || target.ValueDeclaration.Kind != ast.KindSourceFile {
				fail("Implements currently requires a class defined in this project")
				continue
			}
			if target == file.Symbol || seen[target] {
				fail("An interface cannot be implemented twice or by itself")
				continue
			}
			seen[target] = true
			for previous := range seen {
				if previous != target && (strings.HasPrefix(strings.ToLower(previous.Name)+"_", strings.ToLower(target.Name)+"_") || strings.HasPrefix(strings.ToLower(target.Name)+"_", strings.ToLower(previous.Name)+"_")) {
					fail("Implemented interface name prefixes must not overlap")
				}
			}
			implementation := InterfaceImplementation{Declaration: node, Interface: target}
			add := func(contract *ast.Node, kind ast.Kind) {
				if strings.Contains(ast.DeclarationName(contract).AsIdentifier().Text, "_") {
					fail("An interface class cannot expose member names containing underscores")
					return
				}
				name := target.Name + "_" + ast.DeclarationName(contract).AsIdentifier().Text
				symbol := file.Locals.Lookup(name)
				var handler *ast.Node
				if symbol != nil {
					if kind == ast.KindGetAccessor || kind == ast.KindLetAccessor || kind == ast.KindSetAccessor {
						handler = ast.PropertyAccessor(symbol, kind)
					} else if symbol.ValueDeclaration != nil && symbol.ValueDeclaration.Kind == kind {
						handler = symbol.ValueDeclaration
					}
				}
				if handler == nil {
					fail(fmt.Sprintf("Missing interface implementation %s (%s)", name, interfaceKind(kind)))
					return
				}
				if !c.interfaceSignatureMatches(contract, kind, handler) {
					c.report(DiagnosticInterfaceSignature, handler, fmt.Sprintf("%s does not match the %s contract (kind, types, parameter modes, Optional/ParamArray, or default values)", name, target.Name))
					return
				}
				implementation.Members = append(implementation.Members, InterfaceMember{Declaration: contract, Implementation: handler, Kind: kind})
			}
			for _, statement := range target.ValueDeclaration.AsSourceFile().Statements.Nodes {
				if ast.IsProcedureDeclaration(statement) {
					if target.Exports.Lookup(ast.DeclarationName(statement).AsIdentifier().Text) == statement.Symbol && statement.AsFunctionDeclaration().Modifiers()&(ast.ModifierPrivate|ast.VisibiltiyFriend) == 0 {
						add(statement, statement.Kind)
					}
				} else if statement.Kind == ast.KindVariableStatement && statement.AsVariableStatement().Keyword != ast.KindCONST {
					for _, field := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
						if target.Exports.Lookup(ast.DeclarationName(field).AsIdentifier().Text) != field.Symbol {
							continue
						}
						t := c.typeOfSymbol(field.Symbol)
						if t.Kind == TypeArray || t.Kind == TypeStructure {
							fail("Array and user-defined type fields in implemented interfaces are not supported")
							continue
						}
						add(field, ast.KindGetAccessor)
						if !object(t.Kind) {
							add(field, ast.KindLetAccessor)
						}
						if object(t.Kind) || t.Kind == TypeVariant {
							add(field, ast.KindSetAccessor)
						}
					}
				}
			}
			c.interfaces[file.Symbol] = append(c.interfaces[file.Symbol], implementation)
		}
	}
	states := make(map[*ast.Symbol]uint8)
	var visit func(*ast.Symbol)
	visit = func(class *ast.Symbol) {
		if states[class] != 0 {
			return
		}
		states[class] = 1
		for _, implementation := range c.interfaces[class] {
			if states[implementation.Interface] == 1 {
				c.report(DiagnosticImplements, implementation.Declaration, "Circular Implements dependency")
			} else {
				visit(implementation.Interface)
			}
		}
		states[class] = 2
	}
	for _, file := range c.files {
		visit(file.Symbol)
	}
}

func interfaceKind(kind ast.Kind) string {
	switch kind {
	case ast.KindGetAccessor:
		return "Property Get"
	case ast.KindLetAccessor:
		return "Property Let"
	case ast.KindSetAccessor:
		return "Property Set"
	case ast.KindSubDeclaration:
		return "Sub"
	default:
		return "Function"
	}
}

func (c *Checker) interfaceSignatureMatches(contract *ast.Node, kind ast.Kind, handler *ast.Node) bool {
	b := c.signatureOfDeclaration(handler)
	if contract.Kind == ast.KindVariableDeclaration {
		t := c.typeOfSymbol(contract.Symbol)
		if kind == ast.KindGetAccessor {
			return len(b.Parameters) == 0 && sameType(t, b.ReturnType)
		}
		return len(b.Parameters) == 1 && sameType(t, b.Parameters[0].Type) && b.Parameters[0].Modifiers&(ast.ParameterOptional|ast.ParameterParamArray) == 0
	}
	a := c.signatureOfDeclaration(contract)
	if !sameType(a.ReturnType, b.ReturnType) || len(a.Parameters) != len(b.Parameters) {
		return false
	}
	for i, p := range a.Parameters {
		q := b.Parameters[i]
		setterValue := (kind == ast.KindLetAccessor || kind == ast.KindSetAccessor) && i == len(a.Parameters)-1
		if !sameType(p.Type, q.Type) || !setterValue && p.IsByRef() != q.IsByRef() || p.Modifiers&(ast.ParameterOptional|ast.ParameterParamArray) != q.Modifiers&(ast.ParameterOptional|ast.ParameterParamArray) {
			return false
		}
		if p.Modifiers&ast.ParameterOptional != 0 && !c.interfaceDefaultsMatch(p, q) {
			return false
		}
	}
	return true
}

func (c *Checker) interfaceDefaultsMatch(a, b *Parameter) bool {
	var key func(*ast.Node, *Type) string
	key = func(n *ast.Node, t *Type) string {
		if n == nil {
			switch t.Kind {
			case TypeVariant:
				return "missing"
			case TypeString:
				return "string:"
			case TypeClass, TypeObject:
				return "nothing"
			default:
				return "number:0"
			}
		}
		if n.Kind == ast.KindParenthesizedExpression {
			return key(n.AsParenthesizedExpression().Inner, t)
		}
		if n.Kind == ast.KindNOTHING {
			return "nothing"
		}
		if n.Kind == ast.KindNULL {
			return "null"
		}
		if n.Kind == ast.KindEMPTY && t.Kind == TypeVariant {
			return "empty"
		}
		if text, ok := c.interfaceConstantString(n, 0); ok && (t.Kind == TypeString || t.Kind == TypeVariant) {
			return "string:" + text
		}
		if number, ok := c.numericExpression(n); ok {
			switch t.Kind {
			case TypeByte, TypeInteger, TypeLong, TypeLongLong, TypeLongPtr:
				number = math.RoundToEven(number)
			case TypeSingle:
				number = float64(float32(number))
			case TypeBoolean:
				if number != 0 {
					number = -1
				}
			}
			return fmt.Sprintf("number:%g", number)
		}
		return ""
	}
	x, y := key(a.Default, a.Type), key(b.Default, b.Type)
	if x != "" && y != "" {
		return x == y
	}
	return a.Default != nil && b.Default != nil && a.Default.DeeplyEquals(b.Default)
}

func (c *Checker) interfaceConstantString(n *ast.Node, depth int) (string, bool) {
	if n == nil || depth > 256 {
		return "", false
	}
	if n.Kind == ast.KindStringLiteral {
		return n.AsStringLiteral().Text, true
	}
	if n.Kind == ast.KindParenthesizedExpression {
		return c.interfaceConstantString(n.AsParenthesizedExpression().Inner, depth+1)
	}
	if symbol := c.symbols[n]; ast.IsConstantSymbol(symbol) && symbol.ValueDeclaration != nil && symbol.ValueDeclaration.Kind == ast.KindVariableDeclaration {
		return c.interfaceConstantString(symbol.ValueDeclaration.AsVariableDeclaration().Initializer, depth+1)
	}
	if n.Kind == ast.KindBinaryExpression {
		e := n.AsBinaryExpression()
		if e.OperatorToken.Kind == ast.KindAMPERSAND || e.OperatorToken.Kind == ast.KindPLUS {
			a, okA := c.interfaceConstantString(e.Left, depth+1)
			b, okB := c.interfaceConstantString(e.Right, depth+1)
			if okA && okB && len(a)+len(b) <= 1<<20 {
				return a + b, true
			}
		}
	}
	return "", false
}
