// Package pgtest gives each test an isolated PostgreSQL schema.
//
// Tests that need a database call DSN(t). It reads TEST_DATABASE_URL, creates
// a throwaway schema, and returns a DSN whose search_path points at it. When
// TEST_DATABASE_URL is unset the test is skipped, unless PULSE_REQUIRE_PG=1
// (set in CI), in which case it fails.
package pgtest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// DSN returns a PostgreSQL URL bound to a fresh schema that is dropped when the test ends.
func DSN(t testing.TB) string {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("PULSE_REQUIRE_PG") == "1" {
			t.Fatal("TEST_DATABASE_URL is required (PULSE_REQUIRE_PG=1)")
		}
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL test")
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	schema := "pulse_test_" + hex.EncodeToString(buf)
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`)
		admin.Close()
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}
