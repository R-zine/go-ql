package backend

import (
	"errors"

	"go-ql/ast"
	"go-ql/storage"
)

type ResultColumn struct {
	Type storage.ColumnType
	Name string
}

type Results struct {
	Columns []ResultColumn
	Rows    [][]storage.MemoryCell
}

var (
	ErrTableDoesNotExist   = errors.New("table does not exist")
	ErrTableAlreadyExists  = errors.New("table already exists")
	ErrColumnDoesNotExist  = errors.New("column does not exist")
	ErrInvalidSelectItem   = errors.New("invalid select item")
	ErrInvalidDatatype     = errors.New("invalid datatype")
	ErrInvalidValue        = errors.New("invalid value")
	ErrMissingValues       = errors.New("incorrect number of values")
	ErrDuplicatePrimaryKey = errors.New("duplicate primary key")
)

type Backend interface {
	CreateTable(*ast.CreateTableStatement) error
	Insert(*ast.InsertStatement) error
	Select(*ast.SelectStatement) (*Results, error)
}
