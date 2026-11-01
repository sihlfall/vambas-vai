# Project-local Implements

Class modules can implement the default interfaces of other classes in the same
project. Given `IValue.cls` with `Public Function Read(ByVal amount As Long) As Long`,
`Worker.cls` supplies the implementation:

```vb
Implements IValue

Private Function IValue_Read(ByVal amount As Long) As Long
    IValue_Read = amount + 40
End Function
```

With `Dim value As IValue` and `Set value = New Worker`, `value.Read(2)` returns
42 through the prefixed handler. Multiple interfaces can define the same name
and select different handlers. Worker's own public `Read` remains a separate member.

The [runnable example](../examples/interfaces/Main.bas) returns 42:

```sh
go run ./cmd/vambas-compile -root examples/interfaces -entry Main.Main -o /tmp/interfaces.ll
clang-20 -O2 /tmp/interfaces.ll runtime/*.c -lm -o /tmp/interfaces
/tmp/interfaces
echo $? # 42
```

## Checking and calls

`Implements` resolves in type scope and can appear between module procedures.
Standard modules cannot implement interfaces. The checker diagnoses unknown or
external targets, duplicate/self implementation, overlapping interface prefixes,
and cycles in the Implements graph. Interface public member names cannot contain
underscores. Private and Friend declarations are excluded from the contract.

Every public method/accessor requires a corresponding handler of the same kind.
Parameter/result types retain nominal class and array constraints. Parameter
modes, Optional/ParamArray positions, and optional default values must match;
parameter names may differ. Calls bind names and defaults against the interface
declaration. The final Property Let/Set value parameter has ByVal semantics even
if its spelling omits ByVal or says ByRef.

Public scalar fields contribute Get/Let contracts for value fields, Get/Set for
Object/class fields, and Get/Let/Set for Variant fields. Interface field accesses
dispatch to property handlers without touching the interface class's payload.
Such fields use temporaries when passed ByRef. Ordinary concrete fields retain
their storage semantics. `New IValue` remains valid and executes the interface
class's own bodies or field accessors.

Explicit default-member calls, implicit scalar reads, and supported scalar library
conversions select the declared interface. Private handlers are absent from the
concrete class's late-bound member table. Erasing a reference into Object or an
object-valued Variant uses the concrete object's public invocation table.

Interface references work in typed parameters/results, globals, object fields,
Variants, Collections, and object arrays. Casts accept the interfaces explicitly
declared by the concrete class; implemented interfaces are not inherited.
Nothing casts succeed; member access on Nothing raises 91. Unsupported casts
raise 13 before replacing storage. Existing ByRef aliasing, normal-return
copy-back, error propagation, and cleanup policies apply.

## Private ABI and verification

Views retain the original object pointer. Casts and method resolution allocate
no wrappers or extra owners. `Is` preserves concrete/interface/Variant identity;
ordinary loads, calls, and assignments retain/release references through the
existing ownership path, including the concrete lifecycle hooks.

`ClassDefinition.Interfaces` maps requested interface identities and member IDs
to implementation procedures. `OpInterfaceCheck` proves non-Nothing interface
support; `OpInterfaceCall` retains the checked signature and all possible
implementation effects. The verifier checks mapping completeness, signatures,
member ownership, and receiver proofs. Interface checks do not authorize
`OpClassFieldRef` or direct concrete calls: these require an exact `OpClassCheck`.

The object header remains 24 bytes. The private `VbaObjectType` descriptor is now
64 bytes, adding an interface count and table pointer after the existing invoke
callback. Interface descriptors are 24 bytes and member entries are 16 bytes.
C static assertions and LLVM metadata agree on these layouts. Dispatch thunks
call resolved pointers with the checked native signature, preserving ByRef
addresses and owned results.

External type-library interfaces, COM QueryInterface, and Windows interface
pointers remain outside this private ABI. Public array/UDT fields in implemented
interfaces receive explicit diagnostics; supported array/UDT method signatures
retain their typed ABI. Host-module, event, and reference-cycle collection limits
remain as described in [CLASSES.md](CLASSES.md).

These rules follow Microsoft's [Implements statement](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/implements-statement),
[MS-VBAL 5.2.4.2 and 5.3.1.9](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=60),
and [setter value-parameter semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/d19bfeaa-cca6-43d8-9188-27355b8ee025).

## Validation

Checker cases cover contract diagnostics and immutable semantic queries. IR
mutation tests reject invalid mappings, signatures, effects, and receiver proofs.
Native cases exercise multiple interfaces, methods and property modes, public
fields, named/optional/ParamArray arguments, defaults, identity, arrays/Variants,
ByRef failures, error details, and termination. C probes independently check
allocation-free resolution, casts, and ownership at `-O0` and `-O2`.

The unchanged stdVBA `stdICallable.cls` has a reproducible execution probe:

```sh
python3 llvm/audits/stdvba/run-implements-probe.py --source ../stdvba/src
```

It compiles all four original declarations with a separate implementing class,
verifies LLVM, and executes all four contracts at `-O0` and `-O2`, expecting 42.
The [full audit](audits/stdvba/after-implements/README.md) records remaining module
blockers; compiling an interface alone does not establish utility API support.
