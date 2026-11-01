const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const { Registry, parseRawGrammar, INITIAL } = require('vscode-textmate');
const oniguruma = require('vscode-oniguruma');

// Exercise the same tokenizer/regex engine used by VS Code, rather than testing
// grammar regexes in JavaScript's different regular-expression dialect.
const manifest = require('./package.json');
const contribution = manifest.contributes.grammars.find(item => item.language === 'vba');
const grammarPath = path.join(__dirname, contribution.path);
const grammar = (async () => {
    await oniguruma.loadWASM(fs.readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm')));
    const registry = new Registry({
        onigLib: Promise.resolve({ createOnigScanner: oniguruma.createOnigScanner, createOnigString: oniguruma.createOnigString }),
        loadGrammar: async scope => scope === contribution.scopeName
            ? parseRawGrammar(fs.readFileSync(grammarPath, 'utf8'), grammarPath) : null,
    });
    return registry.loadGrammar(contribution.scopeName);
})();

async function tokenize(source) {
    const tokenizer = await grammar;
    let stack = INITIAL;
    return source.split(/\r\n|\n|\r/).map(line => {
        const result = tokenizer.tokenizeLine(line, stack);
        stack = result.ruleStack;
        return { line, tokens: result.tokens };
    });
}

function scopesAt(result, needle) {
    const index = result.line.indexOf(needle);
    assert.notEqual(index, -1, `Missing fixture text: ${needle}`);
    const token = result.tokens.find(token => token.startIndex <= index && token.endIndex > index);
    assert.ok(token, `No token at ${needle}`);
    return token.scopes;
}

function scope(result, needle, expected) {
    assert.ok(scopesAt(result, needle).includes(expected), `${needle}: ${JSON.stringify(scopesAt(result, needle))}; expected ${expected}`);
}

test('VBA grammar is contributed for the registered language', async () => {
    assert.equal(contribution.scopeName, 'source.vba');
    assert.ok(manifest.contributes.languages.some(item => item.id === 'vba' && item.extensions.includes('.bas')));
    assert.ok(await grammar);
});

test('keywords, declarations, labels, and type characters are case insensitive', async () => {
    const lines = await tokenize('pUbLiC fUnCtIoN Run$(ByVal count As Long) As String\nIf True Then GoTo done\ndone:\n100 Exit Function\nEnd Function');
    scope(lines[0], 'pUbLiC', 'storage.modifier.vba');
    scope(lines[0], 'Run$', 'entity.name.function.vba');
    scope(lines[0], 'ByVal', 'storage.modifier.vba');
    scope(lines[0], 'Long', 'storage.type.vba');
    scope(lines[0], 'String', 'storage.type.vba');
    scope(lines[1], 'If', 'keyword.control.vba');
    scope(lines[1], 'True', 'constant.language.vba');
    scope(lines[2], 'done', 'entity.name.label.vba');
    scope(lines[3], '100', 'entity.name.label.vba');
});

test('VBA quotes, comments, and annotations do not leak into code', async () => {
    const lines = await tokenize(`text = "He said ""don't: Rem If"" 😀" & value ' real comment
Rem String "ignored"
remember = 1: obj.Rem = 2
If True Then rEm after Then
'@TestMethod("Example")
text = "unterminated
Dim nextValue As Long`);
    scope(lines[0], 'Rem', 'string.quoted.double.vba');
    assert.ok(!scopesAt(lines[0], 'Rem').some(name => name.startsWith('comment.')));
    scope(lines[0], '""', 'constant.character.escape.quote.vba');
    scope(lines[0], 'value', 'variable.other.readwrite.vba');
    scope(lines[0], 'real comment', 'comment.line.apostrophe.vba');
    scope(lines[1], 'ignored', 'comment.line.rem.vba');
    scope(lines[2], 'remember', 'variable.other.readwrite.vba');
    scope(lines[2], 'Rem', 'variable.other.property.vba');
    scope(lines[3], 'after', 'comment.line.rem.vba');
    scope(lines[4], '@TestMethod', 'storage.type.annotation.vba');
    scope(lines[6], 'Dim', 'keyword.control.vba');
});

test('numbers, dates, file channels, named arguments, and continuations stay distinct', async () => {
    const lines = await tokenize('n = &HFF& + &o77 + .5e-2 + 1D+3 + 2!\r\ntext$ = Left$("abc", 1): n = 7 \\ 2 Mod 3\r\nd = #12/31/2026 1:23 PM#\r\nPrint #1, "x"\r\nCall Work(count:=1, _\r\n  value:=2)');
    for (const number of ['&HFF&', '&o77', '.5e-2', '1D+3', '2!']) scope(lines[0], number, 'constant.numeric.vba');
    scope(lines[1], 'text$', 'variable.other.readwrite.vba');
    scope(lines[1], 'Left$', 'support.function.vba');
    scope(lines[1], '\\', 'keyword.operator.vba');
    scope(lines[1], 'Mod', 'keyword.operator.word.vba');
    scope(lines[2], '#12', 'constant.other.date.vba');
    assert.ok(!scopesAt(lines[3], '1').includes('constant.other.date.vba'));
    scope(lines[4], ':=', 'keyword.operator.vba');
    scope(lines[4], '_', 'punctuation.separator.continuation.vba');
});

test('conditional compilation and bracketed names are bounded to their contexts', async () => {
    const lines = await tokenize('#If VBA7 Then\nPublic Declare PtrSafe Sub Work Lib "api" ()\n#ElseIf Win64 Then\n#Else\n#End If\n[If] = "#Else"\nobj.[End] = 1');
    for (const index of [0, 2, 3, 4]) scope(lines[index], '#', 'keyword.control.directive.vba');
    scope(lines[0], 'Then', 'keyword.control.vba');
    scope(lines[1], 'Public', 'storage.modifier.vba');
    scope(lines[1], 'PtrSafe', 'storage.modifier.vba');
    scope(lines[5], '[If]', 'variable.other.readwrite.vba');
    scope(lines[5], '#Else', 'string.quoted.double.vba');
    scope(lines[6], '[End]', 'variable.other.readwrite.vba');
});

test('String constructors and suffixes do not turn type names into function calls', async () => {
    const lines = await tokenize('Dim text As String\ntext = String$(2, "x") & String(2, "y")');
    scope(lines[0], 'String', 'storage.type.vba');
    scope(lines[1], 'String$', 'support.function.vba');
    scope(lines[1], 'String(', 'support.function.vba');
});
