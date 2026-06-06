package backend

import (
	"errors"
	"go-ql/ast"
	"go-ql/storage"
)

type Cell interface {
	AsText() string
	AsInt() int32
}

type Results struct {
	Columns []struct {
		Type storage.ColumnType
		Name string
	}
	Rows [][]storage.MemoryCell
}

var (
	ErrTableDoesNotExist  = errors.New("Table does not exist")
	ErrColumnDoesNotExist = errors.New("Column does not exist")
	ErrInvalidSelectItem  = errors.New("Select item is not valid")
	ErrInvalidDatatype    = errors.New("Invalid datatype")
	ErrMissingValues      = errors.New("Missing values")
    ErrDuplicatePrimaryKey =    errors.New("duplicate primary key")
)

type Backend interface {
	CreateTable(*ast.CreateTableStatement) error
	Insert(*ast.InsertStatement) error
	Select(*ast.SelectStatement) (*Results, error)
}
