# Class modules and COM support plan

Status: Milestones 1 and 2 implemented for the native class subset documented in
[the class execution contract](llvm/CLASSES.md). Milestones 3–5 remain a roadmap;
VBA/VB6 host differential validation remains outstanding.

This plan combines executable `.cls` support with COM interoperability. It extends
the [compiler design](VBA_LLVM_Compiler_Design.md#10-external-interfaces-windows-com-and-linux-substitutes)
and records the compatibility direction agreed during planning.

## Objective and compatibility decisions

Compile ordinary VBA/VB6 class modules and preserve their observable object
behavior, including interactions with external libraries. Users should not need
to predict every way their classes might be used or add a new annotation to make
an otherwise valid VBA/VB6 class compatible.

- Design every compiled class for COM-compatible object behavior on Windows from
  the start: identity, lifetime, interface access, properties, and late binding.
- External visibility and activation follow the source/project metadata and the
  selected VBA or VB6 compatibility profile. COM interface support does not imply
  that every class is registered or externally creatable.
- Internal calls may use direct calls and the compiler's private ABI when doing
  so preserves observable behavior. Public COM calls use explicit ABI adapters.
- Do not classify private classes as incapable of external interaction merely
  from their visibility. Validate which interfaces and operations are permitted
  by the selected language profile.
- Linux preserves language semantics and supports deterministic substitutes for
  external dependencies. It does not need Windows COM activation or a COM emulator.
- Deliver support incrementally. Diagnose unsupported constructs or boundary
  interactions explicitly; never silently change their semantics.

VBA's `PublicNotCreatable` is an example of why visibility and activation are
separate: another project can use an instance supplied by its defining project
without being allowed to construct the class directly. See
[Microsoft's cross-project class example](https://learn.microsoft.com/en-us/previous-versions/office/troubleshoot/office-developer/set-up-vb-project-using-class).

## Current foundations and gaps

- `project/config.go` retains manifest references, but does not resolve type libraries.
- `compiler.ProgramOptions` currently accepts source modules without external references.
- The binder and checker already model local class types, `Me`, declared receivers,
  properties, default members, and argument/storage plans.
- Lowering executes ordinary class modules through class descriptors, fields,
  implicit receivers, and direct member calls. Host modules remain unsupported.
- `runtime/objects.h` and `runtime/objects.c` implement descriptor-based private
  reference-counted handles shared by compiled classes and built-in objects.
  These handles are not COM interface pointers.
- `CreateObject` currently supports only the virtual testing library.
- Compiled classes, Collection, and testing objects share native dynamic invocation;
  statically resolved calls retain direct or specialized lowering.
- Native object-valued Variants, object fields, lifecycle hooks, defaults, and
  lazy/predeclared instances execute. COM value marshaling remains separate work.
- `llvm/emit.go` rejects external effects. Windows-specific runtime code exists,
  but Windows execution and COM ABI compatibility have not been established.
- The current runtime assumes 64-bit pointers.

## Architecture

Keep language semantics in the shared frontend and semantic IR. Keep library
discovery and platform ABI details outside the binder.

```text
Source classes                 External library metadata
       |                                  |
       +------ Binder and checker --------+
                         |
                Typed semantic IR
                         |
          Object and invocation contracts
             /           |            \
    Compiled classes   Built-ins   External implementations
                                   /                 \
                         Windows COM adapters   Linux substitutes
```

Distinguish a class implementation, an interface, and an object instance.
Class descriptors should describe fields, members, supported interfaces, and
lifecycle behavior. Interface descriptions should retain stable identity and
member contracts. Platform adapters determine concrete COM layouts and calls.

Reserve the ability to expose the same compiled instance through multiple
interfaces while preserving object identity and a coherent lifetime. Do not
compare arbitrary interface-pointer addresses as object identity. Any adapter
or wrapper design must preserve identity across repeated boundary crossings.

Private runtime handles must never be stored in Automation fields that require
real interface pointers. Define conversion and ownership at every such boundary.

## Milestone 1: Native execution of ordinary class modules

Implement a small but complete class slice before Windows COM execution.

### Work

1. Add class identities, field descriptions, and member associations to semantic IR.
2. Declare all class layouts and member signatures before lowering bodies, allowing
   forward and mutually referring class declarations.
3. Give instance methods and property accessors an implicit receiver parameter.
   Resolve `Me` and unqualified instance-member access through that receiver.
4. Lower class-level variables to instance fields, distinct from standard-module
   globals. Preserve field storage identity for supported ByRef arguments.
5. Generalize hard-coded object kinds into descriptors with allocation, ownership,
   and cleanup operations. Retain compatibility with existing built-in objects.
6. Support `New`, `Set`, `Nothing`, `Is`, typed class parameters/results, and direct
   calls to instance methods.
7. Execute Property Get/Let/Set using the checker's existing call/storage plans.
   Properties remain calls; public fields expose actual storage.
8. Keep receivers alive through calls and borrowed field access. Clean up supported
   managed fields and partially constructed instances on failure.

### Acceptance example

`Counter.cls`:

```vb
Private count As Long

Public Sub Increment()
    count = count + 1
End Sub

Public Property Get Value() As Long
    Value = count
End Property
```

`Main.bas`:

```vb
Public Function Main() As Long
    Dim counter As Counter
    Set counter = New Counter
    counter.Increment
    Main = counter.Value
End Function
```

Done when the example compiles and returns 1, separate instances have independent
fields, aliases preserve identity, and normal/error cleanup passes ownership tests.
Unsupported field types or class features must receive explicit diagnostics.

### Implementation status

Implemented with an executable [Counter project](examples/classes/Main.bas),
verified class/receiver/field IR, descriptor-based allocation and cleanup, direct
methods and Property Get/Let/Set, typed parameters/results, and ByRef field storage.
Fields support the existing scalar, String, Variant, array, and UDT subsets.
Milestone 2 adds object-reference fields and the remaining native object contract.

Native tests cover independent instances, aliases, receiver/borrowed-field lifetime,
nominal casts, properties versus field aliases, nested managed payloads, and error
cleanup. Runtime allocation-failure tests cover partially initialized instances.
See [CLASSES.md](llvm/CLASSES.md) for the supported slice and remaining diagnostics.

## Milestone 2: Complete the ordinary object execution contract

Build on the first slice without requiring Windows COM infrastructure.

- Add `Class_Initialize` and `Class_Terminate` semantics, with explicit policies
  validated against VBA/VB6 for initialization failure, termination, and reentrancy.
- Add lazy `As New`, predeclared instances, captured `With` receivers, and default members.
- Support object-valued Variants and the required conversions between typed class,
  `Object`, and Variant storage, including ByRef behavior.
- Generalize dynamic method/property invocation for `As Object` and Variant receivers.
  Preserve method/get/let/set distinctions, named/optional arguments, and Missing.
- Route compiled classes, built-ins, and substitutes through a shared invocation
  contract while retaining direct calls for statically resolved local members.
- Extend supported field/container types deliberately, including object references,
  and verify cleanup for nested managed values.
- Document reference-cycle behavior. Do not introduce a cycle collector that changes
  observable class termination behavior without a separate compatibility decision.

Done when representative class programs work through typed and dynamic references
and lifecycle tests cover success, failures, aliasing, and reentrant calls.

### Implementation status

Implemented with descriptor lifecycle hooks and shared invocation adapters for
compiled classes and existing built-in substitutes. Lazy locals/globals/fields,
predeclared roots, captured object With, default members, object-valued Variants,
constrained object ByRef views and normal-return adaptation, object fields, and
object references inside Variant arrays/Collections now execute.

Native tests cover named/optional/ParamArray arguments, Missing, accessor kinds,
Sub/function distinctions, receiver capture, initializer failure, exact error
preservation, resurrection, at-most-once termination, and reentrant replacement.
Allocation-failure tests include user initialization and managed child objects.
The runtime deliberately uses reference counting without a cycle collector.

The lifecycle policy follows MS-VBAL and is covered by native conformance tests;
no VBA/VB6 reference host was available for differential execution. Host-specific
initialization error wrapping and project-reset behavior remain compatibility
validation work. Typed object/class arrays now execute with nominal constraints,
reference ownership, and Variant/ByRef support; see [object arrays](llvm/OBJECT_ARRAYS.md).
Dynamic UDT value boundaries remain explicitly unsupported. See
[the lifecycle, invocation, and cycle policies](llvm/CLASSES.md).

## Milestone 3: External reference metadata and editor support

This frontend work can proceed independently of native class execution. It is not
a prerequisite for the first local-class example.

### Metadata model and importing

Introduce a platform-independent reference package containing:

- Library GUID, version, locale, name, dependencies, and provenance.
- Interfaces, coclasses, default/source interfaces, aliases, enums, and constants.
- Member names, DISPIDs, invocation kinds, and signatures.
- Parameter names, order, types, passing modes, defaults, and optional flags.
- ABI metadata needed later, including interface IDs, calling conventions, vtable
  information, and distinctions such as HRESULT versus logical return values.

Start with a Windows extractor using type-library APIs and a versioned portable
metadata format. Use `LoadTypeLibEx` with `REGKIND_NONE` where appropriate to avoid
registration during extraction. Pin library identity and content for reproducible
builds; include dependency and format changes in cache invalidation.

### Frontend integration

1. Resolve manifest references outside the binder and pass immutable descriptions
   into `compiler.ProgramOptions` before binding.
2. Preserve reference order and implement language-compatible lookup precedence,
   qualification, ambiguity handling, and missing-reference diagnostics.
3. Adapt imported declarations into ordinary symbol/signature lookup. Keep external
   identity separate from built-in intrinsic identity and preserve snapshot ownership.
4. Distinguish interfaces from creatable coclasses. Check construction, interface
   assignments, properties, defaults, and argument compatibility.
5. Expose completions, signature help, documentation, and navigable generated
   declarations in the language server.

Done when code using `Scripting.Dictionary` can be checked on Windows and Linux
from the same metadata, without activating the implementation, and invalid member
calls receive useful diagnostics.

## Milestone 4: External invocation IR and Windows COM consumption

### Shared execution contract

- Add explicit external activation/invocation plans and IR operations with interface
  identity, member identity or dynamic name, invocation kind, and argument bindings.
- Model external effects, ByRef writes, fallible conversions, and ownership. Replace
  the backend's blanket rejection only for implemented external operations.
- Evaluate VBA expressions in the required order before arranging COM arguments.
  Preserve completed side effects and writes if later evaluation or invocation fails.
- Exercise the contract through deterministic Linux substitutes using the same
  interface identities, without redefining VBA value or error semantics.

### Windows adapter

1. Establish a Windows x64 build/test baseline and validate runtime layouts against
   SDK types, including supported BSTR, VARIANT, and SAFEARRAY ownership paths.
2. Define apartment initialization, thread ownership, message pumping, reentrancy,
   and shutdown. Keep interface pointers within their permitted apartment context.
3. Implement activation for supported `CreateObject` and typed `New` cases.
4. Implement `IDispatch` invocation: name resolution, methods, property get/put/putref,
   named arguments, omitted arguments, defaults, result conversion, and ByRef values.
5. Initially permit typed dispatch-capable interfaces to use metadata-resolved
   dispatch calls. Diagnose interfaces that require unimplemented vtable invocation.
6. Map HRESULT, EXCEPINFO, and applicable IErrorInfo data into the existing VBA error
   machinery, including `Err`, `On Error`, and `Resume` behavior.
7. Support passing compiled objects across the boundary for the implemented interface
   subset. Supply real COM interfaces with stable identity and lifetime, rather than
   limiting the adapter permanently to receiving external objects.

### Acceptance examples

```vb
Public Function Main() As Long
    Dim dictionary As Object
    Set dictionary = CreateObject("Scripting.Dictionary")
    dictionary.Add "answer", 42
    Main = dictionary.Item("answer")
End Function
```

Also execute the typed equivalent using `As Scripting.Dictionary` and
`New Scripting.Dictionary`. A controlled COM fixture must additionally accept,
retain, return, and invoke a compiled class object through the supported interface
contract, preserving identity and cleanup.

Done when these programs run on Windows and deliberate failures reach VBA error
handlers with correct error information, surviving ByRef writes, and no leaks.

## Milestone 5: Broader interfaces, events, and COM deployment

Deliver these as separately tested increments:

- COM enumeration for `For Each`, including enumerator ownership and exit/error cleanup.
- Additional Automation values and array forms: Date, Currency, Decimal, and others
  required by the selected compatibility targets.
- Direct vtable calls for dual/custom interfaces, with target-specific ABI validation.
- `Implements` and complete interface-contract checking for compiled classes.
- `GetObject`, running-object lookup, and additional supported activation forms.
- Events and `WithEvents`, including connection points, sink ownership, disconnection,
  and synchronous reentrancy.
- Public type-library generation, class factories, registration or manifest deployment,
  and the chosen VBA/VB6 exposure and instancing profiles.
- Separate `AddressOf` callback adapters from COM event/interface callbacks. Neither
  may allow private error propagation to unwind through foreign frames.

Ordinary `.cls` support does not automatically supply Office document modules,
UserForms, ActiveX controls, designer state, or host globals. Plan those host services
as explicit additional capabilities.

## Architecture and compatibility constraints

### Bitness

Windows x64 is the initial proposed execution target because the current runtime
assumes 64-bit pointers. It is not sufficient for loading existing 32-bit VB6 DLLs
in-process. If those DLLs are an early acceptance requirement, bring forward an
x86 backend/runtime or a deliberately designed out-of-process bridge. A bridge
has marshaling and deployment requirements and is not a transparent universal fix.

### Compatibility evidence

Use the VBA/VB6 specifications, reference-host experiments, and representative
programs to resolve details. Do not infer compatibility solely from successful
Linux execution or from Automation-shaped memory layouts. In particular, class
visibility, interface exposure, lifecycle errors, and callbacks need explicit
conformance cases for each selected profile.

## Validation strategy

- **Frontend:** reference precedence, missing libraries, snapshot isolation, class
  visibility, interface compatibility, properties, optional arguments, and defaults.
- **Native classes:** independent instances, aliases, field ByRef access, receiver
  lifetime, managed fields, construction failure, termination, and dynamic calls.
- **IR/backend:** verification of receiver/field operations, external effects,
  evaluation order, resource scopes, and error edges; execution at O0 and O2.
- **Linux substitutes:** deterministic calls and failures exercising the shared
  contracts without Windows activation.
- **Windows COM fixture:** a small controlled server with properties, named/optional
  parameters, ByRef mutation, arrays, object round trips, deliberate errors, and
  lifetime counters. Add reentrancy and events as their milestones arrive.
- **Real interoperability:** Dictionary smoke tests, followed by selected existing
  libraries and VBA/VB6 differential tests. Do not make Office installation a
  prerequisite for the core test suite.
- **Regression:** retain the existing built-in Collection/testing behavior and run
  the repository's appropriate frontend, lowering, runtime, and full checks.

## Recommended implementation order

1. Class descriptors, instance fields, implicit receivers, and a verified IR contract.
2. Native construction, methods, properties, and cleanup: the Counter acceptance case.
3. Lifecycle, lazy construction, dynamic invocation, and object-valued Variants.
4. Portable reference metadata and frontend integration, independently schedulable
   alongside the preceding class work.
5. External-call IR, Linux substitutes, and a Windows execution/fixture baseline.
6. Windows Automation consumption and supported compiled-object round trips.
7. Broader interfaces, events, host capabilities, and externally creatable servers.

The first implementation PR should establish the class/instance IR contract and
the smallest executable class slice that can be reviewed and tested coherently.
General COM execution is a later delivery, but COM-compatible identity, lifetime,
and interface access constrain that initial design.

## References

- [Existing compiler architecture](VBA_LLVM_Compiler_Design.md)
- [Checker call and storage plans](checker/CALLS.md)
- [Current Collection object contract](llvm/COLLECTION.md)
- [Current Variant contract](llvm/VARIANT.md)
- [LoadTypeLibEx](https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-loadtypelibex)
- [IDispatch::Invoke](https://learn.microsoft.com/en-us/windows/win32/api/oaidl/nf-oaidl-idispatch-invoke)
- [COM initialization](https://learn.microsoft.com/en-us/windows/win32/learnwin32/initializing-the-com-library)
- [32-bit/64-bit process interoperability](https://learn.microsoft.com/en-us/windows/win32/winprog64/process-interoperability)
