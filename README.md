# go-ql

A lightweight SQL-like query engine written in Go. This project implements a simple in-memory database with a custom lexer, parser, and execution engine, with early support for indexing and persistence.

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
- No full query optimizer (only primary key fast path)

## Future Work

- Secondary indexes
- Query planner / optimizer
- Write-ahead logging (WAL)
- Improved type system
- Disk-based storage engine
