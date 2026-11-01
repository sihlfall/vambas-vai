import fs from 'node:fs/promises';
import path from 'node:path';

import type {CompilationResult, ExecutionOptionsWithEnv} from '../../types/compilation/compilation.interfaces.js';
import type {AsmResultSource} from '../../types/asmresult/asmresult.interfaces.js';
import type {ParseFiltersAndOutputOptions} from '../../types/features/filters.interfaces.js';
import {BaseCompiler} from '../base-compiler.js';
import {splitArguments} from '../../shared/common-utils.js';
import {BaseParser} from './argument-parsers.js';

export type SourceMap = Record<string, {lineStarts: number[]; file: string | null}>;
export type VbaResult = CompilationResult & {vbaSources?: SourceMap};

// This adapter runs the project's compiler directly. LLVM IR is the primary
// output, with scalar SSA values and phis built by the Go backend itself.
export class VbaCompiler extends BaseCompiler {
    static get key() {
        return 'vba';
    }

    override getArgumentParserClass() {
        return BaseParser;
    }

    override getCompilerResultLanguageId() {
        return 'llvm-ir';
    }

    override isCfgCompiler() {
        return false;
    }

    override prepareArguments(
        userOptions: string[],
        _filters: ParseFiltersAndOutputOptions,
        _backendOptions: Record<string, unknown>,
        inputFilename: string,
        outputFilename: string,
    ) {
        return [
            '-root', path.dirname(inputFilename),
            '-source-encoding', 'utf8',
            ...splitArguments(this.compiler.options || ''),
            ...userOptions,
            '-o', this.filename(outputFilename),
        ];
    }

    override async runCompiler(
        compiler: string,
        options: string[],
        inputFilename: string,
        execOptions: ExecutionOptionsWithEnv,
        _filters?: ParseFiltersAndOutputOptions,
    ): Promise<VbaResult> {
        const root = path.dirname(inputFilename);
        const files = await readSources(root);
        const sources: SourceMap = {};
        for (const [filename, contents] of files) {
            const lineStarts = [0];
            for (let i = 0; i < contents.length; i++) {
                if (contents[i] === 10) lineStarts.push(i + 1);
            }
            sources[filename] = {lineStarts, file: filename === inputFilename ? null : path.relative(root, filename)};
        }
        execOptions = {...(execOptions || this.getDefaultExecOptions()), customCwd: path.dirname(inputFilename)};
        const result = await this.exec(compiler, options, execOptions);
        // vambas-compile diagnoses UTF-8 byte offsets. CE expects line:column.
        result.stderr = result.stderr.split('\n').map(line => {
            const match = /^(.*):(\d+): (.*)$/.exec(line);
            if (!match || !sources[match[1]]) return line;
            const source = files.get(match[1])!;
            const starts = sources[match[1]].lineStarts;
            const offset = Math.min(Number(match[2]), source.length);
            const index = lineIndex(starts, offset);
            const column = source.subarray(starts[index], offset).toString('utf8').length + 1;
            return `${match[1]}:${index + 1}:${column}: error: ${match[3]}`;
        }).join('\n');
        return {
            ...this.transformToCompilationResult(result, inputFilename),
            languageId: this.getCompilerResultLanguageId(),
            instructionSet: 'llvm',
            vbaSources: sources,
        };
    }

    override async processAsm(
        result: {asm: string; vbaSources?: SourceMap},
        filters: ParseFiltersAndOutputOptions,
    ) {
        // Preserve region comments until source links have been attached, even
        // when the user has asked CE to hide comment-only lines.
        const parsed = await this.llvmIr.processFromFilters(result.asm, {...filters, commentOnly: false});
        let source: AsmResultSource | null = null;
        for (const line of parsed.asm) {
            if (/^\s*(define\b|\}|[\w.-]+:)/.test(line.text)) source = null;
            const region = /; S\d+ retry=S\d+ next=S\d+ (.+):(\d+)\.\.\d+/.exec(line.text);
            if (region) {
                const entry = result.vbaSources?.[region[1]];
                source = entry ? {file: entry.file, line: lineIndex(entry.lineStarts, Number(region[2])) + 1} : null;
            }
            if (source) line.source = {...source};
        }
        if (filters.commentOnly) parsed.asm = parsed.asm.filter(line => !/^\s*;/.test(line.text));
        return parsed;
    }
}

async function readSources(root: string): Promise<Map<string, Buffer>> {
    const sources = new Map<string, Buffer>();
    async function visit(directory: string) {
        for (const entry of await fs.readdir(directory, {withFileTypes: true})) {
            const filename = path.join(directory, entry.name);
            if (entry.isDirectory()) await visit(filename);
            else if (entry.isFile() && /\.(bas|cls|frm)$/i.test(entry.name)) {
                sources.set(filename, await fs.readFile(filename));
            }
        }
    }
    await visit(root);
    return sources;
}

function lineIndex(starts: number[], offset: number): number {
    let low = 0;
    let high = starts.length;
    while (low + 1 < high) {
        const middle = (low + high) >>> 1;
        if (starts[middle] <= offset) low = middle;
        else high = middle;
    }
    return low;
}
