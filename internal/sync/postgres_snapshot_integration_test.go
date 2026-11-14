//go:build integration

package sync

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"nextera-pulse/internal/store"
)

// Destructive-to-test-state: inserts timeline policies and (re)writes today's
// snapshots. Run ONLY against disposable nextera_pulse_test with synthetic data.
func TestPostgresSnapshotIntegration(t *testing.T) {
	dsn := os.Getenv("PULSE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("PULSE_TEST_DATABASE_URL must point to disposable test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Path != "/nextera_pulse_test" {
		t.Fatal("refusing non-PostgreSQL or non-test database")
	}
	db, err := store.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.SeedStatusPolicy(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.SeedCurriculumTimeline(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM students s JOIN student_events se ON se.student_id=s.id WHERE s.login='test_student' AND se.event_id=97 AND se.cohort_id=4`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one synthetic test_student, got %d", count)
	}

	n, err := RefreshSnapshots(ctx, db, 97, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generated %d snapshots", n)
	var days, month int
	var status string
	err = db.QueryRowContext(ctx, `SELECT sms.active_program_days,sms.curriculum_month,sms.pace_status FROM student_metric_snapshots sms JOIN students s ON s.id=sms.student_id WHERE s.login='test_student' AND sms.event_id=97 ORDER BY sms.snapshot_date DESC LIMIT 1`).Scan(&days, &month, &status)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 || days < 0 || month < 1 || status == "" {
		t.Fatalf("invalid snapshot: generated=%d days=%d month=%d status=%q", n, days, month, status)
	}
	t.Logf("verified snapshot: activeDays=%d month=%d status=%s", days, month, status)
}
