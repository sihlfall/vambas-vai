package scanner

import "sihlfall.ch/vambas-vai/ast"

func isDecimalDigit(ch rune) bool { return '0' <= ch && ch <= '9' }

// Declarations commonly contain decimal, exponent, hex, and octal constants.
// Keep their original spelling; numeric interpretation belongs to the checker.
func (s *ScannerState) scanNumber(withinLine bool) ast.Kind {
	start := s.pos
	for isDecimalDigit(s.char()) {
		s.pos++
	}
	floating := false
	if s.char() == '.' {
		floating = true
		s.pos++
		for isDecimalDigit(s.char()) {
			s.pos++
		}
	}
	if ch := s.char(); ch == 'E' || ch == 'e' || ch == 'D' || ch == 'd' {
		floating = true
		s.pos++
		if s.char() == '+' || s.char() == '-' {
			s.pos++
		}
		if !isDecimalDigit(s.char()) {
			s.errorAt(ast.DiagnosticInvalidNumber, start, s.pos-start, "Digits expected after numeric exponent")
		}
		for isDecimalDigit(s.char()) {
			s.pos++
		}
	}
	s.TokenValue = s.Input[start:s.pos]
	if !withinLine && !floating {
		s.wasLineNumber = true
		return ast.KindLineNumber
	}
	return ast.KindNumericLiteral
}

func (s *ScannerState) scanRadixNumber() ast.Kind {
	start := s.pos
	hex := s.charAt(1) == 'H' || s.charAt(1) == 'h'
	s.pos += 2
	digits := s.pos
	for {
		ch := s.char()
		valid := '0' <= ch && ch <= '7'
		if hex {
			valid = isDecimalDigit(ch) || 'a' <= ch && ch <= 'f' || 'A' <= ch && ch <= 'F'
		}
		if !valid {
			break
		}
		s.pos++
	}
	if s.pos == digits {
		s.errorAt(ast.DiagnosticInvalidNumber, start, s.pos-start, "Digits expected after numeric base prefix")
	}
	s.TokenValue = s.Input[start:s.pos]
	return ast.KindNumericLiteral
}
