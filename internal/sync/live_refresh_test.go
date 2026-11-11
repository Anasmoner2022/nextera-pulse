package sync

import (
	"context"
	"testing"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

type staticEnrollments struct {
	users []zeroone.EventUser
}

func (s staticEnrollments) AllEventUsers(context.Context, int) ([]zeroone.EventUser, error) {
	return s.users, nil
}

func TestRefreshPilotLevelsRebuildsPaceSnapshot(t *testing.T) {
	db, err := store.Connect(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, "../../migrations/sqlite.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at) VALUES
		(10, 'learner', 'Learner', 'One', 'learner@example.com', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	enrolled := time.Now().UTC().AddDate(0, 0, -200).Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at) VALUES
		(1, 97, 4, ?, 1, 0, ?)`, enrolled, enrolled); err != nil {
		t.Fatal(err)
	}

	result, err := RefreshPilotLevels(ctx, db, staticEnrollments{users: []zeroone.EventUser{{
		ID: 1, UserID: 10, UserLogin: "learner", CreatedAt: time.Now().UTC(), Level: 30,
	}}}, 97, 4)
	if err != nil {
		t.Fatal(err)
	}
	if result.UpdatedRows != 1 {
		t.Fatalf("updated rows = %d", result.UpdatedRows)
	}
	var status string
	var level float64
	if err := db.QueryRowContext(ctx, `SELECT pace_status, actual_level FROM student_metric_snapshots WHERE student_id = 1 AND event_id = 97`).Scan(&status, &level); err != nil {
		t.Fatal(err)
	}
	if status != "GREEN" || level != 30 {
		t.Fatalf("snapshot status %s level %v", status, level)
	}
}
