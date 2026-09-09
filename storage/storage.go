package storage

import (
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ColumnType uint

const (
	TextType ColumnType = iota
	IntType
)

var ErrInvalidDatabase = errors.New("invalid database")

type MemoryCell []byte

type Table struct {
	Columns     []string
	ColumnTypes []ColumnType
	Rows        [][]MemoryCell

	PrimaryKeyColumn int
	PrimaryKeyIndex  map[int32]int
}

func NewIntCell(value int32) MemoryCell {
	cell := make(MemoryCell, 4)
	binary.BigEndian.PutUint32(cell, uint32(value))
	return cell
}

func (mc MemoryCell) AsInt() (int32, error) {
	if len(mc) != 4 {
		return 0, fmt.Errorf("%w: INT cell has %d bytes, want 4", ErrInvalidDatabase, len(mc))
	}
	return int32(binary.BigEndian.Uint32(mc)), nil
}

func (mc MemoryCell) AsText() string {
	return string(mc)
}

type Store interface {
	SaveTables(map[string]*Table) error
	LoadTables() (map[string]*Table, error)
}

type FileStore struct {
	Path string
}

func (fs *FileStore) SaveTables(tables map[string]*Table) error {
	if fs == nil || fs.Path == "" {
		return errors.New("database path is required")
	}
	if err := ValidateTables(tables); err != nil {
		return err
	}

	directory := filepath.Dir(fs.Path)
	base := filepath.Base(fs.Path)
	temporary, err := os.CreateTemp(directory, "."+base+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary database: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := gob.NewEncoder(temporary).Encode(tables); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode database: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync database: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	if err := os.Rename(temporaryPath, fs.Path); err != nil {
		return fmt.Errorf("replace database: %w", err)
	}

	removeTemporary = false
	return nil
}

func (fs *FileStore) LoadTables() (map[string]*Table, error) {
	if fs == nil || fs.Path == "" {
		return nil, errors.New("database path is required")
	}

	file, err := os.Open(fs.Path)
	if os.IsNotExist(err) {
		return map[string]*Table{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer file.Close()

	tables := map[string]*Table{}
	if err := gob.NewDecoder(file).Decode(&tables); err != nil {
		return nil, fmt.Errorf("decode database: %w", err)
	}
	if tables == nil {
		tables = map[string]*Table{}
	}
	if err := ValidateTables(tables); err != nil {
		return nil, err
	}

	return tables, nil
}

// ValidateTables checks persisted invariants and rebuilds derived primary-key indexes.
func ValidateTables(tables map[string]*Table) error {
	if tables == nil {
		return fmt.Errorf("%w: table map is nil", ErrInvalidDatabase)
	}

	for tableName, table := range tables {
		if tableName == "" {
			return fmt.Errorf("%w: table name is empty", ErrInvalidDatabase)
		}
		if table == nil {
			return fmt.Errorf("%w: table %q is nil", ErrInvalidDatabase, tableName)
		}
		if len(table.Columns) == 0 {
			return fmt.Errorf("%w: table %q has no columns", ErrInvalidDatabase, tableName)
		}
		if len(table.Columns) != len(table.ColumnTypes) {
			return fmt.Errorf("%w: table %q has mismatched column metadata", ErrInvalidDatabase, tableName)
		}

		seenColumns := make(map[string]struct{}, len(table.Columns))
		for index, column := range table.Columns {
			if column == "" {
				return fmt.Errorf("%w: table %q has an empty column name", ErrInvalidDatabase, tableName)
			}
			if _, exists := seenColumns[column]; exists {
				return fmt.Errorf("%w: table %q has duplicate column %q", ErrInvalidDatabase, tableName, column)
			}
			seenColumns[column] = struct{}{}
			if table.ColumnTypes[index] != TextType && table.ColumnTypes[index] != IntType {
				return fmt.Errorf("%w: table %q column %q has unknown type", ErrInvalidDatabase, tableName, column)
			}
		}

		if table.PrimaryKeyColumn < -1 || table.PrimaryKeyColumn >= len(table.Columns) {
			return fmt.Errorf("%w: table %q has invalid primary-key column", ErrInvalidDatabase, tableName)
		}
		if table.PrimaryKeyColumn >= 0 && table.ColumnTypes[table.PrimaryKeyColumn] != IntType {
			return fmt.Errorf("%w: table %q has a non-INT primary key", ErrInvalidDatabase, tableName)
		}

		primaryKeyIndex := make(map[int32]int)
		for rowIndex, row := range table.Rows {
			if len(row) != len(table.Columns) {
				return fmt.Errorf("%w: table %q row %d has %d cells, want %d", ErrInvalidDatabase, tableName, rowIndex, len(row), len(table.Columns))
			}
			for columnIndex, cell := range row {
				if table.ColumnTypes[columnIndex] == IntType && len(cell) != 4 {
					return fmt.Errorf("%w: table %q row %d column %q is not a valid INT", ErrInvalidDatabase, tableName, rowIndex, table.Columns[columnIndex])
				}
			}

			if table.PrimaryKeyColumn >= 0 {
				key, err := row[table.PrimaryKeyColumn].AsInt()
				if err != nil {
					return err
				}
				if _, exists := primaryKeyIndex[key]; exists {
					return fmt.Errorf("%w: table %q has duplicate primary key %d", ErrInvalidDatabase, tableName, key)
				}
				primaryKeyIndex[key] = rowIndex
			}
		}
		table.PrimaryKeyIndex = primaryKeyIndex
	}

	return nil
}
