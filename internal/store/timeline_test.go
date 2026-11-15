package store

import (
	"context"
	"testing"

	"nextera-pulse/internal/store/pgtest"
)

func newTimelineDB(t *testing.T) *DB {
	t.Helper()
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSeedTimelineOnceAndNeverOverwrite(t *testing.T) {
	db := newTimelineDB(t)
	ctx := context.Background()
	const event = 97
	if err := db.SeedCurriculumTimeline(ctx, event); err != nil {
		t.Fatal(err)
	}
	v, milestones, err := db.ActiveTimeline(ctx, event)
	if err != nil {
		t.Fatal(err)
	}
	if v.Label != PlaceholderTimelineLabel || v.Status != TimelineDraft || v.Approved || len(milestones) != 9 {
		t.Fatalf("seeded version = %+v with %d months", v, len(milestones))
	}
	if milestones[4].MinLevel != 9 || milestones[4].ExpectedLevel != 13 || milestones[18].ExpectedLevel != 62 {
		t.Fatalf("placeholder values changed: %+v %+v", milestones[4], milestones[18])
	}

	// Someone edits a target and a status policy in the database.
	if _, err := db.ExecContext(ctx, `UPDATE curriculum_timeline SET min_level = 11 WHERE month = 4 AND version_id = $1`, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE status_policy SET pause_clock = FALSE, description = 'edited' WHERE record_type = 'restricted'`); err != nil {
		t.Fatal(err)
	}
	// Restart twice: migrate and seed again.
	for i := 0; i < 2; i++ {
		if err := db.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
		if err := db.SeedCurriculumTimeline(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	_, milestones, _ = db.ActiveTimeline(ctx, event)
	if milestones[4].MinLevel != 11 {
		t.Fatalf("restart overwrote the edited target: %+v", milestones[4])
	}
	var versions, rows int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM curriculum_timeline_versions WHERE event_id = $1`, event).Scan(&versions)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM curriculum_timeline WHERE event_id = $1`, event).Scan(&rows)
	if versions != 1 || rows != 9 {
		t.Fatalf("after restarts: %d versions, %d rows; want 1 and 9", versions, rows)
	}
	var pause bool
	var description string
	if err := db.QueryRowContext(ctx, `SELECT pause_clock, description FROM status_policy WHERE record_type = 'restricted'`).Scan(&pause, &description); err != nil || pause || description != "edited" {
		t.Fatalf("restart overwrote the edited status policy: pause=%v %q %v", pause, description, err)
	}
	// A deleted default policy comes back; edited ones stay.
	if _, err := db.ExecContext(ctx, `DELETE FROM status_policy WHERE record_type = 'away'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var away int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM status_policy WHERE record_type = 'away'`).Scan(&away)
	if away != 1 {
		t.Fatal("missing default status policy was not restored")
	}
}

func addVersion(t *testing.T, db *DB, event int, label, status, createdAt string, minLevel float64) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(context.Background(), `INSERT INTO curriculum_timeline_versions (event_id, label, status, created_at)
		VALUES ($1, $2, $3, $4) RETURNING id`, event, label, status, createdAt).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `INSERT INTO curriculum_timeline (event_id, version_id, month, min_level, expected_level)
		VALUES ($1, $2, 4, $3::double precision, $3::double precision + 2)`, event, id, minLevel); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestActiveTimelinePrefersLatestApprovedThenLatestDraft(t *testing.T) {
	db := newTimelineDB(t)
	ctx := context.Background()
	const event = 120 // not the default event
	if v, ok, err := db.ActiveTimelineVersion(ctx, event); err != nil || ok || v.EventID != event {
		t.Fatalf("no versions yet: %+v %v %v", v, ok, err)
	}
	addVersion(t, db, event, "draft-a", TimelineDraft, "2026-01-01T00:00:00Z", 1)
	addVersion(t, db, event, "draft-b", TimelineDraft, "2026-02-01T00:00:00Z", 2)
	check := func(wantLabel string, wantMin float64) {
		t.Helper()
		v, m, err := db.ActiveTimeline(ctx, event)
		if err != nil || v.Label != wantLabel || m[4].MinLevel != wantMin || len(m) != 1 {
			t.Fatalf("active = %+v %+v (%v), want %s with min %.0f", v, m, err, wantLabel, wantMin)
		}
	}
	check("draft-b", 2) // newest draft

	approved := addVersion(t, db, event, "approved-1", TimelineApproved, "2025-12-01T00:00:00Z", 3)
	check("approved-1", 3) // an approved version wins over newer drafts
	addVersion(t, db, event, "approved-2", TimelineApproved, "2026-03-01T00:00:00Z", 4)
	check("approved-2", 4) // newest approved
	if v, _, _ := db.ActiveTimelineVersion(ctx, event); !v.Approved || v.Status != TimelineApproved {
		t.Fatalf("approved flag = %+v", v)
	}

	if _, err := db.ExecContext(ctx, `UPDATE curriculum_timeline_versions SET status = 'retired' WHERE event_id = $1 AND label = 'approved-2'`, event); err != nil {
		t.Fatal(err)
	}
	check("approved-1", 3) // retired versions are never used
	if _, err := db.ExecContext(ctx, `UPDATE curriculum_timeline_versions SET status = 'retired' WHERE id = $1`, approved); err != nil {
		t.Fatal(err)
	}
	check("draft-b", 2)

	// Seeding never adds placeholders to an event that already has versions,
	// and other events are independent.
	if err := db.SeedCurriculumTimeline(ctx, event); err != nil {
		t.Fatal(err)
	}
	check("draft-b", 2)
	if err := db.SeedCurriculumTimeline(ctx, 97); err != nil {
		t.Fatal(err)
	}
	if v, m, _ := db.ActiveTimeline(ctx, 97); v.Label != PlaceholderTimelineLabel || len(m) != 9 {
		t.Fatalf("default event = %+v (%d months)", v, len(m))
	}
	check("draft-b", 2)
}

func TestMigrateFilesPreVersioningRowsUnderDraftV0(t *testing.T) {
	db := newTimelineDB(t)
	ctx := context.Background()
	// A database from before versioning: rows with no version.
	if _, err := db.ExecContext(ctx, `ALTER TABLE curriculum_timeline ALTER COLUMN version_id DROP NOT NULL;
		INSERT INTO curriculum_timeline (event_id, month, min_level, expected_level) VALUES (300, 2, 5, 7), (300, 4, 10, 13)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	v, m, err := db.ActiveTimeline(ctx, 300)
	if err != nil || v.Label != PlaceholderTimelineLabel || v.Status != TimelineDraft || len(m) != 2 || m[4].MinLevel != 10 {
		t.Fatalf("upgraded = %+v %+v %v", v, m, err)
	}
}
