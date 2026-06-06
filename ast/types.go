package ast

import (
	"go-ql/lexer"
)

type Ast struct {
	Statements []*Statement
}

type AstKind uint

const (
	SelectKind AstKind = iota
	CreateTableKind
	InsertKind
)

type Statement struct {
	SelectStatement      *SelectStatement
	CreateTableStatement *CreateTableStatement
	InsertStatement      *InsertStatement
	Kind                 AstKind
}

type InsertStatement struct {
	Table  lexer.Token
	Values *[]*Expression
}

type expressionKind uint

const (
	LiteralKind expressionKind = iota
	WildcardKind
)

type Expression struct {
	Literal *lexer.Token
	Kind    expressionKind
}

type ColumnDefinition struct {
    Name       lexer.Token
    Datatype   lexer.Token
    PrimaryKey bool
}

type CreateTableStatement struct {
	Name lexer.Token
	Cols *[]*ColumnDefinition
}

type WhereClause struct {
	Left     lexer.Token
	Operator lexer.Token
	Right    lexer.Token
}

type SelectStatement struct {
	Item  []*Expression
	From  lexer.Token
	Where *WhereClause
}

type ExpressionKind uint
