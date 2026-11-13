package store

import (
	"context"
	"testing"
)

func TestValidateDSNRequiresPostgresOrSQLite(t *testing.T) {
	for _, dsn := range []string{"", "mysql://u:p@h/db", "postgres://"} {
		if err := ValidateDSN(dsn); err == nil {
			t.Fatalf("%q accepted", dsn)
		}
	}
	for _, dsn := range []string{
		"postgres://u:p@localhost:5432/db?sslmode=disable",
		"postgresql://u@db/x",
		":memory:",
		"file:dev-pulse.sqlite",
		"dev-pulse.sqlite",
	} {
		if err := ValidateDSN(dsn); err != nil {
			t.Fatalf("%q rejected: %v", dsn, err)
		}
	}
}

func TestConnectSQLiteMemoryMigratesPolicies(t *testing.T) {
	db, err := Connect(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.DriverName != "sqlite" {
		t.Fatalf("driver %s", db.DriverName)
	}
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM status_policy`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("policies = %d", n)
	}
}
