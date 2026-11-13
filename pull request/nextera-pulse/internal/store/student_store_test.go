package store

import (
	"context"
	"path/filepath"
	"testing"
)

// This is a hermetic empty-database smoke test, not a 52-student cohort test.
func TestListStudentsWithPace(t *testing.T) {
	ctx := context.Background()
	db, err := Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, "../../migrations/sqlite.sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	students, err := db.ListStudentsWithPace(ctx, 4, "", "")
	if err != nil {
		t.Fatalf("ListStudentsWithPace: %v", err)
	}
	if len(students) != 0 {
		t.Fatalf("expected empty database, got %d students", len(students))
	}
}
