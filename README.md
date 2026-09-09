# go-ql

A lightweight SQL-like query engine written in Go. The project implements an in-memory database with a custom lexer, parser, execution engine, primary-key indexing, and atomic file persistence.

## Features

- Basic SQL-like syntax support:
  - CREATE TABLE
  - INSERT INTO
  - SELECT
- Custom lexer and parser (AST-based)
- In-memory table storage
- WHERE filtering with basic operators (=, !=, <, >, <=, >=)
- Primary key support with indexing for fast lookups
- Basic persistence (save/load tables from disk)
- Multiline input and recoverable query errors in the interactive shell

## Example

```sql
CREATE TABLE users (
    id INT PRIMARY KEY,
    name TEXT
);

INSERT INTO users VALUES (1, 'Phil');
INSERT INTO users VALUES (2, 'Kate');

SELECT * FROM users;
SELECT * FROM users WHERE id = 1;
SELECT * FROM users WHERE name = 'Phil';
```

## Architecture

- Lexer → converts SQL text into tokens
- Parser (AST) → builds structured query representation
- Backend → executes queries against in-memory tables
- Storage → handles persistence to disk

## Limitations

- No joins
- No aggregations
- Limited SQL grammar
- No concurrency control
- Only basic types (INT, TEXT)
- INT values are signed 32-bit integers
- No full query optimizer (only primary key fast path)

## Development

Run the complete test suite and static checks with:

```sh
go test ./...
go vet ./...
```

Enable the repository's pre-commit checks once after cloning:

```sh
git config core.hooksPath .githooks
```

The hook checks staged whitespace, Go formatting, module consistency, module
integrity, `go vet`, and the complete uncached test suite before each commit.
On Unix-like systems, if Git reports that the hook is not executable, run
`chmod +x .githooks/pre-commit` once.

Run the isolated in-memory and persistence benchmarks with:

```sh
go test ./backend ./storage -run=NoTests -bench=Benchmark
```

## Future Work

- Secondary indexes
- Query planner / optimizer
- Write-ahead logging (WAL)
- Improved type system
- Disk-based storage engine
