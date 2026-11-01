import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {test} from 'node:test';
import {fileURLToPath} from 'node:url';

import {prepareNativeIR} from './native-ir.js';
import type {SourceMap} from './vba.js';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const config = fs.readFileSync(path.join(here, '.work/upstream/etc/config/vba.local.properties'), 'utf8');
const clang = /^compiler.vba-native.clang=(.+)$/m.exec(config)?.[1];
const compiler = path.join(here, '.work/bin/vambas-compile');

function run(exe: string, args: string[]) {
    const result = spawnSync(exe, args, {encoding: 'utf8', timeout: 30000, maxBuffer: 16 * 1024 * 1024});
    assert.ifError(result.error);
    assert.equal(result.status, 0, `${exe}: ${result.stderr}`);
    return result;
}

for (const example of fs.readdirSync(path.join(root, 'examples'), {withFileTypes: true}).filter(entry => entry.isDirectory())) {
    test(`Clang accepts ${example.name} with retained procedures and native line tables`, {skip: !clang && 'Clang 20+ not configured'}, () => {
        const directory = path.join(root, 'examples', example.name);
        const sources: SourceMap = {};
        const visit = (dir: string) => {
            for (const entry of fs.readdirSync(dir, {withFileTypes: true})) {
                const filename = path.join(dir, entry.name);
                if (entry.isDirectory()) visit(filename);
                else if (/\.(bas|cls|frm)$/i.test(entry.name)) {
                    const contents = fs.readFileSync(filename);
                    const lineStarts = [0];
                    for (let i = 0; i < contents.length; i++) if (contents[i] === 10) lineStarts.push(i + 1);
                    sources[filename] = {lineStarts, file: path.relative(directory, filename)};
                }
            }
        };
        visit(directory);
        const input = Object.keys(sources)[0];
        const emitted = run(compiler, ['-root', directory]);
        const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'vba-ce-native-'));
        try {
            const llvm = path.join(temp, 'module.ll');
            const assembly = path.join(temp, 'module.s');
            for (const optimization of ['-O0', '-O2']) {
                fs.writeFileSync(llvm, prepareNativeIR(emitted.stdout, sources, input, optimization !== '-O0'));
                const result = run(clang!, ['-S', '-x', 'ir', optimization, '-gline-tables-only', '-Wno-override-module', llvm, '-o', assembly]);
                assert(!/invalid debug info|ignoring debug info/i.test(result.stderr), result.stderr);
                const text = fs.readFileSync(assembly, 'utf8');
                const procedures = Array.from(emitted.stdout.matchAll(/^define internal i32 @(vba\.p\d+)\(/gm), match => match[1]);
                for (const procedure of procedures) {
                    assert(text.includes(`${procedure}:`), `${optimization} discarded ${procedure}`);
                }
                assert(/\.loc\s+\d+\s+\d+/.test(text), 'Native assembly has no source locations');
            }
        } finally {
            fs.rmSync(temp, {recursive: true, force: true});
        }
    });
}
