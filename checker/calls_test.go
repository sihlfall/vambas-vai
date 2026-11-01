package checker_test

import (
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func procedureBody(p *compiler.Program, module, name string) []*ast.Node {
	return p.Module(module).Locals.Lookup(name).ValueDeclaration.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
}

func statementCall(statement *ast.Node) *ast.Node { return statement.AsCallStatement().Expression }

func TestCallArgumentMappingDefaultsAndNamedSymbols(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(n As Long)
Call Target(n, last:="named", middle:=2)
Call Target(n, , "last")
Call Target(n)
End Sub
Sub Target(ByRef first As Long, Optional ByVal middle As Long = 42, Optional ByVal last As String = "default")
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := procedureBody(p, "Main", "Run")
	r := c.GetCallResolution(statementCall(body[0]))
	if r == nil || r.Kind != checker.CallProcedure || !r.Valid || r.Deferred || len(r.Arguments) != 3 {
		t.Fatal("call was not completely resolved")
	}
	for i, want := range []int{0, 2, 1} {
		b := r.Arguments[i]
		if b.SourceIndex != i || b.ParameterIndex != want || b.Parameter != r.Signature.Parameters[want] {
			t.Fatal("named arguments were reordered or incorrectly mapped")
		}
		if b.Argument.Kind == ast.KindNamedArgument && c.GetSymbolAtLocation(b.Argument.AsNamedArgument().Name) != b.Parameter.Symbol {
			t.Fatal("named label did not resolve to its parameter")
		}
		if b.Argument.Kind == ast.KindNamedArgument && c.GetStorageAtLocation(b.Argument.AsNamedArgument().Name).Addressable {
			t.Fatal("parameter label was mistaken for caller storage")
		}
	}
	if r.Arguments[0].Mode != checker.ArgumentReference || r.Arguments[1].Mode != checker.ArgumentValue {
		t.Fatal("declared passing modes were lost")
	}
	second := c.GetCallResolution(statementCall(body[1]))
	if second.Arguments[1].Mode != checker.ArgumentDefault || second.Arguments[1].Default != second.Signature.Parameters[1].Default {
		t.Fatal("omitted middle argument lost its default")
	}
	third := c.GetCallResolution(statementCall(body[2]))
	if len(third.Arguments) != 3 || third.Arguments[1].SourceIndex != -1 || third.Arguments[2].Mode != checker.ArgumentDefault {
		t.Fatal("unmapped optional parameters were not supplied")
	}
}

func TestCallArgumentErrors(t *testing.T) {
	for _, test := range []struct {
		call string
		code checker.DiagnosticCode
	}{
		{"Call Target()", checker.DiagnosticArgumentCount},
		{"Call Target(,)", checker.DiagnosticArgumentCount},
		{"Call Target(1, 2, 3)", checker.DiagnosticArgumentCount},
		{"Call Target(1, first:=2)", checker.DiagnosticDuplicateArgument},
		{"Call Target(1, unknown:=2)", checker.DiagnosticUnknownArgument},
	} {
		t.Run(test.call, func(t *testing.T) {
			p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run()\n" + test.call + "\nEnd Sub\nSub Target(ByVal first As Long, Optional second As Long)\nEnd Sub"})
			c := p.GetTypeChecker()
			found := false
			for _, d := range c.GetDiagnostics(nil) {
				if d.Code == test.code {
					found = true
				}
			}
			if !found || c.GetCallResolution(statementCall(procedureBody(p, "Main", "Run")[0])).Valid {
				t.Fatalf("invalid call was accepted: %+v", c.GetDiagnostics(nil))
			}
		})
	}
}

func TestImplicitParenthesesRetainVbaPassingSemantics(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(n As Long)
Target n
Target(n)
Call Target(n)
Target(n, 1)
End Sub
Sub Target(ByRef value As Long, Optional ByVal other As Long)
End Sub`})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticInvalidArgument {
		t.Fatalf("implicit call syntax diagnostics: %+v", ds)
	}
	body := procedureBody(p, "Main", "Run")
	for i, want := range []checker.ArgumentMode{checker.ArgumentReference, checker.ArgumentTemporary, checker.ArgumentReference} {
		if got := c.GetCallResolution(statementCall(body[i])).Arguments[0]; got.Mode != want {
			t.Fatalf("call %d: mode %v, want %v", i, got.Mode, want)
		}
	}
}

func TestDefaultProcedureStatementCalls(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(command As Command)
command 1
Call command(2)
End Sub`},
		compiler.SourceFileInput{FileName: "Command.cls", Text: `Public Sub Execute(ByVal value As Long)
Attribute Execute.VB_UserMemId = 0
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	for _, statement := range procedureBody(p, "Main", "Run") {
		r := c.GetCallResolution(statementCall(statement))
		if r.Kind != checker.CallProcedure || r.DefaultMember != p.Module("Command").Locals.Lookup("Execute") || r.ResultType.Kind != checker.TypeVoid {
			t.Fatal("default Sub statement call was not resolved")
		}
	}
}

func TestByRefStorageAndParenthesizedTemporaries(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(n As Long, i As Integer, values() As Long)
Call Target(n)
Call Target((i))
Target (i)
Call Target(1 + 2)
Call Target(values(0))
Call Target(Value)
Call Target(i)
End Sub
Sub Target(ByRef value As Long)
End Sub
Property Get Value() As Long
Value = 1
End Property`})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticByRefType {
		t.Fatalf("unexpected ByRef diagnostics: %+v", ds)
	}
	body := procedureBody(p, "Main", "Run")
	for i, want := range []checker.ArgumentMode{checker.ArgumentReference, checker.ArgumentTemporary, checker.ArgumentTemporary, checker.ArgumentTemporary, checker.ArgumentReference, checker.ArgumentTemporary, checker.ArgumentInvalid} {
		b := c.GetCallResolution(statementCall(body[i])).Arguments[0]
		if b.Mode != want {
			t.Errorf("call %d: got mode %v, want %v", i, b.Mode, want)
		}
		if want == checker.ArgumentTemporary && b.CopyBack != nil {
			t.Fatal("a value/property/parenthesized argument acquired copy-back")
		}
	}
	indexed := c.GetCallResolution(statementCall(body[4])).Arguments[0].Storage
	if indexed.Kind != checker.StorageArrayElement || !indexed.Addressable {
		t.Fatal("array element did not retain addressable storage")
	}
	property := c.GetCallResolution(statementCall(body[5])).Arguments[0].Storage
	if property.Kind != checker.StorageProperty || property.Addressable {
		t.Fatal("property result was passed as its own storage")
	}
}

func TestVariantReferenceViewsAndObjectCopyBack(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(n As Long, item As Box, obj As Object, v As Variant)
Call AnyValue(n)
Call ObjectValue(item)
Call BoxValue(obj)
Call BoxValue(v)
Call BoxValue((v))
End Sub
Sub AnyValue(ByRef arg As Variant)
End Sub
Sub ObjectValue(ByRef arg As Object)
End Sub
Sub BoxValue(ByRef arg As Box)
End Sub`},
		compiler.SourceFileInput{FileName: "Box.cls", Text: ""})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticByRefType {
		t.Fatalf("object passing diagnostics: %+v", ds)
	}
	body := procedureBody(p, "Main", "Run")
	first := c.GetCallResolution(statementCall(body[0])).Arguments[0]
	if first.Mode != checker.ArgumentReference || !first.PreserveStorageType || first.Storage.Type.Kind != checker.TypeLong {
		t.Fatal("Variant ByRef view lost the original storage constraints")
	}
	for _, i := range []int{1, 2} {
		b := c.GetCallResolution(statementCall(body[i])).Arguments[0]
		if b.Mode != checker.ArgumentCopyBack || b.CopyBack == nil || b.CopyBack.Kind != checker.ConversionSet {
			t.Fatal("object type adaptation did not retain normal-return copy-back")
		}
	}
	if c.GetCallResolution(statementCall(body[4])).Arguments[0].Mode != checker.ArgumentTemporary {
		t.Fatal("parenthesized Variant object argument did not use a temporary")
	}
}

func TestParamArrayPackingOmissionsAndNamedRejection(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(n As Long)
Call Pack(1, n, , "last")
Call Pack(1)
Call Pack(first:=1)
Call OptionalValue()
End Sub
Sub Pack(ByVal first As Long, ParamArray rest() As Variant)
End Sub
Sub OptionalValue(Optional value As Variant)
End Sub`})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticInvalidArgument {
		t.Fatalf("ParamArray diagnostics: %+v", ds)
	}
	body := procedureBody(p, "Main", "Run")
	r := c.GetCallResolution(statementCall(body[0]))
	if r.ParamArray == nil || len(r.Arguments) != 4 || r.Arguments[1].ParamArrayIndex != 0 || r.Arguments[2].Mode != checker.ArgumentMissing || r.Arguments[3].ParamArrayIndex != 2 {
		t.Fatal("ParamArray packing lost order/omissions")
	}
	if r.Arguments[1].Mode != checker.ArgumentReference || !r.Arguments[1].PreserveStorageType {
		t.Fatal("ParamArray's Variant reference view was erased")
	}
	empty := c.GetCallResolution(statementCall(body[1]))
	if empty.ParamArray == nil || len(empty.Arguments) != 1 || !empty.Valid {
		t.Fatal("empty ParamArray was treated as a missing required argument")
	}
	if c.GetCallResolution(statementCall(body[3])).Arguments[0].Mode != checker.ArgumentMissing {
		t.Fatal("unspecified Optional Variant did not retain Missing")
	}
}

func TestWholeArraysAndArrayArgumentCompatibility(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run()
Dim values(1 To 3, 1 To 4) As Long
Dim wrong() As Integer
Call Target(values())
Call Target(wrong)
End Sub
Sub Target(ByRef items() As Long)
End Sub`})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticByRefType {
		t.Fatalf("array argument diagnostics: %+v", ds)
	}
	body := procedureBody(p, "Main", "Run")
	r := c.GetCallResolution(statementCall(body[2]))
	if r.Arguments[0].Mode != checker.ArgumentReference || r.Arguments[0].Storage.Type.Kind != checker.TypeArray || r.Arguments[0].Storage.Resolution.Kind != checker.CallArrayReference {
		t.Fatal("whole-array designator was indexed or copied")
	}
}

func TestIndexingResolutionAndInvalidScalarApplications(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long)
Dim values(1 To 3, 1 To 4) As Long
values(1, 2) = 3
result = values(1)
result = values(, 2)
result = values(row:=1, col:=2)
result = result(0)
End Sub`})
	c := p.GetTypeChecker()
	ds := c.GetDiagnostics(nil)
	counts := make(map[checker.DiagnosticCode]int)
	for _, d := range ds {
		counts[d.Code]++
	}
	if counts[checker.DiagnosticArgumentCount] != 1 || counts[checker.DiagnosticInvalidArgument] != 3 || counts[checker.DiagnosticNotCallable] != 1 {
		t.Fatalf("index diagnostics: %+v", ds)
	}
	left := procedureBody(p, "Main", "Run")[1].AsLetStatement().LExpression
	if r := c.GetCallResolution(left); r.Kind != checker.CallArrayIndex || !r.Valid || r.Signature != nil {
		t.Fatal("array write was confused with a procedure call")
	}
	if storage := c.GetStorageAtLocation(left); storage.Kind != checker.StorageArrayElement || len(storage.Indices) != 2 || !storage.Writable {
		t.Fatal("indexed write storage was not retained")
	}
}

func TestDefaultMemberCallsDictionaryAccessAndSetters(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(bag As Bag, result As Long)
result = bag("a")
bag("a") = result
result = bag!Field
bag!Field = result
With bag
result = !Other
End With
Call Take(bag("a"))
End Sub
Sub Take(ByRef n As Long)
End Sub`},
		compiler.SourceFileInput{FileName: "Bag.cls", Text: `Property Get Item(ByVal key As String) As Long
Attribute Item.VB_UserMemId = 0
Item = 1
End Property
Property Let Item(ByVal key As String, ByVal rhs As Long)
End Property`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := procedureBody(p, "Main", "Run")
	for i, want := range []checker.CallKind{checker.CallPropertyGet, checker.CallPropertyLet, checker.CallPropertyGet, checker.CallPropertyLet} {
		node := body[i].AsLetStatement().RExpression
		if i%2 == 1 {
			node = body[i].AsLetStatement().LExpression
		}
		r := c.GetCallResolution(node)
		if r == nil || r.Kind != want || r.DefaultMember != p.Module("Bag").Locals.Lookup("Item") || !r.Valid {
			t.Fatalf("default operation %d was not resolved", i)
		}
		if i >= 2 && (r.Arguments[0].StringValue == nil || *r.Arguments[0].StringValue != "Field" || c.GetSymbolAtLocation(node.AsPropertyAccessExpression().Name) != nil) {
			t.Fatal("dictionary access was treated as named-member lookup")
		}
	}
	with := body[4].AsWithStatement().Statement.AsBlock().Statements.Nodes[0].AsLetStatement().RExpression
	if r := c.GetCallResolution(with); r.Receiver != body[4].AsWithStatement().Expression || *r.Arguments[0].StringValue != "Other" {
		t.Fatal("dictionary access lost its captured With receiver")
	}
	argument := c.GetCallResolution(statementCall(body[5])).Arguments[0]
	if argument.Mode != checker.ArgumentTemporary || argument.CopyBack != nil {
		t.Fatal("default getter result acquired property copy-back")
	}
}

func TestDefaultValueConversionsAndDefaultLvalues(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(box As Box, result As Long)
result = box
box = result
result = box + 1
Call Take(box)
Set box = New Box
End Sub
Sub Take(ByRef n As Long)
End Sub`},
		compiler.SourceFileInput{FileName: "Box.cls", Text: `Property Get Value() As Long
Attribute Value.VB_UserMemId = 0
Value = 1
End Property
Property Let Value(ByVal rhs As Long)
End Property`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := procedureBody(p, "Main", "Run")
	rhs := body[0].AsLetStatement().RExpression
	if c.GetTypeAtLocation(rhs).Kind != checker.TypeClass || len(c.GetValueResolutions(rhs)) != 1 || c.GetAssignmentConversion(body[0]).From.Kind != checker.TypeLong {
		t.Fatal("implicit default value replaced the object reference's syntax type")
	}
	left := body[1].AsLetStatement().LExpression
	if c.GetStorageAtLocation(left).Kind != checker.StorageProperty || c.GetCallResolution(left).Kind != checker.CallPropertyLet {
		t.Fatal("default lvalue was treated as object variable storage")
	}
	if c.GetCallResolution(statementCall(body[3])).Arguments[0].Mode != checker.ArgumentTemporary {
		t.Fatal("default value passed to ByRef did not use temporary storage")
	}
	if c.GetStorageAtLocation(body[4].AsSetStatement().LExpression).Kind != checker.StorageVariable {
		t.Fatal("Set accidentally invoked a default member")
	}
}

func TestParameterlessProcedureResultConsumesFollowingArgumentList(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long)
result = Values(0)
result = Values()(0)
End Sub
Function Values() As Long()
End Function`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := procedureBody(p, "Main", "Run")
	r := c.GetCallResolution(body[0].AsLetStatement().RExpression)
	if r.Kind != checker.CallArrayIndex || r.Prefix == nil || r.Prefix.Kind != checker.CallProcedure || r.ResultType.Kind != checker.TypeLong {
		t.Fatal("parameterless function result was not indexed using the unconsumed argument list")
	}
	if c.GetCallResolution(body[1].AsLetStatement().RExpression).Prefix != nil {
		t.Fatal("explicit nested applications acquired an extra invocation")
	}
}

func TestWritesThroughFunctionResultsUseResultStorage(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run()
Values()(0) = 1
Values(0) = 2
GetBox() = 3
Scalar() = 4
End Sub
Function Values() As Long()
End Function
Function GetBox() As Box
End Function
Function Scalar() As Long
End Function`},
		compiler.SourceFileInput{FileName: "Box.cls", Text: `Property Get Value() As Long
Attribute Value.VB_UserMemId = 0
End Property
Property Let Value(ByVal rhs As Long)
End Property`})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticInvalidStorage {
		t.Fatalf("function-result storage diagnostics: %+v", ds)
	}
	body := procedureBody(p, "Main", "Run")
	for _, i := range []int{0, 1} {
		if c.GetStorageAtLocation(body[i].AsLetStatement().LExpression).Kind != checker.StorageArrayElement {
			t.Fatal("write did not index the returned array")
		}
	}
	property := c.GetCallResolution(body[2].AsLetStatement().LExpression)
	if property.Kind != checker.CallPropertyLet || property.Prefix == nil || property.Prefix.Kind != checker.CallProcedure {
		t.Fatal("function was not invoked before assigning its result's default member")
	}
}

func TestInvalidMissingAndCircularDefaults(t *testing.T) {
	for _, member := range []string{
		"", `Private Property Get Value() As Long
Attribute Value.VB_UserMemId = 0
End Property`, `Property Get Value() As Box
Attribute Value.VB_UserMemId = 0
End Property`,
	} {
		p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run(box As Box, n As Long)\nn = box\nEnd Sub"}, compiler.SourceFileInput{FileName: "Box.cls", Text: member})
		if ds := p.GetSemanticDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticDefaultMember {
			t.Fatalf("invalid default was not diagnosed: %+v", ds)
		}
	}
}

func TestResolutionQueriesAreConcurrentAndLeaveSyntaxUntouched(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run(n As Long)\nCall Target(value:=n)\nEnd Sub\nSub Target(value As Long)\nEnd Sub"})
	call := statementCall(procedureBody(p, "Main", "Run")[0])
	label := call.AsCallExpression().Arguments.Nodes[0].AsNamedArgument().Name
	if label.Symbol != nil {
		t.Fatal("binder unexpectedly resolved a parameter label")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := p.GetTypeChecker()
			if r := c.GetCallResolution(call); r == nil || !r.Valid {
				t.Error("resolution unavailable")
			}
			c.GetStorageAtLocation(call)
			c.GetValueResolutions(call)
		}()
	}
	wg.Wait()
	if label.Symbol != nil || len(p.BindDiagnostics()) != 0 {
		t.Fatal("checker changed binding metadata")
	}
	noErrors(t, p.GetTypeChecker())
}
