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
	Values *[]*expression
}

type expressionKind uint

const (
	LiteralKind expressionKind = iota
	    WildcardKind
)

type expression struct {
	Literal *lexer.Token
	Kind    expressionKind
}

type ColumnDefinition struct {
	Name     lexer.Token
	Datatype lexer.Token
}

type CreateTableStatement struct {
	Name lexer.Token
	Cols *[]*ColumnDefinition
}

type SelectStatement struct {
	Item []*expression
	From lexer.Token
}

type ExpressionKind uint

