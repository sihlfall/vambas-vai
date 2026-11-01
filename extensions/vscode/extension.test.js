const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function harness(failFirstStart) {
    const clients = [];
    const commands = new Map();
    const settings = new Map();
    const errors = [];
    class FakeClient {
        constructor(id, name, server, options) {
            this.server = server;
            this.options = options;
            this.disposals = 0;
            clients.push(this);
        }
        createDefaultErrorHandler() {
            return { closed: () => ({ action: 2 }), error: () => ({ action: 1 }) };
        }
        async start() {
            if (failFirstStart && clients.length === 1) {
                const error = new Error('Invalid project configuration');
                assert.equal(this.options.initializationFailedHandler(error), false);
                throw error;
            }
        }
        async dispose() { this.disposals++; }
    }
    const vscode = {
        workspace: {
            workspaceFolders: [],
            createFileSystemWatcher: () => ({ dispose() {} }),
            getConfiguration: () => ({ get: (name, fallback) => settings.get(name) ?? fallback }),
        },
        commands: { registerCommand: (name, handler) => { commands.set(name, handler); return { dispose() {} }; } },
        window: { showErrorMessage: async message => { errors.push(message); } },
    };
    const sandbox = {
        module: { exports: {} },
        require(name) {
            if (name === 'vscode') return vscode;
            if (name === 'vscode-languageclient/node') return { LanguageClient: FakeClient, TransportKind: { stdio: 0 }, CloseAction: { DoNotRestart: 1 } };
            throw new Error(`Unexpected import ${name}`);
        },
    };
    vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'extension.js'), 'utf8'), sandbox);
    return { extension: sandbox.module.exports, clients, commands, settings, errors };
}

test('initialization failure stops automatic restarts and allows an explicit restart', async () => {
    const h = harness(true);
    // Activation must resolve despite startup failure so the command stays usable.
    await h.extension.activate({ subscriptions: [] });
    assert.equal(h.clients.length, 1);
    assert.equal(h.clients[0].disposals, 1);
    assert.equal(h.clients[0].options.errorHandler.closed().action, 1);
    assert.match(h.errors[0], /Invalid project configuration/);
    h.settings.set('sourceEncoding', 'windows1252');
    await h.commands.get('vba.restartServer')();
    assert.equal(h.clients.length, 2);
    assert.equal(h.clients[1].options.initializationOptions.sourceEncoding, 'windows1252');
    assert.equal(h.clients[1].options.errorHandler.closed().action, 2);
    await h.extension.deactivate();
    assert.equal(h.clients[1].disposals, 1);
});

test('normal startup uses auto decoding and retains the standard crash policy', async () => {
    const h = harness(false);
    await h.extension.activate({ subscriptions: [] });
    assert.equal(h.clients[0].options.initializationOptions.sourceEncoding, 'auto');
    assert.equal(h.clients[0].options.errorHandler.closed().action, 2);
    await h.extension.deactivate();
});
