package checker

import "sihlfall.ch/vambas-vai/ast"

// A resolution is an allocation/mutation plan, distinct from an index or call.
// Scope types are unchanged by ReDim, especially for a Variant holding an array.
// Valid is a static check, not a guarantee that allocation/Preserve will succeed.
type ReDimResolution struct {
	Node                      *ast.Node
	Storage                   *Storage
	ArrayType                 *Type
	Bounds                    []*ArrayBoundResolution
	Preserve                  bool
	Valid, Deferred           bool
	CheckPreservedBounds      bool
	CheckPreservedElementType bool
	CheckArrayLock            bool
}
type ArrayBoundResolution struct {
	Node                             *ast.Node
	LowerBound, UpperBound           *ast.Node // nil lower retains the module's default bound.
	LowerConversion, UpperConversion *Conversion
}

func (c *Checker) GetReDimResolution(declaration *ast.Node) *ReDimResolution {
	return c.reDims[declaration]
}

func (c *Checker) checkReDim(statement *ast.Node) {
	for _, node := range statement.AsReDimStatement().Declarations.Nodes {
		d := node.AsReDimDeclaration()
		t := c.typeOfNode(d.Expression)
		r := &ReDimResolution{Node: node, Storage: c.storageOf(d.Expression), Preserve: statement.AsReDimStatement().PreserveKeyword != nil, Valid: true, CheckArrayLock: true}
		r.CheckPreservedBounds = r.Preserve
		c.reDims[node] = r
		errorAt := func(n *ast.Node, message string) { r.Valid = false; c.report(DiagnosticReDim, n, message) }
		if t.Kind == TypeError {
			r.Valid = false
		} else if r.Storage.Kind == StorageDynamic {
			r.Deferred = true
		} else if !r.Storage.Writable || !r.Storage.Addressable {
			errorAt(d.Expression, "ReDim requires writable array or Variant storage, not a property result or value")
		}
		if len(d.Dimensions.Nodes) == 0 || len(d.Dimensions.Nodes) > 60 {
			errorAt(node, "ReDim requires between one and sixty dimensions")
		}
		if c.paramArrayStorage(r.Storage) {
			errorAt(d.Expression, "ReDim cannot resize a ParamArray")
		}
		if symbol := r.Storage.Symbol; symbol != nil && symbol.ValueDeclaration != nil {
			decl := symbol.ValueDeclaration
			if dimensions := declaredDimensions(decl); dimensions != nil && len(dimensions.Nodes) > 0 {
				errorAt(d.Expression, "ReDim cannot resize a fixed-size array")
			}
		}
		var requested *Type
		var name *ast.Node
		if d.Expression.Kind == ast.KindIdentifier {
			name = d.Expression
		} else if d.Expression.Kind == ast.KindPropertyAccessExpression {
			name = d.Expression.AsPropertyAccessExpression().Name
		}
		if d.Type != nil || name != nil && name.AsIdentifier().TypeHint != ast.KindNONE {
			requested = c.declaredType(name, d.Type, nil)
			if requested.Kind == TypeArray || d.Type != nil && d.Type.AsTypeReference().Length != nil {
				errorAt(d.Type, "ReDim's As clause requires an element type without array or fixed-string syntax")
			}
		}
		element := c.intrinsic[TypeUnknown]
		switch t.Kind {
		case TypeArray:
			element = t.ElementType
			if requested != nil && requested.Kind != TypeError && !sameType(requested, element) {
				errorAt(node, "ReDim cannot change the element type of a declared array")
			}
		case TypeVariant:
			r.Deferred = true
			r.CheckPreservedElementType = r.Preserve && requested != nil
			if requested != nil {
				element = requested
			}
		case TypeUnknown:
			r.Deferred = true
		case TypeError:
			r.Valid = false
		default:
			errorAt(d.Expression, "ReDim requires a resizable array or a Variant")
		}
		if requested != nil && requested.Kind == TypeError {
			r.Valid = false
		}
		r.ArrayType = &Type{Kind: TypeArray, ElementType: element, Rank: len(d.Dimensions.Nodes)}
		for _, dimension := range d.Dimensions.Nodes {
			dim := dimension.AsArrayDimension()
			bound := &ArrayBoundResolution{Node: dimension, LowerBound: dim.LowerBound, UpperBound: dim.UpperBound}
			convert := func(expr *ast.Node) *Conversion {
				if expr == nil {
					return nil
				}
				from, to := c.simpleType(expr), c.intrinsic[TypeLong]
				kind := conversionKind(from, to, false)
				if kind == ConversionInvalid {
					r.Valid = false
					if from.Kind != TypeError {
						c.report(DiagnosticReDim, expr, "An array bound must be coercible to Long")
					}
				}
				if kind == ConversionDeferred || from.Kind == TypeVariant {
					r.Deferred = true
				}
				return &Conversion{Kind: kind, From: from, To: to}
			}
			bound.LowerConversion, bound.UpperConversion = convert(dim.LowerBound), convert(dim.UpperBound)
			r.Bounds = append(r.Bounds, bound)
		}
		c.types[node] = r.ArrayType
	}
}

func declaredDimensions(decl *ast.Node) *ast.NodeList {
	switch decl.Kind {
	case ast.KindVariableDeclaration, ast.KindFieldDeclaration:
		return decl.AsVariableDeclaration().Dimensions
	case ast.KindParameter:
		return decl.AsParameterDeclaration().Dimensions
	}
	return nil
}
