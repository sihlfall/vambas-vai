// Exercises the actual executable with Microsoft's protocol client. It runs
// without a VS Code extension host; editor UI behavior is a separate manual check.
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const {
    createProtocolConnection, StreamMessageReader, StreamMessageWriter,
} = require('vscode-languageserver-protocol/node');

function decodeSemanticTokens(text, response, legend) {
    assert.ok(Array.isArray(response.data));
    assert.equal(response.data.length % 5, 0);
    const lines = text.split(/\r\n|\r|\n/);
    const tokens = [];
    let line = 0, character = 0;
    for (let i = 0; i < response.data.length; i += 5) {
        const [deltaLine, deltaStart, length, type, modifiers] = response.data.slice(i, i + 5);
        if (deltaLine) character = 0;
        line += deltaLine;
        character += deltaStart;
        assert.ok(length > 0 && type < legend.tokenTypes.length);
        assert.ok(character + length <= lines[line].length);
        tokens.push({ line, character, text: lines[line].slice(character, character + length), type: legend.tokenTypes[type], modifiers });
    }
    return tokens;
}

async function main() {
    const executable = process.argv[2] || path.resolve(__dirname, '../../bin/vambas-language-server' + (process.platform === 'win32' ? '.exe' : ''));
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'vba-lsp-'));
    const child = spawn(executable, [], { stdio: 'pipe' });
    const exited = new Promise((resolve, reject) => {
        child.once('error', reject);
        child.once('exit', (code) => code === 0 ? resolve() : reject(new Error(`server exit: ${code}`)));
    });
    // Attach immediately so a spawn error cannot become an unhandled rejection.
    exited.catch(() => {});
    let stderr = '';
    child.stderr.on('data', chunk => { stderr += chunk; });
    const connection = createProtocolConnection(new StreamMessageReader(child.stdout), new StreamMessageWriter(child.stdin));
    const timeout = setTimeout(() => {
        console.error('Server smoke test timed out.');
        process.exitCode = 1;
        child.kill();
    }, 10000);
    try {
        // A real ANSI export must load without rewriting it as UTF-8.
        const utilities = Buffer.from('\' Gr\xfc\xdfe \x80\r\nPublic Function Answer(ByVal count As Long) As Long\r\n Answer = count\r\nEnd Function\r\n', 'latin1');
        fs.writeFileSync(path.join(root, 'Utilities.bas'), utilities);
        const shapesURI = pathToFileURL(path.join(root, 'Shapes.bas')).href;
        fs.writeFileSync(path.join(root, 'Shapes.bas'), 'Public Type Point\n    X As Long\n    Y As Double\nEnd Type\nPublic Enum Color\n    Red = 1\n    Blue\n    ObjectError = vbObjectError + 3000\nEnd Enum\n');
        const uri = pathToFileURL(path.join(root, 'Main # é.bas')).href;
        // VB_Name supplies a valid module identifier despite this filename.
        const line = ' text = "😀": n = Utilities.Answer(count:=1)';
        const text = 'Attribute VB_Name = "Main"\nOption Explicit\nSub Run()\n Dim n As Long\n Dim text As String\n' + line + '\n Dim point As Shapes.Point\n point.X = Color.Blue\n ReDim values(0 To n) As Long\n ReDim Preserve values(0 To n + 1)\nEnd Sub\n';
        let receiveDiagnostics;
        const diagnostics = new Promise(resolve => { receiveDiagnostics = resolve; });
        connection.onNotification('textDocument/publishDiagnostics', params => {
            if (params.uri === uri && params.version === 1) receiveDiagnostics(params.diagnostics);
        });
        connection.listen();
        const initialized = await connection.sendRequest('initialize', {
            processId: process.pid,
            rootUri: pathToFileURL(root).href,
            capabilities: { textDocument: {
                documentSymbol: { hierarchicalDocumentSymbolSupport: true },
                semanticTokens: {
                    requests: { full: true, range: true }, formats: ['relative'],
                    tokenTypes: ['namespace', 'class', 'enum', 'struct', 'parameter', 'variable', 'property', 'enumMember', 'function', 'method', 'label'],
                    tokenModifiers: ['declaration', 'readonly', 'defaultLibrary'],
                },
            } },
        });
        assert.equal(initialized.capabilities.positionEncoding, 'utf-16');
        assert.deepEqual(initialized.capabilities.completionProvider.triggerCharacters, ['.']);
        assert.equal(initialized.capabilities.completionProvider.resolveProvider, false);
        const provider = initialized.capabilities.semanticTokensProvider;
        assert.equal(provider.full, true);
        assert.equal(provider.range, true);
        await connection.sendNotification('initialized', {});
        await connection.sendNotification('textDocument/didOpen', { textDocument: { uri, languageId: 'vba', version: 1, text } });
        assert.deepEqual(await diagnostics, []);
        const params = { textDocument: { uri }, position: { line: 5, character: line.indexOf('count') + 2 } };
        const hover = await connection.sendRequest('textDocument/hover', params);
        assert.equal(hover.contents.value, 'ByVal count As Long');
        const definitions = await connection.sendRequest('textDocument/definition', params);
        assert.equal(definitions.length, 1);
        assert.equal(definitions[0].uri, pathToFileURL(path.join(root, 'Utilities.bas')).href);
        assert.equal(definitions[0].range.start.line, 1);
        assert.deepEqual(fs.readFileSync(path.join(root, 'Utilities.bas')), utilities);
        const symbols = await connection.sendRequest('textDocument/documentSymbol', { textDocument: { uri } });
        assert.equal(symbols[0].name, 'Run');
        assert.deepEqual(symbols[0].children.map(symbol => symbol.name), ['n', 'text', 'point', 'values']);
        const shapes = await connection.sendRequest('textDocument/documentSymbol', { textDocument: { uri: shapesURI } });
        assert.equal(shapes[0].kind, 23); // Struct
        assert.equal(shapes[0].children[0].name, 'X');
        assert.equal(shapes[1].kind, 10); // Enum
        assert.equal(shapes[1].children[1].name, 'Blue');
        const field = await connection.sendRequest('textDocument/hover', { textDocument: { uri }, position: { line: 7, character: 7 } });
        assert.equal(field.contents.value, 'Point.X As Long');
        const member = await connection.sendRequest('textDocument/hover', { textDocument: { uri }, position: { line: 7, character: 19 } });
        assert.equal(member.contents.value, 'Color.Blue As Long = 2');
        const errorValue = await connection.sendRequest('textDocument/hover', { textDocument: { uri: shapesURI }, position: { line: 7, character: 9 } });
        assert.equal(errorValue.contents.value, 'Color.ObjectError As Long = -2147218504');

        const semantic = await connection.sendRequest('textDocument/semanticTokens/full', { textDocument: { uri } });
        const tokens = decodeSemanticTokens(text, semantic, provider.legend);
        for (const [name, type] of [['Utilities', 'namespace'], ['Answer', 'function'], ['count', 'parameter']]) {
            const token = tokens.find(token => token.line === 5 && token.text === name);
            assert.ok(token, `Missing semantic token: ${name}`);
            assert.equal(token.character, line.indexOf(name)); // JS indexes, like LSP, count UTF-16 units.
            assert.equal(token.type, type);
        }
        assert.ok(tokens.some(token => token.text === 'Point' && token.type === 'struct'));
        assert.ok(tokens.some(token => token.text === 'X' && token.type === 'property'));
        const blue = tokens.find(token => token.text === 'Blue');
        assert.equal(blue.type, 'enumMember');
        assert.ok(blue.modifiers & (1 << provider.legend.tokenModifiers.indexOf('readonly')));
        const part = await connection.sendRequest('textDocument/semanticTokens/range', {
            textDocument: { uri }, range: { start: { line: 7, character: 0 }, end: { line: 8, character: 0 } },
        });
        assert.deepEqual(decodeSemanticTokens(text, part, provider.legend), tokens.filter(token => token.line === 7));
        const edited = text.replace(/\bn\b/g, 'changed');
        await connection.sendNotification('textDocument/didChange', {
            textDocument: { uri, version: 2 }, contentChanges: [{ text: edited }],
        });
        const updated = decodeSemanticTokens(edited,
            await connection.sendRequest('textDocument/semanticTokens/full', { textDocument: { uri } }), provider.legend);
        assert.ok(updated.some(token => token.text === 'changed' && token.type === 'variable'));
        assert.ok(!updated.some(token => token.text === 'n'));

        const incomplete = edited.replace(' point.X = Color.Blue', ' point.');
        await connection.sendNotification('textDocument/didChange', {
            textDocument: { uri, version: 3 }, contentChanges: [{ text: incomplete }],
        });
        const completions = await connection.sendRequest('textDocument/completion', {
            textDocument: { uri }, position: { line: 7, character: 7 },
            context: { triggerKind: 2, triggerCharacter: '.' },
        });
        assert.equal(completions.isIncomplete, false);
        assert.deepEqual(completions.items.map(item => [item.label, item.kind, item.detail]), [
            ['X', 5, 'X As Long'], ['Y', 5, 'Y As Double'],
        ]);
        const xEdit = completions.items[0].textEdit;
        assert.deepEqual(xEdit, { range: { start: { line: 7, character: 7 }, end: { line: 7, character: 7 } }, newText: 'X' });
        await connection.sendNotification('textDocument/didChange', {
            textDocument: { uri, version: 4 }, contentChanges: [{ range: xEdit.range, text: 'y' }],
        });
        const prefixed = await connection.sendRequest('textDocument/completion', {
            textDocument: { uri }, position: { line: 7, character: 8 }, context: { triggerKind: 1 },
        });
        assert.equal(prefixed.items[1].filterText, 'Y');
        assert.deepEqual(prefixed.items[1].textEdit, {
            range: { start: { line: 7, character: 7 }, end: { line: 7, character: 8 } }, newText: 'Y',
        });
        assert.equal(await connection.sendRequest('shutdown'), null);
        await connection.sendNotification('exit');
        await exited;
        assert.equal(stderr, '');
        console.log('Server smoke test passed: legacy exports, diagnostics, Unicode hover, definitions, symbols, semantic tokens, UDT completion/edits, shutdown.');
    } finally {
        clearTimeout(timeout);
        connection.dispose();
        child.kill();
        fs.rmSync(root, { recursive: true, force: true });
    }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
