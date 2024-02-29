// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package labels

import (
	"fmt"
	"unicode/utf8"
)

// lexerToken represents constant definition for lexer token.
type lexerToken int

// newToken parses token from string.
func newToken(s string) (lexerToken, bool) {
	switch s {
	case ")":
		return closedParToken, true
	case "(":
		return openParToken, true
	case ",":
		return commaToken, true
	case "!":
		return doesNotExistToken, true
	case "!=":
		return neqToken, true
	case "==":
		return doubleEQToken, true
	case "=":
		return eqToken, true
	case ">":
		return gtToken, true
	case ">=":
		return gteToken, true
	case "<":
		return ltToken, true
	case "<=":
		return lteToken, true
	case "notin":
		return notInToken, true
	case "in":
		return inToken, true
	default:
		return 0, false
	}
}

// String converts token to string.
func (t lexerToken) String() string {
	//nolint:exhaustive
	switch t {
	case closedParToken:
		return ")"
	case openParToken:
		return "("
	case commaToken:
		return ","
	case doesNotExistToken:
		return "!"
	case doubleEQToken:
		return "=="
	case eqToken:
		return "="
	case gtToken:
		return ">"
	case gteToken:
		return ">="
	case ltToken:
		return "<"
	case lteToken:
		return "<="
	case neqToken:
		return "!="
	case notInToken:
		return "notin"
	case inToken:
		return "in"
	default:
		return ""
	}
}

const (
	// errorToken represents scan error.
	errorToken lexerToken = iota
	// endOfStringToken represents end of string.
	endOfStringToken
	// closedParToken represents close parenthesis.
	closedParToken
	// commaToken represents the comma.
	commaToken
	// doesNotExistToken represents logic not.
	doesNotExistToken
	// doubleEQToken represents double equals.
	doubleEQToken
	// eqToken represents equal.
	eqToken
	// gtToken represents greater than.
	gtToken
	// gteToken represents greater than or equal.
	gteToken
	// IdentifierToken represents identifier, e.g. keys and values.
	IdentifierToken
	// inToken represents in.
	inToken
	// ltToken represents less than.
	ltToken
	// lteToken represents less than or equal.
	lteToken
	// neqToken represents not equal.
	neqToken
	// notInToken represents not in.
	notInToken
	// openParToken represents open parenthesis.
	openParToken
)

// isWhitespace returns true if the rune is a space, tab, or newline.
func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

// isSpecialSymbol detects if the character ch can be an operator.
func isSpecialSymbol(ch rune) bool {
	switch ch {
	case '=', '!', '(', ')', ',', '>', '<':
		return true
	}

	return false
}

// lexer represents the lexer struct for label selector.
// It contains necessary information to tokenize the input string.
type lexer struct {
	// s stores the string to be tokenized
	s string
	// pos is the position currently tokenized
	pos int
}

// read returns the character currently lexed
// increment the position and check the buffer overflow.
func (l *lexer) read() (rune, int) {
	if l.pos >= len(l.s) {
		return 0, 0
	}

	b, width := utf8.DecodeRuneInString(l.s[l.pos:])
	l.pos += width

	return b, width
}

// scanIDOrKeyword scans string to recognize literal token (for example 'in') or an identifier.
func (l *lexer) scanIDOrKeyword() (tok lexerToken, lit string) {
	var buffer []rune
IdentifierLoop:
	for {
		switch ch, width := l.read(); {
		case ch == 0:
			break IdentifierLoop
		case isSpecialSymbol(ch) || isWhitespace(ch):
			l.pos -= width

			break IdentifierLoop
		default:
			buffer = append(buffer, ch)
		}
	}

	s := string(buffer)
	if val, ok := newToken(s); ok { // is a literal token?
		return val, s
	}

	return IdentifierToken, s // otherwise is an identifier
}

// scanSpecialSymbol scans string starting with special symbol.
// special symbol identify non literal operators. "!=", "==", "=".
func (l *lexer) scanSpecialSymbol() (lexerToken, string) {
	var (
		buffer  []rune
		token   lexerToken
		literal string
	)

SpecialSymbolLoop:
	for {
		switch ch, width := l.read(); {
		case ch == 0:
			break SpecialSymbolLoop
		case isSpecialSymbol(ch):
			buffer = append(buffer, ch)

			if t, ok := newToken(string(buffer)); ok {
				token = t
				literal = string(buffer)
			} else if token != 0 {
				l.pos -= width

				break SpecialSymbolLoop
			}
		default:
			l.pos -= width

			break SpecialSymbolLoop
		}
	}

	if token == 0 {
		return errorToken, fmt.Sprintf("error expected: keyword found '%s'", string(buffer))
	}

	return token, literal
}

// skipWhiteSpaces consumes all blank characters
// returning the first non blank character.
func (l *lexer) skipWhiteSpaces(ch rune, width int) (rune, int) {
	for {
		if !isWhitespace(ch) {
			return ch, width
		}

		ch, width = l.read()
	}
}

// lex returns a pair of Token and the literal.
// Literal is meaningful only for IdentifierToken token.
func (l *lexer) lex() (tok lexerToken, lit string) {
	switch ch, width := l.skipWhiteSpaces(l.read()); {
	case ch == 0:
		return endOfStringToken, ""
	case isSpecialSymbol(ch):
		l.pos -= width

		return l.scanSpecialSymbol()
	default:
		l.pos -= width

		return l.scanIDOrKeyword()
	}
}
