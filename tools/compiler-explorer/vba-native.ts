import fs from 'node:fs/promises';
import path from 'node:path';

import type {ExecutionOptionsWithEnv} from '../../types/compilation/compilation.interfaces.js';
import type {ParseFiltersAndOutputOptions} from '../../types/features/filters.interfaces.js';
import {prepareNativeIR} from './vba-native-ir.js';
import {VbaCompiler, type VbaResult} from './vba.js';

const optimizationOption = /^-O[0-3sgz]$/;

export class VbaNativeCompiler extends VbaCompiler {
    static override get key() {
        return 'vba-native';
    }

    override getCompilerResultLanguageId() {
        return 'asm';
    }

    override async runCompiler(
        compiler: string,
        options: string[],
        inputFilename: string,
        execOptions: ExecutionOptionsWithEnv,
        filters?: ParseFiltersAndOutputOptions,
    ): Promise<VbaResult> {
        const optimizations = options.filter(option => optimizationOption.test(option));
        const optimization = optimizations.at(-1) || '-O2';
        const vbaOptions = options.filter(option => !optimizationOption.test(option));
        const outputIndex = vbaOptions.lastIndexOf('-o') + 1;
        const outputFilename = vbaOptions[outputIndex];
        const llvmFilename = path.join(path.dirname(inputFilename), 'vba-native.ll');
        vbaOptions[outputIndex] = llvmFilename;
        const frontend = await super.runCompiler(compiler, vbaOptions, inputFilename, execOptions, filters);
        if (frontend.code !== 0) return frontend;

        const llvm = await fs.readFile(llvmFilename, 'utf8');
        await fs.writeFile(llvmFilename, prepareNativeIR(llvm, frontend.vbaSources || {}, inputFilename, optimization !== '-O0'));
        const clang = this.compilerProps<string>(`compiler.${this.compiler.id}.clang`);
        const args = ['-S', '-x', 'ir', optimization, '-gline-tables-only', '-Wno-override-module'];
        if (this.compiler.supportsIntel) args.push(filters?.intel ? '-masm=intel' : '-masm=att');
        args.push(llvmFilename, '-o', outputFilename);
        const native = await this.exec(clang, args, {
            ...(execOptions || this.getDefaultExecOptions()), customCwd: path.dirname(inputFilename),
        });
        const result = this.transformToCompilationResult(native, inputFilename);
        return {
            ...frontend, ...result,
            stdout: [...frontend.stdout, ...result.stdout],
            stderr: [...frontend.stderr, ...result.stderr],
            execTime: (frontend.execTime || 0) + native.execTime,
            languageId: 'asm',
            instructionSet: this.compiler.instructionSet || undefined,
            compilationOptions: [...vbaOptions, '→', clang, ...args],
        };
    }

    override async processAsm(result: {asm: string}, filters: ParseFiltersAndOutputOptions) {
        return {...this.asm.process(result.asm, filters), languageId: 'asm'};
    }
}
