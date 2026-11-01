# Native class modules

Ordinary `.cls` modules now execute through the compiler's private object ABI.
The [Counter example](../examples/classes/Counter.cls) stores a Long per instance,
increments it through a method, and returns it through Property Get. From the
repository root, with LLVM 20 installed:

```sh
go run ./cmd/vambas-compile -root ./examples/classes -entry Main.Main -o /tmp/classes.ll
clang-20 -O2 /tmp/classes.ll runtime/*.c -lm -o /tmp/classes
/tmp/classes
echo $? # 1
```

Class identities, field layouts, and all member signatures are declared before
lowering bodies. Forward and mutually referring class signatures work. Each
method/accessor has an implicit ByVal Object receiver; `Me` and unqualified
instance fields/members resolve through it. Class fields never become project
globals. Public fields expose storage, including supported ByRef aliases.

The executable slice includes `New`, `Set`, `Nothing`, `Is`, typed class
parameters/results, direct methods, and Property Get/Let/Set. Properties use the
checker's selected accessor and argument bindings. A property passed ByRef uses
a temporary; it does not acquire field-like storage or implicit copy-back.
Matching typed class ByRef parameters alias storage. Adapted object ByRef
arguments use typed temporaries and copy back only on normal return; class
constraints are rechecked before committing the replacement. Variant ByRef views
of object storage retain the original class constraint.

Supported fields are Boolean, Integer, Long, Single, Double, variable-length
String, Variant, and the existing supported scalar/UDT array and UDT types.
Fixed arrays initialize per instance; dynamic arrays start unallocated. Nested
managed fields use the existing record initialization and cleanup operations.
Class and UDT fields can own object references. Variant arrays and Collection
items can hold object references as well. Typed arrays of Object/class references
execute through the [object-array path](OBJECT_ARRAYS.md).

Object loads retain an instance until the statement finishes, including while
later arguments run. ByVal receivers own a reference throughout the activation.
Borrowed field addresses keep the originating object alive; supported UDT With
captures and array enumeration retain that owner across statement boundaries.
Cleanup releases locks before instance fields. Failed allocation/field
initialization releases partially initialized payloads without publishing the
instance. Nothing member access raises error 91; a nominal class mismatch raises
error 13. `Is` compares private instance identity, including aliases and objects
round-tripped through built-in Collections.

`Program.Classes` describes nominal identities, fields, and member procedure IDs.
`Procedure.Class` and `Receiver` associate methods with their implicit parameter.
Object values have an erased private representation. `OpClassNew`,
`OpClassCheck`, `OpClassCast`, and `OpClassFieldRef` retain the selected identity.
The verifier requires a successful non-Nothing class check before field access
or an instance call, and checks field types, member associations, and ownership.

The runtime object header carries a descriptor for allocation size, field
initialization/cleanup metadata, and optional custom cleanup. Built-in testing
objects and Collection use the same allocation and reference-counting path.
Descriptors also carry lifecycle and invocation callbacks. Descriptors and private
handles are not COM interfaces; Windows COM adapters,
interface queries, and external activation are later milestones.

## Lifecycle and lazy instances

Parameterless `Class_Initialize` and `Class_Terminate` Subs are lifecycle hooks.
Initialization runs after field storage is initialized and before construction
returns the new reference. A failed initializer releases its construction owner
and preserves its original error, including Source and Description. Managed
fields and children are released on failure. Field allocation failure before the
user initializer starts does not invoke user termination. If user initialization
has started, normal last-reference cleanup applies, including termination; an
explicitly escaped `Me` reference can keep that instance alive after failure.
These failure rules are the native policy, rather than a claim about every host's
Automation error wrapping.

Termination runs at most once, before fields are released. A protective reference
allows the handler to publish `Me` again. A resurrected instance retains its
fields, and its eventual final release skips the already executed terminator.
Termination preserves the caller's ambient Err state. Errors handled within the
terminator stay local. An error that escapes the terminator reports its number
to stderr and ends native execution with a nonzero exit status; it cannot enter
the caller's On Error handler.

These rules follow the [MS-VBAL lifecycle contract](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/62bbe63e-379c-4dc0-8648-7d9050a2f396)
and [error-handling policies](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/47b18690-3175-44d9-9de1-31629f0aacc7).
Validation here uses normative text and executable native conformance cases.
VBA/VB6 host differential execution remains outstanding, including host-specific
initialization error wrapping and project shutdown behavior.

`As New` locals, module globals, and object fields construct on a read while their
storage is Nothing. Assigning Nothing releases the current value; the next read
constructs again, including a read for `Is Nothing`. `VB_PredeclaredId = True`
creates a shared project-lifetime lazy root. The named default-instance variable
cannot be Set-assigned. Class/Variant/Object With headers capture their object
once and keep it alive through nested calls and reassignment of the source
variable. Captured fields retain their owning object and any array locks.

## Invocation and native object Variants

Statically resolved calls retain direct typed calls. Public methods, accessors,
and fields also have generated adapters for a shared descriptor invocation
contract. Object and Variant receivers select the runtime member; private and
Friend members are absent from that contract. Methods/functions, property reads,
Let writes, and Set writes remain distinct, and a Sub cannot supply a value.
Statement calls require a method/function; a property used as a statement reports
invalid property assignment. Set evaluates and coerces its RHS before resolving
the destination receiver and indices.
Default-member attributes support explicit applications and implicit value reads.
Public array fields expose their actual indexed storage through dynamic access.

Invocation captures the receiver before evaluating later arguments. The native
binder matches case-insensitive names, positional arguments, Optional defaults,
Missing, ParamArrays, and typed ByRef views. Matching references preserve writes
on error. Object type adaptation uses normal-return copy-back. Property results
and forced-value arguments remain temporaries. A runtime array capture holds a
descriptor lock; an object capture retains identity and lifetime. Collection and
testing substitutes use this same invocation contract for dynamic calls, while
statically selected built-in operations keep their specialized lowering.

Object-valued Variants use the explicitly private `V_PRIVATE_OBJECT` tag, with
owned references or constrained ByRef views. They never store a private handle
under Automation's VT_DISPATCH or VT_UNKNOWN tags. Copies preserve instance
identity; clear/store operations retain the new reference before releasing the
old one. Let conversions and scalar operations resolve default value members.
VarType reports vbObject (9). A future COM boundary must marshal these private
values through real interface adapters. Scalar runtime member receivers and Is
operands raise error 424; Nothing member access raises 91; nominal class
mismatches raise 13. Missing/inaccessible members raise 438.

## Cycles and remaining limits

The native runtime uses reference counting and does not collect reference cycles.
Classes, object-valued Variants, records, and Collections can form strong cycles.
Code must break those links while it can still reach them; losing all external
roots does not run termination for an unreachable cycle. Resurrection does not
reset the at-most-once termination flag. The private project-clear entry releases
project roots in reverse declaration order; hosts must call it at a quiescent
boundary. It is not an Office/VB6 project-reset or End implementation.

Typed object/class arrays now execute through [the object-array path](OBJECT_ARRAYS.md).
Project-local [Implements](IMPLEMENTS.md) supports interface dispatch and casts.
Class enumeration, fixed-length String fields, host
modules, events, and COM boundaries remain unsupported. UDT values
continue to work through typed operations; dynamic UDT arguments/results or field
operations report type mismatch rather than inventing an Automation record
representation. For counters in class fields currently require an unqualified
or `Me` receiver; other captured field counter forms are diagnosed.

The [object contract example](../examples/object-contract/Main.bas) combines lazy
initialization, late binding, an object-valued Variant, a captured With receiver,
and a default property. It returns 42; compile it with the commands above using
`-root ./examples/object-contract`.

Validation includes the Counter result, independent fields, aliases, nominal
casts, forward signatures, methods/accessors, field versus property ByRef,
receiver lifetime, managed payloads, and error cleanup. Runtime tests inject
allocation failures through each construction allocation and check that no owners
survive. Native execution tests verify, JIT, optimize, and compile at O0 and O2,
with allocation tracking on both success and failure paths.
