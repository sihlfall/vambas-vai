package checker

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/stdlib"
)

// prepareLibraryCall applies the existing library result refinement and returns
// the first positional formal. Run after publishing the in-progress resolution,
// before ordinary argument binding; user declarations have no catalog identity.
func (c *Checker) prepareLibraryCall(r *CallResolution, args []*ast.Node) int {
	symbol := r.Signature.Declaration.Symbol
	if symbol == nil {
		return 0
	}
	switch symbol.Library {
	case stdlib.Len, stdlib.LenB:
		// The declaration returns Variant. Preserve the current Long refinement
		// for statically typed String, Date and Currency values, including
		// incomplete named calls. Variant inputs retain the declared result.
		if len(args) == 1 {
			expression := args[0]
			if expression.Kind == ast.KindNamedArgument {
				expression = expression.AsNamedArgument().Expression
			}
			if kind := c.typeOfNode(expression).Kind; kind == TypeString || kind == TypeDate || kind == TypeCurrency {
				r.ResultType = c.intrinsic[TypeLong]
			}
		}
	case stdlib.InStr:
		// InStr(string1, string2) omits Start without an empty argument. The
		// three/four-argument form supplies Start first. Named calls use names.
		if len(args) == 2 && args[0].Kind != ast.KindNamedArgument && args[1].Kind != ast.KindNamedArgument {
			return 1
		}
	}
	return 0
}

// finishLibraryCall refines from mapped arguments without rewriting the selected
// signature. Keep the existing behavior even when another argument is invalid,
// so editor queries retain the useful result type of an incomplete call.
func (c *Checker) finishLibraryCall(r *CallResolution) {
	symbol := r.Signature.Declaration.Symbol
	if symbol == nil {
		return
	}
	// Scalar library conversions must evaluate an interface's statically
	// selected default member before erasing its reference into a Variant.
	// Otherwise the runtime would select the concrete class's default member.
	switch symbol.Library {
	case stdlib.CByte, stdlib.CInt, stdlib.CLng, stdlib.CBool, stdlib.CStr,
		stdlib.CSng, stdlib.CDbl, stdlib.CDate, stdlib.CVDate, stdlib.CCur,
		stdlib.CDec, stdlib.CVErr, stdlib.AscW, stdlib.StrComp, stdlib.InStr,
		stdlib.Left, stdlib.LeftString, stdlib.StringRepeat, stdlib.StringRepeatString, stdlib.Split:
		for _, a := range r.Arguments {
			if a.Expression == nil {
				continue
			}
			t := c.typeOfNode(a.Expression)
			if t.Kind == TypeClass && c.interfaceTargets[t.Symbol] && c.GetDefaultMember(t) != nil {
				if c.simpleType(a.Expression).Kind == TypeError {
					r.Valid = false
				}
			}
		}
	}
	switch symbol.Library {
	case stdlib.Mid, stdlib.MidString:
		// Mid has a Variant(String)/Null result; Mid$ is the String form. The
		// String-only compiler uses an unboxed String for known String input.
		for _, argument := range r.Arguments {
			if argument.ParameterIndex == 0 && argument.Expression != nil && c.typeOfNode(argument.Expression).Kind == TypeString {
				r.ResultType = c.intrinsic[TypeString]
			}
		}
	}
}
