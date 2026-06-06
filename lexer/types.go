package lexer

type location struct {
	Line uint
	Col  uint
}

type Keyword string

const (
	SelectKeyword Keyword = "select"
	FromKeyword   Keyword = "from"
	AsKeyword     Keyword = "as"
	TableKeyword  Keyword = "table"
	CreateKeyword Keyword = "create"
	InsertKeyword Keyword = "insert"
	IntoKeyword   Keyword = "into"
	ValuesKeyword Keyword = "values"
	WhereKeyword  Keyword = "where"
	IntKeyword    Keyword = "int"
	TextKeyword   Keyword = "text"
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
	LiteralKind
)

type Token struct {
	Value string
	Kind  TokenKind
	Loc   location
}

type cursor struct {
	pointer uint
	loc     location
}

func (t *Token) Equals(other *Token) bool {
	return t.Value == other.Value && t.Kind == other.Kind
}

type lexer func(string, cursor) (*Token, cursor, bool)
