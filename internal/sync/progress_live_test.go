package sync

import (
	"context"
	"testing"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

func TestApplyLiveProgressUpdatesDoneFlag(t *testing.T) {
	db, err := store.Connect(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, "../../migrations/sqlite.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at) VALUES (42, 'learner', 'Learner', 'One', 'learner@example.com', 'user', '2026-01-01', '2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	grade := 1.0
	updated, err := ApplyLiveProgress(ctx, db, []zeroone.LiveProgress{{
		ID: 9, UserID: 42, ObjectID: 3, IsDone: true, Grade: &grade,
		Object: &zeroone.LiveObject{ID: 3, Name: "ascii-art", Type: "project", Path: "/cairo/ascii-art"},
	}})
	if err != nil || updated != 1 {
		t.Fatalf("updated %d err %v", updated, err)
	}
	var done int
	var name string
	if err := db.QueryRowContext(ctx, `SELECT p.is_done, o.name FROM learning_progress p JOIN learning_objects o ON o.source_object_id = p.object_id`).Scan(&done, &name); err != nil {
		t.Fatal(err)
	}
	if done != 1 || name != "ascii-art" {
		t.Fatalf("done %d name %s", done, name)
	}
}
