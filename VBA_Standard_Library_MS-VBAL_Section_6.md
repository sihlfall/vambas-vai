# VBA Standard Library — complete name index (MS-VBAL §6)

**Sources:** [Microsoft Learn — §6 VBA Standard Library](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/c645c903-9bd4-4849-8735-3136e867536a); [MS-VBAL specification, v20241112 (PDF)](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf); [specification version history](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/d5418146-0bd2-45eb-9c7a-fd9502722c74).

**Scope:** All names explicitly defined in §6.1 of the linked Microsoft *VBA Language Specification*: predefined enums and their members; global procedural modules, their constants, functions, subroutines, and properties; and predefined classes/members. Names ending in `$` and byte-oriented (`B`) variants are indexed individually. Host application object models (Excel, Word, Access, etc.) and syntax/operators elsewhere in the language specification are **not** part of this inventory.

**Version note:** This is checked against the **12 November 2024 (revision 2.2)** specification, whose PDF is accessible page-by-page. Microsoft lists later revisions (including 18 August 2026); changes made only in those newer revisions have not been independently verified here. Clicking any name opens the applicable numbered PDF page. Values are shown for constants; descriptions are deliberately brief.

## Contents

- [1. Predefined enums and constants (§6.1.1)](#1-predefined-enums-and-constants-611)
- [2. Predefined procedural modules (§6.1.2)](#2-predefined-procedural-modules-612)
- [3. Predefined classes (§6.1.3)](#3-predefined-classes-613)

## 1. Predefined enums and constants (§6.1.1)

### 6.1.1.1 [`FormShowConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — Form display mode

| Name | Value |
|---|---:|
| [`vbModal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` |
| [`vbModeless`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `0` |

### 6.1.1.2 [`VbAppWinStyle`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — External application window style

| Name | Value |
|---|---:|
| [`vbHide`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `0` |
| [`vbMaximizedFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `3` |
| [`vbMinimizedFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `2` |
| [`vbMinimizedNoFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `6` |
| [`vbNormalFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` |
| [`vbNormalNoFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `4` |

### 6.1.1.3 [`VbCalendar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — Calendar type

| Name | Value |
|---|---:|
| [`vbCalGreg`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `0` |
| [`vbCalHijri`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` |

### 6.1.1.4 [`VbCallType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — `CallByName` invocation type

| Name | Value |
|---|---:|
| [`vbGet`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `2` |
| [`vbLet`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `4` |
| [`vbMethod`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` |
| [`vbSet`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `8` |

### 6.1.1.5 [`VbCompareMethod`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — String comparison type

| Name | Value |
|---|---:|
| [`vbBinaryCompare`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` |
| [`vbTextCompare`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `1` |

### 6.1.1.6 [`VbDateTimeFormat`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — Built-in date/time display formats

| Name | Value |
|---|---:|
| [`vbGeneralDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` |
| [`vbLongDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `1` |
| [`vbLongTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `3` |
| [`vbShortDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `2` |
| [`vbShortTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `4` |

### 6.1.1.7 [`VbDayOfWeek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — Day-of-week and first-day settings

| Name | Value |
|---|---:|
| [`vbFriday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `6` |
| [`vbMonday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `2` |
| [`vbSaturday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `7` |
| [`vbSunday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `1` |
| [`vbThursday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `5` |
| [`vbTuesday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `3` |
| [`vbUseSystemDayOfWeek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` |
| [`vbWednesday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `4` |

### 6.1.1.8 [`VbFileAttribute`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — File attribute flags

| Name | Value |
|---|---:|
| [`vbNormal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` |
| [`vbReadOnly`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `1` |
| [`vbHidden`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `2` |
| [`VbSystem`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `4` |
| [`vbVolume`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `8` |
| [`vbDirectory`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `16` |
| [`vbArchive`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `32` |
| [`vbAlias`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `64` |

### 6.1.1.9 [`VbFirstWeekOfYear`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) — First-week-of-year rules

| Name | Value |
|---|---:|
| [`vbFirstFourDays`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `2` |
| [`vbFirstFullWeek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `3` |
| [`vbFirstJan1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `1` |
| [`vbUseSystem`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `0` |

### 6.1.1.10 [`VbIMEStatus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) — Input Method Editor modes

| Name | Value |
|---|---:|
| [`vbIMEAlphaDbl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `7` |
| [`vbIMEAlphaSng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `8` |
| [`vbIMEDisable`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `3` |
| [`vbIMEHiragana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `4` |
| [`vbIMEKatakanaDbl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `5` |
| [`vbIMEKatakanaSng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `6` |
| [`vbIMEModeAlpha`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `8` |
| [`vbIMEModeAlphaFull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `7` |
| [`vbIMEModeDisable`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `3` |
| [`vbIMEModeHangul`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `10` |
| [`vbIMEModeHangulFull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `9` |
| [`vbIMEModeHiragana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `4` |
| [`vbIMEModeKatakana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `5` |
| [`vbIMEModeKatakanaHalf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `6` |
| [`vbIMEModeNoControl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` |
| [`vbIMEModeOff`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` |
| [`vbIMEModeOn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1` |
| [`vbIMENoOp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` |
| [`vbIMEOff`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` |
| [`vbIMEOn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1` |

### 6.1.1.11 [`VbMsgBoxResult`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) — Message-box button results

| Name | Value |
|---|---:|
| [`vbAbort`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `3` |
| [`vbCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` |
| [`vbIgnore`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `5` |
| [`vbNo`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `7` |
| [`vbOK`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1` |
| [`vbRetry`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `4` |
| [`vbYes`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `6` |

### 6.1.1.12 [`VbMsgBoxStyle`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) — Message-box flags (buttons, icons, modality and position)

| Name | Value |
|---|---:|
| [`vbAbortRetryIgnore`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` |
| [`vbApplicationModal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` |
| [`vbCritical`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `16` |
| [`vbDefaultButton1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` |
| [`vbDefaultButton2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `256` |
| [`vbDefaultButton3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `512` |
| [`vbDefaultButton4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `768` |
| [`vbExclamation`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `48` |
| [`vbInformation`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `64` |
| [`vbMsgBoxHelpButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `16384` |
| [`vbMsgBoxRight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `524288` |
| [`vbMsgBoxRtlReading`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1048576` |
| [`vbMsgBoxSetForeground`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `65536` |
| [`vbOKCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `1` |
| [`vbOKOnly`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `0` |
| [`vbQuestion`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `32` |
| [`vbRetryCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `5` |
| [`vbSystemModal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4096` |
| [`vbYesNo`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4` |
| [`vbYesNoCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `3` |

### 6.1.1.13 [`VbQueryClose`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) — Form-closing reasons

| Name | Value |
|---|---:|
| [`vbAppTaskManager`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `3` |
| [`vbAppWindows`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `2` |
| [`vbFormCode`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `1` |
| [`vbFormControlMenu`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `0` |
| [`vbFormMDIForm`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4` |

### 6.1.1.14 [`VbStrConv`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) — String conversion flags

| Name | Value |
|---|---:|
| [`vbFromUnicode`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `128` |
| [`vbHiragana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `32` |
| [`vbKatakana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `16` |
| [`vbLowerCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `2` |
| [`vbNarrow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `8` |
| [`vbProperCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `3` |
| [`vbUnicode`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `64` |
| [`vbUpperCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `1` |
| [`vbWide`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4` |

### 6.1.1.15 [`VbTriState`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) — Three-way Boolean/default setting

| Name | Value |
|---|---:|
| [`vbFalse`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `0` |
| [`vbTrue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `-1` |
| [`vbUseDefault`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `-2` |

### 6.1.1.16 [`VbVarType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) — Value-type codes returned by `VarType`

| Name | Value |
|---|---:|
| [`vbArray`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `8192` |
| [`vbBoolean`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `11` |
| [`vbByte`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `17` |
| [`vbCurrency`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `6` |
| [`vbDataObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `13` |
| [`vbDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `7` |
| [`vbDecimal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `14` |
| [`vbDouble`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `5` |
| [`vbEmpty`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `0` |
| [`vbError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `10` |
| [`vbInteger`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `2` |
| [`vbLong`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `3` |
| [`vbLongLong`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `20` |
| [`vbNull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `1` |
| [`vbObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `9` |
| [`vbSingle`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `4` |
| [`vbString`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `8` |
| [`vbUserDefinedType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `36` |
| [`vbVariant`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `12` |

## 2. Predefined procedural modules (§6.1.2)

### 6.1.2.1 [`ColorConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) — Eight predefined RGB color values.

| Constant | Value | Meaning |
|---|---|---|
| [`vbBlack`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `0` | — |
| [`vbBlue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16711680` | — |
| [`vbCyan`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16776960` | — |
| [`vbGreen`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `65280` | — |
| [`vbMagenta`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16711935` | — |
| [`vbRed`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `255` | — |
| [`vbWhite`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16777215` | — |
| [`vbYellow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `65535` | — |

### 6.1.2.2 [`Constants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) — Special-character strings and an object error base.

| Constant | Value | Meaning |
|---|---|---|
| [`vbBack`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(8)` | Backspace |
| [`vbCr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(13)` | Carriage return |
| [`vbCrLf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(13) + Chr$(10)` | Windows line ending |
| [`vbFormFeed`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(12)` | Form feed |
| [`vbLf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(10)` | Line feed |
| [`vbNewLine`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `implementation-defined` | Line separator |
| [`vbNullChar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(0)` | Null character |
| [`vbTab`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(9)` | Horizontal tab |
| [`vbVerticalTab`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(11)` | Vertical tab |
| [`vbNullString`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `null string pointer` | Null-string value |
| [`vbObjectError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `-2147221504` | Object error code base |

### 6.1.2.3 [`Conversion`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) — Explicit type conversions and numeric-to-string conversions.

**Functions**

| Name | Brief description |
|---|---|
| [`CBool`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | Convert to Boolean |
| [`CByte`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=181) | Convert to Byte |
| [`CCur`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=181) | Convert to Currency |
| [`CDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=182) | Convert to Date |
| [`CVDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=182) | Date conversion variant |
| [`CDbl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=182) | Convert to Double |
| [`CDec`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=183) | Convert to Decimal Variant |
| [`CInt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=183) | Convert to Integer |
| [`CLng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=183) | Convert to Long |
| [`CLngLng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=184) | Convert to LongLong (when supported) |
| [`CLngPtr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=184) | Convert to pointer-sized integer |
| [`CSng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=185) | Convert to Single |
| [`CStr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=185) | Convert to String |
| [`CVar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=185) | Convert to Variant |
| [`CVErr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=186) | Create Error Variant |
| [`Error`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=186) | Error message for an error number |
| [`Error$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=186) | String-returning error message variant |
| [`Fix`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=187) | Truncate fractional part toward zero |
| [`Hex`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=188) | Hexadecimal representation |
| [`Hex$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=188) | String-returning hexadecimal variant |
| [`Int`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=188) | Round down to an integer |
| [`Oct`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=189) | Octal representation |
| [`Oct$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=189) | String-returning octal variant |
| [`Str`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=190) | Convert number to string |
| [`Str$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=190) | String-returning numeric conversion |
| [`Val`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=190) | Parse leading numeric text |

### 6.1.2.4 [`DateTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=191) — Date/time extraction, arithmetic and clock properties.

**Functions**

| Name | Brief description |
|---|---|
| [`DateAdd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=191) | Add interval to a date |
| [`DateDiff`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=192) | Count intervals between dates |
| [`DatePart`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=194) | Extract a date/time component |
| [`DateSerial`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=195) | Build a date from year/month/day |
| [`DateValue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=196) | Convert date text into a date |
| [`Day`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=196) | Day of month |
| [`Hour`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=197) | Hour of day |
| [`Minute`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=197) | Minute component |
| [`Month`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=197) | Month number |
| [`Second`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=198) | Second component |
| [`TimeSerial`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=198) | Build time from hour/minute/second |
| [`TimeValue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=199) | Convert time text into a time |
| [`Weekday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=199) | Day-of-week number |
| [`Year`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=200) | Year component |

**Properties**

| Name | Brief description |
|---|---|
| [`Calendar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=200) | Current calendar selection |
| [`Date`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | Current system date; can be set |
| [`Date$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | String-returning date property |
| [`Now`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | Current date and time |
| [`Time`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | Current system time; can be set |
| [`Time$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | String-returning time property |
| [`Timer`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | Seconds since midnight |

### 6.1.2.5 [`FileSystem`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) — File, directory and sequential-file access.

**Functions**

| Name | Brief description |
|---|---|
| [`CurDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | Current directory |
| [`CurDir$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | String-returning current directory |
| [`Dir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | File/directory name search |
| [`EOF`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=203) | End-of-file flag |
| [`FileAttr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=203) | Open-file mode |
| [`FileDateTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=204) | File timestamp |
| [`FileLen`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=204) | File length in bytes |
| [`FreeFile`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=205) | Unused file number |
| [`Loc`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=205) | Current file position |
| [`LOF`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=206) | Length of open file |
| [`Seek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=206) | Position in open file |

**Subroutines**

| Name | Brief description |
|---|---|
| [`ChDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=207) | Change current directory |
| [`ChDrive`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=207) | Change current drive |
| [`FileCopy`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=207) | Copy a file |
| [`Kill`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=208) | Delete file(s) |
| [`MkDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=208) | Create directory |
| [`RmDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=209) | Remove directory |
| [`SetAttr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=209) | Set file attributes |

### 6.1.2.6 [`Financial`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=210) — Finance, depreciation, interest, payments and valuation.

**Functions**

| Name | Brief description |
|---|---|
| [`DDB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=210) | Double-declining-balance depreciation |
| [`FV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=210) | Future value |
| [`IPmt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=211) | Interest portion of a payment |
| [`IRR`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=212) | Internal rate of return |
| [`MIRR`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=213) | Modified internal rate of return |
| [`NPer`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=213) | Number of payment periods |
| [`NPV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=214) | Net present value |
| [`Pmt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=215) | Periodic payment amount |
| [`PPmt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=215) | Principal portion of a payment |
| [`PV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=216) | Present value |
| [`Rate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=217) | Interest rate per period |
| [`SLN`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=218) | Straight-line depreciation |
| [`SYD`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=218) | Sum-of-years-digits depreciation |

### 6.1.2.7 [`Information`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=219) — Type tests, IME state and color/type information.

**Functions**

| Name | Brief description |
|---|---|
| [`IMEStatus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=219) | Current input-method status |
| [`IsArray`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=219) | Test whether value is an array |
| [`IsDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test whether expression is a date |
| [`IsEmpty`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test for Empty |
| [`IsError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test for Error value |
| [`IsMissing`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test for missing optional argument |
| [`IsNull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=221) | Test for Null |
| [`IsNumeric`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=221) | Test for numeric value |
| [`IsObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=222) | Test for object reference |
| [`QBColor`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=222) | Color from historical QB palette |
| [`RGB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=223) | Color from red, green and blue |
| [`TypeName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=223) | Type name as text |
| [`VarType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=224) | Variant subtype code |

### 6.1.2.8 [`Interaction`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=226) — System interaction, dialogs, settings and runtime object invocation.

**Functions**

| Name | Brief description |
|---|---|
| [`CallByName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=226) | Invoke object member by name |
| [`Choose`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=226) | Select list entry by index |
| [`Command`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=227) | Process command-line arguments |
| [`CreateObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=227) | Create automation object |
| [`DoEvents`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=228) | Yield execution to event processing |
| [`Environ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=228) | Read environment setting |
| [`Environ$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=228) | String-returning environment setting |
| [`GetAllSettings`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=229) | Read application's saved settings |
| [`GetAttr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=229) | Read file attributes |
| [`GetObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=230) | Retrieve automation object |
| [`GetSetting`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=231) | Read one saved setting |
| [`IIf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=231) | Conditional value selection |
| [`InputBox`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=232) | Show text-input dialog |
| [`MsgBox`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=233) | Show message dialog |
| [`Partition`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=235) | Place number in a range bucket |
| [`Shell`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=236) | Run an external program |
| [`Switch`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=237) | First matching condition's value |

**Subroutines**

| Name | Brief description |
|---|---|
| [`AppActivate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=238) | Activate a program window |
| [`Beep`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=238) | Sound system alert |
| [`DeleteSetting`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=238) | Delete saved application setting |
| [`SaveSetting`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=239) | Save application setting |
| [`SendKeys`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=239) | Send keystrokes |

### 6.1.2.9 [`KeyCodeConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) — Keyboard, mouse button and virtual-key codes.

| Constant | Value | Meaning |
|---|---|---|
| [`vbKeyLButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `1` | — |
| [`vbKeyRButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `2` | — |
| [`vbKeyCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `3` | — |
| [`vbKeyMButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `4` | — |
| [`vbKeyBack`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `8` | — |
| [`vbKeyTab`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `9` | — |
| [`vbKeyClear`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `12` | — |
| [`vbKeyReturn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `13` | — |
| [`vbKeyShift`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `16` | — |
| [`vbKeyControl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `17` | — |
| [`vbKeyMenu`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `18` | — |
| [`vbKeyPause`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `19` | — |
| [`vbKeyCapital`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `20` | — |
| [`vbKeyEscape`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `27` | — |
| [`vbKeySpace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `32` | — |
| [`vbKeyPageUp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `33` | — |
| [`vbKeyPageDown`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `34` | — |
| [`vbKeyEnd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `35` | — |
| [`vbKeyHome`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `36` | — |
| [`vbKeyLeft`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `37` | — |
| [`vbKeyUp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `38` | — |
| [`vbKeyRight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `39` | — |
| [`vbKeyDown`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `40` | — |
| [`vbKeySelect`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `41` | — |
| [`vbKeyPrint`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `42` | — |
| [`vbKeyExecute`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `43` | — |
| [`vbKeySnapshot`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `44` | — |
| [`vbKeyInsert`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `45` | — |
| [`vbKeyDelete`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `46` | — |
| [`vbKeyHelp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `47` | — |
| [`vbKeyNumlock`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `144` | — |
| [`vbKeyA`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `65` | — |
| [`vbKeyB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `66` | — |
| [`vbKeyC`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `67` | — |
| [`vbKeyD`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `68` | — |
| [`vbKeyE`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `69` | — |
| [`vbKeyF`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `70` | — |
| [`vbKeyG`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `71` | — |
| [`vbKeyH`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `72` | — |
| [`vbKeyI`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `73` | — |
| [`vbKeyJ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `74` | — |
| [`vbKeyK`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `75` | — |
| [`vbKeyL`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `76` | — |
| [`vbKeyM`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `77` | — |
| [`vbKeyN`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `78` | — |
| [`vbKeyO`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `79` | — |
| [`vbKeyP`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `80` | — |
| [`vbKeyQ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `81` | — |
| [`vbKeyR`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `82` | — |
| [`vbKeyS`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `83` | — |
| [`vbKeyT`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `84` | — |
| [`vbKeyU`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `85` | — |
| [`vbKeyV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `86` | — |
| [`vbKeyW`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `87` | — |
| [`vbKeyX`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `88` | — |
| [`vbKeyY`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `89` | — |
| [`vbKeyZ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `90` | — |
| [`vbKey0`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `48` | — |
| [`vbKey1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `49` | — |
| [`vbKey2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `50` | — |
| [`vbKey3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `51` | — |
| [`vbKey4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `52` | — |
| [`vbKey5`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `53` | — |
| [`vbKey6`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `54` | — |
| [`vbKey7`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `55` | — |
| [`vbKey8`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `56` | — |
| [`vbKey9`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `57` | — |
| [`vbKeyNumpad0`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `96` | — |
| [`vbKeyNumpad1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `97` | — |
| [`vbKeyNumpad2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `98` | — |
| [`vbKeyNumpad3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `99` | — |
| [`vbKeyNumpad4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `100` | — |
| [`vbKeyNumpad5`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `101` | — |
| [`vbKeyNumpad6`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `102` | — |
| [`vbKeyNumpad7`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `103` | — |
| [`vbKeyNumpad8`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `104` | — |
| [`vbKeyNumpad9`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `105` | — |
| [`vbKeyMultiply`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `106` | — |
| [`vbKeyAdd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `107` | — |
| [`vbKeySeparator`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `108` | — |
| [`vbKeySubtract`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `109` | — |
| [`vbKeyDecimal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `110` | — |
| [`vbKeyDivide`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `111` | — |
| [`vbKeyF1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `112` | — |
| [`vbKeyF2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `113` | — |
| [`vbKeyF3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `114` | — |
| [`vbKeyF4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `115` | — |
| [`vbKeyF5`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `116` | — |
| [`vbKeyF6`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `117` | — |
| [`vbKeyF7`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `118` | — |
| [`vbKeyF8`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `119` | — |
| [`vbKeyF9`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `120` | — |
| [`vbKeyF10`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `121` | — |
| [`vbKeyF11`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `122` | — |
| [`vbKeyF12`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `123` | — |
| [`vbKeyF13`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `124` | — |
| [`vbKeyF14`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `125` | — |
| [`vbKeyF15`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `126` | — |
| [`vbKeyF16`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `127` | — |

### 6.1.2.10 [`Math`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) — Numeric and trigonometric calculations; random numbers.

**Functions**

| Name | Brief description |
|---|---|
| [`Abs`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | Absolute value |
| [`Atn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | Arctangent |
| [`Cos`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=246) | Cosine |
| [`Exp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=246) | Exponential (e to a power) |
| [`Log`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=246) | Natural logarithm |
| [`Rnd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=247) | Pseudo-random number |
| [`Round`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=248) | Round to decimal places |
| [`Sgn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=248) | Sign of a number |
| [`Sin`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=249) | Sine |
| [`Sqr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=249) | Square root |
| [`Tan`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=249) | Tangent |

**Subroutines**

| Name | Brief description |
|---|---|
| [`Randomize`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=250) | Initialize random-number generator |

### 6.1.2.11 [`Strings`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=250) — Character codes, text search, slicing, formatting and case conversion.

**Functions**

| Name | Brief description |
|---|---|
| [`Asc`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=250) | Character code of first character |
| [`AscB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=251) | First byte of a character |
| [`AscW`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=251) | Unicode character code |
| [`Chr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | Character from numeric code |
| [`Chr$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | String-returning character conversion |
| [`ChrB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | Byte-character from code |
| [`ChrB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | String-returning byte-character conversion |
| [`ChrW`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=253) | Unicode character from code |
| [`ChrW$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=253) | String-returning Unicode conversion |
| [`Filter`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=253) | Filter string array entries |
| [`Format`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=254) | Format a value as text |
| [`Format$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=256) | String-returning formatter |
| [`FormatCurrency`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=256) | Format as currency |
| [`FormatDateTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=257) | Format as date/time |
| [`FormatNumber`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=258) | Format as number |
| [`FormatPercent`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=259) | Format as percentage |
| [`InStr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=260) | Find substring, left to right |
| [`InStrB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=260) | Byte-oriented substring search |
| [`InStrRev`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=261) | Find substring from right |
| [`Join`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=262) | Join array elements into text |
| [`LCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | Lowercase conversion |
| [`LCase$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | String-returning lowercase conversion |
| [`Left`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | Leftmost characters |
| [`LeftB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | Leftmost bytes |
| [`Left$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | String-returning left substring |
| [`LeftB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | String-returning left byte substring |
| [`Len`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | Length of text/variable |
| [`LenB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | Length in bytes |
| [`LTrim`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Remove leading spaces |
| [`RTrim`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Remove trailing spaces |
| [`Trim`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Remove outer spaces |
| [`LTrim$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | String-returning leading trim |
| [`RTrim$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | String-returning trailing trim |
| [`Trim$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | String-returning outer trim |
| [`Mid`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Substring starting at a position |
| [`MidB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Byte-oriented substring |
| [`Mid$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | String-returning substring |
| [`MidB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | String-returning byte substring |
| [`MonthName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | Localized name of month |
| [`Replace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | Replace substring occurrences |
| [`Right`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=267) | Rightmost characters |
| [`RightB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=267) | Rightmost bytes |
| [`Right$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=268) | String-returning right substring |
| [`RightB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=268) | String-returning right byte substring |
| [`Space`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=268) | Make string of spaces |
| [`Space$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=269) | String-returning spaces |
| [`Split`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=269) | Split text into array |
| [`StrComp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=269) | Compare two strings |
| [`StrConv`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=270) | Convert string case/encoding |
| [`String`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=271) | Repeat a character |
| [`String$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=272) | String-returning repeated characters |
| [`StrReverse`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=272) | Reverse text characters |
| [`UCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=272) | Uppercase conversion |
| [`UCase$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=273) | String-returning uppercase conversion |
| [`WeekdayName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=273) | Localized weekday name |

### 6.1.2.12 [`SystemColorConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) — System-dependent user-interface color codes.

| Constant | Value | Meaning |
|---|---|---|
| [`vbScrollBars`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000000` | Scroll bar |
| [`vbDesktop`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000001` | Desktop |
| [`vbActiveTitleBar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000002` | Active title bar |
| [`vbInactiveTitleBar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000003` | Inactive title bar |
| [`vbMenuBar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000004` | Menu background |
| [`vbWindowBackground`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000005` | Window background |
| [`vbWindowFrame`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000006` | Window frame |
| [`vbMenuText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000007` | Menu text |
| [`vbWindowText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000008` | Window text |
| [`vbTitleBarText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000009` | Title bar text |
| [`vbActiveBorder`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000A` | Active border |
| [`vbInactiveBorder`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000B` | Inactive border |
| [`vbApplicationWorkspace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000C` | MDI workspace |
| [`vbHighlight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000D` | Selected item background |
| [`vbHighlightText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000E` | Selected item text |
| [`vbButtonFace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000F` | Button face |
| [`vbButtonShadow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000010` | Button shadow |
| [`vbGrayText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000011` | Disabled text |
| [`vbButtonText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000012` | Button text |
| [`vbInactiveCaptionText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000013` | Inactive caption text |
| [`vb3DHighlight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000014` | 3D highlight |
| [`vb3DDKShadow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000015` | 3D dark shadow |
| [`vb3DLight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000016` | 3D light |
| [`vb3DFace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000F` | 3D face |
| [`vb3Dshadow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000010` | 3D shadow |
| [`vbInfoText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000017` | Tooltip text |
| [`vbInfoBackground`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) | `&H80000018` | Tooltip background |

## 3. Predefined classes (§6.1.3)

### 6.1.3.1 [`Collection`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) — Ordered collection of keyed/positioned items.

| Member | Brief description |
|---|---|
| [`Collection.Count`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) | Number of items |
| [`Collection.Item`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) | Look up item by position/key |
| [`Collection.Add`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=276) | Insert an item |
| [`Collection.Remove`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=276) | Delete an item |

### 6.1.3.2 [`Err`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=277) — Global error-state object.

| Member | Brief description |
|---|---|
| [`Err.Clear`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=277) | Reset error state |
| [`Err.Raise`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=277) | Raise a runtime error |
| [`Err.Description`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=278) | Text describing the error |
| [`Err.HelpContext`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=278) | Help topic identifier |
| [`Err.HelpFile`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Help-file path |
| [`Err.LastDllError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Last DLL-call error code |
| [`Err.Number`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Numeric error code |
| [`Err.Source`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Origin of error |

### 6.1.3.3 [`Global`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=280) — Global procedures for object lifecycle.

| Member | Brief description |
|---|---|
| [`Global.Load`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=280) | Load an object/form |
| [`Global.Unload`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=280) | Unload an object/form |

---

### Inventory counts

- **16 enums** containing **124 enumerated constant names**.
- **12 procedural modules:** **145 module constants**, **160 function name entries**, **13 subroutine name entries**, **7 property name entries**.
- **3 predefined classes** containing **14 named members**.
- **Total named entries:** 494 (counting category names; class-member names are qualified; aliases are separate entries).

**Conventions:** `VBA` is the global standard-library project. Many names are available without module qualification; for example `VBA.Strings.Left` may be called `Left`. Identically named constants in different enum families are listed at their definitions. The enumerated names follow Microsoft’s published capitalization (VBA identifiers are case-insensitive). `LongLong` support is implementation-dependent, as described by Microsoft.
