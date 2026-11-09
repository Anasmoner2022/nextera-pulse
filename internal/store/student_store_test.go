package store

import (
	"context"
	"testing"
)

func TestListStudentsWithPace(t *testing.T) {
	db, err := Connect(":memory:")
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx, "../../migrations/sqlite.sql"); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	students, err := db.ListStudentsWithPace(ctx, 4, "", "")
	if err != nil {
		t.Fatalf("ListStudentsWithPace failed: %v", err)
	}

	if len(students) != 0 {
		t.Fatalf("expected 0 students in empty db, got %d", len(students))
	}
}
