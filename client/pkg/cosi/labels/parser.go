// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package labels

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
)

// scannedItem contains the Token and the literal produced by the lexer.
type scannedItem struct {
	literal string
	token   lexerToken
}

// parser data structure contains the label selector parser data structure.
type parser struct {
	currentItem *scannedItem
	l           *lexer
	position    int
}

// parserContext represents context during parsing:
// some literal for example 'in' and 'notin' can be
// recognized as operator for example 'x in (a)' but
// it can be recognized as value for example 'value in (in)'.
type parserContext int

const (
	keyAndOperatorContext parserContext = iota
	valuesContext
)

// lookahead func returns the current token and string. No increment of current position.
func (p *parser) lookahead(context parserContext) (lexerToken, string) {
	if p.currentItem == nil {
		token, literal := p.l.lex()

		p.currentItem = &scannedItem{
			token:   token,
			literal: literal,
		}
	}

	token, literal := p.currentItem.token, p.currentItem.literal

	if context == valuesContext {
		//nolint:exhaustive
		switch token {
		case inToken, notInToken:
			token = IdentifierToken
		}
	}

	return token, literal
}

// consume returns current token and string. Increments the position.
func (p *parser) consume(context parserContext) (lexerToken, string) {
	tok, lit := p.lookahead(context)

	p.position++

	p.currentItem = nil

	return tok, lit
}

// parse runs the left recursive descending algorithm
// on input string. It returns a list of Term objects.
func (p *parser) parse() (*resource.LabelQuery, error) {
	var terms []resource.LabelTerm

	for {
		tok, lit := p.lookahead(valuesContext)
		//nolint:exhaustive
		switch tok {
		case IdentifierToken, doesNotExistToken:
			r, err := p.parseTerm()
			if err != nil {
				return nil, fmt.Errorf("unable to parse term: %w", err)
			}

			terms = append(terms, *r)

			t, l := p.consume(valuesContext)
			//nolint:exhaustive
			switch t {
			case endOfStringToken:
				return &resource.LabelQuery{
					Terms: terms,
				}, nil
			case commaToken:
				t2, l2 := p.lookahead(valuesContext)
				if t2 != IdentifierToken && t2 != doesNotExistToken {
					return nil, fmt.Errorf("found '%s', expected: identifier after ','", l2)
				}
			default:
				return nil, fmt.Errorf("found '%s', expected: ',' or 'end of string'", l)
			}
		case endOfStringToken:
			return &resource.LabelQuery{
				Terms: terms,
			}, nil
		default:
			return nil, fmt.Errorf("found '%s', expected: !, identifier, or 'end of string'", lit)
		}
	}
}

func (p *parser) parseTerm() (*resource.LabelTerm, error) {
	var (
		term resource.LabelTerm
		err  error
	)

	tok, key := p.consume(valuesContext)

	doesNotExist := tok == doesNotExistToken

	if doesNotExist {
		tok, key = p.consume(valuesContext)

		term.Invert = true
	}

	term.Key = key

	if tok != IdentifierToken {
		err = fmt.Errorf("found '%s', expected: identifier", key)

		return nil, err
	}

	if t, _ := p.lookahead(valuesContext); t == endOfStringToken || t == commaToken {
		term.Op = resource.LabelOpExists

		return &term, nil
	} else if doesNotExist {
		return nil, fmt.Errorf("expected EOF or ',', found %s", tok.String())
	}

	tok, lit := p.consume(keyAndOperatorContext)
	//nolint:exhaustive
	switch tok {
	case notInToken:
		term.Invert = true

		fallthrough
	case inToken:
		term.Op = resource.LabelOpIn
	case neqToken:
		term.Invert = true

		fallthrough
	case eqToken, doubleEQToken:
		term.Op = resource.LabelOpEqual
	// >= is inverse to <
	case gteToken:
		term.Invert = true

		fallthrough
	case ltToken:
		term.Op = resource.LabelOpLTNumeric
	// > is inverse to <=
	case gtToken:
		term.Invert = true

		fallthrough
	case lteToken:
		term.Op = resource.LabelOpLTENumeric
	default:
		allowedTokens := []lexerToken{
			doubleEQToken,
			eqToken,
			neqToken,
			gteToken,
			gtToken,
			inToken,
			notInToken,
			lteToken,
			ltToken,
		}

		return nil, fmt.Errorf("found '%s', expected: %v", lit, strings.Join(xslices.Map(allowedTokens, func(t lexerToken) string { return t.String() }), ", "))
	}

	if term.Op == resource.LabelOpIn {
		term.Value, err = p.parseValues()

		return &term, err
	}

	term.Value, err = p.parseExactValue()

	return &term, err
}

// parseValues parses the values for set based matching (x,y,z).
func (p *parser) parseValues() ([]string, error) {
	tok, lit := p.consume(valuesContext)
	if tok != openParToken {
		return nil, fmt.Errorf("found '%s' expected: '('", lit)
	}

	tok, lit = p.lookahead(valuesContext)
	//nolint:exhaustive
	switch tok {
	case IdentifierToken, commaToken:
		s, err := p.parseIdentifiersList() // handles general cases
		if err != nil {
			return nil, err
		}

		if tok, _ = p.consume(valuesContext); tok != closedParToken {
			return nil, fmt.Errorf("found '%s', expected: ')'", lit)
		}

		res := maps.Keys(s)
		sort.Strings(res)

		return res, nil
	case closedParToken: // handles "()"
		p.consume(valuesContext)

		return []string{""}, nil
	default:
		return nil, fmt.Errorf("found '%s', expected: ',', ')' or identifier", lit)
	}
}

// parseIdentifiersList parses a (possibly empty) list
// of comma separated (possibly empty) identifiers.
func (p *parser) parseIdentifiersList() (map[string]struct{}, error) {
	s := map[string]struct{}{}

	for {
		tok, lit := p.consume(valuesContext)
		//nolint:exhaustive
		switch tok {
		case IdentifierToken:
			s[lit] = struct{}{}

			tok2, lit2 := p.lookahead(valuesContext)

			//nolint:exhaustive
			switch tok2 {
			case commaToken:
				continue
			case closedParToken:
				return s, nil
			default:
				return nil, fmt.Errorf("found '%s', expected: ',' or ')'", lit2)
			}
		case commaToken: // handled here since we can have "(,"
			if len(s) == 0 {
				s[""] = struct{}{} // to handle (,
			}

			tok2, _ := p.lookahead(valuesContext)
			if tok2 == closedParToken {
				s[""] = struct{}{} // to handle ,)  Double "" removed by StringSet

				return s, nil
			}

			if tok2 == commaToken {
				p.consume(valuesContext)

				s[""] = struct{}{} // to handle ,, Double "" removed by StringSet
			}
		default: // it can be operator
			return s, fmt.Errorf("found '%s', expected: ',', or identifier", lit)
		}
	}
}

// parseExactValue parses the only value for exact match style.
func (p *parser) parseExactValue() ([]string, error) {
	tok, _ := p.lookahead(valuesContext)
	if tok == endOfStringToken || tok == commaToken {
		return []string{""}, nil
	}

	tok, lit := p.consume(valuesContext)
	if tok == IdentifierToken {
		return []string{lit}, nil
	}

	return nil, fmt.Errorf("found '%s', expected: identifier", lit)
}
