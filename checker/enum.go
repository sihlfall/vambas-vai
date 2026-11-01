package checker

import (
	"math"
	"strconv"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
)

// Enum members are Long constants in VBA, not distinct numeric types. Values
// live on the checker, as in typescript-go; the syntax keeps omitted initializers.
func (c *Checker) numericConstant(symbol *ast.Symbol) (value float64, ok bool) {
	// Ambient constants may first be reached from an enum. Resolve their
	// declared type before entering numeric evaluation, avoiding a false cycle.
	if symbol.Flags&ast.SymbolFlagsEnumMember == 0 && c.constants[symbol] == 0 {
		c.resolveConstant(symbol, symbol.ValueDeclaration)
	}
	switch c.numberStates[symbol] {
	case 1:
		c.report(DiagnosticCircularConstant, symbol.ValueDeclaration, "Constant value depends on itself")
		c.numberStates[symbol] = 3
		return 0, false
	case 2:
		return c.constantNumbers[symbol], true
	case 3:
		return 0, false
	}
	c.numberStates[symbol] = 1
	defer func() {
		if ok && c.numberStates[symbol] != 3 {
			c.constantNumbers[symbol], c.numberStates[symbol] = value, 2
		} else {
			c.numberStates[symbol], ok = 3, false
		}
	}()
	decl := symbol.ValueDeclaration
	if decl.Kind == ast.KindEnumMember {
		initializer := decl.AsEnumMember().Initializer
		if initializer == nil {
			value, ok = 0, true
			members := decl.Parent.AsEnumDeclaration().Members.Nodes
			for i, member := range members {
				if member == decl && i > 0 {
					value, ok = c.numericConstant(members[i-1].Symbol)
					value++
					break
				}
			}
		} else {
			valid := true
			var visit func(*ast.Node) bool
			visit = func(n *ast.Node) bool {
				if target := c.symbols[n]; target != nil && target.Flags&ast.SymbolFlagsEnumMember != 0 && ast.SourceFileOf(target.ValueDeclaration) == ast.SourceFileOf(decl) && target.ValueDeclaration.Pos() >= decl.Pos() {
					valid = false
				}
				return n.ForEachChild(visit)
			}
			visit(initializer)
			if !valid {
				c.report(DiagnosticEnumValue, initializer, "An enum initializer cannot refer to itself or a later enum member in the module")
				return 0, false
			}
			value, ok = c.numericExpression(initializer)
		}
		value = math.RoundToEven(value) // VBA Let-coercion to Long.
		if !ok || math.IsInf(value, 0) || math.IsNaN(value) || value < math.MinInt32 || value > math.MaxInt32 {
			c.report(DiagnosticEnumValue, decl, "Enum member requires a constant coercible to Long without overflow")
			return 0, false
		}
		c.enumValues[symbol] = int64(value)
		return value, true
	}
	if decl.Kind == ast.KindVariableDeclaration && ast.IsConstantSymbol(symbol) {
		d := decl.AsVariableDeclaration()
		value, ok = c.numericExpression(d.Initializer)
		kind := c.typeOfSymbol(symbol).Kind
		switch kind {
		case TypeBoolean:
			if value != 0 {
				value = -1
			}
		case TypeByte, TypeInteger, TypeLong, TypeLongLong:
			value = math.RoundToEven(value)
		case TypeSingle:
			value = float64(float32(value))
		}
		switch kind {
		case TypeByte:
			ok = ok && value >= 0 && value <= 255
		case TypeInteger:
			ok = ok && value >= math.MinInt16 && value <= math.MaxInt16
		case TypeLong:
			ok = ok && value >= math.MinInt32 && value <= math.MaxInt32
		}
		return value, ok && !math.IsInf(value, 0) && !math.IsNaN(value)
	}
	return 0, false
}

func constantNumber(text string) (float64, bool) {
	text = strings.ToLower(strings.TrimSpace(text))
	hint := TypeUnknown
	if len(text) > 0 {
		hint = hintType(ast.IdentifierTypeHint(text[len(text)-1]))
	}
	if hint != TypeUnknown {
		text = text[:len(text)-1]
	}
	if strings.HasPrefix(text, "&h") || strings.HasPrefix(text, "&o") {
		base := 16
		if text[1] == 'o' {
			base = 8
		}
		v, err := strconv.ParseUint(text[2:], base, 64)
		if err != nil {
			return 0, false
		}
		if hint == TypeLongLong {
			return float64(int64(v)), true
		}
		if hint == TypeLong || v > 0xffff {
			return float64(int32(v)), v <= math.MaxUint32
		}
		return float64(int16(v)), true
	}
	bits := 64
	if hint == TypeSingle {
		bits = 32
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(text, "d", "e"), bits)
	return v, err == nil && !math.IsInf(v, 0) && !math.IsNaN(v)
}

func (c *Checker) numericExpression(node *ast.Node) (value float64, ok bool) {
	if node == nil {
		return 0, false
	}
	defer func() {
		if !ok {
			return
		}
		if math.IsInf(value, 0) || math.IsNaN(value) {
			ok = false
			return
		}
		// Constant arithmetic has the same declared-width overflow constraints
		// as ordinary arithmetic, before the final Long coercion of an enum.
		switch c.typeOfNode(node).Kind {
		case TypeInteger:
			ok = value >= math.MinInt16 && value <= math.MaxInt16
		case TypeLong:
			ok = value >= math.MinInt32 && value <= math.MaxInt32
		case TypeSingle:
			value = float64(float32(value))
			ok = !math.IsInf(value, 0)
		case TypeLongLong:
			ok = math.Abs(value) < math.Exp2(53) // Wider exact integer evaluation remains deferred.
		case TypeString:
			// Literal/constant numeric strings can be coerced to Long. General
			// string operations still need the shared VBA constant evaluator.
			if node.Kind == ast.KindBinaryExpression {
				ok = false
			}
		}
		if ok {
			c.constantExpressions[node] = value
		}
	}()
	if symbol := c.symbols[node]; ast.IsConstantSymbol(symbol) {
		return c.numericConstant(symbol)
	}
	switch node.Kind {
	case ast.KindNumericLiteral:
		return constantNumber(node.AsNumericLiteral().Text)
	case ast.KindStringLiteral:
		v, err := strconv.ParseFloat(strings.TrimSpace(node.AsStringLiteral().Text), 64)
		return v, err == nil
	case ast.KindTRUE:
		return -1, true
	case ast.KindFALSE, ast.KindEMPTY:
		return 0, true
	case ast.KindParenthesizedExpression:
		return c.numericExpression(node.AsParenthesizedExpression().Inner)
	case ast.KindPrefixUnaryExpression:
		n := node.AsPrefixUnaryExpression()
		v, ok := c.numericExpression(n.Operand)
		if !ok {
			return 0, false
		}
		switch n.Operator {
		case ast.KindMINUS:
			return -v, true
		case ast.KindPLUS:
			return v, true
		case ast.KindNOT:
			i, ok := constantLong(v)
			return float64(^i), ok
		}
	case ast.KindBinaryExpression:
		n := node.AsBinaryExpression()
		a, okA := c.numericExpression(n.Left)
		b, okB := c.numericExpression(n.Right)
		if !okA || !okB {
			return 0, false
		}
		switch n.OperatorToken.Kind {
		case ast.KindPLUS:
			return a + b, true
		case ast.KindMINUS:
			return a - b, true
		case ast.KindMULT:
			return a * b, true
		case ast.KindDIV:
			return a / b, b != 0
		case ast.KindPOW:
			return math.Pow(a, b), true
		case ast.KindEQ, ast.KindNEQ, ast.KindLT, ast.KindLEQ, ast.KindGT, ast.KindGEQ:
			truth := false
			switch n.OperatorToken.Kind {
			case ast.KindEQ:
				truth = a == b
			case ast.KindNEQ:
				truth = a != b
			case ast.KindLT:
				truth = a < b
			case ast.KindLEQ:
				truth = a <= b
			case ast.KindGT:
				truth = a > b
			case ast.KindGEQ:
				truth = a >= b
			}
			if truth {
				return -1, true
			}
			return 0, true
		}
		i, okI := constantLong(a)
		j, okJ := constantLong(b)
		if !okI || !okJ {
			return 0, false
		}
		switch n.OperatorToken.Kind {
		case ast.KindINTDIV:
			if j != 0 {
				return float64(int64(i) / int64(j)), true
			}
		case ast.KindMOD:
			if j != 0 {
				return float64(int64(i) % int64(j)), true
			}
		case ast.KindAND:
			return float64(i & j), true
		case ast.KindOR:
			return float64(i | j), true
		case ast.KindXOR:
			return float64(i ^ j), true
		case ast.KindEQV:
			return float64(^(i ^ j)), true
		case ast.KindIMP:
			return float64(^i | j), true
		}
	}
	return 0, false
}

func constantLong(v float64) (int32, bool) {
	v = math.RoundToEven(v)
	return int32(v), !math.IsNaN(v) && v >= math.MinInt32 && v <= math.MaxInt32
}

// GetNumericLiteralValue exposes the literal interpretation shared with enum
// checking. It is pure: consumers of a completed checker do not populate caches.
func (c *Checker) GetNumericLiteralValue(node *ast.Node) (float64, bool) {
	if node == nil || node.Kind != ast.KindNumericLiteral {
		return 0, false
	}
	return constantNumber(node.AsNumericLiteral().Text)
}

// GetNumericConstantValue and GetNumericConstantExpression read values prepared
// during checking. Like enum values, they never evaluate or mutate on query.
func (c *Checker) GetNumericConstantValue(symbol *ast.Symbol) (float64, bool) {
	value, ok := c.constantNumbers[symbol]
	return value, ok && c.numberStates[symbol] == 2
}
func (c *Checker) GetNumericConstantExpression(node *ast.Node) (float64, bool) {
	value, ok := c.constantExpressions[node]
	return value, ok
}
