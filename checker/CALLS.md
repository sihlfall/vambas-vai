# Calls, default members, and storage

These checker-owned records provide a shared contract for language services and
future semantic IR. The parser's `CallExpression` still represents VBA calls,
indexing, and default-member applications. Checking resolves the operation without
rewriting syntax or changing binder symbols/flow.

```go
operation := checker.GetCallResolution(expression)
storage := checker.GetStorageAtLocation(expression)
valueCalls := checker.GetValueResolutions(expression)
```

Queries read completed maps and may run concurrently. Treat returned records as
immutable. `GetValueResolutions` returns a copied slice of immutable operation
records. Syntax equality and traversal do not follow semantic records.

## Operation records

`CallResolution.Kind` distinguishes a procedure, Property Get/Let/Set, array index,
whole-array reference, dynamic application, or invalid application. It retains the selected `Signature`,
`Target`, optional `Receiver`, result type, and argument bindings. An implicit
default-member operation also records `DefaultMember`. `WithTarget` identifies a
captured With receiver; its header must not be evaluated again for each access.

`Valid` records static compatibility, and `Deferred` indicates unresolved work.
Neither a valid call nor a matching signature proves runtime success. Known
argument errors still retain signatures and result types where possible, so
signature help works on incomplete calls. Scalar applications such as `n(0)` are
diagnosed; they do not receive a fabricated call signature.

Bare Function/Get reads are checked against an empty argument list. Explicit
applications match their own argument list once; a callee reference does not also
invoke the procedure. AddressOf retains a reference without invoking it.

Argument-list queues preserve a useful VBA distinction: if a parameterless
function returns an array, `Values(i)` first invokes Values and applies the
unconsumed list to its result. The final index resolution links that invocation
through `Prefix`. `Values()(i)` instead has two syntax nodes with their own
resolutions. Writes through returned arrays/default-member objects retain the
same distinction; a scalar function result is not writable storage.

## Parameter mapping and passing

`CallResolution.Arguments` retains arguments in source order, with unmapped
optional defaults appended in parameter order and a setter RHS last. Each
`ArgumentBinding` has a `Parameter`, `ParameterIndex`, original `Argument`, value
`Expression`, and `SourceIndex`. An implicit default or setter RHS has source
index -1. This mapping is **not an execution schedule**: lowering must implement
the specified evaluation order and partial effects separately.

[Library policies](library.go) run around ordinary argument binding and select
exceptions through the resolved catalog member ID. InStr's two-positional-argument
form starts at its String1 formal while preserving source order. Existing Len/LenB
and Mid/Mid$ result refinements leave the declaration's signature intact and retain
useful result information on invalid calls. A shadowing user procedure receives
none of these policies.

Named labels match case-insensitively and resolve through
`GetSymbolAtLocation(label)` to the formal parameter. Positional/named collisions,
unknown names, excess arguments, and omitted required arguments are diagnosed.
Missing Optional parameters retain their explicit `Default` syntax, an implicit
type default, or the `ArgumentMissing` marker for an Optional Variant without a
default. These are distinct from an explicit Empty or Null value.

`ParamArray` identifies the variadic formal even when there are no elements.
Packed bindings carry a zero-based `ParamArrayIndex`; omitted elements preserve
Missing. ParamArray calls require positional arguments. A setter's final value
formal is reserved for its RHS, so optional/ParamArray index parameters cannot
consume it.

| Argument mode | Meaning |
| --- | --- |
| `ArgumentValue` | Initialize a ByVal parameter with the recorded coercion. |
| `ArgumentReference` | Alias addressable caller storage. |
| `ArgumentTemporary` | Evaluate/coerce into temporary storage for a ByRef formal, with no copy-back. |
| `ArgumentCopyBack` | Adapt an object storage type through a temporary and Set-copy it back only on **normal return**. |
| `ArgumentDefault` | Supply the Optional parameter's explicit or type default. |
| `ArgumentMissing` | Preserve Missing for an Optional Variant or omitted ParamArray element. |
| `ArgumentDeferred` | Passing/storage semantics require further work. |
| `ArgumentInvalid` | The argument is incompatible or missing. |

Ordinary scalar ByRef variables require matching declared storage types.
Parentheses around the argument force value evaluation and therefore allow a
coercing temporary: `Call Take((integerVariable))` differs from
`Call Take(integerVariable)` when Take expects Long. Arithmetic expressions,
constants, function results, and property results also use value temporaries.
For a single implicit statement argument, adjacent `Take(n)` also treats its
parentheses as argument-value parentheses. `Call Take(n)` instead uses list
delimiters and can retain the storage alias. Parenthesized multi-argument lists
without Call are diagnosed once the target resolves to a procedure.
Properties do **not** gain VB.NET-style copy-back merely by being passed ByRef.

A Variant ByRef formal can view typed caller storage. `PreserveStorageType` keeps
the original storage constraint explicit, rather than pretending the caller's
variable becomes a Variant. ParamArray elements use these Variant views when the
argument is a variable. Object ByRef parameters of differing declared object
types record both input and normal-return copy-back conversions. A Variant
variable passed directly to a specific object ByRef formal is incompatible;
parenthesizing it instead permits a runtime object coercion into a temporary.

Call-site `ByVal expression` is reserved for external procedures. It is diagnosed
for local procedure calls; declaring a formal `ByVal` remains ordinary supported
syntax. External procedure declarations and ABI-specific argument overrides are
not implemented by this layer.

The rules follow [MS-VBAL argument processing](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/1fb9af32-fc48-4c4f-998a-ed8047048ca5),
the [VBA ByRef explanation](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/byref-argument-type-mismatch),
[argument lists](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/5b35d806-1305-4427-a120-d25a71c45c02),
and [ParamArray call restrictions](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/a-procedure-with-a-paramarray-argument-cannot-be-called-with-named-arguments).

## Default members and lvalues

For local classes, `Attribute Member.VB_UserMemId = 0` selects the default member.
The attribute can occur within its procedure or at module level. Merged accessors
share that identity. Default operations require the appropriate public Function,
Get, Let, or Set; statement calls may invoke a default Function/Sub rather than
read a default property. Missing, private, ambiguous, and circular default-value chains
produce diagnostics.

`bag(key)` selects the default getter, and assignments select the corresponding
Let/Set. `bag!Field` supplies the same default member with a String argument whose
value is `Field`; `ArgumentBinding.StringValue` records it without inserting a
synthetic string literal. The dictionary name remains syntax, not a member or
local-variable reference. This also works with a captured With receiver. See
[dictionary access](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/f20c9ebc-3365-4614-9788-1cd50a504574).
Statement-specific validation follows [MS-VBAL Call statements](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/f7c864a8-8fce-49dc-8347-30dc749d6576).

Scalar evaluation can invoke a parameterless default getter/function, including
chains returning another class. `GetValueResolutions(source)` exposes that chain
while the source node's type/symbol continues to describe the object reference.
A Let assignment to an object default member resolves its setter; Set assignment
to a normal object variable continues to refer to the variable itself.

`Storage.Kind` distinguishes values/constants, variables, array elements,
properties, dynamic targets, and invalid applications. It retains type, symbol,
receiver/captured With target, indices, and related operation. `Writable` and
`Addressable` are separate: a property can be writable without being directly
addressable. Function result variables remain variable storage.

Known array ranks are checked, and indices must be non-omitted positional values
convertible to Long. `a()` denotes the whole array and can alias compatible array
parameter storage. Ordinary indexing retains its receiver and index expressions;
runtime bounds and lifetimes still belong to later lowering/runtime work.

Dynamic Object/Variant members/applications keep a Variant result and deferred
resolution. No interface is invented. Fixed-length String passing, array value
temporaries, whole-array assignment ownership, full accessor/interface contracts,
and external library signatures remain deferred. The binder flow graph remains a
conservative binding graph; these semantic plans do not turn it into an executable
CFG or implement runtime mutations/copy-back.
