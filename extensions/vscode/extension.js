const vscode = require('vscode');
const { LanguageClient, TransportKind, CloseAction } = require('vscode-languageclient/node');

let client;

async function activate(context) {
    if ((vscode.workspace.workspaceFolders?.length ?? 0) > 1) {
        throw new Error('VBA currently supports one workspace folder. Open the project in a separate window.');
    }
    const watcher = vscode.workspace.createFileSystemWatcher('**/*.{bas,BAS,cls,CLS,frm,FRM,vbp,VBP}');
    context.subscriptions.push(watcher);
    async function start() {
        const config = vscode.workspace.getConfiguration('vba');
        let initializationFailed = false;
        let defaultErrorHandler;
        const next = new LanguageClient('vba', 'VBA Language Server', {
            command: config.get('server.path', 'vambas-language-server'),
            transport: TransportKind.stdio,
        }, {
            documentSelector: [{ scheme: 'file', language: 'vba' }],
            initializationOptions: {
                projectFile: config.get('projectFile', ''),
                sourceEncoding: config.get('sourceEncoding', 'auto'),
            },
            synchronize: { fileEvents: watcher },
            initializationFailedHandler: error => {
                initializationFailed = true;
                void vscode.window.showErrorMessage(`VBA server initialization failed: ${error.message}. After fixing the settings, run VBA: Restart Language Server.`);
                return false;
            },
            errorHandler: {
                error: (...args) => defaultErrorHandler.error(...args),
                closed: () => initializationFailed
                    ? { action: CloseAction.DoNotRestart, handled: true, message: 'Initialization failed; restart manually after fixing the project settings.' }
                    : defaultErrorHandler.closed(),
            },
        });
        defaultErrorHandler = next.createDefaultErrorHandler();
        client = next;
        try {
            await next.start();
        } catch {
            // Keep activation successful so the restart command remains available.
            // The language client has already logged the startup error.
            initializationFailed = true;
            await next.dispose().catch(() => {});
        }
    }
    context.subscriptions.push(vscode.commands.registerCommand('vba.restartServer', async () => {
        if (client) await client.dispose().catch(() => {});
        await start();
    }));
    await start();
}

async function deactivate() {
    if (client) await client.dispose().catch(() => {});
}

module.exports = { activate, deactivate };
