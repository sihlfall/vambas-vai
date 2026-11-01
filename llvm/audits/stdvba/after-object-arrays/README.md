# stdVBA compilation audit

Source revision: `4d0ebb32a7161376cf54f0cf84013ee711a718e8`. Audited 2026-10-27T19:25:47.910591+00:00.

All 93 VBA source files were tested unchanged, using the normal project loader and compiler CLI. Each file gets a separate manifest so alternate WIP classes do not collide. Every procedure is compiled, including unreachable code. Successful outputs pass LLVM assembly/verification and native linking at both `-O0` and `-O2` on Linux. The native link driver does not call the library APIs; this is not a functional or Windows interoperability test.

## What compiles

| Source | VB module name | Procedures | Assessment |
|---|---|---:|---|
| `WIP/stdDictionary/stdIHashable.cls` | `stdIHashable` | 2 | Interface: two empty member bodies. |
| `WIP/stdGithub.cls` | `stdGithub` | 4 | WIP stub: four empty member bodies. |
| `WIP/stdStruct.cls` | `stdCStruct` | 0 | Comment-only skeleton; VB_Name is stdCStruct; no procedures. |
| `stdICallable.cls` | `stdICallable` | 4 | Interface: four empty member bodies. |
| `stdQuadTree.cls` | `stdQuadTree` | 22 |  |

2 of 27 top-level distribution modules compile; 3 additional successes are under WIP. Dependency retries produced 1 additional successes. Compilation/linking alone does not establish functional API support.

## First blocking stage

| Group | Files | Parse | Binding | Checking | Lowering | Compiled |
|---|---:|---:|---:|---:|---:|---:|
| top-level | 27 | 22 | 1 | 2 | 0 | 2 |
| WIP | 66 | 51 | 2 | 8 | 2 | 3 |

Stages describe the earliest observed blocker, not an exhaustive list of missing features. The CLI also reports recovery and downstream diagnostics; diagnostic counts are not independent defects. Dependencies are added only for unknown declared project types, preferring top-level modules and then same-directory WIP modules. Ambiguous types and external Office/COM references are never stubbed. Late-bound dependencies may remain undetected. A top-level-only project build also fails. A recursive `-root ../stdvba/src` additionally mixes alternate WIP implementations.

## Useful next candidates

- `stdRegex.cls` (binding): Member "Replace" does not exist in "VBA". This is only the first observed gate, not a complete support claim.
- Most other top-level modules first hit conditional compilation, `Declare`/`PtrSafe` DLL imports, events/`Implements`, or other unsupported syntax. Office and external COM dependencies are additional obstacles.

## Per-module results

Dependency results below are retries, not replacements for the standalone stage. See [results.json](results.json) for hashes, diagnostic counts/codes, excerpts, and selected dependency source lists.

| Source | Standalone | With typed dependencies | First diagnostic |
|---|---|---|---|
| `WIP/JSONLogic.cls` | parse | — | WIP/JSONLogic.cls:31: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/Pointers/stdPointer v2.cls` | parse | — | WIP/Pointers/stdPointer v2.cls:17: Identifier expected |
| `WIP/Pointers/stdPointer.cls` | parse | — | WIP/Pointers/stdPointer.cls:24: Identifier expected |
| `WIP/STD_Runtimes_CLR.cls` | parse | — | WIP/STD_Runtimes_CLR.cls:20: Identifier expected |
| `WIP/stdCOMDispatch.cls` | parse | — | WIP/stdCOMDispatch.cls:65: Identifier expected |
| `WIP/stdCSV/CSVinterface.cls` | parse | — | WIP/stdCSV/CSVinterface.cls:37: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdCSV/stdCSV.cls` | parse | — | WIP/stdCSV/stdCSV.cls:274: , expected |
| `WIP/stdCallableScheduler.bas` | parse | — | WIP/stdCallableScheduler.bas:14: Function expected |
| `WIP/stdCallback2.cls` | parse | — | WIP/stdCallback2.cls:40: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdCanvas.cls` | parse | — | WIP/stdCanvas.cls:58: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdCrypto/MD5.cls` | parse | — | WIP/stdCrypto/MD5.cls:32: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdCrypto/stdCrypt.cls` | parse | — | WIP/stdCrypto/stdCrypt.cls:21: Identifier expected |
| `WIP/stdDLL/inspiration/cUniversalDLLCalls.cls` | parse | — | WIP/stdDLL/inspiration/cUniversalDLLCalls.cls:16: Identifier expected |
| `WIP/stdDLL/stdDLLFunc.cls` | parse | — | WIP/stdDLL/stdDLLFunc.cls:11: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdDate.cls` | parse | — | WIP/stdDate.cls:125: Then expected |
| `WIP/stdDictionary/SS/stdDictionary2.cls` | parse | — | WIP/stdDictionary/SS/stdDictionary2.cls:335: Statement expected; this syntax may not be supported yet |
| `WIP/stdDictionary/stdDictionary.cls` | parse | — | WIP/stdDictionary/stdDictionary.cls:86:  expected |
| `WIP/stdDictionary/stdIHashable.cls` | compiled | — | Interface: two empty member bodies. |
| `WIP/stdDictionary/trick/stdDictionary.cls` | parse | — | WIP/stdDictionary/trick/stdDictionary.cls:29: Identifier expected |
| `WIP/stdEndpoint.cls` | parse | — | WIP/stdEndpoint.cls:11: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdEnumProvider/FibonacciIteratorExample.cls` | checking | — | WIP/stdEnumProvider/FibonacciIteratorExample.cls:18: Type "stdEnumProvider" is not defined in this project |
| `WIP/stdEnumProvider/STD_Types_IniVariantEnum.bas` | parse | — | WIP/stdEnumProvider/STD_Types_IniVariantEnum.bas:29: Identifier expected |
| `WIP/stdError/stdDebug.cls` | parse | — | WIP/stdError/stdDebug.cls:78: ) expected |
| `WIP/stdError/stdError.cls` | parse | — | WIP/stdError/stdError.cls:12: Identifier expected |
| `WIP/stdError.cls` | lowering | — | WIP/stdError.cls:115: Semantic IR does not yet support statement kind 279 |
| `WIP/stdErrorWIP.cls` | checking | parse | stdArray.cls:73: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdExcelLibraries/stdExcelWorkbook.cls` | checking | — | WIP/stdExcelLibraries/stdExcelWorkbook.cls:13: Type "workbook" is not defined in this project |
| `WIP/stdExcelLibraries/stdExcelWorksheet.cls` | binding | — | WIP/stdExcelLibraries/stdExcelWorksheet.cls:34: Exit does not have a matching enclosing loop or procedure |
| `WIP/stdExcelLibraries/stdShapeEvents.cls` | parse | — | WIP/stdExcelLibraries/stdShapeEvents.cls:52: , expected |
| `WIP/stdExcelLibraries/stdXLEvents.cls` | parse | — | WIP/stdExcelLibraries/stdXLEvents.cls:10: Identifier expected |
| `WIP/stdGithub.cls` | compiled | — | WIP stub: four empty member bodies. |
| `WIP/stdHTTPServer.cls` | parse | — | WIP/stdHTTPServer.cls:27: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdICallableScheduler.bas` | checking | lowering | WIP/stdICallableScheduler.bas:17: Semantic IR does not yet support undeclared storage |
| `WIP/stdIResource.cls` | parse | — | WIP/stdIResource.cls:51: A property's assignment value cannot be Optional, ParamArray, or have a default |
| `WIP/stdIShellExtension.cls` | checking | parse | WIP/stdShell.cls:857: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdIni.cls` | parse | — | WIP/stdIni.cls:57: , expected |
| `WIP/stdMath.cls` | parse | — | WIP/stdMath.cls:66: , expected |
| `WIP/stdObject/IFauxInterface.cls` | parse | — | WIP/stdObject/IFauxInterface.cls:104: Identifier expected |
| `WIP/stdObject/stdObject.cls` | parse | — | WIP/stdObject/stdObject.cls:103: Identifier expected |
| `WIP/stdOpenCL/stdOpenCL.cls` | parse | — | WIP/stdOpenCL/stdOpenCL.cls:39: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdPowerPointLibraries/ShapeHook.cls` | parse | — | WIP/stdPowerPointLibraries/ShapeHook.cls:10: , expected |
| `WIP/stdRegex2.cls` | parse | — | WIP/stdRegex2.cls:121: Function expected |
| `WIP/stdResource.cls` | parse | — | WIP/stdResource.cls:12: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdRibbon.cls` | parse | — | WIP/stdRibbon.cls:12: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdRuntimeJS.cls` | parse | — | WIP/stdRuntimeJS.cls:24: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdSettings.cls` | parse | — | WIP/stdSettings.cls:10: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdSharepointSite.cls` | lowering | — | WIP/stdSharepointSite.cls:49: Semantic IR does not yet support conversion from String to Long |
| `WIP/stdShell.cls` | parse | — | WIP/stdShell.cls:857: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdShellMTP.cls` | binding | — | WIP/stdShellMTP.cls:79: Member "GetFolder" does not exist in "TThis" |
| `WIP/stdString.cls` | parse | — | WIP/stdString.cls:12: Identifier expected |
| `WIP/stdStringBuilder.cls` | parse | — | WIP/stdStringBuilder.cls:168: Unterminated string literal |
| `WIP/stdStruct.cls` | compiled | — | Comment-only skeleton; VB_Name is stdCStruct; no procedures. |
| `WIP/stdTable/stdITable.cls` | parse | — | WIP/stdTable/stdITable.cls:20: Identifier expected |
| `WIP/stdTable/stdITableRow.cls` | checking | parse | WIP/stdTable/stdITable.cls:20: Identifier expected |
| `WIP/stdTable/stdRow v0.0.1.cls` | parse | — | WIP/stdTable/stdRow v0.0.1.cls:21: Then expected |
| `WIP/stdTable/stdTable.cls` | parse | — | WIP/stdTable/stdTable.cls:11: Identifier expected |
| `WIP/stdTable/stdXLEvents.cls` | parse | — | WIP/stdTable/stdXLEvents.cls:10: Identifier expected |
| `WIP/stdTable/stdXLTable.cls` | parse | — | WIP/stdTable/stdXLTable.cls:10: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdTable/stdXLTableRow.cls` | parse | — | WIP/stdTable/stdXLTableRow.cls:10: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdTimer.cls` | parse | — | WIP/stdTimer.cls:55: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdTimerServerWIP.bas` | parse | — | WIP/stdTimerServerWIP.bas:6: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdUIAutomationElement.cls` | parse | — | WIP/stdUIAutomationElement.cls:13: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdWebSocket/stdWebSocket.cls` | parse | — | WIP/stdWebSocket/stdWebSocket.cls:63: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdWebSocket/stdWebSocketNew.cls` | parse | — | WIP/stdWebSocket/stdWebSocketNew.cls:63: Module declaration or procedure expected; this syntax may not be supported yet |
| `WIP/stdWordLibraries/stdWordDocument.cls` | checking | — | WIP/stdWordLibraries/stdWordDocument.cls:13: Type "Range" is not defined in this project |
| `WIP/stdXML.cls` | checking | — | WIP/stdXML.cls:89: Type "ISerialisable" is not defined in this project |
| `stdAcc.cls` | parse | — | stdAcc.cls:105: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdArray.cls` | parse | — | stdArray.cls:73: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdCOM.cls` | parse | — | stdCOM.cls:79: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdCallback.cls` | parse | — | stdCallback.cls:11: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdClipboard.cls` | parse | — | stdClipboard.cls:93: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdEnumerator.cls` | parse | — | stdEnumerator.cls:190: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdFiber.cls` | checking | compiled |  |
| `stdHTML.cls` | parse | — | stdHTML.cls:72: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdHTTP.cls` | parse | — | stdHTTP.cls:120: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdHTTPAuthenticators.bas` | checking | parse | stdHTTP.cls:120: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdICallable.cls` | compiled | — | Interface: four empty member bodies. |
| `stdImage.cls` | parse | — | stdImage.cls:146: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdJSON.cls` | parse | — | stdJSON.cls:65: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdLambda.cls` | parse | — | stdLambda.cls:62: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdPerformance.cls` | parse | — | stdPerformance.cls:32: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdProcess.cls` | parse | — | stdProcess.cls:35: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdQuadTree.cls` | compiled | — |  |
| `stdRefArray.cls` | parse | — | stdRefArray.cls:22: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdReg.cls` | parse | — | stdReg.cls:131: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdRegex.cls` | binding | — | stdRegex.cls:468: Member "Replace" does not exist in "VBA" |
| `stdRibbon.cls` | parse | — | stdRibbon.cls:11: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdSentry.cls` | parse | — | stdSentry.cls:85: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdTimer.cls` | parse | — | stdTimer.cls:42: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdUIElement.cls` | parse | — | stdUIElement.cls:62: , expected |
| `stdWebSocket.cls` | parse | — | stdWebSocket.cls:39: Identifier expected |
| `stdWebView.cls` | parse | — | stdWebView.cls:42: Module declaration or procedure expected; this syntax may not be supported yet |
| `stdWindow.cls` | parse | — | stdWindow.cls:96: Module declaration or procedure expected; this syntax may not be supported yet |

## Reproduce

```sh
python3 llvm/audits/stdvba/run.py --source ../stdvba/src --output /tmp/stdvba-audit
```

Requires Go, Clang, llvm-as, and opt (version 20 preferred). Set `GOCACHE` if the default cache is not writable. `--full-diagnostics /tmp/stdvba-diagnostics` preserves full compiler messages. All manifests, binaries, and intermediate LLVM files use temporary directories; the stdVBA checkout is read-only.
