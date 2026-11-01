package scanner

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
)

type keywordTrieNode struct {
	tkID       ast.Kind
	successors [26]*keywordTrieNode
}

type keywordTrie struct {
	root keywordTrieNode
}

func (trie *keywordTrie) populate() {
	/* NOTE: Keywords to insert must only contain capital letters A-Z */
	insert := func(s string, tkID ast.Kind) {
		cur := &trie.root
		for i := 0; i < len(s); i++ {
			idx := s[i] - 65
			nxt := cur.successors[idx]
			if nxt == nil {
				nxt = &keywordTrieNode{}
				cur.successors[idx] = nxt
			}
			cur = nxt
		}
		cur.tkID = tkID
	}
	keywords := []struct {
		text string
		id   ast.Kind
	}{
		{text: "ABS", id: ast.KindABS},
		{text: "ANY", id: ast.KindANY},
		{text: "ARRAY", id: ast.KindARRAY},
		{text: "CBOOL", id: ast.KindCBOOL},
		{text: "CBYTE", id: ast.KindCBYTE},
		{text: "CCUR", id: ast.KindCCUR},
		{text: "CDATE", id: ast.KindCDATE},
		{text: "CDBL", id: ast.KindCDBL},
		{text: "CDEC", id: ast.KindCDEC},
		{text: "CINT", id: ast.KindCINT},
		{text: "CIRCLE", id: ast.KindCIRCLE},
		{text: "CLNG", id: ast.KindCLNG},
		{text: "CLNGLNG", id: ast.KindCLNGLNG},
		{text: "CLNGPTR", id: ast.KindCLNGPTR},
		{text: "CSNG", id: ast.KindCSNG},
		{text: "CSTR", id: ast.KindCSTR},
		{text: "CURRENCY", id: ast.KindCURRENCY},
		{text: "CVAR", id: ast.KindCVAR},
		{text: "CVERR", id: ast.KindCVERR},
		{text: "DEBUG", id: ast.KindDEBUG},
		{text: "DOEVENTS", id: ast.KindDOEVENTS},
		{text: "EXIT", id: ast.KindEXIT},
		{text: "FIX", id: ast.KindFIX},
		{text: "INPUTB", id: ast.KindINPUTB},
		{text: "INT", id: ast.KindINT},
		{text: "LBOUND", id: ast.KindLBOUND},
		{text: "LEN", id: ast.KindLEN},
		{text: "LENB", id: ast.KindLENB},
		{text: "LONGLONG", id: ast.KindLONGLONG},
		{text: "LONGPTR", id: ast.KindLONGPTR},
		{text: "MIDB", id: ast.KindMIDB},
		{text: "OPTION", id: ast.KindOPTION},
		{text: "PSET", id: ast.KindPSET},
		{text: "SCALE", id: ast.KindSCALE},
		{text: "SGN", id: ast.KindSGN},
		{text: "UBOUND", id: ast.KindUBOUND},
		{text: "ACCESS", id: ast.KindACCESS},
		{text: "ADDRESSOF", id: ast.KindADDRESSOF},
		{text: "ALIAS", id: ast.KindALIAS},
		{text: "AND", id: ast.KindAND},
		{text: "ATTRIBUTE", id: ast.KindATTRIBUTE},
		{text: "APPEND", id: ast.KindAPPEND},
		{text: "AS", id: ast.KindAS},
		{text: "BEGINPROPERTY", id: ast.KindBEGINPROPERTY},
		{text: "BEGIN", id: ast.KindBEGIN},
		{text: "BINARY", id: ast.KindBINARY},
		{text: "BOOLEAN", id: ast.KindBOOLEAN},
		{text: "BYVAL", id: ast.KindBYVAL},
		{text: "BYREF", id: ast.KindBYREF},
		{text: "BYTE", id: ast.KindBYTE},
		{text: "CALL", id: ast.KindCALL},
		{text: "CASE", id: ast.KindCASE},
		{text: "CDECL", id: ast.KindCDECL},
		{text: "CLASS", id: ast.KindCLASS},
		{text: "CLOSE", id: ast.KindCLOSE},
		{text: "CONST", id: ast.KindCONST},
		{text: "DATABASE", id: ast.KindDATABASE},
		{text: "DATE", id: ast.KindDATE},
		{text: "DECLARE", id: ast.KindDECLARE},
		{text: "DEFBOOL", id: ast.KindDEFBOOL},
		{text: "DEFBYTE", id: ast.KindDEFBYTE},
		{text: "DEFDATE", id: ast.KindDEFDATE},
		{text: "DEFDBL", id: ast.KindDEFDBL},
		{text: "DEFCUR", id: ast.KindDEFCUR},
		{text: "DEFINT", id: ast.KindDEFINT},
		{text: "DEFLNG", id: ast.KindDEFLNG},
		{text: "DEFLNGLNG", id: ast.KindDEFLNGLNG},
		{text: "DEFLNGPTR", id: ast.KindDEFLNGPTR},
		{text: "DEFOBJ", id: ast.KindDEFOBJ},
		{text: "DEFSNG", id: ast.KindDEFSNG},
		{text: "DEFSTR", id: ast.KindDEFSTR},
		{text: "DEFVAR", id: ast.KindDEFVAR},
		{text: "DIM", id: ast.KindDIM},
		{text: "DO", id: ast.KindDO},
		{text: "DOUBLE", id: ast.KindDOUBLE},
		{text: "EACH", id: ast.KindEACH},
		{text: "ELSE", id: ast.KindELSE},
		{text: "ELSEIF", id: ast.KindELSEIF},
		{text: "EMPTY", id: ast.KindEMPTY},
		{text: "ENDPROPERTY", id: ast.KindENDPROPERTY},
		{text: "END", id: ast.KindEND},
		{text: "ENUM", id: ast.KindENUM},
		{text: "EQV", id: ast.KindEQV},
		{text: "ERASE", id: ast.KindERASE},
		{text: "ERROR", id: ast.KindERROR},
		{text: "EVENT", id: ast.KindEVENT},
		{text: "FALSE", id: ast.KindFALSE},
		{text: "FRIEND", id: ast.KindFRIEND},
		{text: "FOR", id: ast.KindFOR},
		{text: "FUNCTION", id: ast.KindFUNCTION},
		{text: "GET", id: ast.KindGET},
		{text: "GLOBAL", id: ast.KindGLOBAL},
		{text: "GOSUB", id: ast.KindGOSUB},
		{text: "GOTO", id: ast.KindGOTO},
		{text: "IF", id: ast.KindIF},
		{text: "IMP", id: ast.KindIMP},
		{text: "IMPLEMENTS", id: ast.KindIMPLEMENTS},
		{text: "IN", id: ast.KindIN},
		{text: "INPUT", id: ast.KindINPUT},
		{text: "IS", id: ast.KindIS},
		{text: "INTEGER", id: ast.KindINTEGER},
		{text: "LOCK", id: ast.KindLOCK},
		{text: "LONG", id: ast.KindLONG},
		{text: "LOOP", id: ast.KindLOOP},
		{text: "LET", id: ast.KindLET},
		{text: "LIB", id: ast.KindLIB},
		{text: "LIKE", id: ast.KindLIKE},
		{text: "LSET", id: ast.KindLSET},
		{text: "ME", id: ast.KindME},
		{text: "MID", id: ast.KindMID},
		{text: "MOD", id: ast.KindMOD},
		{text: "NAME", id: ast.KindNAME},
		{text: "NEXT", id: ast.KindNEXT},
		{text: "NEW", id: ast.KindNEW},
		{text: "NOT", id: ast.KindNOT},
		{text: "NOTHING", id: ast.KindNOTHING},
		{text: "NULL", id: ast.KindNULL},
		{text: "OBJECT", id: ast.KindOBJECT},
		{text: "ON", id: ast.KindON},
		{text: "OPEN", id: ast.KindOPEN},
		{text: "OPTIONAL", id: ast.KindOPTIONAL},
		{text: "OR", id: ast.KindOR},
		{text: "OUTPUT", id: ast.KindOUTPUT},
		{text: "PARAMARRAY", id: ast.KindPARAMARRAY},
		{text: "PRESERVE", id: ast.KindPRESERVE},
		{text: "PRINT", id: ast.KindPRINT},
		{text: "PRIVATE", id: ast.KindPRIVATE},
		{text: "PROPERTY", id: ast.KindPROPERTY},
		{text: "PTRSAFE", id: ast.KindPTRSAFE},
		{text: "PUBLIC", id: ast.KindPUBLIC},
		{text: "PUT", id: ast.KindPUT},
		{text: "RANDOM", id: ast.KindRANDOM},
		{text: "RANDOMIZE", id: ast.KindRANDOMIZE},
		{text: "RAISEEVENT", id: ast.KindRAISEEVENT},
		{text: "READ", id: ast.KindREAD},
		{text: "REDIM", id: ast.KindREDIM},
		{text: "REM", id: ast.KindREM},
		{text: "RESET", id: ast.KindRESET},
		{text: "RESUME", id: ast.KindRESUME},
		{text: "RETURN", id: ast.KindRETURN},
		{text: "RSET", id: ast.KindRSET},
		{text: "SEEK", id: ast.KindSEEK},
		{text: "SELECT", id: ast.KindSELECT},
		{text: "SET", id: ast.KindSET},
		{text: "SHARED", id: ast.KindSHARED},
		{text: "SINGLE", id: ast.KindSINGLE},
		{text: "SPC", id: ast.KindSPC},
		{text: "STATIC", id: ast.KindSTATIC},
		{text: "STEP", id: ast.KindSTEP},
		{text: "STOP", id: ast.KindSTOP},
		{text: "STRING", id: ast.KindSTRING},
		{text: "SUB", id: ast.KindSUB},
		{text: "TAB", id: ast.KindTAB},
		{text: "TEXT", id: ast.KindTEXT},
		{text: "THEN", id: ast.KindTHEN},
		{text: "TO", id: ast.KindTO},
		{text: "TRUE", id: ast.KindTRUE},
		{text: "TYPE", id: ast.KindTYPE},
		{text: "TYPEOF", id: ast.KindTYPEOF},
		{text: "UNLOCK", id: ast.KindUNLOCK},
		{text: "UNTIL", id: ast.KindUNTIL},
		{text: "VARIANT", id: ast.KindVARIANT},
		{text: "VERSION", id: ast.KindVERSION},
		{text: "WEND", id: ast.KindWEND},
		{text: "WHILE", id: ast.KindWHILE},
		{text: "WIDTH", id: ast.KindWIDTH},
		{text: "WITH", id: ast.KindWITH},
		{text: "WITHEVENTS", id: ast.KindWITHEVENTS},
		{text: "WRITE", id: ast.KindWRITE},
		{text: "XOR", id: ast.KindXOR},
	}
	for _, entry := range keywords {
		insert(entry.text, entry.id)
	}
}

type ScanFlags uint32

const (
	ScanFlagNone                     ScanFlags = 0
	ScanFlagExplicitWhitespace       ScanFlags = 1
	ScanFlagExplicitLineContinuation ScanFlags = 2
)

var myKeywordTrie keywordTrie

func init() {
	myKeywordTrie.populate()
}

type ScannerState struct {
	Comments       []ast.Comment
	Input          string
	pos            int // Current position in text (and ending position of current token)
	tokenFullStart int
	tokenStart     int
	TokenValue     string
	tokenFlags     ast.TokenFlags
	wasLineNumber  bool
	wasWithinLine  bool
	onError        ErrorCallback
}

// ErrorCallback mirrors typescript-go's scanner error callback. Offsets are bytes.
type ErrorCallback func(code ast.DiagnosticCode, pos, length int, message string)

func (s *ScannerState) SetOnError(callback ErrorCallback) { s.onError = callback }
func (s *ScannerState) TokenStart() int                   { return s.tokenStart }
func (s *ScannerState) TokenEnd() int                     { return s.pos }
func (s *ScannerState) TokenFlags() ast.TokenFlags        { return s.tokenFlags }
func (s *ScannerState) TokenRange() core.TextRange        { return core.NewTextRange(s.tokenStart, s.pos) }

func (s *ScannerState) errorAt(code ast.DiagnosticCode, pos, length int, message string) {
	if s.onError != nil {
		s.onError(code, pos, length, message)
	}
}

func (s *ScannerState) isLineBreak() bool {
	if s.char() == '\r' || s.char() == '\n' {
		return true
	}
	return strings.HasPrefix(s.Input[s.pos:], "\u2028") || strings.HasPrefix(s.Input[s.pos:], "\u2029")
}

func (s *ScannerState) scanComment(start int) {
	content := s.pos
	line := strings.LastIndexAny(s.Input[:start], "\r\n") + 1
	standalone := strings.TrimSpace(s.Input[line:start]) == ""
	if content == start {
		content++
	} // Apostrophe; Rem has already been consumed.
	for s.char() >= 0 && !s.isLineBreak() {
		s.pos++
	}
	s.Comments = append(s.Comments, ast.Comment{Range: core.NewTextRange(start, s.pos), Text: s.Input[content:s.pos], Standalone: standalone})
}

func (state *ScannerState) TokenFullStart() int {
	return state.tokenFullStart
}

func (state *ScannerState) char() rune {
	if state.pos < len(state.Input) {
		return rune(state.Input[state.pos])
	}
	return -1
}

func (state *ScannerState) charAt(offset int) rune {
	if state.pos+offset < len(state.Input) {
		return rune(state.Input[state.pos+offset])
	}
	return -1
}

func (state *ScannerState) scanWhitespace() {
	for {
		ch := state.char()
		if ch != ' ' && ch != '\t' {
			break
		}
		state.pos++
	}
}

func (s *ScannerState) scanWhitespaceOrLineContinuation() bool {
	wasLineContinuation := false
	for {
		s.scanWhitespace()
		if s.char() == '_' {
			end := s.pos + 1
			for end < len(s.Input) && (s.Input[end] == ' ' || s.Input[end] == '\t') {
				end++
			}
			if end < len(s.Input) && s.Input[end] == '\r' {
				end++
			}
			if end >= len(s.Input) || s.Input[end] != '\n' {
				return wasLineContinuation
			}
			s.pos = end + 1
			wasLineContinuation = true
		} else {
			return wasLineContinuation
		}
	}
}

func (state *ScannerState) tryScanKeyword(ch rune) ast.Kind {
	cur := &myKeywordTrie.root
	for {
		if !('A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z') {
			break
		}
		nxt := cur.successors[(ch&0xdf)-65]
		if nxt == nil {
			break
		}
		cur = nxt
		state.pos++
		ch = state.char()
	}
	return cur.tkID
}

func isIdentifierPart(ch rune) bool {
	return 'A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z' ||
		'0' <= ch && ch <= '9' || ch == '_' || ch == '$'
}

func (state *ScannerState) tryFindColon() bool {
	pos := state.pos
	state.scanWhitespaceOrLineContinuation()
	ch := state.char()
	state.pos = pos
	return ch == ':'
}

func (s *ScannerState) Scan(scanFlags ScanFlags) ast.Kind {
	s.tokenFullStart = s.pos
	s.TokenValue = ""
	s.tokenFlags = ast.TokenFlagsNone
	wasLineNumber := s.wasLineNumber
	s.wasLineNumber = false
	for {
		s.tokenStart = s.pos
		wasWithinLine := s.wasWithinLine
		s.wasWithinLine = true
		ch := s.char()
		switch ch {
		case '\r':
			if s.charAt(1) == '\n' {
				s.pos += 2
			} else {
				s.pos++
			}
			s.wasWithinLine = false
			return ast.KindNEWLINE
		case '\n': // Todo: What about u2028, u2029 ?
			s.pos++
			s.wasWithinLine = false
			return ast.KindNEWLINE
		case '\'':
			s.scanComment(s.pos)
		case '\t', ' ':
			s.pos++
			wasLineContinuation := s.scanWhitespaceOrLineContinuation()
			// Trivia does not turn an indented line label into an expression.
			s.wasWithinLine = wasWithinLine
			if scanFlags&ScanFlagExplicitLineContinuation != 0 && wasLineContinuation {
				s.wasLineNumber = wasLineNumber
				return ast.KindLineContinuation
			}
			if scanFlags&ScanFlagExplicitWhitespace != 0 {
				s.wasLineNumber = wasLineNumber
				return ast.KindWS
			}
		case '_':
			s.pos++
			return ast.KindUNDERSCORE
		case ',':
			s.pos++
			return ast.KindCOMMA
		case ':':
			if s.charAt(1) == '=' {
				s.pos += 2
				return ast.KindASSIGN
			} else {
				s.pos++
				return ast.KindCOLON
			}
		case ';':
			s.pos++
			return ast.KindSEMICOLON
		case '!':
			s.pos++
			return ast.KindEXCLAMATIONPOINT
		case '.':
			if isDecimalDigit(s.charAt(1)) {
				return s.scanNumber(wasWithinLine)
			}
			s.pos++
			return ast.KindDOT
		case '#':
			s.pos++
			return ast.KindHASH
		case '@':
			s.pos++
			return ast.KindAT
		case '%':
			s.pos++
			return ast.KindPERCENT
		case '$':
			s.pos++
			return ast.KindDOLLAR
		case '&':
			if next := s.charAt(1); next == 'H' || next == 'h' || next == 'O' || next == 'o' {
				return s.scanRadixNumber()
			}
			s.pos++
			return ast.KindAMPERSAND
		case '/':
			s.pos++
			return ast.KindDIV
		case '\\':
			s.pos++
			return ast.KindINTDIV
		case '=':
			ch = s.charAt(1)
			switch ch {
			case '>':
				s.pos += 2
				return ast.KindGEQ
			case '<':
				s.pos += 2
				return ast.KindLEQ
			default:
				s.pos++
				return ast.KindEQ
			}
		case '>':
			ch = s.charAt(1)
			switch ch {
			case '<':
				s.pos += 2
				return ast.KindNEQ
			case '=':
				s.pos += 2
				return ast.KindGEQ
			default:
				s.pos++
				return ast.KindGT
			}
		case '(':
			s.pos++
			return ast.KindLPAREN
		case '<':
			ch = s.charAt(1)
			switch ch {
			case '>':
				s.pos += 2
				return ast.KindNEQ
			case '=':
				s.pos += 2
				return ast.KindLEQ
			default:
				s.pos++
				return ast.KindLT
			}
		case '-':
			s.pos++
			return ast.KindMINUS
		case '*':
			s.pos++
			return ast.KindMULT
		case '+':
			s.pos++
			return ast.KindPLUS
		case '^':
			s.pos++
			return ast.KindPOW
		case ')':
			s.pos++
			return ast.KindRPAREN
		case '[':
			s.pos++
			return ast.KindL_SQUARE_BRACKET
		case ']':
			s.pos++
			return ast.KindR_SQUARE_BRACKET
		case '{':
			s.pos++
			return ast.KindL_BRACE
		case '}':
			s.pos++
			return ast.KindR_BRACE
		case '"':
			var sb strings.Builder
			s.pos++
			start := s.pos
			for {
				ch = s.char()
				if ch == -1 || s.isLineBreak() {
					sb.WriteString(s.Input[start:s.pos])
					s.tokenFlags |= ast.TokenFlagsUnterminated
					s.errorAt(ast.DiagnosticUnterminatedString, s.tokenStart, s.pos-s.tokenStart, "Unterminated string literal")
					break
				}
				if ch == '"' {
					if s.charAt(1) != '"' {
						sb.WriteString(s.Input[start:s.pos])
						s.pos++
						break
					}
					s.pos++
					sb.WriteString(s.Input[start:s.pos])
					start = s.pos + 1
				}
				s.pos++
			}
			s.TokenValue = sb.String()
			return ast.KindStringLiteral
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return s.scanNumber(wasWithinLine)
		default:
			if ch < 0 {
				return ast.KindEndOfFile
			}
			start := s.pos
			if 'A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z' {
				tk := s.tryScanKeyword(ch)
				if tk != ast.KindNONE && !isIdentifierPart(s.char()) {
					// Property is contextual: adding the procedure keyword must
					// not turn a previously valid Property: label into a call.
					if tk == ast.KindPROPERTY && (wasLineNumber || !wasWithinLine) && s.tryFindColon() {
						s.TokenValue = s.Input[start:s.pos]
						return ast.KindIdentifierLabel
					}
					if tk == ast.KindREM {
						s.scanComment(start)
						continue
					}
					return tk
				}
				for {
					ch = s.char()
					if !isIdentifierPart(ch) {
						break
					}
					s.pos++
				}
				s.TokenValue = s.Input[start:s.pos]
				// Todo: Some keywords can be used as labels as well!
				if (wasLineNumber || !wasWithinLine) && (ch == ':' || s.tryFindColon()) {
					return ast.KindIdentifierLabel
				}
				return ast.KindIdentifier
			}
			if ch >= utf8.RuneSelf {
				r, size := utf8.DecodeRuneInString(s.Input[s.pos:])
				s.pos += size
				if r == '\u2028' || r == '\u2029' {
					s.wasWithinLine = false
					return ast.KindNEWLINE
				}
				if r == '\ufeff' && start == 0 {
					s.wasWithinLine = wasWithinLine
					continue
				}
			} else {
				s.pos++
			}
			s.TokenValue = s.Input[start:s.pos]
			s.errorAt(ast.DiagnosticInvalidCharacter, start, s.pos-start, fmt.Sprintf("Invalid character %q", s.TokenValue))
			return ast.KindUnknown
		}
	}
}
