package main

import (
	"errors"
	"strings"
	"testing"

	"go-ql/ast"
	"go-ql/backend"
	"go-ql/storage"
)

type memoryStore struct {
	tables map[string]*storage.Table
}

func (store *memoryStore) LoadTables() (map[string]*storage.Table, error) {
	if store.tables == nil {
		store.tables = map[string]*storage.Table{}
	}
	return store.tables, nil
}

func (store *memoryStore) SaveTables(tables map[string]*storage.Table) error {
	store.tables = tables
	return nil
}

type failingStore struct {
	err error
}

func (store *failingStore) LoadTables() (map[string]*storage.Table, error) {
	return nil, store.err
}

func (store *failingStore) SaveTables(map[string]*storage.Table) error {
	return store.err
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

type errorBackend struct {
	createErr error
	insertErr error
	selectErr error
}

func (database errorBackend) CreateTable(*ast.CreateTableStatement) error {
	return database.createErr
}

func (database errorBackend) Insert(*ast.InsertStatement) error {
	return database.insertErr
}

func (database errorBackend) Select(*ast.SelectStatement) (*backend.Results, error) {
	return nil, database.selectErr
}

func TestRunHandlesStartupAndInputErrors(t *testing.T) {
	loadErr := errors.New("load failed")
	if err := run(strings.NewReader(""), &strings.Builder{}, &failingStore{err: loadErr}); !errors.Is(err, loadErr) {
		t.Fatalf("run() load error = %v, want wrapped load error", err)
	}

	readErr := errors.New("read failed")
	if err := run(errorReader{err: readErr}, &strings.Builder{}, &memoryStore{}); !errors.Is(err, readErr) {
		t.Fatalf("run() input error = %v, want wrapped read error", err)
	}

	if err := run(strings.NewReader(""), &strings.Builder{}, &memoryStore{}); err != nil {
		t.Fatalf("run() clean EOF error = %v", err)
	}
}

func TestExecuteSourceUsesEachStatement(t *testing.T) {
	database := newMainTestBackend(t)
	var output strings.Builder

	if err := executeSource(`CREATE TABLE alpha (id INT); CREATE TABLE beta (id INT);`, &output, database); err != nil {
		t.Fatalf("executeSource() error = %v", err)
	}
	if got := strings.Count(output.String(), "ok\n"); got != 2 {
		t.Fatalf("ok count = %d, want 2; output:\n%s", got, output.String())
	}
	for _, table := range []string{"alpha", "beta"} {
		if err := executeSource("SELECT * FROM "+table+";", &output, database); err != nil {
			t.Errorf("select from %s error = %v", table, err)
		}
	}
}

func TestRunREPLSupportsMultilineInputAndRecoversFromErrors(t *testing.T) {
	database := newMainTestBackend(t)
	input := strings.NewReader(`CREATE TABLE users (
id INT PRIMARY KEY,
name TEXT
);
INSERT INTO users VALUES (1, 'semi;colon');
SELECT name FROM users;
CREATE TABLE users (other TEXT);
BROKEN;
SELECT name FROM users WHERE id = 1;
`)
	var output strings.Builder

	if err := runREPL(input, &output, database); err != nil {
		t.Fatalf("runREPL() error = %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "| name |") || strings.Count(got, "| semi;colon |") != 2 {
		t.Errorf("projected text was not rendered twice:\n%s", got)
	}
	if !strings.Contains(got, "error: parse error") {
		t.Errorf("syntax error was not reported:\n%s", got)
	}
	if !strings.Contains(got, "error: table already exists") {
		t.Errorf("backend error was not reported:\n%s", got)
	}
	if !strings.HasSuffix(got, "# ") {
		t.Errorf("REPL did not terminate cleanly at EOF; output suffix = %q", got[max(0, len(got)-20):])
	}
}

func TestRunREPLReportsIncompleteEOF(t *testing.T) {
	database := newMainTestBackend(t)
	var output strings.Builder
	if err := runREPL(strings.NewReader("SELECT * FROM users"), &output, database); err != nil {
		t.Fatalf("runREPL() error = %v", err)
	}
	if !strings.Contains(output.String(), "incomplete statement at end of input") {
		t.Errorf("output = %q, want incomplete statement error", output.String())
	}
}

func TestHasStatementTerminatorIgnoresQuotedSemicolons(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{`INSERT INTO t VALUES (';')`, false},
		{`SELECT ";" FROM t`, false},
		{`INSERT INTO t VALUES ('it''s;fine');`, true},
		{`SELECT "a"";b" FROM t;`, true},
		{`SELECT * FROM t;`, true},
	}
	for _, test := range tests {
		if got := hasStatementTerminator(test.source); got != test.want {
			t.Errorf("hasStatementTerminator(%q) = %v, want %v", test.source, got, test.want)
		}
	}
}

func TestPrintResultsRejectsBrokenResultInvariant(t *testing.T) {
	results := &backend.Results{
		Columns: []backend.ResultColumn{{Name: "id", Type: storage.IntType}},
		Rows:    [][]storage.MemoryCell{{}},
	}
	if err := printResults(&strings.Builder{}, results); err == nil {
		t.Fatal("printResults() unexpectedly accepted a mismatched row")
	}
	if err := printResults(&strings.Builder{}, nil); err == nil {
		t.Fatal("printResults(nil) unexpectedly succeeded")
	}
	invalidInteger := &backend.Results{
		Columns: []backend.ResultColumn{{Name: "id", Type: storage.IntType}},
		Rows:    [][]storage.MemoryCell{{{1}}},
	}
	if err := printResults(&strings.Builder{}, invalidInteger); !errors.Is(err, storage.ErrInvalidDatabase) {
		t.Fatalf("printResults(invalid INT) error = %v, want ErrInvalidDatabase", err)
	}
	unknownType := &backend.Results{
		Columns: []backend.ResultColumn{{Name: "id", Type: 99}},
		Rows:    [][]storage.MemoryCell{{{1}}},
	}
	if err := printResults(&strings.Builder{}, unknownType); err == nil {
		t.Fatal("printResults(unknown type) unexpectedly succeeded")
	}
}

func TestExecuteSourcePropagatesBackendErrors(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		database backend.Backend
	}{
		{"create", `CREATE TABLE users (id INT);`, errorBackend{createErr: errors.New("create failed")}},
		{"insert", `INSERT INTO users VALUES (1);`, errorBackend{insertErr: errors.New("insert failed")}},
		{"select", `SELECT * FROM users;`, errorBackend{selectErr: errors.New("select failed")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := executeSource(test.source, &strings.Builder{}, test.database); err == nil {
				t.Fatal("executeSource() unexpectedly succeeded")
			}
		})
	}
}

func newMainTestBackend(t *testing.T) *backend.MemoryBackend {
	t.Helper()
	database, err := backend.NewMemoryBackend(&memoryStore{})
	if err != nil {
		t.Fatalf("NewMemoryBackend() error = %v", err)
	}
	return database
}
