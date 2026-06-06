package main

import (
	"fmt"
	"time"

	"go-ql/ast"
	"go-ql/backend"
	"go-ql/lexer"
	"go-ql/storage"
)

func main() {
	store := &storage.FileStore{
		Path: "goql.db",
	}

	mb := backend.NewMemoryBackend(store)


	err := mb.CreateTable(&ast.CreateTableStatement{
		Name: lexer.Token{
			Value: "users",
		},
		Cols: &[]*ast.ColumnDefinition{
			{
				Name: lexer.Token{
					Value: "id",
				},
				Datatype: lexer.Token{
					Value: "int",
				},
				PrimaryKey: true,
			},
			{
				Name: lexer.Token{
					Value: "name",
				},
				Datatype: lexer.Token{
					Value: "text",
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	start := time.Now()

	for i := 0; i < 20000; i++ {
		err := mb.Insert(&ast.InsertStatement{
			Table: lexer.Token{
				Value: "users",
			},
			Values: &[]*ast.Expression{
				{
					Kind: ast.LiteralKind,
					Literal: &lexer.Token{
						Kind:  lexer.NumericKind,
						Value: fmt.Sprintf("%d", i),
					},
				},
				{
					Kind: ast.LiteralKind,
					Literal: &lexer.Token{
						Kind:  lexer.StringKind,
						Value: fmt.Sprintf("user_%d", i),
					},
				},
			},
		})

		if err != nil {
			panic(err)
		}
	}

	fmt.Printf(
		"Inserted 20000 rows in %s\n",
		time.Since(start),
	)
}