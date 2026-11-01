# Collection

The built-in `Collection` class is available to the shared frontend and native
compiler as `Collection` or `VBA.Collection`. Project-defined types keep ordinary
lookup precedence. The implementation is independently authored; it does not use
Rubberduck implementation code.

## Source behavior

```vb
Dim parts As New Collection
parts.Add "last", Key:="tail"
parts.Add "first", Before:="tail"
parts.Add "middle", After:=1

Dim value As Variant
For Each value In parts
    ' first, middle, last
Next value

value = parts(1)          ' Default Item member; indexes start at 1.
value = parts.Item("tail")
parts.Remove "tail"
```

Supported members are `Add(Item, [Key], [Before], [After])`, read-only
`Item(Index)`, `Remove(Index)`, and read-only `Count As Long`. Item is the default
member; `parts!tail` also supplies a string key. Named arguments, omitted optional
arguments, explicit/implicit calls, and calls through `As Object` are supported.
Arguments evaluate in source order. Before/After select an existing position or
key and cannot both be supplied. Removing an item renumbers subsequent positions.
String indexes always select keys: `parts("1")` and `parts(1)` are distinct.
These contracts follow Microsoft's [Collection reference](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/collection-object),
[Add](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/add-method-visual-basic-for-applications),
and [Item](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/item-method-visual-basic-for-applications) documentation.

`New`, lazy `As New`, `Set`, `Nothing`, `Is`, module globals, parameters, function
returns, ByVal copies of the reference, and ByRef aliases use the existing private
object lifetime model. `With` retains its receiver once, including when the source
variable is subsequently reassigned. `As New` recreates a cleared variable lazily.

Scalar/Empty/Null items and supported arrays are stored as owned Variant copies;
Strings preserve UTF-16 contents and arrays preserve bounds and values. Reading a
value returns another snapshot. Passing such an Item to a ByRef Variant formal
passes a temporary, so assigning that parameter cannot replace the stored item.

Supported private object items retain identity rather than copying the object:

```vb
Dim children As New Collection, child As New Collection
children.Add child, "child"
Dim found As Collection
Set found = children("child")
' found Is child
```

An object Item can be used in a `Set` expression, passed to an Object/class formal,
compared with `Is`, or supplied to object identity assertions. `For Each` supports Variant counters
for value items and Object/supported-class counters for object items. The iterator
retains the original Collection even if its source variable is cleared. Nested
enumerators have independent cursors. Exit, error propagation, and GoSub Return
release captures and iterators through normal IR resource scopes.

The live iterator skips removed unvisited nodes. An insertion at its next position
or at the tail remains visible until exhaustion has been observed. Already visited
nodes are not revisited. These mutation corner cases have native regression tests
but have not been differentially validated against a VBA host.

## Architecture and limits

Ambient class/accessor declarations and default-member metadata feed ordinary
checker member lookup, argument binding, and call resolution. As in `typescript-go`,
the frontend owns name resolution; lowering consumes that resolution. It emits
`OpCollectionInvoke`, object operations, and enumeration operations into verified
typed IR. The LLVM emitter uses the existing status/Result/Try convention, so
runtime errors participate in `On Error` and `Resume`.

The C implementation uses ordered linked nodes and independent retained cursors.
Lookup is linear. An insertion prepares its key and value before linking the node;
allocation/copy failure leaves the collection unchanged. Cursor advancement follows
a successful value copy, allowing retry after failure. Common errors include 457
for duplicate keys, 5 for invalid indexes or Before/After, 91 for Nothing, 438 for
the wrong receiver, and ordinary Variant conversion/allocation errors. Late-bound
argument names and arity are validated at runtime.

The current compatibility profile has explicit limits:

- Keys fold ASCII case independently of Option Compare. Other UTF-16 units compare
  exactly, including embedded NULs. Locale-sensitive Unicode key collation is not
  implemented.
- Collection references and object items are private runtime handles, **not COM
  interface pointers**. Automation VARIANT and SAFEARRAY layouts stay unchanged.
  Object-valued Variants, mixed object/value iteration into a Variant counter,
  general user classes, Object arrays/UDT fields, and external COM interop remain
  unsupported. Retrieving an object item in a value-Variant context raises 13;
  use an Object/class context instead.
- `CreateObject` remains restricted to the existing virtual testing library;
  construct Collections with `New`. Reference cycles require explicit removal or
  clearing of the references that form the cycle; there is no cycle collector.

## Try it and validation

```sh
go run ./cmd/vambas-compile -root examples/collection -entry Main.Main -o /tmp/collection.ll
clang-20 -O2 /tmp/collection.ll runtime/*.c -lm -o /tmp/collection
/tmp/collection
# Expected exit status: 42.

go test ./checker ./lowering ./testrunner -run Collection
go test ./llvm -run 'TestNativeCollection|TestCollectionRuntime'
python3 llvm/audits/vba-regex/run-tests.py
```

Tests cover indexing/keys/order, snapshots, object identity, ByRef/ByVal, With,
enumeration/mutation, lazy construction, errors, allocation failures, and cleanup.
The native harness verifies LLVM and executes original/optimized bitcode and Clang
O0/O2 binaries, with allocation tracking. The complete, unchanged original regex
project now passes **203 tests in eight modules at both O0 and O2**, including
Collection-based `StaticRegex.SplitByRegex`. Native validation is on Linux x86-64;
Windows execution and COM ABI compatibility are not established.
