package checker

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
)

func callKind(sig *Signature) CallKind {
	switch sig.Declaration.Kind {
	case ast.KindGetAccessor:
		return CallPropertyGet
	case ast.KindLetAccessor:
		return CallPropertyLet
	case ast.KindSetAccessor:
		return CallPropertySet
	default:
		return CallProcedure
	}
}

func (c *Checker) invocation(site, target, receiver *ast.Node, sig *Signature, args []*ast.Node, rhs *ast.Node, defaultMember *ast.Symbol) *CallResolution {
	if old := c.calls[site]; old != nil {
		return old
	}
	r := &CallResolution{Kind: callKind(sig), Node: site, Target: target, Receiver: receiver,
		Signature: sig, DefaultMember: defaultMember, ResultType: sig.ReturnType, Valid: true}
	c.calls[site], c.resolved[site] = r, sig
	position := c.prepareLibraryCall(r, args)
	if site.Kind == ast.KindPropertyAccessExpression {
		r.WithTarget = site.AsPropertyAccessExpression().WithTarget
	}
	if target.Kind == ast.KindPropertyAccessExpression {
		r.WithTarget = target.AsPropertyAccessExpression().WithTarget
	}
	if sig.Declaration.AsFunctionDeclaration().Modifiers()&ast.ModifierPrivate != 0 && ast.SourceFileOf(sig.Declaration) != c.owners[site] {
		r.Valid = false
		if !c.hasBoundStorageError(site) {
			c.report(DiagnosticMember, site, "The selected accessor or procedure is private")
		}
	}
	parameters := sig.Parameters
	setter := r.Kind == CallPropertyLet || r.Kind == CallPropertySet
	count := len(parameters)
	if setter && count != 0 {
		count-- // The RHS supplies the value parameter, never an index argument.
		r.ResultType = parameters[count].Type
	}
	paramArray := -1
	if count != 0 && parameters[count-1].Modifiers&ast.ParameterParamArray != 0 {
		paramArray, r.ParamArray = count-1, parameters[count-1]
	}
	used := make([]bool, count)
	names := make(map[string]int)
	for i, p := range parameters[:count] {
		names[ast.SymbolNameKey(p.Name)] = i
	}
	packed := 0
	implicitParentheses := site.Kind == ast.KindCallExpression && site.AsCallExpression().HasParentheses && site.Parent != nil && site.Parent.Kind == ast.KindCallStatement && site.Parent.AsCallStatement().CallKeyword == nil
	if implicitParentheses && (len(args) > 1 || len(args) == 1 && (args[0].Kind == ast.KindNamedArgument || args[0].Kind == ast.KindOmittedExpression)) {
		c.callError(r, DiagnosticInvalidArgument, site, "Implicit procedure calls cannot enclose an argument list in parentheses")
	}
	for sourceIndex, arg := range args {
		binding := &ArgumentBinding{Argument: arg, Expression: arg, ParameterIndex: -1, SourceIndex: sourceIndex, ParamArrayIndex: -1}
		binding.ForcedValue = implicitParentheses && len(args) == 1
		if site.Kind == ast.KindPropertyAccessExpression && site.AsPropertyAccessExpression().OperatorToken.Kind == ast.KindEXCLAMATIONPOINT && arg == site.AsPropertyAccessExpression().Name {
			text := arg.AsIdentifier().Text
			binding.StringValue = &text
		}
		r.Arguments = append(r.Arguments, binding)
		index := position
		if arg.Kind == ast.KindNamedArgument {
			named := arg.AsNamedArgument()
			binding.Expression = named.Expression
			var ok bool
			index, ok = names[ast.SymbolNameKey(named.Name.AsIdentifier().Text)]
			if !ok {
				c.callError(r, DiagnosticUnknownArgument, named.Name, "No parameter has this name")
				binding.Mode = ArgumentInvalid
				continue
			}
			c.symbols[named.Name], c.types[named.Name] = parameters[index].Symbol, parameters[index].Type
			if paramArray >= 0 {
				c.callError(r, DiagnosticInvalidArgument, named.Name, "Calls to a ParamArray procedure must use positional arguments")
			}
		} else {
			position++
			if paramArray >= 0 && index >= paramArray {
				index = paramArray
			}
		}
		if index >= count {
			binding.Mode = ArgumentInvalid
			c.callError(r, DiagnosticArgumentCount, arg, "Too many arguments")
			continue
		}
		binding.Parameter, binding.ParameterIndex = parameters[index], index
		if index == paramArray {
			binding.ParamArrayIndex, packed = packed, packed+1
		} else if used[index] {
			binding.Mode = ArgumentInvalid
			c.callError(r, DiagnosticDuplicateArgument, arg, "More than one argument supplies this parameter")
			continue
		} else {
			used[index] = true
		}
		if binding.Expression.Kind == ast.KindOmittedExpression {
			if index == paramArray {
				binding.Mode = ArgumentMissing
			} else if parameters[index].Modifiers&ast.ParameterOptional != 0 {
				c.defaultArgument(binding)
			} else {
				binding.Mode = ArgumentInvalid
				c.callError(r, DiagnosticArgumentCount, arg, "A required argument cannot be omitted")
			}
			continue
		}
		c.bindArgument(r, binding, false)
	}
	for i, p := range parameters[:count] {
		if used[i] || i == paramArray {
			continue
		}
		binding := &ArgumentBinding{Parameter: p, ParameterIndex: i, SourceIndex: -1, ParamArrayIndex: -1}
		r.Arguments = append(r.Arguments, binding)
		if p.Modifiers&ast.ParameterOptional == 0 {
			binding.Mode = ArgumentInvalid
			c.callError(r, DiagnosticArgumentCount, site, fmt.Sprintf("Argument for %q is required", p.Name))
		} else {
			c.defaultArgument(binding)
		}
	}
	if setter && len(parameters) != 0 && rhs != nil {
		binding := &ArgumentBinding{Argument: rhs, Expression: rhs, Parameter: parameters[count], ParameterIndex: count, SourceIndex: -1, ParamArrayIndex: -1}
		r.Arguments = append(r.Arguments, binding)
		// A setter's RHS is assigned to its value parameter, not aliased to it.
		c.bindArgument(r, binding, true)
	}
	c.finishLibraryCall(r)
	return r
}

func (c *Checker) callError(r *CallResolution, code DiagnosticCode, node *ast.Node, message string) {
	r.Valid = false
	if !c.boundErrors[node] {
		c.report(code, node, message)
	}
}

func (c *Checker) defaultArgument(b *ArgumentBinding) {
	b.Default, b.Mode = b.Parameter.Default, ArgumentDefault
	if b.Default == nil && b.Parameter.Type.Kind == TypeVariant {
		b.Mode = ArgumentMissing
	}
	if b.Default != nil {
		from, to := c.typeOfNode(b.Default), b.Parameter.Type
		b.Conversion = &Conversion{Kind: conversionKind(from, to, object(to.Kind)), From: from, To: to}
	}
}

func (c *Checker) bindArgument(r *CallResolution, b *ArgumentBinding, valueParameter bool) {
	expr := b.Expression
	forced := b.ForcedValue
	if expr.Kind == ast.KindPrefixUnaryExpression && expr.AsPrefixUnaryExpression().Operator == ast.KindBYVAL {
		// Call-site ByVal is external-procedure syntax. Internal calls must not
		// silently reinterpret it as an ordinary value argument.
		forced, expr = true, expr.AsPrefixUnaryExpression().Operand
		c.callError(r, DiagnosticInvalidArgument, b.Expression, "Call-site ByVal requires an external procedure")
	}
	b.ForcedValue = forced || expr.Kind == ast.KindParenthesizedExpression
	from, to := c.intrinsic[TypeString], b.Parameter.Type
	if b.StringValue != nil {
		from = c.intrinsic[TypeString]
	} else {
		from = c.typeOfNode(expr)
	}
	if b.ParamArrayIndex >= 0 {
		to = c.intrinsic[TypeVariant]
	}
	set := object(to.Kind) || to.Kind == TypeVariant && object(from.Kind)
	b.Conversion = &Conversion{Kind: conversionKind(from, to, set), From: from, To: to}
	if b.StringValue == nil {
		b.Storage = c.storageOf(expr)
	} else {
		b.Storage = &Storage{Node: expr, Kind: StorageValue, Type: from}
	}
	if dependency := c.calls[expr]; dependency != nil {
		r.Valid = r.Valid && dependency.Valid
		r.Deferred = r.Deferred || dependency.Deferred
	}
	if from.Kind == TypeClass && !object(to.Kind) && to.Kind != TypeVariant && to.Kind != TypeArray {
		from = c.simpleType(expr)
		b.Storage = &Storage{Node: expr, Kind: StorageValue, Type: from}
		b.Conversion = &Conversion{Kind: conversionKind(from, to, false), From: from, To: to}
	}
	if from.Kind == TypeError || to.Kind == TypeError {
		b.Mode, r.Valid = ArgumentInvalid, false
		return
	}
	if from.Kind == TypeUnknown || to.Kind == TypeUnknown {
		b.Mode, r.Deferred = ArgumentDeferred, true
		return
	}
	if fixedStringStorage(b.Storage) {
		b.Mode, r.Deferred = ArgumentDeferred, true
		return
	}
	byref := b.Parameter.IsByRef() || b.ParamArrayIndex >= 0
	if to.Kind == TypeArray && !byref {
		b.Mode = ArgumentInvalid
		c.callError(r, DiagnosticByRefType, b.Argument, "A declared array parameter must be ByRef")
		return
	}
	if from.Kind == TypeStructure && to.Kind == TypeVariant || to.Kind == TypeStructure && !b.Parameter.IsByRef() {
		b.Mode = ArgumentInvalid
		c.callError(r, DiagnosticArgumentType, b.Argument, "A user-defined type requires a matching ByRef parameter")
		return
	}
	if !valueParameter && byref && !forced && b.Storage.Addressable && b.StringValue == nil {
		switch {
		case to.Kind == TypeVariant:
			b.Mode, b.PreserveStorageType = ArgumentReference, from.Kind != TypeVariant
			b.Conversion.Kind = ConversionIdentity
		case sameType(from, to):
			b.Mode = ArgumentReference
			b.Conversion.Kind = ConversionIdentity
		case object(from.Kind) && object(to.Kind):
			b.Mode = ArgumentCopyBack
			b.CopyBack = &Conversion{Kind: ConversionSet, From: to, To: from}
		case from.Kind == TypeVariant && object(to.Kind):
			b.Mode = ArgumentInvalid
			c.callError(r, DiagnosticByRefType, b.Argument, "An object ByRef parameter requires object storage; parenthesize the argument to pass a value")
		default:
			b.Mode = ArgumentInvalid
			c.callError(r, DiagnosticByRefType, b.Argument, "ByRef argument storage has a different declared type; parenthesize it to allow coercion")
		}
		return
	}
	if to.Kind == TypeArray || from.Kind == TypeArray && to.Kind != TypeVariant && !(to.Kind == TypeString && from.ElementType.Kind == TypeByte) {
		if sameType(from, to) {
			b.Mode = ArgumentTemporary
			return
		}
		b.Mode = ArgumentInvalid
		c.callError(r, DiagnosticArgumentType, b.Argument, "An array parameter requires compatible array storage")
		return
	}
	if b.Conversion.Kind == ConversionInvalid {
		b.Mode = ArgumentInvalid
		c.callError(r, DiagnosticArgumentType, b.Argument, "Argument cannot be converted to the parameter type")
		return
	}
	if b.Conversion.Kind == ConversionDeferred {
		b.Mode, r.Deferred = ArgumentDeferred, true
		return
	}
	b.Mode = ArgumentValue
	if byref {
		b.Mode = ArgumentTemporary
	}
}

func fixedStringStorage(s *Storage) bool {
	symbol := s.Symbol
	if symbol == nil && s.Resolution != nil && (s.Resolution.Kind == CallArrayIndex || s.Resolution.Kind == CallArrayReference) {
		symbol = s.Resolution.Target.Symbol
	}
	if symbol == nil || symbol.ValueDeclaration == nil || (symbol.ValueDeclaration.Kind != ast.KindVariableDeclaration && symbol.ValueDeclaration.Kind != ast.KindFieldDeclaration) {
		return false
	}
	annotation := symbol.ValueDeclaration.AsVariableDeclaration().Type
	return annotation != nil && annotation.Kind == ast.KindTypeReference && annotation.AsTypeReference().Length != nil
}

func sameType(a, b *Type) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == TypeClass || a.Kind == TypeStructure {
		return a.Symbol == b.Symbol
	}
	if a.Kind == TypeArray {
		return sameType(a.ElementType, b.ElementType) && (a.Rank == 0 || b.Rank == 0 || a.Rank == b.Rank)
	}
	return true
}

func (c *Checker) indexCall(node, target *ast.Node, t *Type, args []*ast.Node) *Type {
	r := &CallResolution{Kind: CallArrayIndex, Node: node, Target: target, Receiver: target, ResultType: t.ElementType, Valid: true}
	if target.Kind == ast.KindPropertyAccessExpression {
		r.WithTarget = target.AsPropertyAccessExpression().WithTarget
	}
	c.calls[node] = r
	if len(args) == 0 {
		// a() denotes the whole array, not an element at zero indices.
		r.Kind = CallArrayReference
		r.ResultType = t
		return t
	}
	if t.Rank != 0 && len(args) != t.Rank {
		c.callError(r, DiagnosticArgumentCount, node, "The number of indices does not match the array rank")
	}
	for i, arg := range args {
		binding := &ArgumentBinding{Argument: arg, Expression: arg, ParameterIndex: -1, SourceIndex: i, ParamArrayIndex: -1, Mode: ArgumentValue}
		r.Arguments = append(r.Arguments, binding)
		if arg.Kind == ast.KindNamedArgument || arg.Kind == ast.KindOmittedExpression {
			binding.Mode = ArgumentInvalid
			c.callError(r, DiagnosticInvalidArgument, arg, "Array indices must be positional and cannot be omitted")
			continue
		}
		from, to := c.simpleType(arg), c.intrinsic[TypeLong]
		kind := conversionKind(from, to, false)
		binding.Conversion = &Conversion{Kind: kind, From: from, To: to}
		if kind == ConversionInvalid {
			binding.Mode, r.Valid = ArgumentInvalid, false
			if from.Kind != TypeError {
				c.callError(r, DiagnosticArgumentType, arg, "Array index must be convertible to Long")
			}
		} else if kind == ConversionDeferred {
			binding.Mode, r.Deferred = ArgumentDeferred, true
		}
	}
	return r.ResultType
}
