import path from 'node:path';

import type {SourceMap} from './vba.js';

// Inspection-only metadata: keep private source procedures observable under
// optimization and carry our existing region provenance through Clang codegen.
// The ordinary LLVM/SSA views retain exactly the compiler's emitted module.
export function prepareNativeIR(ir: string, sources: SourceMap, inputFilename: string, optimized: boolean): string {
    const lines = ir.split('\n');
    const metadata: string[] = [];
    let next = Math.max(-1, ...Array.from(ir.matchAll(/^!(\d+)\s*=/gm), match => Number(match[1]))) + 1;
    const node = (text: string) => {
        const id = next++;
        metadata.push(`!${id} = ${text}`);
        return `!${id}`;
    };
    const files = new Map<string, string>();
    const file = (filename: string) => {
        if (!files.has(filename)) files.set(filename, node(`!DIFile(filename: ${quote(path.basename(filename))}, directory: ${quote(path.dirname(filename))})`));
        return files.get(filename)!;
    };
    // DWARF has no VBA language tag. C's tag is used only for line tables;
    // these nodes describe no C types, ABI, or variable semantics.
    const unit = node(`distinct !DICompileUnit(language: DW_LANG_C, file: ${file(inputFilename)}, producer: "vambas-compile / Compiler Explorer", isOptimized: ${optimized}, runtimeVersion: 0, emissionKind: LineTablesOnly)`);
    const types = node('!{null}');
    const signature = node(`!DISubroutineType(types: ${types})`);
    const debugVersion = node('!{i32 2, !"Debug Info Version", i32 3}');
    const dwarfVersion = node('!{i32 2, !"Dwarf Version", i32 4}');
    const retained: string[] = [];
    const regionPattern = /; S\d+ retry=S\d+ next=S\d+ (.+):(\d+)\.\.\d+/;
    const labelPattern = /^[\w$.-]+:/;
    const sourceLine = (filename: string, offset: number) => {
        const starts = sources[filename]?.lineStarts || [0];
        let low = 0;
        let high = starts.length;
        while (low + 1 < high) {
            const middle = (low + high) >>> 1;
            if (starts[middle] <= offset) low = middle;
            else high = middle;
        }
        return low + 1;
    };

    for (let i = 0; i < lines.length; i++) {
        const definition = /^define internal i32 @(vba\.p\d+)\(/.exec(lines[i]);
        if (!definition) continue;
        let end = i + 1;
        while (end < lines.length && lines[end] !== '}') end++;
        const regions = lines.slice(i + 1, end).flatMap(line => {
            const match = regionPattern.exec(line);
            return match ? [{filename: match[1], offset: Number(match[2])}] : [];
        });
        // The synthetic return region includes the procedure's declaration
        // range, whose start precedes the starts of its body statements.
        const filename = regions[0]?.filename || inputFilename;
        const offset = Math.min(...regions.filter(region => region.filename === filename).map(region => region.offset), Number.MAX_SAFE_INTEGER);
        const declarationLine = sourceLine(filename, offset === Number.MAX_SAFE_INTEGER ? 0 : offset);
        const name = /^; (.+)$/.exec(lines[i - 1] || '')?.[1] || definition[1];
        const scope = node(`distinct !DISubprogram(name: ${quote(name)}, linkageName: ${quote(definition[1])}, scope: ${file(filename)}, file: ${file(filename)}, line: ${declarationLine}, type: ${signature}, scopeLine: ${declarationLine}, spFlags: DISPFlagDefinition${optimized ? ' | DISPFlagOptimized' : ''}, unit: ${unit})`);
        const locations = new Map<number, string>();
        const location = (line: number) => {
            if (!locations.has(line)) locations.set(line, node(`!DILocation(line: ${line}, column: 1, scope: ${scope})`));
            return locations.get(line)!;
        };
        lines[i] = lines[i].replace(/ \{$/, ` !dbg ${scope} {`);
        retained.push(`ptr @${definition[1]}`);
        let currentLine = declarationLine;
        let inSwitch = false;
        for (let j = i + 1; j < end; j++) {
            if (labelPattern.test(lines[j])) {
                // Phi nodes precede their region comment. Use that block's
                // provenance for them as well as for its later instructions.
                for (let k = j + 1; k < end && !labelPattern.test(lines[k]); k++) {
                    const region = regionPattern.exec(lines[k]);
                    if (region) { currentLine = sourceLine(region[1], Number(region[2])); break; }
                }
            }
            const region = regionPattern.exec(lines[j]);
            if (region) currentLine = sourceLine(region[1], Number(region[2]));
            if (/^\s+switch\b/.test(lines[j])) { inSwitch = true; continue; }
            if (inSwitch) {
                if (/^\s*\]/.test(lines[j])) {
                    lines[j] += `, !dbg ${location(currentLine)}`;
                    inSwitch = false;
                }
                continue;
            }
            if (/^\s+[^\s;]/.test(lines[j])) lines[j] += `, !dbg ${location(currentLine)}`;
        }
        i = end;
    }
    if (retained.length) lines.push(`@llvm.used = appending global [${retained.length} x ptr] [${retained.join(', ')}], section "llvm.metadata"`);
    lines.push(`!llvm.dbg.cu = !{${unit}}`, `!llvm.module.flags = !{${debugVersion}, ${dwarfVersion}}`, ...metadata);
    return lines.join('\n');
}

function quote(text: string): string {
    let escaped = '"';
    for (const byte of Buffer.from(text)) {
        escaped += byte >= 32 && byte < 127 && byte !== 34 && byte !== 92 ? String.fromCharCode(byte) : `\\${byte.toString(16).padStart(2, '0')}`;
    }
    return escaped + '"';
}
