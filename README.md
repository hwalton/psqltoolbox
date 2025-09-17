# psqltoolbox

A Go library providing utilities for working with PostgreSQL databases, including connection URL parsing, database dumping, and migration helpers.

## Features

- **ParsePostgresURL**: Parse and validate PostgreSQL connection URLs.
- **PgDumpToFile**: Run `pg_dump` with timeout and output to a file.
- **DropTablesAndMigrate**: Drop all tables and run migrations using the `migrate` CLI.

## Installation

Add to your Go project:

```sh
go get github.com/hwalton/psqltoolbox
```

## Usage

### Parse a PostgreSQL URL

```go
user, pass, host, port, db, err := psqltoolbox.ParsePostgresURL("postgres://alice:secret@db.example.com:5432/mydb")
if err != nil {
    // handle error
}
```

### Dump a Database to File

```go
ctx := context.Background()
err := psqltoolbox.PgDumpToFile(ctx, "postgres://user:pass@host:5432/dbname", "backup.dump", 10*time.Second)
if err != nil {
    // handle error
}
```

### Drop All Tables and Run Migrations

```go
conn, err := pgx.Connect(ctx, dbURL)
if err != nil {
    // handle error
}
defer conn.Close(ctx)

err = psqltoolbox.DropTablesAndMigrate(ctx, conn, dbURL, "/path/to/migrations")
if err != nil {
    // handle error
}
```

## Requirements

- Go 1.18+
- [pgx](https://github.com/jackc/pgx) Go driver
- `pg_dump` must be available in your `PATH` for dump operations
- [migrate CLI](https://github.com/golang-migrate/migrate) for migrations

## Testing

Run all tests:

```sh
go test ./...
```

## License

Apache 2.0. See [LICENSE](LICENSE).