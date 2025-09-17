package psqltoolbox

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ParsePostgresURL parses a PostgreSQL connection URL and returns the
// username, password, host, port and database name.
// It validates that all five components are non-empty and returns an error otherwise.
func ParsePostgresURL(raw string) (user, pass, host, port, db string, err error) {
	if raw == "" {
		return "", "", "", "", "", fmt.Errorf("empty db url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme == "" {
		// accept urls without explicit scheme? keep requirement same as callers
	}

	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	host = u.Hostname()
	port = u.Port()
	db = strings.TrimPrefix(u.Path, "/")

	if user == "" || pass == "" || host == "" || port == "" || db == "" {
		return "", "", "", "", "", fmt.Errorf("incomplete database URL; got user=%q host=%q port=%q db=%q", user, host, port, db)
	}
	return user, pass, host, port, db, nil
}

func DropTablesAndMigrate(ctx context.Context, conn *pgx.Conn, dbURL, migrationsPath string) error {
	const dropSQL = `
DO
$$
DECLARE
    _tbl text;
BEGIN
    FOR _tbl IN
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'public'
    LOOP
        EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(_tbl) || ' CASCADE';
    END LOOP;
END
$$;
`

	fmt.Printf("[%s] Clearing all tables in the database...\n", time.Now().Format(time.RFC3339))
	if _, err := conn.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("drop tables: %w", err)
	}
	fmt.Printf("[%s] All tables cleared in the database.\n", time.Now().Format(time.RFC3339))

	if migrationsPath != "" {
		fmt.Printf("[%s] Running DB migrations from %s...\n", time.Now().Format(time.RFC3339), migrationsPath)
		mctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(mctx, "migrate", "-database", dbURL, "-path", migrationsPath, "up")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("migrate up failed: %w", err)
		}
		fmt.Printf("[%s] Migrations applied.\n", time.Now().Format(time.RFC3339))
	} else {
		fmt.Printf("[%s] No migrations path provided; skipping migrate.\n", time.Now().Format(time.RFC3339))
	}

	return nil
}

// PgDumpToFile runs pg_dump for the database described by dbURL and writes the
// dump to outFile. A timeout is applied by deriving a child context from parentCtx.
func PgDumpToFile(parentCtx context.Context, dbURL, outFile string, timeout time.Duration) error {
	user, pass, host, port, db, err := ParsePostgresURL(dbURL)
	if err != nil {
		return fmt.Errorf("parse db url: %w", err)
	}

	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pg_dump",
		"-h", host,
		"-p", port,
		"-U", user,
		"-d", db,
		"-F", "c",
		"-b",
		"-v",
		"-f", outFile,
	)

	// pass PGPASSWORD in env for pg_dump
	cmd.Env = append(os.Environ(), "PGPASSWORD="+pass)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}
	return nil
}
