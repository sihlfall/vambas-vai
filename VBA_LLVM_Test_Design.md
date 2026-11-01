My recommendation is to treat Rubberduck as a compatibility layer on top of a general-purpose test runner, rather than implementing Rubberduck testing directly inside your VBA compiler.

Since you're developing both a language server and a VBA → LLVM → native executable compiler, you have an opportunity to build something quite powerful: a test system that executes existing Rubberduck-style VBA tests as native code, independently of Excel, Access, or the VBA editor.

The important design decision is to separate four concerns:

1. Test discovery — identifying tests and lifecycle hooks from VBA source.
2. Test compilation — compiling tests and application code to LLVM without introducing special VBA language semantics.
3. Test execution — initializing VBA runtime state, invoking tests, handling assertions and errors, and collecting results.
4. IDE integration — exposing tests, execution commands, and results through your language tooling.

I would design the system roughly like this:

VBA Frontend

Lexer · Parser · Semantic analysis · Annotations

Compiler pipeline

Typed IR → LLVM IR → Native code

Test discovery

Rubberduck annotations → Test manifest

Native Test Runner

VBA runtime · Assertions · Fixtures · Error handling · Fakes

Results and integrations

CLI · JSON/JUnit · VS Code Test Explorer · CI

The compiler and test discovery component should share the same semantic model. This is especially important because Rubberduck's annotations look like comments to VBA, but represent structural metadata to its testing framework.

## 1. Represent Rubberduck annotations in your semantic model

Rubberduck test discovery uses annotations embedded in VBA comments.

For example:

```

'@TestModule
Option Explicit

Private Assert As New Rubberduck.AssertClass

'@TestMethod("Arithmetic")
Public Sub AdditionWorks()
    Assert.AreEqual CLng(4), CLng(2 + 2)
End Sub

```

The core annotations include:

| Annotation                | Purpose                                       |
| ------------------------- | --------------------------------------------- |
| `@TestModule`             | Identifies a standard module containing tests |
| `@TestMethod("Category")` | Identifies a test procedure                   |
| `@ModuleInitialize`       | Setup once per module                         |
| `@ModuleCleanup`          | Cleanup once per module                       |
| `@TestInitialize`         | Setup before each test                        |
| `@TestCleanup`            | Cleanup after each test                       |

Rubberduck supports optional category arguments, and lifecycle hooks are independent of the test methods themselves.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://github.com\&sz=32)

GitHub

+1



I would retain these annotations in your AST and attach their interpreted meaning during semantic analysis.

For example, a semantic node could contain:

```

struct ProcedureSymbol {
    name: String,
    signature: ProcedureSignature,
    source_range: SourceRange,
    annotations: Vec<Annotation>,
}

```

Then have a dedicated discovery pass:

`Semantic Model → RubberduckTestDiscoverer → TestManifest`

Crucially, the manifest should contain resolved procedure identities, not just procedure names.

An example:

```

{
  "id": "MyProject.MathTests.AdditionWorks",
  "module": "MathTests",
  "procedure": "AdditionWorks",
  "category": "Arithmetic",
  "source": {
    "file": "MathTests.bas",
    "line": 7
  }
}

```

The discovery phase can run without producing LLVM IR. That makes it lightweight enough for editor integration.

I would not implement discovery using a separate regex-based source scanner. You already have a proper VBA frontend, and using it avoids inconsistencies involving annotations, conditional compilation, module names, and procedure signatures.

## 2. Provide a virtual Rubberduck runtime library

This is probably the most important part of the design.

Existing Rubberduck tests reference types such as:

- `Rubberduck.AssertClass`
- `Rubberduck.PermissiveAssertClass`
- `Rubberduck.FakesProvider`

These are ordinarily supplied through Rubberduck's COM library. Some tests use early binding, while others instantiate the types through `CreateObject`.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://rubberduckvba.blog\&sz=32)

Rubberduck News

+1



I would not try to load the actual Rubberduck COM implementation into your native executable.

Instead, implement a compatible subset as part of your compiler's runtime.

The compiler should recognize `Rubberduck` as a supported library reference and provide the appropriate class definitions, methods, and object construction semantics.

For example, this existing source:

```

Private Assert As Rubberduck.AssertClass

Public Sub ModuleInitialize()
    Set Assert = New Rubberduck.AssertClass
End Sub

```

Would instantiate your own native implementation.

Internally, the calls might eventually lower to functions such as:

```

void rd_assert_are_equal(
    TestContext* context,
    const VbaVariant* expected,
    const VbaVariant* actual,
    const VbaVariant* message
);

```

You don't necessarily need a COM implementation at all for this path. You need to emulate the source-visible type contract and dispatch it to your native runtime.

The same compatibility layer should handle `CreateObject("Rubberduck.AssertClass")`, when used.

### Treat assertions as a runtime service

An important implementation detail: assertions should update the active test context, not necessarily terminate execution or raise a VBA runtime error.

That distinction matters for `On Error GoTo` and `On Error Resume Next`, and for tests that make several assertions.

I would have a central `TestContext` containing the current test's assertion results, failures, timing, and runtime state.

Also, be careful with equality semantics. Rubberduck's default `AssertClass` uses strict type-sensitive equality, while `PermissiveAssertClass` uses more VBA-like comparisons.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://rubberduckvba.blog\&sz=32)

Rubberduck News



That means a generic comparison based on VBA's `=` operator is not sufficient. Your implementation must account for VBA types, `Variant` values, `Nothing`, `Null`, array bounds, object identity, and numeric coercion rules.

This is an excellent opportunity to reuse the runtime type system you already need for the compiler.

## 3. Generate the test entry points at compile time

I'd generate a native test registration table alongside the compiled application.

Conceptually, the compiler could emit something equivalent to:

```

static TestDescriptor tests[] = {
    {
        .id = "MathTests.AdditionWorks",
        .invoke = &MathTests_AdditionWorks,
        .initialize = &MathTests_TestInitialize,
        .cleanup = &MathTests_TestCleanup
    }
};

```

You could generate an LLVM constant table directly, or build a small synthetic entry-point module and let LLVM compile it normally.

I slightly prefer the latter initially. It is easy to inspect and debug.

This also means you don't need VBA reflection or procedure lookup by name at runtime. The compiler has already resolved everything.

For private procedures, generated entry points can be created inside the appropriate module's compilation unit so that visibility restrictions don't prevent the test runner from calling lifecycle hooks.

An important benefit is that you can reuse this entry-point infrastructure for other purposes, such as calling VBA procedures from non-VBA code.

## 4. Make the test runner its own component

The runner should consume the compiled test descriptors and invoke procedures through your VBA runtime.

Its lifecycle would look like:

ModuleInitialize

Once when the test module starts

TestInitialize

Before each selected test

TestMethod

Test

Execute test and collect assertions

TestCleanup

After each test, including ordinary failures

ModuleCleanup

Once when the test module finishes

Repeat the per-test lifecycle for every selected test.

The difficult part isn't the basic invocation mechanism. It's state management.

VBA test modules can contain module-level variables. The module initialization and cleanup routines affect that shared module state.

Therefore, simply creating a fresh process per test may not reproduce Rubberduck's fixture lifecycle.

I'd use one worker process per test module by default, executing tests sequentially within that worker. This retains shared state and lifecycle behavior while isolating modules from one another.

Rubberduck itself runs its tests sequentially and does not promise a particular test order.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://rubberduckvba.blog\&sz=32)

Rubberduck News



The native runner should additionally distinguish an assertion failure from an unhandled VBA error, compilation error, crash, timeout, or explicitly inconclusive result.

One particularly important rule: route ordinary VBA errors through your compiler's existing `Err` and `On Error` machinery. The test runner should only classify an error as unhandled once VBA's own error-handling semantics have been applied.

## 5. Design Fakes support into your compiler now, but implement it later

Rubberduck's `FakesProvider` can intercept built-in VBA functions such as `MsgBox`, `Rnd`, and `Now`. Tests can configure return values and verify invocation counts or parameters.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://rubberduckvba.blog\&sz=32)

Rubberduck News



For example:

```

Fakes.MsgBox.Returns vbYes

Call SomeBusinessLogic

Fakes.MsgBox.Verify.Once

```

This has a significant compiler implication.

If your generated LLVM code calls a native `MsgBox` implementation directly, introducing a fake later becomes difficult.

Instead, I'd introduce a dispatchable runtime boundary for fakeable VBA built-ins:

```

Compiled VBA call
       |
       v
VBA intrinsic dispatch
       |
       +-- Fake registered? --> Fake implementation
       |
       +-- Otherwise -------> Native implementation

```

The lookup and interception can be optimized away when appropriate, but the architectural boundary should exist.

The fake registry should be associated with the active test context, not with a process-wide unscoped global, so it can be reset safely between tests.

This will give you a natural place to implement `Returns`, `ReturnsWhen`, invocation recording, `Verify`, and eventually `RaisesError` and `ByRef` argument assignment.

The more advanced Rubberduck COM mocking functionality is a separate problem. Rubberduck's 2025 release added Moq-based COM mocking support, which relies on a rather different mechanism. I wouldn't attempt to reproduce that initially.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://rubberduckvba.blog\&sz=32)

Rubberduck News



## 6. Keep testing separate from LSP

I would make the language server and compiler share test discovery, but keep running and building tests out of the LSP process.

A CLI could look like:

```

vbac test --list
vbac test
vbac test --filter MathTests.AdditionWorks
vbac test --report junit.xml

```

This gives you immediate command-line and CI support, without depending on the editor.

For VS Code integration, use the native [Testing API](https://code.visualstudio.com/api/extension-guides/testing?utm_source=chatgpt.com) with its `TestController`, test items, and run profiles. Standard LSP language features alone do not provide the complete VS Code testing experience.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://code.visualstudio.com\&sz=32)

Visual Studio Code Extension API



Your editor extension can retrieve discovered tests from the same semantic service used by the compiler and delegate execution to the test CLI or runner.

I would use a small editor-independent testing protocol containing stable test IDs, source ranges, invocation commands, and structured test events. That keeps the language server reusable with other clients.

## 7. An incremental implementation roadmap

I'd prioritize features in this order:

| Stage | Features                                                      | Result                               |
| ----- | ------------------------------------------------------------- | ------------------------------------ |
| 1     | Annotation discovery, test manifest, executable entry points  | Discover and invoke test procedures  |
| 2     | `AssertClass`, outcomes, lifecycle hooks                      | Run real Rubberduck tests            |
| 3     | VBA error handling, `PermissiveAssertClass`, robust isolation | Broader compatibility                |
| 4     | CLI reporting and VS Code Testing API                         | Editor and CI integration            |
| 5     | `FakesProvider`, intrinsic interception                       | Test code with built-in dependencies |
| 6     | COM mocking, coverage, advanced debugging                     | Higher-fidelity integration          |

I would also create a small differential compatibility suite: execute the same VBA test modules under Rubberduck and your native compiler, and compare the results. This is particularly useful for checking assertion equality, lifecycle order, `Err` behavior, and handling of unusual `Variant` values.

## One architectural improvement I'd strongly consider

Make your testing infrastructure framework-agnostic from the beginning.

Rather than exposing `RubberduckTest`, define something more general, such as:

```

struct TestCase {
    id: TestId,
    entry_point: FunctionId,
    suite: SuiteId,
    metadata: TestMetadata,
}

trait TestFrameworkAdapter {
    fn discover(&self, model: &SemanticModel)
        -> Vec<TestCase>;
}

```

Then Rubberduck is merely your first test framework adapter.

Everything else—the compilation process, runtime test context, test runner, output formats, and IDE integration—can remain independent of Rubberduck.

That separation matters because you may eventually want a VBA-native testing framework optimized specifically for compiled execution, while still supporting existing Rubberduck tests.

The design principle I'd optimize for is: compile ordinary VBA normally, discover tests statically, and supply a compatible runtime for their dependencies. This should give you the highest degree of source compatibility with the least amount of test-specific behavior embedded in the LLVM backend.

One particularly interesting longer-term benefit is that your own VBA compiler's regression suite could be written as Rubberduck tests, providing an end-to-end validation path for the compiler itself.