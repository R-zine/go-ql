package backend

import (
	"errors"
	"fmt"
	"strconv"

	"go-ql/ast"
	"go-ql/lexer"
	"go-ql/storage"
)

type MemoryBackend struct {
	tables map[string]*storage.Table
	store  storage.Store
}

func NewMemoryBackend(store storage.Store) (*MemoryBackend, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}

	tables, err := store.LoadTables()
	if err != nil {
		return nil, fmt.Errorf("load tables: %w", err)
	}
	if err := storage.ValidateTables(tables); err != nil {
		return nil, fmt.Errorf("load tables: %w", err)
	}

	return &MemoryBackend{tables: tables, store: store}, nil
}

func (mb *MemoryBackend) CreateTable(statement *ast.CreateTableStatement) error {
	if statement == nil {
		return fmt.Errorf("%w: CREATE TABLE statement is nil", ErrInvalidValue)
	}
	if statement.Name.Value == "" {
		return fmt.Errorf("%w: table name is empty", ErrInvalidValue)
	}
	if statement.Name.Kind != lexer.IdentifierKind {
		return fmt.Errorf("%w: table name must be an identifier", ErrInvalidValue)
	}
	if _, exists := mb.tables[statement.Name.Value]; exists {
		return fmt.Errorf("%w: %s", ErrTableAlreadyExists, statement.Name.Value)
	}
	if statement.Cols == nil || len(*statement.Cols) == 0 {
		return fmt.Errorf("%w: table must have at least one column", ErrInvalidValue)
	}

	table := &storage.Table{
		PrimaryKeyColumn: -1,
		PrimaryKeyIndex:  make(map[int32]int),
	}
	seenColumns := make(map[string]struct{}, len(*statement.Cols))

	for index, column := range *statement.Cols {
		if column == nil || column.Name.Value == "" || column.Name.Kind != lexer.IdentifierKind {
			return fmt.Errorf("%w: column %d has no name", ErrInvalidValue, index)
		}
		if _, exists := seenColumns[column.Name.Value]; exists {
			return fmt.Errorf("%w: duplicate column %s", ErrInvalidValue, column.Name.Value)
		}
		seenColumns[column.Name.Value] = struct{}{}

		var columnType storage.ColumnType
		if column.Datatype.Kind != lexer.KeywordKind {
			return fmt.Errorf("%w: %s", ErrInvalidDatatype, column.Datatype.Value)
		}
		switch column.Datatype.Value {
		case string(lexer.IntKeyword):
			columnType = storage.IntType
		case string(lexer.TextKeyword):
			columnType = storage.TextType
		default:
			return fmt.Errorf("%w: %s", ErrInvalidDatatype, column.Datatype.Value)
		}

		table.Columns = append(table.Columns, column.Name.Value)
		table.ColumnTypes = append(table.ColumnTypes, columnType)

		if column.PrimaryKey {
			if table.PrimaryKeyColumn != -1 {
				return fmt.Errorf("%w: multiple primary keys are not supported", ErrInvalidValue)
			}
			if columnType != storage.IntType {
				return fmt.Errorf("%w: primary key must be INT", ErrInvalidDatatype)
			}
			table.PrimaryKeyColumn = index
		}
	}

	mb.tables[statement.Name.Value] = table
	if err := mb.store.SaveTables(mb.tables); err != nil {
		delete(mb.tables, statement.Name.Value)
		return fmt.Errorf("persist table %s: %w", statement.Name.Value, err)
	}
	return nil
}

func (mb *MemoryBackend) Insert(statement *ast.InsertStatement) error {
	if statement == nil {
		return fmt.Errorf("%w: INSERT statement is nil", ErrInvalidValue)
	}
	if statement.Table.Kind != lexer.IdentifierKind {
		return fmt.Errorf("%w: table name must be an identifier", ErrInvalidValue)
	}
	table, exists := mb.tables[statement.Table.Value]
	if !exists {
		return fmt.Errorf("%w: %s", ErrTableDoesNotExist, statement.Table.Value)
	}
	if statement.Values == nil || len(*statement.Values) != len(table.Columns) {
		valueCount := 0
		if statement.Values != nil {
			valueCount = len(*statement.Values)
		}
		return fmt.Errorf("%w: got %d, want %d", ErrMissingValues, valueCount, len(table.Columns))
	}

	row := make([]storage.MemoryCell, len(table.Columns))
	for index, value := range *statement.Values {
		if value == nil || value.Kind != ast.LiteralKind || value.Literal == nil {
			return fmt.Errorf("%w for column %s: literal required", ErrInvalidValue, table.Columns[index])
		}

		cell, err := tokenToCell(value.Literal, table.ColumnTypes[index])
		if err != nil {
			return fmt.Errorf("column %s: %w", table.Columns[index], err)
		}
		row[index] = cell
	}

	var primaryKey int32
	hasPrimaryKey := table.PrimaryKeyColumn >= 0
	if hasPrimaryKey {
		var err error
		primaryKey, err = row[table.PrimaryKeyColumn].AsInt()
		if err != nil {
			return err
		}
		if _, exists := table.PrimaryKeyIndex[primaryKey]; exists {
			return fmt.Errorf("%w: %d", ErrDuplicatePrimaryKey, primaryKey)
		}
	}

	rowIndex := len(table.Rows)
	table.Rows = append(table.Rows, row)
	if hasPrimaryKey {
		table.PrimaryKeyIndex[primaryKey] = rowIndex
	}

	if err := mb.store.SaveTables(mb.tables); err != nil {
		table.Rows = table.Rows[:rowIndex]
		if hasPrimaryKey {
			delete(table.PrimaryKeyIndex, primaryKey)
		}
		return fmt.Errorf("persist insert into %s: %w", statement.Table.Value, err)
	}
	return nil
}

func tokenToCell(token *lexer.Token, columnType storage.ColumnType) (storage.MemoryCell, error) {
	switch columnType {
	case storage.IntType:
		if token.Kind != lexer.NumericKind {
			return nil, fmt.Errorf("%w: INT requires a numeric literal", ErrInvalidValue)
		}
		value, err := strconv.ParseInt(token.Value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a valid INT", ErrInvalidValue, token.Value)
		}
		return storage.NewIntCell(int32(value)), nil
	case storage.TextType:
		if token.Kind != lexer.StringKind {
			return nil, fmt.Errorf("%w: TEXT requires a string literal", ErrInvalidValue)
		}
		return storage.MemoryCell(token.Value), nil
	default:
		return nil, fmt.Errorf("%w: unknown column type", ErrInvalidDatatype)
	}
}

type projection struct {
	columns []ResultColumn
	indexes []int
}

func buildProjection(statement *ast.SelectStatement, table *storage.Table) (*projection, error) {
	if len(statement.Item) == 0 {
		return nil, fmt.Errorf("%w: at least one item is required", ErrInvalidSelectItem)
	}

	result := &projection{}
	if statement.Item[0] != nil && statement.Item[0].Kind == ast.WildcardKind {
		if len(statement.Item) != 1 {
			return nil, fmt.Errorf("%w: wildcard cannot be combined with other items", ErrInvalidSelectItem)
		}
		for index, name := range table.Columns {
			result.indexes = append(result.indexes, index)
			result.columns = append(result.columns, ResultColumn{Name: name, Type: table.ColumnTypes[index]})
		}
		return result, nil
	}

	for _, item := range statement.Item {
		if item == nil || item.Kind != ast.LiteralKind || item.Literal == nil || item.Literal.Kind != lexer.IdentifierKind {
			return nil, fmt.Errorf("%w: expected a column name", ErrInvalidSelectItem)
		}
		index := columnIndex(table, item.Literal.Value)
		if index < 0 {
			return nil, fmt.Errorf("%w: %s", ErrColumnDoesNotExist, item.Literal.Value)
		}
		result.indexes = append(result.indexes, index)
		result.columns = append(result.columns, ResultColumn{Name: table.Columns[index], Type: table.ColumnTypes[index]})
	}

	return result, nil
}

type wherePlan struct {
	columnIndex int
	operator    string
	intValue    int32
	textValue   string
}

func buildWherePlan(table *storage.Table, clause *ast.WhereClause) (*wherePlan, error) {
	if clause == nil {
		return nil, nil
	}
	if clause.Left.Kind != lexer.IdentifierKind {
		return nil, fmt.Errorf("%w: invalid WHERE column", ErrColumnDoesNotExist)
	}
	if clause.Operator.Kind != lexer.SymbolKind {
		return nil, fmt.Errorf("%w: invalid WHERE operator", ErrInvalidValue)
	}

	index := columnIndex(table, clause.Left.Value)
	if index < 0 {
		return nil, fmt.Errorf("%w: %s", ErrColumnDoesNotExist, clause.Left.Value)
	}
	plan := &wherePlan{columnIndex: index, operator: clause.Operator.Value}

	switch table.ColumnTypes[index] {
	case storage.IntType:
		if clause.Right.Kind != lexer.NumericKind {
			return nil, fmt.Errorf("%w: INT comparison requires a numeric literal", ErrInvalidValue)
		}
		value, err := strconv.ParseInt(clause.Right.Value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a valid INT", ErrInvalidValue, clause.Right.Value)
		}
		plan.intValue = int32(value)
		switch plan.operator {
		case "=", "!=", "<", ">", "<=", ">=":
		default:
			return nil, fmt.Errorf("%w: unsupported INT operator %q", ErrInvalidValue, plan.operator)
		}
	case storage.TextType:
		if clause.Right.Kind != lexer.StringKind {
			return nil, fmt.Errorf("%w: TEXT comparison requires a string literal", ErrInvalidValue)
		}
		plan.textValue = clause.Right.Value
		if plan.operator != "=" && plan.operator != "!=" {
			return nil, fmt.Errorf("%w: unsupported TEXT operator %q", ErrInvalidValue, plan.operator)
		}
	default:
		return nil, fmt.Errorf("%w: unknown column type", ErrInvalidDatatype)
	}

	return plan, nil
}

func evaluateWhere(table *storage.Table, row []storage.MemoryCell, plan *wherePlan) (bool, error) {
	if len(row) != len(table.Columns) {
		return false, fmt.Errorf("%w: row has %d cells, want %d", storage.ErrInvalidDatabase, len(row), len(table.Columns))
	}
	if plan == nil {
		return true, nil
	}

	switch table.ColumnTypes[plan.columnIndex] {
	case storage.IntType:
		left, err := row[plan.columnIndex].AsInt()
		if err != nil {
			return false, err
		}
		switch plan.operator {
		case "=":
			return left == plan.intValue, nil
		case "!=":
			return left != plan.intValue, nil
		case "<":
			return left < plan.intValue, nil
		case ">":
			return left > plan.intValue, nil
		case "<=":
			return left <= plan.intValue, nil
		case ">=":
			return left >= plan.intValue, nil
		}
	case storage.TextType:
		left := row[plan.columnIndex].AsText()
		if plan.operator == "=" {
			return left == plan.textValue, nil
		}
		if plan.operator == "!=" {
			return left != plan.textValue, nil
		}
	}

	return false, fmt.Errorf("%w: unsupported WHERE expression", ErrInvalidValue)
}

func projectRow(row []storage.MemoryCell, plan *projection) []storage.MemoryCell {
	result := make([]storage.MemoryCell, len(plan.indexes))
	for index, sourceIndex := range plan.indexes {
		result[index] = append(storage.MemoryCell(nil), row[sourceIndex]...)
	}
	return result
}

func (mb *MemoryBackend) Select(statement *ast.SelectStatement) (*Results, error) {
	if statement == nil {
		return nil, fmt.Errorf("%w: SELECT statement is nil", ErrInvalidValue)
	}
	if statement.From.Kind != lexer.IdentifierKind {
		return nil, fmt.Errorf("%w: table name must be an identifier", ErrInvalidValue)
	}
	table, exists := mb.tables[statement.From.Value]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrTableDoesNotExist, statement.From.Value)
	}

	projectionPlan, err := buildProjection(statement, table)
	if err != nil {
		return nil, err
	}
	where, err := buildWherePlan(table, statement.Where)
	if err != nil {
		return nil, err
	}

	results := &Results{
		Columns: append([]ResultColumn(nil), projectionPlan.columns...),
		Rows:    make([][]storage.MemoryCell, 0),
	}

	if where != nil && table.PrimaryKeyColumn >= 0 &&
		where.columnIndex == table.PrimaryKeyColumn && where.operator == "=" {
		rowIndex, found := table.PrimaryKeyIndex[where.intValue]
		if !found {
			return results, nil
		}
		if rowIndex < 0 || rowIndex >= len(table.Rows) {
			return nil, fmt.Errorf("%w: primary-key index points outside table", storage.ErrInvalidDatabase)
		}
		matches, err := evaluateWhere(table, table.Rows[rowIndex], where)
		if err != nil {
			return nil, err
		}
		if matches {
			results.Rows = append(results.Rows, projectRow(table.Rows[rowIndex], projectionPlan))
		}
		return results, nil
	}

	for _, row := range table.Rows {
		matches, err := evaluateWhere(table, row, where)
		if err != nil {
			return nil, err
		}
		if matches {
			results.Rows = append(results.Rows, projectRow(row, projectionPlan))
		}
	}

	return results, nil
}

func columnIndex(table *storage.Table, name string) int {
	for index, column := range table.Columns {
		if column == name {
			return index
		}
	}
	return -1
}
