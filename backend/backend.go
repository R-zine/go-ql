package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go-ql/ast"
	"go-ql/lexer"
	"go-ql/storage"
	"strconv"
	"time"
)

type MemoryBackend struct {
	tables map[string]*storage.Table
	store  storage.Store
}



func timeTrack(name string, start time.Time) {
	fmt.Printf("[TIMER] %s took %s\n", name, time.Since(start))
}

func NewMemoryBackend(store storage.Store) *MemoryBackend {
	tables, _ := store.LoadTables()

	return &MemoryBackend{
		tables: tables,
		store:  store,
	}
}

func (mb *MemoryBackend) CreateTable(crt *ast.CreateTableStatement) error {
	t := &storage.Table{
		PrimaryKeyColumn: -1,
		PrimaryKeyIndex:  make(map[int32]int),
	}

	if crt.Cols == nil {
		mb.tables[crt.Name.Value] = t
		return mb.store.SaveTables(mb.tables)
	}

	for i, col := range *crt.Cols {
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

		// PRIMARY KEY HANDLING
		if col.PrimaryKey {
			if t.PrimaryKeyColumn != -1 {
				return fmt.Errorf("multiple primary keys not supported")
			}

			if dt != storage.IntType {
				return fmt.Errorf("primary key must be INT (for now)")
			}

			t.PrimaryKeyColumn = i
			t.PrimaryKeyIndex = make(map[int32]int)
		}
	}

	mb.tables[crt.Name.Value] = t

	return mb.store.SaveTables(mb.tables)
}

func (mb *MemoryBackend) Insert(inst *ast.InsertStatement) error {
		start := time.Now()
	defer timeTrack("INSERT", start)
	table, ok := mb.tables[inst.Table.Value]
	if !ok {
		return ErrTableDoesNotExist
	}

	if inst.Values == nil {
		return nil
	}

	if len(*inst.Values) != len(table.Columns) {
		return ErrMissingValues
	}

	row := make([]storage.MemoryCell, 0, len(table.Columns))

	for _, value := range *inst.Values {
		if value.Kind != ast.LiteralKind {
			return fmt.Errorf("only literal values supported")
		}

		row = append(row, mb.tokenToCell(value.Literal))
	}

	if table.PrimaryKeyColumn >= 0 {
		pkCell := row[table.PrimaryKeyColumn]

		if table.ColumnTypes[table.PrimaryKeyColumn] != storage.IntType {
			return fmt.Errorf("primary key must be INT")
		}

		pk := pkCell.AsInt()

		// enforce uniqueness
		if _, exists := table.PrimaryKeyIndex[pk]; exists {
			return fmt.Errorf("duplicate primary key: %d", pk)
		}

		// insert row index into index
		rowIndex := len(table.Rows)
		table.PrimaryKeyIndex[pk] = rowIndex
	}

	// append row
	table.Rows = append(table.Rows, row)

	// persist
	return mb.store.SaveTables(mb.tables)
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

func compareInt(
	left int32,
	op string,
	right int32,
) (bool, error) {
	switch op {
	case "=":
		return left == right, nil

	case "!=":
		return left != right, nil

	case "<":
		return left < right, nil

	case ">":
		return left > right, nil

	case "<=":
		return left <= right, nil

	case ">=":
		return left >= right, nil
	}

	return false, fmt.Errorf(
		"unsupported operator %s",
		op,
	)
}

func (mb *MemoryBackend) evaluateWhere(
	table *storage.Table,
	row []storage.MemoryCell,
	where *ast.WhereClause,
) (bool, error) {
	columnIndex := -1

	for i, col := range table.Columns {
		if col == where.Left.Value {
			columnIndex = i
			break
		}
	}

	if columnIndex == -1 {
		return false, ErrColumnDoesNotExist
	}

	switch table.ColumnTypes[columnIndex] {

	case storage.IntType:
		rowValue := row[columnIndex].AsInt()

		searchValue, err := strconv.Atoi(where.Right.Value)
		if err != nil {
			return false, err
		}

		return compareInt(
			rowValue,
			where.Operator.Value,
			int32(searchValue),
		)

	case storage.TextType:
		rowValue := row[columnIndex].AsText()

		switch where.Operator.Value {
		case "=":
			return rowValue == where.Right.Value, nil

		case "!=":
			return rowValue != where.Right.Value, nil

		default:
			return false, fmt.Errorf(
				"unsupported operator for text: %s",
				where.Operator.Value,
			)
		}

	default:
		return false, fmt.Errorf("unsupported column type")
	}
}

func applyProjection(
	slct *ast.SelectStatement,
	table *storage.Table,
	row []storage.MemoryCell,
) []storage.MemoryCell {

	result := []storage.MemoryCell{}

	for _, exp := range slct.Item {
		switch exp.Kind {

		case ast.WildcardKind:
			return append([]storage.MemoryCell{}, row...)

		case ast.LiteralKind:
			lit := exp.Literal

			if lit.Kind != lexer.IdentifierKind {
				continue
			}

			for i, col := range table.Columns {
				if col == lit.Value {
					result = append(result, row[i])
					break
				}
			}

		default:
			continue
		}
	}

	return result
}

func (mb *MemoryBackend) Select(slct *ast.SelectStatement) (*Results, error) {
	start := time.Now()
	defer timeTrack("SELECT", start)

	table, ok := mb.tables[slct.From.Value]
	if !ok {
		return nil, ErrTableDoesNotExist
	}

	columns := []struct {
		Type storage.ColumnType
		Name string
	}{}

	if slct.Where != nil &&
		table.PrimaryKeyColumn >= 0 &&
		slct.Where.Operator.Value == "=" &&
		slct.Where.Left.Value == table.Columns[table.PrimaryKeyColumn] {

		pkVal, err := strconv.Atoi(slct.Where.Right.Value)
		if err != nil {
			return nil, err
		}

		rowIdx, ok := table.PrimaryKeyIndex[int32(pkVal)]
		if !ok {
			return &Results{
				Columns: columns,
				Rows:    [][]storage.MemoryCell{},
			}, nil
		}

		row := table.Rows[rowIdx]

		// build columns once
		for i, col := range table.Columns {
			columns = append(columns, struct {
				Type storage.ColumnType
				Name string
			}{
				Type: table.ColumnTypes[i],
				Name: col,
			})
		}

		// projection
		result := applyProjection(slct, table, row)

		return &Results{
			Columns: columns,
			Rows:    [][]storage.MemoryCell{result},
		}, nil
	}

	// full scan fallback
	results := [][]storage.MemoryCell{}

	for rowIdx, row := range table.Rows {
		if slct.Where != nil {
			matches, err := mb.evaluateWhere(table, row, slct.Where)
			if err != nil {
				return nil, err
			}
			if !matches {
				continue
			}
		}

		if rowIdx == 0 && len(columns) == 0 {
			for i, col := range table.Columns {
				columns = append(columns, struct {
					Type storage.ColumnType
					Name string
				}{
					Type: table.ColumnTypes[i],
					Name: col,
				})
			}
		}

		results = append(results, applyProjection(slct, table, row))
	}

	return &Results{
		Columns: columns,
		Rows:    results,
	}, nil
}