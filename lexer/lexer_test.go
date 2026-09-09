package lexer

import "testing"

func TestLexStatementAndKeywordBoundaries(t *testing.T) {
	tokens, err := Lex(`SELECT fromage, text_data FROM "Odd Name" WHERE id <= -12;`)
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}

	want := []struct {
		value string
		kind  TokenKind
	}{
		{"select", KeywordKind},
		{"fromage", IdentifierKind},
		{",", SymbolKind},
		{"text_data", IdentifierKind},
		{"from", KeywordKind},
		{"Odd Name", IdentifierKind},
		{"where", KeywordKind},
		{"id", IdentifierKind},
		{"<=", SymbolKind},
		{"-12", NumericKind},
		{";", SymbolKind},
	}
	if len(tokens) != len(want) {
		t.Fatalf("Lex() returned %d tokens, want %d: %#v", len(tokens), len(want), tokens)
	}
	for index, expected := range want {
		if tokens[index].Value != expected.value || tokens[index].Kind != expected.kind {
			t.Errorf("token %d = (%q, %d), want (%q, %d)", index, tokens[index].Value, tokens[index].Kind, expected.value, expected.kind)
		}
	}
}

func TestLexDelimitedValuesAndEscapes(t *testing.T) {
	tokens, err := Lex(`SELECT "a""b" FROM table_name WHERE name = 'O''Brien';`)
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}
	if got := tokens[1]; got.Kind != IdentifierKind || got.Value != `a"b` {
		t.Errorf("quoted identifier = (%q, %d), want (%q, %d)", got.Value, got.Kind, `a"b`, IdentifierKind)
	}
	if got := tokens[7]; got.Kind != StringKind || got.Value != "O'Brien" {
		t.Errorf("string = (%q, %d), want (%q, %d)", got.Value, got.Kind, "O'Brien", StringKind)
	}
}

func TestLexRejectsUnsupportedNumericSyntax(t *testing.T) {
	for _, source := range []string{"1.5", "1e2", ".5", "+", "-"} {
		t.Run(source, func(t *testing.T) {
			if _, err := Lex(source); err == nil {
				t.Fatalf("Lex(%q) unexpectedly succeeded", source)
			}
		})
	}
}

func TestLexTracksLinesAndColumns(t *testing.T) {
	tokens, err := Lex("SELECT\r\n  name\r\nFROM users;")
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}
	if tokens[1].Loc.Line != 1 || tokens[1].Loc.Col != 2 {
		t.Errorf("name location = %d:%d, want 1:2", tokens[1].Loc.Line, tokens[1].Loc.Col)
	}
	if tokens[2].Loc.Line != 2 || tokens[2].Loc.Col != 0 {
		t.Errorf("FROM location = %d:%d, want 2:0", tokens[2].Loc.Line, tokens[2].Loc.Col)
	}
}

func TestLexTracksNewlinesInsideStrings(t *testing.T) {
	tokens, err := Lex("'first\nsecond'\nname")
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}
	if tokens[1].Loc.Line != 2 || tokens[1].Loc.Col != 0 {
		t.Errorf("identifier location = %d:%d, want 2:0", tokens[1].Loc.Line, tokens[1].Loc.Col)
	}
}

func FuzzLexNeverPanics(f *testing.F) {
	for _, source := range []string{
		"",
		"SELECT * FROM users;",
		"SELECT * FROM users WHERE id;",
		`INSERT INTO "quoted" VALUES (-2147483648, 'text');`,
		"\x00\xff",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = Lex(source)
	})
}
