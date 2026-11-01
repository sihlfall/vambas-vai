import assert from 'node:assert/strict';

const base = (process.argv[2] || 'http://127.0.0.1:10240').replace(/\/$/, '');
async function json(url, init = {}) {
    const response = await fetch(base + url, {...init, signal: AbortSignal.timeout(30000)});
    assert.equal(response.status, 200, `${url}: ${await response.clone().text()}`);
    return response.json();
}
const compilers = await json('/api/compilers/vba', {headers: {Accept: 'application/json'}});
for (const id of ['vba-llvm']) {
    assert(compilers.some(compiler => compiler.id === id), `Missing compiler: ${id}`);
}

const source = `Option Explicit
Function Count(ByVal limit As Long) As Long
    Dim i As Long
    While i < limit
        i = i + 1
    Wend
    Count = i
End Function`;

async function compile(id, text = source, userArguments = '', commentOnly = false, files = [], intel = false) {
    return json(`/api/compiler/${id}/compile`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json', Accept: 'application/json'},
        body: JSON.stringify({
            source: text, lang: 'vba', files,
            options: {userArguments, filters: {commentOnly, directives: false, labels: true, intel}},
        }),
    });
}

const llvm = await compile('vba-llvm');
assert.equal(llvm.code, 0, JSON.stringify(llvm.stderr));
assert(llvm.asm.some(line => line.text.includes('phi i32')), 'LLVM output has no loop phi');
assert(!llvm.asm.some(line => line.text.includes('alloca')), 'Scalar locals still allocate stack slots');
assert(llvm.asm.some(line => line.source?.line === 5), 'Source-to-LLVM highlighting is missing');

const filtered = await compile('vba-llvm', source, '', true);
assert.equal(filtered.code, 0);
assert(filtered.asm.some(line => line.source?.line === 5), 'Hiding comments lost source links');
assert(!filtered.asm.some(line => /^\s*;/.test(line.text)), 'Comment filter did not hide comments');

const removed = await compile('vba-llvm', source, '-ssa=false');
assert.notEqual(removed.code, 0, 'The removed -ssa option was accepted');

const invalid = await compile('vba-llvm', `' café 😀\nOption Explicit\nFunction Broken() As Long\n    Broken = missing\nEnd Function`);
assert.notEqual(invalid.code, 0, 'Invalid VBA compiled successfully');
assert(invalid.stderr.some(line => line.tag?.line === 4 || /:4:\d+:/.test(line.text)),
    `Expected a diagnostic on line 4: ${JSON.stringify(invalid.stderr)}`);

const forbidden = await compile('vba-llvm', source, '-root=/');
assert.notEqual(forbidden.code, 0, 'Compiler input-directory override was accepted');

const modules = await compile('vba-llvm', 'Function Answer() As Long\n Answer = Helpers.Value()\nEnd Function', '', false,
    [{filename: 'Helpers.bas', contents: 'Public Function Value() As Long\n Value = 42\nEnd Function'}]);
assert.equal(modules.code, 0, JSON.stringify(modules.stderr));
assert(modules.asm.some(line => line.source?.file === 'Helpers.bas' && line.source.line === 2),
    'Additional module source links are missing or refer to the main editor');

const moduleError = await compile('vba-llvm', 'Sub Run()\nEnd Sub', '', false,
    [{filename: 'Helpers.bas', contents: 'Option Explicit\nPublic Function Value() As Long\n Value = missing\nEnd Function'}]);
assert.notEqual(moduleError.code, 0);
assert(moduleError.stderr.some(line => /Helpers.bas:3:\d+:/.test(line.text)),
    `Additional module diagnostic has the wrong line: ${JSON.stringify(moduleError.stderr)}`);

if (compilers.some(compiler => compiler.id === 'vba-native')) {
    const native = await compile('vba-native');
    assert.equal(native.code, 0, JSON.stringify(native.stderr));
    assert.equal(native.languageId, 'asm');
    assert(native.asm.some(line => /vba[.]p1/.test(line.text)), 'Clang optimized away the inspected procedure');
    assert(!native.asm.some(line => /define internal/.test(line.text)), 'Native view still contains LLVM IR');
    assert(native.asm.some(line => line.source?.line > 0), 'Native instructions have no VBA source links');
    assert(!native.stderr.some(line => /invalid debug info|ignoring debug info/i.test(line.text)),
        `Clang rejected the source metadata: ${JSON.stringify(native.stderr)}`);
    const arraySource = `Option Explicit
Function ArraySum(ByRef values() As Long) As Long
    Dim i As Long
    For i = LBound(values) To UBound(values)
        ArraySum = ArraySum + values(i)
    Next i
End Function`;
    const arrayNative = await compile('vba-native', arraySource, '-O2');
    assert.equal(arrayNative.code, 0, JSON.stringify(arrayNative.stderr));
    assert(arrayNative.asm.some(line => /vba[.]p1/.test(line.text)), 'Array procedure was removed');
    assert(!arrayNative.asm.some(line => /^\s*(call\w*|bl|blr)\s/.test(line.text)),
        'Typed scalar array sum still calls runtime helpers');
    assert(arrayNative.asm.some(line => line.source?.line === 5), 'Array access lost its VBA source line');
    const unoptimized = await compile('vba-native', source, '-O0');
    assert.equal(unoptimized.code, 0, JSON.stringify(unoptimized.stderr));
    assert(unoptimized.asm.some(line => line.source?.line === 5), 'Unoptimized loop instructions lost their VBA source line');
    assert.notEqual(unoptimized.asm.map(line => line.text).join('\n'), native.asm.map(line => line.text).join('\n'),
        'Changing Clang optimization level had no effect');
    const aggressive = await compile('vba-native', source, '-O3');
    assert.equal(aggressive.code, 0, JSON.stringify(aggressive.stderr));
    const intel = await compile('vba-native', source, '-O0', false, [], true);
    assert.equal(intel.code, 0, JSON.stringify(intel.stderr));
    if (native.instructionSet === 'amd64') {
        assert.notEqual(intel.asm.map(line => line.text).join('\n'), unoptimized.asm.map(line => line.text).join('\n'),
            'The Intel syntax filter had no effect');
    }
    const invalidNative = await compile('vba-native', 'Option Explicit\nSub Run()\n missing = 1\nEnd Sub');
    assert.notEqual(invalidNative.code, 0);
    assert(invalidNative.stderr.some(line => line.tag?.line === 3 || /:3:\d+:/.test(line.text)),
        'Native view lost frontend diagnostics');
    console.log('PASS: Clang native assembly, inline array access, optimization levels, source links, private procedure retention, syntax filter, and frontend diagnostics.');
} else {
    console.log('SKIP: native assembly checks (install Clang 20+ or select CLANG, then restart).');
}
console.log('PASS: upstream API, LLVM with direct SSA, source links, comment filtering, option validation, Unicode diagnostics, additional modules, and input isolation.');
