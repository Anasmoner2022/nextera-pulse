package store

import (
	"context"
	"testing"

	"nextera-pulse/internal/store/pgtest"
)

// kpiFixture: event 97, cohort 4 has learners 1–5, cohort 1 has learner 6.
// Mentor 7 has learners 1–3 (current), mentor 8 had learner 4 (ended).
func kpiFixture(t *testing.T) (*DB, context.Context) {
	t.Helper()
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO students (id, source_user_id, login, source_role) VALUES
		(1, 1, 'l1', 'user'), (2, 2, 'l2', 'user'), (3, 3, 'l3', 'user'),
		(4, 4, 'l4', 'user'), (5, 5, 'l5', 'user'), (6, 6, 'l6', 'user'),
		(7, 7, 'mentor.a', 'admin campus'), (8, 8, 'mentor.b', 'admin campus')`)
	mustExec(t, db, `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at)
		SELECT id, 97, CASE WHEN id = 6 THEN 1 ELSE 4 END, '2026-01-01', 1, 0, '2026-01-01' FROM students WHERE id <= 6`)
	mustExec(t, db, `INSERT INTO student_metric_snapshots (student_id, event_id, snapshot_date, enrolled_at, active_program_days, curriculum_month,
		actual_level, minimum_level, expected_level, checkpoint_target, gap_to_minimum, gap_to_expected, pace_status) VALUES
		(1, 97, '2026-11-01', '2026-01-01', 300, 10, 9, 8, 9, 0, 1, 0, 'GREEN'),
		(2, 97, '2026-11-01', '2026-01-01', 300, 10, 8, 8, 9, 0, 0, -1, 'YELLOW'),
		(3, 97, '2026-11-01', '2026-01-01', 300, 10, 9, 8, 9, 0, 1, 0, 'GREEN'),
		(4, 97, '2026-11-01', '2026-01-01', 300, 10, 2, 8, 9, 0, -6, -7, 'RED'),
		(5, 97, '2026-11-01', '2026-01-01', 300, 10, 2, 8, 9, 0, -6, -7, 'PAUSED'),
		(6, 97, '2026-11-01', '2026-01-01', 300, 10, 9, 8, 9, 0, 1, 0, 'GREEN')`)
	// learner 3 has an open "restricted" record (flagged in the seeded policy).
	mustExec(t, db, `INSERT INTO student_source_records (student_id, type_name, start_at) VALUES (3, 'restricted', '2026-10-01')`)
	mustExec(t, db, `INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, is_primary, ends_at) VALUES
		(1, 7, 'Mentor A', TRUE, NULL), (2, 7, 'Mentor A', TRUE, NULL), (3, 7, 'Mentor A', TRUE, NULL),
		(4, 8, 'Mentor B', TRUE, '2026-10-01')`)
	return db, ctx
}

func mustExec(t *testing.T, db *DB, query string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
}

func TestCohortHealthUsesGreenOverMeasurable(t *testing.T) {
	db, ctx := kpiFixture(t)
	summary, err := db.GetCohortHealthSummary(ctx, 97, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Cohort 4: GREEN 1, YELLOW 2, GREEN 3 (restricted → excluded), RED 4, PAUSED 5.
	// Measurable = 1, 2, 4 → 1 of 3 on track.
	if summary.OnTrackCount != 1 || summary.MeasurableStudents != 3 || summary.OnTrackRate == nil {
		t.Fatalf("summary = %+v", summary)
	}
	if got := *summary.OnTrackRate; got < 33.3 || got > 33.4 {
		t.Fatalf("rate = %v, want 33.3", got)
	}
	if summary.NearTrackCount != 1 || summary.OffTrackCount != 1 || summary.TotalStudents != 5 || summary.KPI.ExcludedByPolicy != 1 {
		t.Fatalf("summary = %+v", summary)
	}

	// The exclusion comes from status_policy, not a list in code.
	mustExec(t, db, `UPDATE status_policy SET exclude_from_active_kpis = FALSE WHERE record_type = 'restricted'`)
	summary, err = db.GetCohortHealthSummary(ctx, 97, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.OnTrackCount != 2 || summary.MeasurableStudents != 4 || *summary.OnTrackRate != 50 {
		t.Fatalf("after unflagging restricted: %+v", summary)
	}
	mustExec(t, db, `INSERT INTO student_source_records (student_id, type_name, start_at) VALUES (4, 'observation', '2026-10-01')`)
	mustExec(t, db, `UPDATE status_policy SET exclude_from_active_kpis = TRUE WHERE record_type = 'observation'`)
	summary, err = db.GetCohortHealthSummary(ctx, 97, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.OnTrackCount != 2 || summary.MeasurableStudents != 3 {
		t.Fatalf("after flagging observation: %+v", summary)
	}
}

func TestCohortHealthWithNoMeasurableStudentsIsNull(t *testing.T) {
	db, ctx := kpiFixture(t)
	summary, err := db.GetCohortHealthSummary(ctx, 97, 4, map[int64]bool{5: true}) // only the paused learner
	if err != nil {
		t.Fatal(err)
	}
	if summary.OnTrackRate != nil || summary.KPI.HasMeasurable || summary.MeasurableStudents != 0 || summary.TotalStudents != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	empty, err := db.GetCohortHealthSummary(ctx, 97, 99, nil)
	if err != nil || empty.OnTrackRate != nil || empty.KPI.HasMeasurable {
		t.Fatalf("empty cohort = %+v, %v", empty, err)
	}
}

func TestOnTrackKPIsProgramCohortAndMentor(t *testing.T) {
	db, ctx := kpiFixture(t)
	report, err := db.OnTrackKPIs(ctx, 97)
	if err != nil {
		t.Fatal(err)
	}
	// Program: measurable 1, 2, 4, 6 → GREEN 1 and 6 → 2 of 4.
	if p := report.Program; p.OnTrack != 2 || p.Measurable != 4 || *p.Rate != 50 || p.Total != 6 || p.Scope != "program" {
		t.Fatalf("program = %+v", p)
	}
	if c := report.Cohorts[1]; c.OnTrack != 1 || c.Measurable != 1 || *c.Rate != 100 {
		t.Fatalf("cohort 1 = %+v", c)
	}
	if c := report.Cohorts[4]; c.OnTrack != 1 || c.Measurable != 3 {
		t.Fatalf("cohort 4 = %+v", c)
	}
	// Mentor A's current students 1, 2, 3 (3 restricted) → 1 of 2.
	if m := report.Mentors[7]; m.OnTrack != 1 || m.Measurable != 2 || *m.Rate != 50 || m.StatusCounts["YELLOW"] != 1 {
		t.Fatalf("mentor A = %+v", m)
	}
	// Mentor B's assignment ended: no current students, so no KPI row.
	if m, found := report.Mentors[8]; found {
		t.Fatalf("ended assignment still counted: %+v", m)
	}
}
