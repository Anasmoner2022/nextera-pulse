package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// DB wraps standard sql.DB with driver identity
type DB struct {
	*sql.DB
	DriverName string
}

// Connect initializes a database connection based on DSN
func Connect(dsn string) (*DB, error) {
	var driverName string
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		driverName = "postgres"
	} else {
		driverName = "sqlite"
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// SQLite settings for concurrency
	if driverName == "sqlite" {
		_, _ = db.Exec("PRAGMA foreign_keys = ON;")
		_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	}

	return &DB{DB: db, DriverName: driverName}, nil
}
