package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go-ql/ast"
	"go-ql/lexer"
	"go-ql/storage"
	"strconv"
)



type MemoryBackend struct {
    tables map[string]*storage.Table
    store  storage.Store
}

func NewMemoryBackend(store storage.Store) *MemoryBackend {
    tables, _ := store.LoadTables()

    return &MemoryBackend{
        tables: tables,
        store:  store,
    }
}

func (mb *MemoryBackend) CreateTable(crt *ast.CreateTableStatement) error {
    t := storage.Table{}
    mb.tables[crt.Name.Value] = &t
    if crt.Cols == nil {

        return nil
    }

    for _, col := range *crt.Cols {
        t.Columns = append(t.Columns, col.Name.Value)

        var dt storage.ColumnType
        switch col.Datatype.Value {
        case "int":
            dt = storage.IntType
        case "text":
            dt = storage.TextType
        default:
            return ErrInvalidDatatype
        }

        t.ColumnTypes = append(t.ColumnTypes, dt)
    }

    if err := mb.store.SaveTables(mb.tables); err != nil {
    return err
}

    return nil
}

func (mb *MemoryBackend) Insert(inst *ast.InsertStatement) error {
    table, ok := mb.tables[inst.Table.Value]
    if !ok {
        return ErrTableDoesNotExist
    }

    if inst.Values == nil {
        return nil
    }

    row := []storage.MemoryCell{}

    if len(*inst.Values) != len(table.Columns) {
        return ErrMissingValues
    }

    for _, value := range *inst.Values {
        if value.Kind != ast.LiteralKind {
            fmt.Println("Skipping non-literal.")
            continue
        }

        row = append(row, mb.tokenToCell(value.Literal))
    }

    table.Rows = append(table.Rows, row)

if err := mb.store.SaveTables(mb.tables); err != nil {
    return err
}

    return nil
}

func (mb *MemoryBackend) tokenToCell(t *lexer.Token) storage.MemoryCell {
    if t.Kind == lexer.NumericKind {
        buf := new(bytes.Buffer)
        i, err := strconv.Atoi(t.Value)
        if err != nil {
            panic(err)
        }

        err = binary.Write(buf, binary.BigEndian, int32(i))
        if err != nil {
            panic(err)
        }
        return storage.MemoryCell(buf.Bytes())
    }

    if t.Kind == lexer.StringKind {
        return storage.MemoryCell(t.Value)
    }

    return nil
}

func (mb *MemoryBackend) Select(slct *ast.SelectStatement) (*Results, error) {
	table, ok := mb.tables[slct.From.Value]
	if !ok {
		return nil, ErrTableDoesNotExist
	}

	results := [][]Cell{}
	columns := []struct {
		Type storage.ColumnType
		Name string
	}{}

	for rowIdx, row := range table.Rows {
		result := []Cell{}
		isFirstRow := rowIdx == 0

		for _, exp := range slct.Item {
			switch exp.Kind {

			case ast.WildcardKind:
				for colIdx, col := range table.Columns {
					if isFirstRow {
						columns = append(columns, struct {
							Type storage.ColumnType
							Name string
						}{
							Type: table.ColumnTypes[colIdx],
							Name: col,
						})
					}

					result = append(result, row[colIdx])
				}

			case ast.LiteralKind:
				lit := exp.Literal

				if lit.Kind != lexer.IdentifierKind {
					return nil, ErrColumnDoesNotExist
				}

				found := false

				for colIdx, tableCol := range table.Columns {
					if tableCol == lit.Value {
						if isFirstRow {
							columns = append(columns, struct {
								Type storage.ColumnType
								Name string
							}{
								Type: table.ColumnTypes[colIdx],
								Name: lit.Value,
							})
						}

						result = append(result, row[colIdx])
						found = true
						break
					}
				}

				if !found {
					return nil, ErrColumnDoesNotExist
				}

			default:
				return nil, ErrColumnDoesNotExist
			}
		}

		results = append(results, result)
	}

	return &Results{
		Columns: columns,
		Rows:    results,
	}, nil
}