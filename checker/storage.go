package checker

import "sihlfall.ch/vambas-vai/ast"

func (c *Checker) storageOf(node *ast.Node) *Storage {
	if s := c.storage[node]; s != nil {
		return s
	}
	s := &Storage{Node: node, Kind: StorageValue, Type: c.intrinsic[TypeUnknown]}
	if node == nil {
		return s
	}
	c.storage[node] = s
	s.Type, s.Symbol = c.typeOfNode(node), c.symbols[node]
	if node.Kind == ast.KindPropertyAccessExpression {
		s.WithTarget = node.AsPropertyAccessExpression().WithTarget
	}
	if node.Kind == ast.KindParenthesizedExpression || isDeclarationName(node) || node.Parent != nil && node.Parent.Kind == ast.KindNamedArgument && node.Parent.AsNamedArgument().Name == node {
		return s // Parentheses force value evaluation, even around a variable.
	}
	if parent := node.Parent; parent != nil && parent.Kind == ast.KindPropertyAccessExpression && parent.AsPropertyAccessExpression().Name == node {
		if parent.AsPropertyAccessExpression().OperatorToken.Kind == ast.KindDOT {
			*s = *c.storageOf(parent)
			s.Node = node
		}
		return s
	}
	if r := c.calls[node]; r != nil {
		s.Resolution, s.Receiver, s.WithTarget = r, r.Receiver, r.WithTarget
		switch r.Kind {
		case CallArrayIndex, CallArrayReference:
			if r.Kind == CallArrayReference {
				base := c.storageOf(r.Target)
				s.Kind, s.Writable, s.Addressable = base.Kind, base.Writable, base.Addressable
			} else {
				s.Kind, s.Writable, s.Addressable = StorageArrayElement, r.Valid, r.Valid
			}
			for _, a := range r.Arguments {
				s.Indices = append(s.Indices, a.Expression)
			}
			return s
		case CallPropertyGet, CallPropertyLet, CallPropertySet:
			s.Kind = StorageProperty
			s.Type = r.ResultType
			s.Writable = r.Valid && (r.Kind == CallPropertyLet || r.Kind == CallPropertySet)
			if !s.Writable && r.Signature != nil && r.Signature.Declaration.Symbol != nil {
				if r.Kind == CallPropertyGet {
					s.Writable = c.propertyWritable(r.Signature.Declaration.Symbol, node)
				}
			}
			return s
		case CallDynamic:
			s.Kind, s.Writable = StorageDynamic, true
			return s
		case CallInvalid:
			s.Kind = StorageInvalid
			return s
		}
	}
	if symbol := s.Symbol; symbol != nil {
		if symbol.Flags&ast.SymbolFlagsProperty != 0 && node.Kind == ast.KindPropertyAccessExpression {
			s.Kind, s.Receiver = StorageVariable, c.memberReceiver(node)
			base := c.storageOf(s.Receiver)
			s.Writable, s.Addressable = base.Writable, base.Addressable
		} else if symbol.Flags&ast.SymbolFlagsFunctionScopedVariable != 0 && !ast.IsConstantSymbol(symbol) && (node.Kind == ast.KindIdentifier || node.Kind == ast.KindPropertyAccessExpression) {
			s.Kind, s.Writable, s.Addressable = StorageVariable, true, true
			if node.Kind == ast.KindPropertyAccessExpression {
				s.Receiver = c.memberReceiver(node)
			}
		} else if symbol.Flags&ast.SymbolFlagsAccessor != 0 {
			s.Kind, s.Writable = StorageProperty, c.propertyWritable(symbol, node)
		}
	}
	// A public field viewed through an implemented default interface is a
	// property contract, and cannot expose concrete payload storage ByRef.
	if s.Kind == StorageVariable && s.Symbol != nil && c.interfaceTargets[s.Symbol.Parent] && s.Symbol.Parent.Exports.Lookup(s.Symbol.Name) == s.Symbol && (s.Receiver != nil || s.WithTarget != nil) {
		s.Addressable = false
	}
	return s
}

func (c *Checker) propertyWritable(symbol *ast.Symbol, site *ast.Node) bool {
	for _, kind := range []ast.Kind{ast.KindLetAccessor, ast.KindSetAccessor} {
		decl := ast.PropertyAccessor(symbol, kind)
		if decl != nil && (decl.AsFunctionDeclaration().Modifiers()&ast.ModifierPrivate == 0 || ast.SourceFileOf(decl) == c.owners[site]) {
			return true
		}
	}
	return false
}

func (c *Checker) collectStorage(node *ast.Node) {
	if node.Kind == ast.KindAttributeStatement {
		return
	}
	switch node.Kind {
	case ast.KindIdentifier, ast.KindPropertyAccessExpression, ast.KindCallExpression, ast.KindParenthesizedExpression:
		c.storageOf(node)
	}
	node.ForEachChild(func(child *ast.Node) bool { c.collectStorage(child); return false })
}

// A ParamArray entity has array query/index syntax, but cannot itself be the
// target of Erase or ReDim. Its Variant elements remain ordinary storage.
func (c *Checker) paramArrayStorage(s *Storage) bool {
	if s == nil {
		return false
	}
	if s.Resolution != nil && s.Resolution.Kind == CallArrayReference {
		return c.paramArrayStorage(c.storageOf(s.Resolution.Target))
	}
	if s.Kind != StorageVariable || s.Symbol == nil || s.Symbol.ValueDeclaration == nil {
		return false
	}
	d := s.Symbol.ValueDeclaration
	return d.Kind == ast.KindParameter && d.AsParameterDeclaration().Modifiers&ast.ParameterParamArray != 0
}
