package main

import (
	"bufio"
	"fmt"
	"go-ql/ast"
	"go-ql/backend"
	"go-ql/storage"
	"os"
	"strings"
)

func main() {
	store := &storage.FileStore{
		Path: "goql.db",
	}

	mb := backend.NewMemoryBackend(store)

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Welcome to gosql.")
	for {
		fmt.Print("# ")
		text, err := reader.ReadString('\n')
		text = strings.NewReplacer(
			"\r\n", "",
			"\n", "",
			"\r", "",
		).Replace(text)

		astVar, err := ast.Parse(text)
		if err != nil {
			panic(err)
		}

		for _, stmt := range astVar.Statements {
			switch stmt.Kind {
			case ast.CreateTableKind:
				err = mb.CreateTable(astVar.Statements[0].CreateTableStatement)
				if err != nil {
					panic(err)
				}
				fmt.Println("ok")
			case ast.InsertKind:
				err = mb.Insert(stmt.InsertStatement)
				if err != nil {
					panic(err)
				}

				fmt.Println("ok")
			case ast.SelectKind:
				results, err := mb.Select(stmt.SelectStatement)
				if err != nil {
					panic(err)
				}

				for _, col := range results.Columns {
					fmt.Printf("| %s ", col.Name)
				}
				fmt.Println("|")

				for i := 0; i < 20; i++ {
					fmt.Printf("=")
				}
				fmt.Println()

				for _, result := range results.Rows {
					fmt.Printf("|")

					for i, cell := range result {
						typ := results.Columns[i].Type
						s := ""
						switch typ {
						case storage.IntType:
							s = fmt.Sprintf("%d", cell.AsInt())
						case storage.TextType:
							s = cell.AsText()
						}

						fmt.Printf(" %s | ", s)
					}

					fmt.Println()
				}

				fmt.Println("ok")
			}
		}
	}
}
