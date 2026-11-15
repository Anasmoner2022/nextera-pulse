package store

import (
	"context"
	"nextera-pulse/internal/store/pgtest"
	"testing"
)

func TestListStudentsWithPace(t *testing.T) {
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	students, err := db.ListStudentsWithPace(ctx, 0, 4, "", "")
	if err != nil {
		t.Fatalf("ListStudentsWithPace failed: %v", err)
	}

	if len(students) != 0 {
		t.Fatalf("expected 0 students in empty db, got %d", len(students))
	}
}

func TestFindIdentityByLoginUses01EduRole(t *testing.T) {
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatalf("failed to connect to memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at) VALUES
		(1, 'campus.admin', 'Campus', 'Admin', 'campus@example.com', 'admin campus', '2026-01-01', '2026-01-01'),
		(2, 'school.admin', 'School', 'Admin', 'school@example.com', 'admin', '2026-01-01', '2026-01-01'),
		(3, 'learner', 'Learner', 'One', 'learner@example.com', 'user', '2026-01-01', '2026-01-01')`)
	if err != nil {
		t.Fatalf("insert students: %v", err)
	}

	mentor, err := db.FindIdentityByLogin(ctx, "@Campus.Admin")
	if err != nil || mentor == nil || mentor.PulseRole != "mentor" {
		t.Fatalf("campus admin = %#v, %v", mentor, err)
	}
	manager, err := db.FindIdentityByLogin(ctx, "school@example.com")
	if err != nil || manager == nil || manager.PulseRole != "manager" {
		t.Fatalf("admin = %#v, %v", manager, err)
	}
	student, err := db.FindIdentityByLogin(ctx, "learner")
	if err != nil || student == nil || student.PulseRole != "student" || student.Login != "learner" {
		t.Fatalf("student = %#v, %v", student, err)
	}
	missing, err := db.FindIdentityByLogin(ctx, "nobody")
	if err != nil || missing != nil {
		t.Fatalf("missing = %#v, %v", missing, err)
	}
}

func TestListStudentsWithPaceKeepsEachEventSnapshot(t *testing.T) {
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at) VALUES
		(10, 'learner', 'Learner', 'One', 'learner@example.com', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at) VALUES
		(1, 97, 4, '2026-01-01T00:00:00Z', 1, 0, '2026-01-01T00:00:00Z'),
		(1, 98, 1, '2026-01-01T00:00:00Z', 40, 0, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO student_metric_snapshots (
		student_id, event_id, snapshot_date, enrolled_at, active_program_days, curriculum_month,
		actual_level, minimum_level, expected_level, checkpoint_target, gap_to_minimum, gap_to_expected,
		pace_status, health_reasons, calculation_version, calculated_at
	) VALUES
		(1, 97, '2026-11-01', '2026-01-01T00:00:00Z', 100, 4, 1, 9, 13, 0, -8, -12, 'RED', '[]', 'v0.2', '2026-11-01T00:00:00Z'),
		(1, 98, '2026-11-09', '2026-01-01T00:00:00Z', 100, 4, 40, 9, 13, 0, 31, 27, 'GREEN', '[]', 'v0.2', '2026-11-09T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	pilot, err := db.ListStudentsWithPace(ctx, 0, 4, "", "")
	if err != nil || len(pilot) != 1 {
		t.Fatalf("pilot rows %d err %v", len(pilot), err)
	}
	if pilot[0].Pace.Status != "RED" || pilot[0].Pace.ActualLevel != 1 {
		t.Fatalf("event 97 borrowed another event snapshot: %+v", pilot[0].Pace)
	}
}
