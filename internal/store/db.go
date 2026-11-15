package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// DB wraps sql.DB. PostgreSQL is the default. SQLite is an optional file store
// selected by the DSN, not by falling through when Postgres is unreachable.
type DB struct {
	*sql.DB
	DriverName string
}

// ErrNotPostgres is returned when DATABASE_URL is neither PostgreSQL nor SQLite.
var ErrNotPostgres = errors.New("DATABASE_URL must be postgres://… or a SQLite DSN (file:….sqlite or :memory:)")

// DriverFor reports "postgres" or "sqlite" for a configured DSN.
func DriverFor(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "", errors.New("DATABASE_URL is required")
	}
	lower := strings.ToLower(dsn)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil || u.Host == "" {
			return "", ErrNotPostgres
		}
		return "postgres", nil
	}
	path := lower
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if dsn == ":memory:" || strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "sqlite:") || strings.HasSuffix(path, ".sqlite") {
		return "sqlite", nil
	}
	return "", ErrNotPostgres
}

// ValidateDSN checks that dsn is a PostgreSQL URL or a SQLite file DSN.
func ValidateDSN(dsn string) error {
	_, err := DriverFor(dsn)
	return err
}

// Connect opens and pings PostgreSQL or SQLite based on the DSN.
func Connect(dsn string) (*DB, error) {
	driverName, err := DriverFor(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	if driverName == "sqlite" {
		if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite foreign keys: %w", err)
		}
		if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite journal: %w", err)
		}
	}
	return &DB{DB: db, DriverName: driverName}, nil
}

// RedactDSN hides the password in a database URL for logs.
func RedactDSN(dsn string) string {
	if driver, err := DriverFor(dsn); err == nil && driver == "sqlite" {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "postgres://…"
	}
	return u.Redacted()
}
