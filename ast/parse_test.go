package ast

import (
	"strings"
	"testing"

	"go-ql/lexer"
)

func TestParseMultipleStatements(t *testing.T) {
	source := `
CREATE TABLE "People" (
    id INT PRIMARY KEY,
    name TEXT
);
INSERT INTO "People" VALUES (-1, 'O''Brien');
SELECT name FROM "People" WHERE id >= -1;
`
	parsed, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed.Statements) != 3 {
		t.Fatalf("statement count = %d, want 3", len(parsed.Statements))
	}

	create := parsed.Statements[0].CreateTableStatement
	if create == nil || create.Name.Value != "People" || len(*create.Cols) != 2 {
		t.Fatalf("unexpected CREATE statement: %#v", create)
	}
	if !(*create.Cols)[0].PrimaryKey {
		t.Error("id column is not marked as the primary key")
	}

	insert := parsed.Statements[1].InsertStatement
	if insert == nil || (*insert.Values)[0].Literal.Value != "-1" || (*insert.Values)[1].Literal.Value != "O'Brien" {
		t.Fatalf("unexpected INSERT statement: %#v", insert)
	}

	selectStatement := parsed.Statements[2].SelectStatement
	if selectStatement == nil || selectStatement.From.Value != "People" || selectStatement.Where == nil {
		t.Fatalf("unexpected SELECT statement: %#v", selectStatement)
	}
	if selectStatement.Where.Operator.Value != ">=" || selectStatement.Where.Right.Kind != lexer.NumericKind {
		t.Errorf("unexpected WHERE clause: %#v", selectStatement.Where)
	}
}

func TestParseKeepsDistinctCreateStatements(t *testing.T) {
	parsed, err := Parse(`CREATE TABLE alpha (id INT); CREATE TABLE beta (id INT);`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := parsed.Statements[0].CreateTableStatement.Name.Value; got != "alpha" {
		t.Errorf("first table = %q, want alpha", got)
	}
	if got := parsed.Statements[1].CreateTableStatement.Name.Value; got != "beta" {
		t.Errorf("second table = %q, want beta", got)
	}
}

func TestParseRejectsInvalidStatementsWithoutPanicking(t *testing.T) {
	tests := map[string]string{
		"missing semicolon":       `SELECT * FROM users`,
		"missing FROM":            `SELECT id;`,
		"empty select list":       `SELECT FROM users;`,
		"literal select item":     `SELECT 1 FROM users;`,
		"mixed wildcard":          `SELECT *, id FROM users;`,
		"missing where operator":  `SELECT * FROM users WHERE id;`,
		"missing where value":     `SELECT * FROM users WHERE id =;`,
		"missing where column":    `SELECT * FROM users WHERE;`,
		"column comparison":       `SELECT * FROM users WHERE id = other;`,
		"empty insert values":     `INSERT INTO users VALUES ();`,
		"identifier insert value": `INSERT INTO users VALUES (value);`,
		"insert trailing comma":   `INSERT INTO users VALUES (1,);`,
		"empty table":             `CREATE TABLE users ();`,
		"invalid datatype":        `CREATE TABLE users (id SELECT);`,
		"incomplete primary key":  `CREATE TABLE users (id INT PRIMARY);`,
		"column trailing comma":   `CREATE TABLE users (id INT,);`,
		"select trailing comma":   `SELECT id, FROM users;`,
		"trailing tokens":         `SELECT * FROM users unexpected;`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(source)
			if err == nil {
				t.Fatalf("Parse(%q) unexpectedly succeeded", source)
			}
			if !strings.Contains(err.Error(), "parse error") {
				t.Errorf("error = %q, want location-aware parse error", err)
			}
		})
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	for _, source := range []string{
		"",
		"SELECT * FROM users;",
		"SELECT * FROM users WHERE id;",
		"INSERT INTO users VALUES (1, 'Phil');",
		"CREATE TABLE users (id INT PRIMARY KEY, name TEXT);",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = Parse(source)
	})
}
