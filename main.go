package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"go-ql/ast"
	"go-ql/backend"
	"go-ql/storage"
)

func main() {
	if err := run(os.Stdin, os.Stdout, &storage.FileStore{Path: "goql.db"}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input io.Reader, output io.Writer, store storage.Store) error {
	memoryBackend, err := backend.NewMemoryBackend(store)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err := runREPL(input, output, memoryBackend); err != nil {
		return fmt.Errorf("input error: %w", err)
	}
	return nil
}

func runREPL(input io.Reader, output io.Writer, database backend.Backend) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	fmt.Fprintln(output, "Welcome to go-ql.")
	var pending strings.Builder

	for {
		if pending.Len() == 0 {
			fmt.Fprint(output, "# ")
		} else {
			fmt.Fprint(output, "> ")
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			if strings.TrimSpace(pending.String()) != "" {
				fmt.Fprintln(output, "error: incomplete statement at end of input")
			}
			return nil
		}

		line := scanner.Text()
		if pending.Len() == 0 && strings.TrimSpace(line) == "" {
			continue
		}
		pending.WriteString(line)
		pending.WriteByte('\n')
		if !hasStatementTerminator(pending.String()) {
			continue
		}

		source := pending.String()
		pending.Reset()
		if err := executeSource(source, output, database); err != nil {
			fmt.Fprintf(output, "error: %v\n", err)
		}
	}
}

func executeSource(source string, output io.Writer, database backend.Backend) error {
	parsed, err := ast.Parse(source)
	if err != nil {
		return err
	}

	for _, statement := range parsed.Statements {
		switch statement.Kind {
		case ast.CreateTableKind:
			if err := database.CreateTable(statement.CreateTableStatement); err != nil {
				return err
			}
			fmt.Fprintln(output, "ok")
		case ast.InsertKind:
			if err := database.Insert(statement.InsertStatement); err != nil {
				return err
			}
			fmt.Fprintln(output, "ok")
		case ast.SelectKind:
			results, err := database.Select(statement.SelectStatement)
			if err != nil {
				return err
			}
			if err := printResults(output, results); err != nil {
				return err
			}
			fmt.Fprintln(output, "ok")
		default:
			return fmt.Errorf("unsupported statement kind %d", statement.Kind)
		}
	}

	return nil
}

func printResults(output io.Writer, results *backend.Results) error {
	if results == nil {
		return fmt.Errorf("invalid query result: result is nil")
	}

	separatorWidth := 1
	for _, column := range results.Columns {
		fmt.Fprintf(output, "| %s ", column.Name)
		separatorWidth += len(column.Name) + 3
	}
	fmt.Fprintln(output, "|")
	fmt.Fprintln(output, strings.Repeat("=", separatorWidth))

	for rowIndex, row := range results.Rows {
		if len(row) != len(results.Columns) {
			return fmt.Errorf("invalid query result: row %d has %d cells, want %d", rowIndex, len(row), len(results.Columns))
		}
		fmt.Fprint(output, "|")
		for columnIndex, cell := range row {
			var value string
			switch results.Columns[columnIndex].Type {
			case storage.IntType:
				integer, err := cell.AsInt()
				if err != nil {
					return err
				}
				value = fmt.Sprintf("%d", integer)
			case storage.TextType:
				value = cell.AsText()
			default:
				return fmt.Errorf("invalid query result: column %d has an unknown type", columnIndex)
			}
			fmt.Fprintf(output, " %s |", value)
		}
		fmt.Fprintln(output)
	}

	return nil
}

func hasStatementTerminator(source string) bool {
	inSingleQuote := false
	inDoubleQuote := false

	for index := 0; index < len(source); index++ {
		switch source[index] {
		case '\'':
			if inDoubleQuote {
				continue
			}
			if inSingleQuote && index+1 < len(source) && source[index+1] == '\'' {
				index++
				continue
			}
			inSingleQuote = !inSingleQuote
		case '"':
			if inSingleQuote {
				continue
			}
			if inDoubleQuote && index+1 < len(source) && source[index+1] == '"' {
				index++
				continue
			}
			inDoubleQuote = !inDoubleQuote
		case ';':
			if !inSingleQuote && !inDoubleQuote {
				return true
			}
		}
	}

	return false
}
