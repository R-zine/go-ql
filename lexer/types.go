package lexer

type Location struct {
	Line uint
	Col  uint
}

type Keyword string

const (
	SelectKeyword  Keyword = "select"
	FromKeyword    Keyword = "from"
	TableKeyword   Keyword = "table"
	CreateKeyword  Keyword = "create"
	InsertKeyword  Keyword = "insert"
	IntoKeyword    Keyword = "into"
	ValuesKeyword  Keyword = "values"
	WhereKeyword   Keyword = "where"
	IntKeyword     Keyword = "int"
	TextKeyword    Keyword = "text"
	PrimaryKeyword Keyword = "primary"
	KeyKeyword     Keyword = "key"
)

type Symbol string

const (
	SemicolonSymbol          Symbol = ";"
	AsteriskSymbol           Symbol = "*"
	CommaSymbol              Symbol = ","
	LeftParenSymbol          Symbol = "("
	RightParenSymbol         Symbol = ")"
	EqualsSymbol             Symbol = "="
	NotEqualsSymbol          Symbol = "!="
	LessThanSymbol           Symbol = "<"
	GreaterThanSymbol        Symbol = ">"
	LessThanOrEqualSymbol    Symbol = "<="
	GreaterThanOrEqualSymbol Symbol = ">="
)

type TokenKind uint

const (
	KeywordKind TokenKind = iota
	SymbolKind
	IdentifierKind
	StringKind
	NumericKind
)

type Token struct {
	Value string
	Kind  TokenKind
	Loc   Location
}

type cursor struct {
	pointer uint
	loc     Location
}

type lexer func(string, cursor) (*Token, cursor, bool)
