package ast

import (
	"fmt"

	"go-ql/lexer"
)

type parser struct {
	tokens []*lexer.Token
	cursor int
}

func Parse(source string) (*Ast, error) {
	tokens, err := lexer.Lex(source)
	if err != nil {
		return nil, err
	}

	p := parser{tokens: tokens}
	result := &Ast{}

	for !p.atEnd() {
		statement, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		result.Statements = append(result.Statements, statement)

		if !p.matchSymbol(lexer.SemicolonSymbol) {
			return nil, p.expected("semicolon after statement")
		}
		for p.matchSymbol(lexer.SemicolonSymbol) {
		}
	}

	return result, nil
}

func (p *parser) parseStatement() (*Statement, error) {
	switch {
	case p.checkKeyword(lexer.SelectKeyword):
		statement, err := p.parseSelectStatement()
		if err != nil {
			return nil, err
		}
		return &Statement{Kind: SelectKind, SelectStatement: statement}, nil
	case p.checkKeyword(lexer.InsertKeyword):
		statement, err := p.parseInsertStatement()
		if err != nil {
			return nil, err
		}
		return &Statement{Kind: InsertKind, InsertStatement: statement}, nil
	case p.checkKeyword(lexer.CreateKeyword):
		statement, err := p.parseCreateTableStatement()
		if err != nil {
			return nil, err
		}
		return &Statement{Kind: CreateTableKind, CreateTableStatement: statement}, nil
	default:
		return nil, p.expected("SELECT, INSERT, or CREATE statement")
	}
}

func (p *parser) parseSelectStatement() (*SelectStatement, error) {
	p.advance() // SELECT

	items := make([]*Expression, 0, 1)
	for {
		if p.checkKeyword(lexer.FromKeyword) {
			break
		}
		if p.atEnd() || p.checkSymbol(lexer.SemicolonSymbol) {
			return nil, p.expected("FROM clause")
		}

		item, err := p.parseSelectItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)

		if p.matchSymbol(lexer.CommaSymbol) {
			if p.checkKeyword(lexer.FromKeyword) {
				return nil, p.expected("select item after comma")
			}
			continue
		}
		break
	}

	if len(items) == 0 {
		return nil, p.expected("select item")
	}
	if !p.matchKeyword(lexer.FromKeyword) {
		return nil, p.expected("FROM clause")
	}

	from, err := p.consumeKind(lexer.IdentifierKind, "table name")
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.Kind == WildcardKind && len(items) != 1 {
			return nil, p.errorAt(itemToken(item), "wildcard cannot be combined with other select items")
		}
	}

	statement := &SelectStatement{Item: items, From: *from}
	if p.matchKeyword(lexer.WhereKeyword) {
		where, err := p.parseWhere()
		if err != nil {
			return nil, err
		}
		statement.Where = where
	}

	return statement, nil
}

func (p *parser) parseSelectItem() (*Expression, error) {
	if p.matchSymbol(lexer.AsteriskSymbol) {
		return &Expression{Kind: WildcardKind}, nil
	}

	token, err := p.consumeKind(lexer.IdentifierKind, "column name or wildcard")
	if err != nil {
		return nil, err
	}
	return &Expression{Kind: LiteralKind, Literal: token}, nil
}

func (p *parser) parseWhere() (*WhereClause, error) {
	left, err := p.consumeKind(lexer.IdentifierKind, "column name in WHERE clause")
	if err != nil {
		return nil, err
	}

	operator, err := p.consumeComparisonOperator()
	if err != nil {
		return nil, err
	}

	right := p.peek()
	if right == nil || (right.Kind != lexer.NumericKind && right.Kind != lexer.StringKind) {
		return nil, p.expected("numeric or string literal in WHERE clause")
	}
	p.advance()

	return &WhereClause{Left: *left, Operator: *operator, Right: *right}, nil
}

func (p *parser) parseInsertStatement() (*InsertStatement, error) {
	p.advance() // INSERT
	if !p.matchKeyword(lexer.IntoKeyword) {
		return nil, p.expected("INTO")
	}

	table, err := p.consumeKind(lexer.IdentifierKind, "table name")
	if err != nil {
		return nil, err
	}
	if !p.matchKeyword(lexer.ValuesKeyword) {
		return nil, p.expected("VALUES")
	}
	if !p.matchSymbol(lexer.LeftParenSymbol) {
		return nil, p.expected("left parenthesis")
	}

	values := make([]*Expression, 0, 1)
	for {
		if p.checkSymbol(lexer.RightParenSymbol) {
			break
		}

		value := p.peek()
		if value == nil || (value.Kind != lexer.NumericKind && value.Kind != lexer.StringKind) {
			return nil, p.expected("numeric or string value")
		}
		p.advance()
		values = append(values, &Expression{Kind: LiteralKind, Literal: value})

		if p.matchSymbol(lexer.CommaSymbol) {
			if p.checkSymbol(lexer.RightParenSymbol) {
				return nil, p.expected("value after comma")
			}
			continue
		}
		break
	}

	if len(values) == 0 {
		return nil, p.expected("at least one value")
	}
	if !p.matchSymbol(lexer.RightParenSymbol) {
		return nil, p.expected("right parenthesis")
	}

	return &InsertStatement{Table: *table, Values: &values}, nil
}

func (p *parser) parseCreateTableStatement() (*CreateTableStatement, error) {
	p.advance() // CREATE
	if !p.matchKeyword(lexer.TableKeyword) {
		return nil, p.expected("TABLE")
	}

	name, err := p.consumeKind(lexer.IdentifierKind, "table name")
	if err != nil {
		return nil, err
	}
	if !p.matchSymbol(lexer.LeftParenSymbol) {
		return nil, p.expected("left parenthesis")
	}

	columns := make([]*ColumnDefinition, 0, 1)
	for {
		if p.checkSymbol(lexer.RightParenSymbol) {
			break
		}

		column, err := p.parseColumnDefinition()
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)

		if p.matchSymbol(lexer.CommaSymbol) {
			if p.checkSymbol(lexer.RightParenSymbol) {
				return nil, p.expected("column definition after comma")
			}
			continue
		}
		break
	}

	if len(columns) == 0 {
		return nil, p.expected("at least one column definition")
	}
	if !p.matchSymbol(lexer.RightParenSymbol) {
		return nil, p.expected("right parenthesis")
	}

	return &CreateTableStatement{Name: *name, Cols: &columns}, nil
}

func (p *parser) parseColumnDefinition() (*ColumnDefinition, error) {
	name, err := p.consumeKind(lexer.IdentifierKind, "column name")
	if err != nil {
		return nil, err
	}

	datatype := p.peek()
	if datatype == nil || datatype.Kind != lexer.KeywordKind ||
		(datatype.Value != string(lexer.IntKeyword) && datatype.Value != string(lexer.TextKeyword)) {
		return nil, p.expected("INT or TEXT column type")
	}
	p.advance()

	primaryKey := false
	if p.matchKeyword(lexer.PrimaryKeyword) {
		if !p.matchKeyword(lexer.KeyKeyword) {
			return nil, p.expected("KEY after PRIMARY")
		}
		primaryKey = true
	}

	return &ColumnDefinition{Name: *name, Datatype: *datatype, PrimaryKey: primaryKey}, nil
}

func (p *parser) consumeComparisonOperator() (*lexer.Token, error) {
	token := p.peek()
	if token == nil || token.Kind != lexer.SymbolKind {
		return nil, p.expected("comparison operator")
	}

	switch lexer.Symbol(token.Value) {
	case lexer.EqualsSymbol, lexer.NotEqualsSymbol, lexer.LessThanSymbol,
		lexer.GreaterThanSymbol, lexer.LessThanOrEqualSymbol, lexer.GreaterThanOrEqualSymbol:
		p.advance()
		return token, nil
	default:
		return nil, p.expected("comparison operator")
	}
}

func (p *parser) consumeKind(kind lexer.TokenKind, expected string) (*lexer.Token, error) {
	token := p.peek()
	if token == nil || token.Kind != kind {
		return nil, p.expected(expected)
	}
	p.advance()
	return token, nil
}

func (p *parser) matchKeyword(keyword lexer.Keyword) bool {
	if !p.checkKeyword(keyword) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) checkKeyword(keyword lexer.Keyword) bool {
	token := p.peek()
	return token != nil && token.Kind == lexer.KeywordKind && token.Value == string(keyword)
}

func (p *parser) matchSymbol(symbol lexer.Symbol) bool {
	if !p.checkSymbol(symbol) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) checkSymbol(symbol lexer.Symbol) bool {
	token := p.peek()
	return token != nil && token.Kind == lexer.SymbolKind && token.Value == string(symbol)
}

func (p *parser) peek() *lexer.Token {
	if p.atEnd() {
		return nil
	}
	return p.tokens[p.cursor]
}

func (p *parser) advance() {
	if !p.atEnd() {
		p.cursor++
	}
}

func (p *parser) atEnd() bool {
	return p.cursor >= len(p.tokens)
}

func (p *parser) expected(description string) error {
	return p.errorAt(p.peek(), "expected "+description)
}

func (p *parser) errorAt(token *lexer.Token, message string) error {
	if token != nil {
		return fmt.Errorf("parse error at %d:%d: %s, got %q", token.Loc.Line, token.Loc.Col, message, token.Value)
	}
	if len(p.tokens) > 0 {
		last := p.tokens[len(p.tokens)-1]
		return fmt.Errorf("parse error after %d:%d: %s", last.Loc.Line, last.Loc.Col, message)
	}
	return fmt.Errorf("parse error: %s", message)
}

func itemToken(item *Expression) *lexer.Token {
	if item == nil {
		return nil
	}
	return item.Literal
}
