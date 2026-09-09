package backend

import (
	"errors"
	"strconv"
	"testing"

	"go-ql/ast"
	"go-ql/lexer"
	"go-ql/storage"
)

type stubStore struct {
	tables  map[string]*storage.Table
	loadErr error
	saveErr error
	saves   int
}

func (store *stubStore) LoadTables() (map[string]*storage.Table, error) {
	if store.loadErr != nil {
		return nil, store.loadErr
	}
	if store.tables == nil {
		return map[string]*storage.Table{}, nil
	}
	return store.tables, nil
}

func (store *stubStore) SaveTables(map[string]*storage.Table) error {
	store.saves++
	return store.saveErr
}

func TestNewMemoryBackendPropagatesAndValidatesLoadErrors(t *testing.T) {
	loadErr := errors.New("load failed")
	if _, err := NewMemoryBackend(&stubStore{loadErr: loadErr}); !errors.Is(err, loadErr) {
		t.Fatalf("NewMemoryBackend() error = %v, want wrapped load error", err)
	}
	if _, err := NewMemoryBackend(nil); err == nil {
		t.Fatal("NewMemoryBackend(nil) unexpectedly succeeded")
	}

	invalid := map[string]*storage.Table{
		"users": {Columns: []string{"id"}, ColumnTypes: []storage.ColumnType{storage.IntType}, PrimaryKeyColumn: -1, Rows: [][]storage.MemoryCell{{}}},
	}
	if _, err := NewMemoryBackend(&stubStore{tables: invalid}); !errors.Is(err, storage.ErrInvalidDatabase) {
		t.Fatalf("NewMemoryBackend(invalid) error = %v, want ErrInvalidDatabase", err)
	}
}

func TestCreateTableValidationAndDuplicateProtection(t *testing.T) {
	database, store := newTestBackend(t)
	createUsers(t, database)
	if store.saves != 1 {
		t.Fatalf("save count = %d, want 1", store.saves)
	}

	if err := database.CreateTable(createStatement("users", column("other", "text", false))); !errors.Is(err, ErrTableAlreadyExists) {
		t.Fatalf("duplicate CreateTable() error = %v, want ErrTableAlreadyExists", err)
	}
	if got := database.tables["users"].Columns; len(got) != 2 || got[0] != "id" {
		t.Fatalf("duplicate create changed schema: %v", got)
	}

	tests := map[string]struct {
		statement *ast.CreateTableStatement
		want      error
	}{
		"nil statement": {nil, ErrInvalidValue},
		"no columns": {
			&ast.CreateTableStatement{Name: identifier("empty")},
			ErrInvalidValue,
		},
		"duplicate columns": {
			createStatement("duplicates", column("id", "int", false), column("id", "int", false)),
			ErrInvalidValue,
		},
		"unknown datatype": {
			createStatement("unknown", column("id", "float", false)),
			ErrInvalidDatatype,
		},
		"text primary key": {
			createStatement("text_key", column("id", "text", true)),
			ErrInvalidDatatype,
		},
		"multiple primary keys": {
			createStatement("many_keys", column("id", "int", true), column("other", "int", true)),
			ErrInvalidValue,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := database.CreateTable(test.statement); !errors.Is(err, test.want) {
				t.Fatalf("CreateTable() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestInsertValidatesTypesRangeCountAndPrimaryKey(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)

	tests := map[string]struct {
		statement *ast.InsertStatement
		want      error
	}{
		"nil statement": {nil, ErrInvalidValue},
		"wrong count": {
			insertStatement("users", numeric("1")),
			ErrMissingValues,
		},
		"string in int": {
			insertStatement("users", text("one"), text("Phil")),
			ErrInvalidValue,
		},
		"number in text": {
			insertStatement("users", numeric("1"), numeric("2")),
			ErrInvalidValue,
		},
		"int overflow": {
			insertStatement("users", numeric("2147483648"), text("Phil")),
			ErrInvalidValue,
		},
		"identifier value": {
			insertStatement("users", &ast.Expression{Kind: ast.LiteralKind, Literal: token("value", lexer.IdentifierKind)}, text("Phil")),
			ErrInvalidValue,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := database.Insert(test.statement); !errors.Is(err, test.want) {
				t.Fatalf("Insert() error = %v, want %v", err, test.want)
			}
		})
	}

	if err := database.Insert(insertStatement("users", numeric("-2147483648"), text("Phil"))); err != nil {
		t.Fatalf("valid Insert() error = %v", err)
	}
	if err := database.Insert(insertStatement("users", numeric("-2147483648"), text("Kate"))); !errors.Is(err, ErrDuplicatePrimaryKey) {
		t.Fatalf("duplicate Insert() error = %v, want ErrDuplicatePrimaryKey", err)
	}
}

func TestSelectProjectionMetadataAndFiltering(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)
	for _, row := range []*ast.InsertStatement{
		insertStatement("users", numeric("1"), text("Phil")),
		insertStatement("users", numeric("2"), text("Kate")),
	} {
		if err := database.Insert(row); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	tests := []struct {
		name       string
		statement  *ast.SelectStatement
		wantNames  []string
		wantValues []string
	}{
		{
			name:       "project text column",
			statement:  selectColumns("users", []string{"name"}, nil),
			wantNames:  []string{"name"},
			wantValues: []string{"Phil", "Kate"},
		},
		{
			name:      "filter skips first physical row",
			statement: selectColumns("users", []string{"name"}, where("id", "!=", "1", lexer.NumericKind)),
			wantNames: []string{"name"}, wantValues: []string{"Kate"},
		},
		{
			name:      "missing primary key still has metadata",
			statement: selectColumns("users", []string{"name"}, where("id", "=", "99", lexer.NumericKind)),
			wantNames: []string{"name"}, wantValues: nil,
		},
		{
			name:      "text predicate",
			statement: selectColumns("users", []string{"id"}, where("name", "=", "Phil", lexer.StringKind)),
			wantNames: []string{"id"}, wantValues: []string{"1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results, err := database.Select(test.statement)
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}
			if len(results.Columns) != len(test.wantNames) {
				t.Fatalf("column count = %d, want %d", len(results.Columns), len(test.wantNames))
			}
			for index, name := range test.wantNames {
				if results.Columns[index].Name != name {
					t.Errorf("column %d = %q, want %q", index, results.Columns[index].Name, name)
				}
			}

			values := make([]string, len(results.Rows))
			for index, row := range results.Rows {
				if len(row) != len(results.Columns) {
					t.Fatalf("row %d has %d cells, want %d", index, len(row), len(results.Columns))
				}
				if results.Columns[0].Type == storage.IntType {
					value, err := row[0].AsInt()
					if err != nil {
						t.Fatal(err)
					}
					values[index] = strconv.FormatInt(int64(value), 10)
				} else {
					values[index] = row[0].AsText()
				}
			}
			if len(values) != len(test.wantValues) {
				t.Fatalf("row count = %d, want %d", len(values), len(test.wantValues))
			}
			for index := range values {
				if values[index] != test.wantValues[index] {
					t.Errorf("row %d value = %q, want %q", index, values[index], test.wantValues[index])
				}
			}
		})
	}
}

func TestSelectEmptyTableWildcardAndResultIsolation(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)

	empty, err := database.Select(&ast.SelectStatement{From: identifier("users"), Item: []*ast.Expression{{Kind: ast.WildcardKind}}})
	if err != nil {
		t.Fatalf("empty Select() error = %v", err)
	}
	if len(empty.Columns) != 2 || len(empty.Rows) != 0 {
		t.Fatalf("empty result = %#v, want two columns and no rows", empty)
	}

	if err := database.Insert(insertStatement("users", numeric("1"), text("Phil"))); err != nil {
		t.Fatal(err)
	}
	first, err := database.Select(selectColumns("users", []string{"name"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	first.Rows[0][0][0] = 'X'
	second, err := database.Select(selectColumns("users", []string{"name"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := second.Rows[0][0].AsText(); got != "Phil" {
		t.Errorf("caller mutation changed stored value to %q", got)
	}
}

func TestSelectSupportsAllIntegerOperatorsAndPrimaryKeyFastPath(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)
	for _, value := range []string{"1", "2", "3"} {
		if err := database.Insert(insertStatement("users", numeric(value), text("user"+value))); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		operator string
		value    string
		wantRows int
	}{
		{"=", "2", 1},
		{"!=", "2", 2},
		{"<", "2", 1},
		{">", "2", 1},
		{"<=", "2", 2},
		{">=", "2", 2},
	}
	for _, test := range tests {
		t.Run(test.operator, func(t *testing.T) {
			results, err := database.Select(selectColumns("users", []string{"id"}, where("id", test.operator, test.value, lexer.NumericKind)))
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}
			if len(results.Rows) != test.wantRows {
				t.Errorf("row count = %d, want %d", len(results.Rows), test.wantRows)
			}
		})
	}
}

func TestSelectPreservesRequestedColumnOrder(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)
	if err := database.Insert(insertStatement("users", numeric("1"), text("Phil"))); err != nil {
		t.Fatal(err)
	}

	results, err := database.Select(selectColumns("users", []string{"name", "id"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Columns) != 2 || results.Columns[0].Name != "name" || results.Columns[0].Type != storage.TextType ||
		results.Columns[1].Name != "id" || results.Columns[1].Type != storage.IntType {
		t.Fatalf("unexpected projection metadata: %#v", results.Columns)
	}
	if got := results.Rows[0][0].AsText(); got != "Phil" {
		t.Errorf("first projected value = %q, want Phil", got)
	}
	id, err := results.Rows[0][1].AsInt()
	if err != nil || id != 1 {
		t.Errorf("second projected value = %d, %v; want 1, nil", id, err)
	}
}

func TestBackendRejectsMalformedDirectASTInput(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)

	if err := database.CreateTable(createStatement("users_two", nil)); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("nil column error = %v, want ErrInvalidValue", err)
	}
	badName := createStatement("users_three", column("id", "int", false))
	badName.Name.Kind = lexer.StringKind
	if err := database.CreateTable(badName); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("bad table token error = %v, want ErrInvalidValue", err)
	}
	badDatatype := createStatement("users_four", column("id", "int", false))
	(*badDatatype.Cols)[0].Datatype.Kind = lexer.IdentifierKind
	if err := database.CreateTable(badDatatype); !errors.Is(err, ErrInvalidDatatype) {
		t.Errorf("bad datatype token error = %v, want ErrInvalidDatatype", err)
	}

	badInsertTable := insertStatement("users", numeric("1"), text("Phil"))
	badInsertTable.Table.Kind = lexer.StringKind
	if err := database.Insert(badInsertTable); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("bad insert table token error = %v, want ErrInvalidValue", err)
	}
	if err := database.Insert(insertStatement("missing", numeric("1"), text("Phil"))); !errors.Is(err, ErrTableDoesNotExist) {
		t.Errorf("missing insert table error = %v, want ErrTableDoesNotExist", err)
	}
	values := []*ast.Expression{nil, text("Phil")}
	if err := database.Insert(&ast.InsertStatement{Table: identifier("users"), Values: &values}); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("nil insert value error = %v, want ErrInvalidValue", err)
	}

	if _, err := database.Select(nil); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("nil Select error = %v, want ErrInvalidValue", err)
	}
	badSelectTable := selectColumns("users", []string{"id"}, nil)
	badSelectTable.From.Kind = lexer.StringKind
	if _, err := database.Select(badSelectTable); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("bad select table token error = %v, want ErrInvalidValue", err)
	}
	if _, err := database.Select(selectColumns("missing", []string{"id"}, nil)); !errors.Is(err, ErrTableDoesNotExist) {
		t.Errorf("missing select table error = %v, want ErrTableDoesNotExist", err)
	}
	badWhere := where("id", "=", "1", lexer.NumericKind)
	badWhere.Operator.Kind = lexer.IdentifierKind
	if _, err := database.Select(selectColumns("users", []string{"id"}, badWhere)); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("bad WHERE operator token error = %v, want ErrInvalidValue", err)
	}
}

func TestSelectDetectsDamagedInMemoryRowsAndIndexes(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)
	database.tables["users"].Rows = [][]storage.MemoryCell{{storage.NewIntCell(1)}}
	if _, err := database.Select(selectColumns("users", []string{"id"}, nil)); !errors.Is(err, storage.ErrInvalidDatabase) {
		t.Fatalf("damaged row Select() error = %v, want ErrInvalidDatabase", err)
	}

	database.tables["users"].Rows = [][]storage.MemoryCell{{storage.NewIntCell(1), storage.MemoryCell("Phil")}}
	database.tables["users"].PrimaryKeyIndex = map[int32]int{1: 99}
	if _, err := database.Select(selectColumns("users", []string{"id"}, where("id", "=", "1", lexer.NumericKind))); !errors.Is(err, storage.ErrInvalidDatabase) {
		t.Fatalf("damaged index Select() error = %v, want ErrInvalidDatabase", err)
	}
}

func TestSelectRejectsInvalidProjectionAndWhere(t *testing.T) {
	database, _ := newTestBackend(t)
	createUsers(t, database)

	tests := map[string]struct {
		statement *ast.SelectStatement
		want      error
	}{
		"missing column": {
			selectColumns("users", []string{"missing"}, nil),
			ErrColumnDoesNotExist,
		},
		"empty projection": {
			&ast.SelectStatement{From: identifier("users")},
			ErrInvalidSelectItem,
		},
		"mixed wildcard": {
			&ast.SelectStatement{From: identifier("users"), Item: []*ast.Expression{{Kind: ast.WildcardKind}, literalIdentifier("id")}},
			ErrInvalidSelectItem,
		},
		"missing where column": {
			selectColumns("users", []string{"id"}, where("missing", "=", "1", lexer.NumericKind)),
			ErrColumnDoesNotExist,
		},
		"wrong int literal type": {
			selectColumns("users", []string{"id"}, where("id", "=", "1", lexer.StringKind)),
			ErrInvalidValue,
		},
		"wrong text literal type": {
			selectColumns("users", []string{"id"}, where("name", "=", "1", lexer.NumericKind)),
			ErrInvalidValue,
		},
		"text ordering": {
			selectColumns("users", []string{"id"}, where("name", ">", "A", lexer.StringKind)),
			ErrInvalidValue,
		},
		"int overflow": {
			selectColumns("users", []string{"id"}, where("id", "=", "2147483648", lexer.NumericKind)),
			ErrInvalidValue,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := database.Select(test.statement); !errors.Is(err, test.want) {
				t.Fatalf("Select() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSaveFailureRollsBackMutations(t *testing.T) {
	database, store := newTestBackend(t)
	saveErr := errors.New("disk full")
	store.saveErr = saveErr

	if err := database.CreateTable(createStatement("failed", column("id", "int", false))); !errors.Is(err, saveErr) {
		t.Fatalf("CreateTable() error = %v, want wrapped save error", err)
	}
	if _, exists := database.tables["failed"]; exists {
		t.Error("failed CREATE remained in memory")
	}

	store.saveErr = nil
	createUsers(t, database)
	store.saveErr = saveErr
	insert := insertStatement("users", numeric("1"), text("Phil"))
	if err := database.Insert(insert); !errors.Is(err, saveErr) {
		t.Fatalf("Insert() error = %v, want wrapped save error", err)
	}
	if len(database.tables["users"].Rows) != 0 || len(database.tables["users"].PrimaryKeyIndex) != 0 {
		t.Fatal("failed INSERT remained in memory or in the primary-key index")
	}

	store.saveErr = nil
	if err := database.Insert(insert); err != nil {
		t.Fatalf("retry after rollback error = %v", err)
	}
}

func BenchmarkMemoryBackendInsert(b *testing.B) {
	database, err := NewMemoryBackend(&stubStore{})
	if err != nil {
		b.Fatal(err)
	}
	if err := database.CreateTable(createStatement("users", column("id", "int", true), column("name", "text", false))); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := database.Insert(insertStatement("users", numeric(strconv.Itoa(index)), text("user"))); err != nil {
			b.Fatal(err)
		}
	}
}

func newTestBackend(t testing.TB) (*MemoryBackend, *stubStore) {
	t.Helper()
	store := &stubStore{}
	database, err := NewMemoryBackend(store)
	if err != nil {
		t.Fatalf("NewMemoryBackend() error = %v", err)
	}
	return database, store
}

func createUsers(t testing.TB, database *MemoryBackend) {
	t.Helper()
	if err := database.CreateTable(createStatement("users", column("id", "int", true), column("name", "text", false))); err != nil {
		t.Fatalf("CreateTable(users) error = %v", err)
	}
}

func createStatement(name string, columns ...*ast.ColumnDefinition) *ast.CreateTableStatement {
	return &ast.CreateTableStatement{Name: identifier(name), Cols: &columns}
}

func column(name, datatype string, primaryKey bool) *ast.ColumnDefinition {
	return &ast.ColumnDefinition{
		Name:       identifier(name),
		Datatype:   *token(datatype, lexer.KeywordKind),
		PrimaryKey: primaryKey,
	}
}

func insertStatement(table string, values ...*ast.Expression) *ast.InsertStatement {
	return &ast.InsertStatement{Table: identifier(table), Values: &values}
}

func numeric(value string) *ast.Expression {
	return &ast.Expression{Kind: ast.LiteralKind, Literal: token(value, lexer.NumericKind)}
}

func text(value string) *ast.Expression {
	return &ast.Expression{Kind: ast.LiteralKind, Literal: token(value, lexer.StringKind)}
}

func literalIdentifier(value string) *ast.Expression {
	return &ast.Expression{Kind: ast.LiteralKind, Literal: token(value, lexer.IdentifierKind)}
}

func selectColumns(table string, columns []string, clause *ast.WhereClause) *ast.SelectStatement {
	items := make([]*ast.Expression, len(columns))
	for index, name := range columns {
		items[index] = literalIdentifier(name)
	}
	return &ast.SelectStatement{From: identifier(table), Item: items, Where: clause}
}

func where(column, operator, value string, valueKind lexer.TokenKind) *ast.WhereClause {
	return &ast.WhereClause{
		Left:     identifier(column),
		Operator: *token(operator, lexer.SymbolKind),
		Right:    *token(value, valueKind),
	}
}

func identifier(value string) lexer.Token {
	return *token(value, lexer.IdentifierKind)
}

func token(value string, kind lexer.TokenKind) *lexer.Token {
	return &lexer.Token{Value: value, Kind: kind}
}
