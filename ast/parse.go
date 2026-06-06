package ast

import (
	"errors"
	"fmt"
	"go-ql/lexer"
)

func tokenFromKeyword(k lexer.Keyword) lexer.Token {
	return lexer.Token{
		Kind:  lexer.KeywordKind,
		Value: string(k),
	}
}

func tokenFromSymbol(s lexer.Symbol) lexer.Token {
	return lexer.Token{
		Kind:  lexer.SymbolKind,
		Value: string(s),
	}
}

func expectToken(tokens []*lexer.Token, cursor uint, t lexer.Token) bool {
	if cursor >= uint(len(tokens)) {
		return false
	}

	return t.Equals(tokens[cursor])
}

func helpMessage(tokens []*lexer.Token, cursor uint, msg string) {
	var c *lexer.Token
	if cursor < uint(len(tokens)) {
		c = tokens[cursor]
	} else {
		c = tokens[cursor-1]
	}

	fmt.Printf("[%d,%d]: %s, got: %s\n", c.Loc.Line, c.Loc.Col, msg, c.Value)
}

func Parse(source string) (*Ast, error) {
	tokens, err := lexer.Lex(source)
	if err != nil {
		return nil, err
	}

	a := Ast{}
	cursor := uint(0)
	for cursor < uint(len(tokens)) {
		stmt, newCursor, ok := parseStatement(tokens, cursor, tokenFromSymbol(lexer.SemicolonSymbol))
		if !ok {
			helpMessage(tokens, cursor, "Expected statement")
			return nil, errors.New("Failed to parse, expected statement")
		}
		cursor = newCursor

		a.Statements = append(a.Statements, stmt)

		atLeastOneSemicolon := false
		for expectToken(tokens, cursor, tokenFromSymbol(lexer.SemicolonSymbol)) {
			cursor++
			atLeastOneSemicolon = true
		}

		if !atLeastOneSemicolon {
			helpMessage(tokens, cursor, "Expected semi-colon delimiter between statements")
			return nil, errors.New("Missing semi-colon between statements")
		}
	}

	return &a, nil
}

func parseStatement(tokens []*lexer.Token, initialCursor uint, delimiter lexer.Token) (*Statement, uint, bool) {
	cursor := initialCursor

	// Look for a SELECT statement
	semicolonToken := tokenFromSymbol(lexer.SemicolonSymbol)
	slct, newCursor, ok := parseSelectStatement(tokens, cursor, semicolonToken)
	if ok {
		return &Statement{
			Kind:            SelectKind,
			SelectStatement: slct,
		}, newCursor, true
	}

	// Look for a INSERT statement
	inst, newCursor, ok := parseInsertStatement(tokens, cursor, semicolonToken)
	if ok {
		return &Statement{
			Kind:            InsertKind,
			InsertStatement: inst,
		}, newCursor, true
	}

	// Look for a CREATE statement
	crtTbl, newCursor, ok := parseCreateTableStatement(tokens, cursor, semicolonToken)
	if ok {
		return &Statement{
			Kind:                 CreateTableKind,
			CreateTableStatement: crtTbl,
		}, newCursor, true
	}

	return nil, initialCursor, false
}

func parseSelectStatement(tokens []*lexer.Token, initialCursor uint, delimiter lexer.Token) (*SelectStatement, uint, bool) {
	cursor := initialCursor
	if !expectToken(tokens, cursor, tokenFromKeyword(lexer.SelectKeyword)) {
		return nil, initialCursor, false
	}
	cursor++

	slct := SelectStatement{}

	exps, newCursor, ok := parseExpressions(tokens, cursor, []lexer.Token{tokenFromKeyword(lexer.FromKeyword), delimiter})
	if !ok {
		return nil, initialCursor, false
	}

	slct.Item = *exps
	cursor = newCursor

	if expectToken(tokens, cursor, tokenFromKeyword(lexer.FromKeyword)) {
		cursor++

		from, newCursor, ok := parseToken(tokens, cursor, lexer.IdentifierKind)
		if !ok {
			helpMessage(tokens, cursor, "Expected FROM token")
			return nil, initialCursor, false
		}

		slct.From = *from
		cursor = newCursor

		if expectToken(tokens, cursor, tokenFromKeyword(lexer.WhereKeyword)) {
			cursor++

			where, newCursor, ok := parseWhere(tokens, cursor)
			if !ok {
				helpMessage(tokens, cursor, "Expected WHERE clause")
				return nil, initialCursor, false
			}

			slct.Where = where
			cursor = newCursor
		}
	}

	return &slct, cursor, true
}

func parseToken(tokens []*lexer.Token, initialCursor uint, kind lexer.TokenKind) (*lexer.Token, uint, bool) {
	cursor := initialCursor

	if cursor >= uint(len(tokens)) {
		return nil, initialCursor, false
	}

	current := tokens[cursor]
	if current.Kind == kind {
		return current, cursor + 1, true
	}

	return nil, initialCursor, false
}

func parseExpressions(tokens []*lexer.Token, initialCursor uint, delimiters []lexer.Token) (*[]*Expression, uint, bool) {
	cursor := initialCursor

	exps := []*Expression{}
outer:
	for {
		if cursor >= uint(len(tokens)) {
			return nil, initialCursor, false
		}

		// Look for delimiter
		current := tokens[cursor]
		for _, delimiter := range delimiters {
			if delimiter.Equals(current) {
				break outer
			}
		}

		// Look for comma
		if len(exps) > 0 {
			if !expectToken(tokens, cursor, tokenFromSymbol(lexer.CommaSymbol)) {
				helpMessage(tokens, cursor, "Expected comma")
				return nil, initialCursor, false
			}

			cursor++
		}

		// Look for expression
		exp, newCursor, ok := parseExpression(tokens, cursor, tokenFromSymbol(lexer.CommaSymbol))
		if !ok {
			helpMessage(tokens, cursor, "Expected expression")
			return nil, initialCursor, false
		}
		cursor = newCursor

		exps = append(exps, exp)
	}

	return &exps, cursor, true
}

func parseExpression(tokens []*lexer.Token, initialCursor uint, _ lexer.Token) (*Expression, uint, bool) {
	cursor := initialCursor

	// Handle *
	if expectToken(tokens, cursor, tokenFromSymbol(lexer.AsteriskSymbol)) {
		return &Expression{
			Kind: WildcardKind,
		}, cursor + 1, true
	}

	kinds := []lexer.TokenKind{
		lexer.IdentifierKind,
		lexer.NumericKind,
		lexer.StringKind,
	}

	for _, kind := range kinds {
		t, newCursor, ok := parseToken(tokens, cursor, kind)
		if ok {
			return &Expression{
				Literal: t,
				Kind:    LiteralKind,
			}, newCursor, true
		}
	}

	return nil, initialCursor, false
}

func parseWhere(
	tokens []*lexer.Token,
	initialCursor uint,
) (*WhereClause, uint, bool) {
	cursor := initialCursor

	left, cursor, ok := parseToken(tokens, cursor, lexer.IdentifierKind)
	if !ok {
		return nil, initialCursor, false
	}

	op := tokens[cursor]
	cursor++

	right := tokens[cursor]
	cursor++

	return &WhereClause{
		Left:     *left,
		Operator: *op,
		Right:    *right,
	}, cursor, true
}

func parseInsertStatement(tokens []*lexer.Token, initialCursor uint, delimiter lexer.Token) (*InsertStatement, uint, bool) {
	cursor := initialCursor

	// Look for INSERT
	if !expectToken(tokens, cursor, tokenFromKeyword(lexer.InsertKeyword)) {
		return nil, initialCursor, false
	}
	cursor++

	// Look for INTO
	if !expectToken(tokens, cursor, tokenFromKeyword(lexer.IntoKeyword)) {
		helpMessage(tokens, cursor, "Expected into")
		return nil, initialCursor, false
	}
	cursor++

	// Look for table name
	table, newCursor, ok := parseToken(tokens, cursor, lexer.IdentifierKind)
	if !ok {
		helpMessage(tokens, cursor, "Expected table name")
		return nil, initialCursor, false
	}
	cursor = newCursor

	// Look for VALUES
	if !expectToken(tokens, cursor, tokenFromKeyword(lexer.ValuesKeyword)) {
		helpMessage(tokens, cursor, "Expected VALUES")
		return nil, initialCursor, false
	}
	cursor++

	// Look for left paren
	if !expectToken(tokens, cursor, tokenFromSymbol(lexer.LeftParenSymbol)) {
		helpMessage(tokens, cursor, "Expected left paren")
		return nil, initialCursor, false
	}
	cursor++

	// Look for expression list
	values, newCursor, ok := parseExpressions(tokens, cursor, []lexer.Token{tokenFromSymbol(lexer.RightParenSymbol)})
	if !ok {
		return nil, initialCursor, false
	}
	cursor = newCursor

	// Look for right paren
	if !expectToken(tokens, cursor, tokenFromSymbol(lexer.RightParenSymbol)) {
		helpMessage(tokens, cursor, "Expected right paren")
		return nil, initialCursor, false
	}
	cursor++

	return &InsertStatement{
		Table:  *table,
		Values: values,
	}, cursor, true
}

func parseCreateTableStatement(tokens []*lexer.Token, initialCursor uint, delimiter lexer.Token) (*CreateTableStatement, uint, bool) {
	cursor := initialCursor

	if !expectToken(tokens, cursor, tokenFromKeyword(lexer.CreateKeyword)) {
		return nil, initialCursor, false
	}
	cursor++

	if !expectToken(tokens, cursor, tokenFromKeyword(lexer.TableKeyword)) {
		return nil, initialCursor, false
	}
	cursor++

	name, newCursor, ok := parseToken(tokens, cursor, lexer.IdentifierKind)
	if !ok {
		helpMessage(tokens, cursor, "Expected table name")
		return nil, initialCursor, false
	}
	cursor = newCursor

	if !expectToken(tokens, cursor, tokenFromSymbol(lexer.LeftParenSymbol)) {
		helpMessage(tokens, cursor, "Expected left parenthesis")
		return nil, initialCursor, false
	}
	cursor++

	cols, newCursor, ok := parseColumnDefinitions(tokens, cursor, tokenFromSymbol(lexer.RightParenSymbol))
	if !ok {
		return nil, initialCursor, false
	}
	cursor = newCursor

	if !expectToken(tokens, cursor, tokenFromSymbol(lexer.RightParenSymbol)) {
		helpMessage(tokens, cursor, "Expected right parenthesis")
		return nil, initialCursor, false
	}
	cursor++

	return &CreateTableStatement{
		Name: *name,
		Cols: cols,
	}, cursor, true
}

func parseColumnDefinitions(
	tokens []*lexer.Token,
	initialCursor uint,
	delimiter lexer.Token,
) (*[]*ColumnDefinition, uint, bool) {

	cursor := initialCursor
	cds := []*ColumnDefinition{}

	for {
		if cursor >= uint(len(tokens)) {
			return nil, initialCursor, false
		}

		// stop condition (BUT must check AFTER comma handling)
		if delimiter.Equals(tokens[cursor]) {
			break
		}

		// comma between columns
		if len(cds) > 0 {
			if !expectToken(tokens, cursor, tokenFromSymbol(lexer.CommaSymbol)) {
				helpMessage(tokens, cursor, "Expected comma")
				return nil, initialCursor, false
			}
			cursor++
		}

		// column name
		id, newCursor, ok := parseToken(tokens, cursor, lexer.IdentifierKind)
		if !ok {
			helpMessage(tokens, cursor, "Expected column name")
			return nil, initialCursor, false
		}
		cursor = newCursor

		// column type
		ty, newCursor, ok := parseToken(tokens, cursor, lexer.KeywordKind)
		if !ok {
			helpMessage(tokens, cursor, "Expected column type")
			return nil, initialCursor, false
		}
		cursor = newCursor

		pk := false

		if cursor+1 < uint(len(tokens)) {

			if expectToken(tokens, cursor, tokenFromKeyword(lexer.PrimaryKeyword)) &&
				expectToken(tokens, cursor+1, tokenFromKeyword(lexer.KeyKeyword)) {

				cursor += 2
				pk = true
			}
		}

		cds = append(cds, &ColumnDefinition{
			Name:       *id,
			Datatype:   *ty,
			PrimaryKey: pk,
		})

		// IMPORTANT: re-check delimiter AFTER consuming full column
		if cursor < uint(len(tokens)) && delimiter.Equals(tokens[cursor]) {
			break
		}
	}

	return &cds, cursor, true
}