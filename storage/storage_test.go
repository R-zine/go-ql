package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryCellIntRoundTrip(t *testing.T) {
	for _, value := range []int32{math.MinInt32, -1, 0, 1, math.MaxInt32} {
		cell := NewIntCell(value)
		got, err := cell.AsInt()
		if err != nil {
			t.Fatalf("AsInt(%d) error = %v", value, err)
		}
		if got != value {
			t.Errorf("AsInt(%d) = %d", value, got)
		}
	}

	if _, err := (MemoryCell{1, 2, 3}).AsInt(); !errors.Is(err, ErrInvalidDatabase) {
		t.Fatalf("short cell error = %v, want ErrInvalidDatabase", err)
	}
}

func TestFileStoreRoundTripAndReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	store := &FileStore{Path: path}
	tables := testTables()
	tables["users"].PrimaryKeyIndex = map[int32]int{99: 100}

	if err := store.SaveTables(tables); err != nil {
		t.Fatalf("first SaveTables() error = %v", err)
	}
	tables["users"].Rows = append(tables["users"].Rows, []MemoryCell{NewIntCell(2), MemoryCell("Kate")})
	if err := store.SaveTables(tables); err != nil {
		t.Fatalf("replacement SaveTables() error = %v", err)
	}

	loaded, err := store.LoadTables()
	if err != nil {
		t.Fatalf("LoadTables() error = %v", err)
	}
	if len(loaded["users"].Rows) != 2 {
		t.Fatalf("loaded row count = %d, want 2", len(loaded["users"].Rows))
	}
	if got := loaded["users"].PrimaryKeyIndex[int32(2)]; got != 1 {
		t.Errorf("rebuilt primary-key index[2] = %d, want 1", got)
	}
	if got := loaded["users"].Rows[1][1].AsText(); got != "Kate" {
		t.Errorf("loaded text = %q, want Kate", got)
	}

	temporaryMatches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".database.db-*.tmp"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(temporaryMatches) != 0 {
		t.Errorf("temporary files were not cleaned up: %v", temporaryMatches)
	}
}

func TestFileStoreRebuildsPersistedPrimaryKeyIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	tables := testTables()
	tables["users"].PrimaryKeyIndex = map[int32]int{99: 99}

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(file).Encode(tables); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	loaded, err := (&FileStore{Path: path}).LoadTables()
	if err != nil {
		t.Fatalf("LoadTables() error = %v", err)
	}
	index := loaded["users"].PrimaryKeyIndex
	if len(index) != 1 || index[1] != 0 {
		t.Errorf("rebuilt index = %v, want map[1:0]", index)
	}
}

func TestFileStoreMissingAndCorruptDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	store := &FileStore{Path: path}

	tables, err := store.LoadTables()
	if err != nil {
		t.Fatalf("LoadTables() for missing file error = %v", err)
	}
	if tables == nil || len(tables) != 0 {
		t.Fatalf("missing database returned %#v, want empty map", tables)
	}

	if err := os.WriteFile(path, []byte("not a gob database"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := store.LoadTables(); err == nil {
		t.Fatal("LoadTables() unexpectedly accepted corrupt data")
	}
}

func TestFileStoreRejectsInvalidPathsAndCleansTemporaryFiles(t *testing.T) {
	if err := (*FileStore)(nil).SaveTables(testTables()); err == nil {
		t.Error("nil FileStore SaveTables() unexpectedly succeeded")
	}
	if _, err := (*FileStore)(nil).LoadTables(); err == nil {
		t.Error("nil FileStore LoadTables() unexpectedly succeeded")
	}
	if err := (&FileStore{}).SaveTables(testTables()); err == nil {
		t.Error("empty path SaveTables() unexpectedly succeeded")
	}
	if _, err := (&FileStore{}).LoadTables(); err == nil {
		t.Error("empty path LoadTables() unexpectedly succeeded")
	}

	root := t.TempDir()
	missingParentPath := filepath.Join(root, "missing", "database.db")
	if err := (&FileStore{Path: missingParentPath}).SaveTables(testTables()); err == nil {
		t.Error("SaveTables() with missing parent unexpectedly succeeded")
	}

	destinationDirectory := filepath.Join(root, "destination")
	if err := os.Mkdir(destinationDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := (&FileStore{Path: destinationDirectory}).SaveTables(testTables()); err == nil {
		t.Error("SaveTables() over a directory unexpectedly succeeded")
	}
	matches, err := filepath.Glob(filepath.Join(root, ".destination-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("failed replacement left temporary files: %v", matches)
	}
	if _, err := (&FileStore{Path: destinationDirectory}).LoadTables(); err == nil {
		t.Error("LoadTables() from a directory unexpectedly succeeded")
	}
}

func TestFileStoreEmptyDatabaseRoundTrip(t *testing.T) {
	store := &FileStore{Path: filepath.Join(t.TempDir(), "empty.db")}
	if err := store.SaveTables(map[string]*Table{}); err != nil {
		t.Fatalf("SaveTables(empty) error = %v", err)
	}
	loaded, err := store.LoadTables()
	if err != nil {
		t.Fatalf("LoadTables(empty) error = %v", err)
	}
	if loaded == nil || len(loaded) != 0 {
		t.Fatalf("LoadTables(empty) = %#v, want empty non-nil map", loaded)
	}
}

func TestInvalidSaveLeavesExistingDatabaseUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	store := &FileStore{Path: path}
	if err := store.SaveTables(testTables()); err != nil {
		t.Fatalf("initial SaveTables() error = %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	invalid := testTables()
	invalid["users"].Rows[0] = []MemoryCell{NewIntCell(1)}
	if err := store.SaveTables(invalid); !errors.Is(err, ErrInvalidDatabase) {
		t.Fatalf("invalid SaveTables() error = %v, want ErrInvalidDatabase", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() after failed save error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Error("failed save changed the existing database")
	}
}

func TestValidateTablesRejectsInvalidData(t *testing.T) {
	tests := map[string]map[string]*Table{
		"nil map": nil,
		"empty table name": {
			"": testTables()["users"],
		},
		"nil table": {
			"users": nil,
		},
		"no columns": {
			"users": {PrimaryKeyColumn: -1},
		},
		"mismatched metadata": {
			"users": {Columns: []string{"id"}, PrimaryKeyColumn: -1},
		},
		"duplicate columns": {
			"users": {Columns: []string{"id", "id"}, ColumnTypes: []ColumnType{IntType, IntType}, PrimaryKeyColumn: -1},
		},
		"unknown type": {
			"users": {Columns: []string{"id"}, ColumnTypes: []ColumnType{99}, PrimaryKeyColumn: -1},
		},
		"invalid primary key": {
			"users": {Columns: []string{"id"}, ColumnTypes: []ColumnType{IntType}, PrimaryKeyColumn: 2},
		},
		"text primary key": {
			"users": {Columns: []string{"id"}, ColumnTypes: []ColumnType{TextType}, PrimaryKeyColumn: 0},
		},
		"bad row width": {
			"users": {Columns: []string{"id"}, ColumnTypes: []ColumnType{IntType}, Rows: [][]MemoryCell{{}}, PrimaryKeyColumn: -1},
		},
		"bad int cell": {
			"users": {Columns: []string{"id"}, ColumnTypes: []ColumnType{IntType}, Rows: [][]MemoryCell{{MemoryCell{1}}}, PrimaryKeyColumn: -1},
		},
		"duplicate primary key": {
			"users": {
				Columns:          []string{"id"},
				ColumnTypes:      []ColumnType{IntType},
				Rows:             [][]MemoryCell{{NewIntCell(1)}, {NewIntCell(1)}},
				PrimaryKeyColumn: 0,
			},
		},
	}

	for name, tables := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateTables(tables); !errors.Is(err, ErrInvalidDatabase) {
				t.Fatalf("ValidateTables() error = %v, want ErrInvalidDatabase", err)
			}
		})
	}
}

func BenchmarkFileStoreSave(b *testing.B) {
	store := &FileStore{Path: filepath.Join(b.TempDir(), "benchmark.db")}
	tables := testTables()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := store.SaveTables(tables); err != nil {
			b.Fatal(err)
		}
	}
}

func testTables() map[string]*Table {
	return map[string]*Table{
		"users": {
			Columns:          []string{"id", "name"},
			ColumnTypes:      []ColumnType{IntType, TextType},
			Rows:             [][]MemoryCell{{NewIntCell(1), MemoryCell("Phil")}},
			PrimaryKeyColumn: 0,
			PrimaryKeyIndex:  map[int32]int{1: 0},
		},
	}
}
