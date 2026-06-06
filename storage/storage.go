package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"os"
)

type ColumnType uint

const (
	TextType ColumnType = iota
	IntType
)

type MemoryCell []byte

type Table struct {
    Columns          []string
    ColumnTypes      []ColumnType
    Rows             [][]MemoryCell

    PrimaryKeyColumn int
    PrimaryKeyIndex  map[int32]int
}

func (mc MemoryCell) AsInt() int32 {
	var i int32
	err := binary.Read(bytes.NewBuffer(mc), binary.BigEndian, &i)
	if err != nil {
		panic(err)
	}

	return i
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

func (fs *FileStore) SaveTables(
	tables map[string]*Table,
) error {
	f, err := os.Create(fs.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	return gob.NewEncoder(f).Encode(tables)
}

func (fs *FileStore) LoadTables() (
	map[string]*Table,
	error,
) {
	f, err := os.Open(fs.Path)
	if os.IsNotExist(err) {
		return map[string]*Table{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tables := map[string]*Table{}

	err = gob.NewDecoder(f).Decode(&tables)
	return tables, err
}
