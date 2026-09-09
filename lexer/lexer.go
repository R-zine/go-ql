package lexer

import (
	"fmt"
	"strings"
)

func lexNumeric(source string, ic cursor) (*Token, cursor, bool) {
	cur := ic
	if cur.pointer >= uint(len(source)) {
		return nil, ic, false
	}

	if source[cur.pointer] == '+' || source[cur.pointer] == '-' {
		cur.pointer++
		cur.loc.Col++
		if cur.pointer >= uint(len(source)) || !isDigit(source[cur.pointer]) {
			return nil, ic, false
		}
	}
	if !isDigit(source[cur.pointer]) {
		return nil, ic, false
	}

	for cur.pointer < uint(len(source)) && isDigit(source[cur.pointer]) {
		cur.pointer++
		cur.loc.Col++
	}
	if cur.pointer < uint(len(source)) && isIdentifierContinuation(source[cur.pointer]) {
		return nil, ic, false
	}

	return &Token{
		Value: source[ic.pointer:cur.pointer],
		Loc:   ic.loc,
		Kind:  NumericKind,
	}, cur, true
}

func lexCharacterDelimited(source string, ic cursor, delimiter byte) (*Token, cursor, bool) {
	cur := ic

	if len(source[cur.pointer:]) == 0 {
		return nil, ic, false
	}

	if source[cur.pointer] != delimiter {
		return nil, ic, false
	}

	cur.loc.Col++
	cur.pointer++

	var value []byte
	for ; cur.pointer < uint(len(source)); cur.pointer++ {
		c := source[cur.pointer]

		if c == delimiter {
			if cur.pointer+1 >= uint(len(source)) || source[cur.pointer+1] != delimiter {
				cur.pointer++
				cur.loc.Col++

				return &Token{
					Value: string(value),
					Loc:   ic.loc,
					Kind:  StringKind,
				}, cur, true
			}

			value = append(value, delimiter)
			cur.pointer++
			cur.loc.Col++
			continue
		}

		value = append(value, c)
		if c == '\n' {
			cur.loc.Line++
			cur.loc.Col = 0
		} else {
			cur.loc.Col++
		}
	}

	return nil, ic, false
}

func lexString(source string, ic cursor) (*Token, cursor, bool) {
	return lexCharacterDelimited(source, ic, '\'')
}

func lexSymbol(source string, ic cursor) (*Token, cursor, bool) {
	c := source[ic.pointer]
	cur := ic
	// Will get overwritten later if not an ignored syntax
	cur.pointer++
	cur.loc.Col++

	switch c {
	// Syntax that should be thrown away
	case '\r':
		if cur.pointer >= uint(len(source)) || source[cur.pointer] != '\n' {
			cur.loc.Line++
			cur.loc.Col = 0
		}
		return nil, cur, true
	case '\n':
		cur.loc.Line++
		cur.loc.Col = 0
		fallthrough
	case '\t':
		fallthrough
	case ' ':
		return nil, cur, true
	}

	// Syntax that should be kept
	symbols := []Symbol{
		NotEqualsSymbol,
		LessThanOrEqualSymbol,
		GreaterThanOrEqualSymbol,
		LessThanSymbol,
		GreaterThanSymbol,
		EqualsSymbol,

		CommaSymbol,
		LeftParenSymbol,
		RightParenSymbol,
		SemicolonSymbol,
		AsteriskSymbol,
	}

	var options []string
	for _, s := range symbols {
		options = append(options, string(s))
	}

	// Use `ic`, not `cur`
	match := longestMatch(source, ic, options)
	// Unknown character
	if match == "" {
		return nil, ic, false
	}

	cur.pointer = ic.pointer + uint(len(match))
	cur.loc.Col = ic.loc.Col + uint(len(match))

	return &Token{
		Value: match,
		Loc:   ic.loc,
		Kind:  SymbolKind,
	}, cur, true
}

func longestMatch(source string, ic cursor, options []string) string {
	var match string
	remaining := source[ic.pointer:]
	for _, option := range options {
		if len(option) <= len(remaining) &&
			strings.EqualFold(remaining[:len(option)], option) &&
			len(option) > len(match) {
			match = option
		}
	}
	return match
}

func lexIdentifier(source string, ic cursor) (*Token, cursor, bool) {
	// Handle separately if is a double-quoted identifier
	if token, newCursor, ok := lexCharacterDelimited(source, ic, '"'); ok {
		token.Kind = IdentifierKind
		return token, newCursor, true
	}

	cur := ic

	c := source[cur.pointer]
	// Non-ASCII identifiers are not supported yet.
	isAlphabetical := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
	if !isAlphabetical {
		return nil, ic, false
	}
	cur.pointer++
	cur.loc.Col++

	value := []byte{c}
	for ; cur.pointer < uint(len(source)); cur.pointer++ {
		c = source[cur.pointer]

		// Non-ASCII identifiers are not supported yet.
		isAlphabetical := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		isNumeric := c >= '0' && c <= '9'
		if isAlphabetical || isNumeric || c == '$' || c == '_' {
			value = append(value, c)
			cur.loc.Col++
			continue
		}

		break
	}

	if len(value) == 0 {
		return nil, ic, false
	}

	return &Token{
		// Unquoted identifiers are case-insensitive.
		Value: strings.ToLower(string(value)),
		Loc:   ic.loc,
		Kind:  IdentifierKind,
	}, cur, true
}

func lexKeyword(source string, ic cursor) (*Token, cursor, bool) {
	cur := ic
	keywords := []Keyword{
		SelectKeyword,
		FromKeyword,
		TableKeyword,
		CreateKeyword,
		InsertKeyword,
		IntoKeyword,
		ValuesKeyword,
		WhereKeyword,
		IntKeyword,
		TextKeyword,
		PrimaryKeyword,
		KeyKeyword,
	}

	var options []string
	for _, k := range keywords {
		options = append(options, string(k))
	}

	match := longestMatch(source, ic, options)
	if match == "" {
		return nil, ic, false
	}

	next := ic.pointer + uint(len(match))
	if next < uint(len(source)) && isIdentifierContinuation(source[next]) {
		return nil, ic, false
	}

	cur.pointer = ic.pointer + uint(len(match))
	cur.loc.Col = ic.loc.Col + uint(len(match))

	return &Token{
		Value: match,
		Kind:  KeywordKind,
		Loc:   ic.loc,
	}, cur, true
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isIdentifierContinuation(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		isDigit(c) || c == '$' || c == '_'
}

func Lex(source string) ([]*Token, error) {
	tokens := []*Token{}
	cur := cursor{}

lex:
	for cur.pointer < uint(len(source)) {
		lexers := []lexer{lexKeyword, lexSymbol, lexString, lexNumeric, lexIdentifier}
		for _, l := range lexers {
			if token, newCursor, ok := l(source, cur); ok {

				cur = newCursor

				// Omit nil tokens for valid, but empty syntax like newlines
				if token != nil {
					tokens = append(tokens, token)
				}

				continue lex
			}
		}

		hint := ""
		if len(tokens) > 0 {
			hint = " after " + tokens[len(tokens)-1].Value
		}

		return nil, fmt.Errorf("unable to lex token%s at %d:%d", hint, cur.loc.Line, cur.loc.Col)
	}

	return tokens, nil
}
