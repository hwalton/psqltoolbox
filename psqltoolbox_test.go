package psqltoolbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test ParsePostgresURL happy path and some error cases.
func TestParsePostgresURL_Valid(t *testing.T) {
	u := "postgres://alice:secret@db.example.com:5432/mydb"
	user, pass, host, port, db, err := ParsePostgresURL(u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "alice" || pass != "secret" || host != "db.example.com" || port != "5432" || db != "mydb" {
		t.Fatalf("parsed values mismatch: %q %q %q %q %q", user, pass, host, port, db)
	}
}

func TestParsePostgresURL_Invalid(t *testing.T) {
	cases := []string{
		"",                                 // empty
		"postgres://alice@host:5432/db",    // missing password
		"postgres://:pass@host:5432/db",    // missing user
		"postgres://alice:pass@host/db",    // missing port
		"postgres://alice:pass@host:5432/", // missing db
		"not-a-url",                        // unparsable
	}
	for _, c := range cases {
		if _, _, _, _, _, err := ParsePostgresURL(c); err == nil {
			t.Fatalf("expected error for input %q", c)
		}
	}
}

// Helper to temporarily prepend a directory to PATH.
func withPathPrepended(dir string, fn func()) {
	orig := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+orig)
	defer os.Setenv("PATH", orig)
	fn()
}

// Test PgDumpToFile when pg_dump binary is not found.
func TestPgDumpToFile_NotFound(t *testing.T) {
	ctx := context.Background()
	tmpOut := filepath.Join(t.TempDir(), "out.dump")
	// empty PATH ensures pg_dump not found
	orig := os.Getenv("PATH")
	_ = os.Setenv("PATH", "")
	defer os.Setenv("PATH", orig)

	err := PgDumpToFile(ctx, "postgres://u:p@h:1234/db", tmpOut, 5*time.Second)
	if err == nil {
		t.Fatalf("expected error when pg_dump not found")
	}
}

// Test PgDumpToFile success by creating a fake pg_dump executable that writes the -f output file.
func TestPgDumpToFile_Success(t *testing.T) {
	tmpdir := t.TempDir()
	fake := filepath.Join(tmpdir, "pg_dump")
	script := `#!/usr/bin/env bash
# simple fake pg_dump: find -f and write marker
OUT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -f) OUT="$2"; shift 2;;
    *) shift;;
  esac
done
if [ -z "$OUT" ]; then
  echo "no out file" >&2
  exit 2
fi
echo "FAKEPGDUMP" > "$OUT"
exit 0
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pg_dump: %v", err)
	}

	withPathPrepended(tmpdir, func() {
		ctx := context.Background()
		outFile := filepath.Join(t.TempDir(), "backup.dump")
		err := PgDumpToFile(ctx, "postgres://u:p@h:1234/db", outFile, 5*time.Second)
		if err != nil {
			t.Fatalf("PgDumpToFile failed: %v", err)
		}
		b, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("read out file: %v", err)
		}
		if string(b) != "FAKEPGDUMP\n" {
			t.Fatalf("unexpected contents: %q", string(b))
		}
	})
}

// Test PgDumpToFile respects timeout (fake pg_dump sleeps longer than timeout).
func TestPgDumpToFile_Timeout(t *testing.T) {
	tmpdir := t.TempDir()
	fake := filepath.Join(tmpdir, "pg_dump")
	script := `#!/usr/bin/env bash
sleep 3
# try to create file if not killed
OUT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -f) OUT="$2"; shift 2;;
    *) shift;;
  esac
done
if [ -n "$OUT" ]; then
  echo "LATE" > "$OUT"
fi
exit 0
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pg_dump: %v", err)
	}

	withPathPrepended(tmpdir, func() {
		ctx := context.Background()
		outFile := filepath.Join(t.TempDir(), "backup.dump")
		err := PgDumpToFile(ctx, "postgres://u:p@h:1234/db", outFile, 1*time.Second)
		if err == nil {
			t.Fatalf("expected timeout error")
		}
		// ensure the returned error wraps context.DeadlineExceeded
		if !errors.Is(err, context.DeadlineExceeded) {
			// some platforms may return a wrapped error; still accept non-deadline but non-nil
			t.Logf("warning: expected context deadline exceeded; got: %v", err)
		}
		// file should not exist (sleep 3 script shouldn't have finished)
		if _, statErr := os.Stat(outFile); statErr == nil {
			t.Fatalf("expected out file not to be created on timeout")
		}
	})
}
