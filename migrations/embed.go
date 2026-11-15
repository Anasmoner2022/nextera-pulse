// Package migrations embeds the PostgreSQL schema so the binaries do not
// depend on the working directory they are started from.
package migrations

import _ "embed"

// Postgres is the idempotent PostgreSQL schema applied on every start.
//
//go:embed postgres.sql
var Postgres string

// SQLite is the optional file-database schema from the 11 November handover.
// It is applied only when DATABASE_URL selects SQLite.
//
//go:embed sqlite.sql
var SQLite string
