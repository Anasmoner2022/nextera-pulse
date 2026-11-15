package sync

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"nextera-pulse/internal/store"
)

// testLockKey keeps tests off the production lock keys (advisory locks are
// database-wide and test packages run in parallel).
func testLockKey() int64 { return 0x7E570000_00000000 | rand.Int63n(1<<40) }

func snapshotRow(t *testing.T, db *store.DB, studentID int64, day time.Time) (days, month int, status string) {
	t.Helper()
	err := db.QueryRowContext(context.Background(), `SELECT active_program_days, curriculum_month, pace_status
		FROM student_metric_snapshots WHERE student_id = $1 AND event_id = 97 AND snapshot_date = $2`,
		studentID, day.UTC().Format("2006-01-02")).Scan(&days, &month, &status)
	if err != nil {
		t.Fatalf("snapshot for student %d on %s: %v", studentID, day.Format("2006-01-02"), err)
	}
	return
}

func snapshotCount(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM student_metric_snapshots`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// No credential, no LiveSource: the job works from stored data only.
func TestSnapshotJobRefreshesFromStoredDataAsTheClockMoves(t *testing.T) {
	db := seededDB(t) // two learners enrolled 200 days ago, no 01Edu source anywhere
	job := NewSnapshotJob(db, 97, 24*time.Hour)
	job.lockKey = testLockKey()
	today := time.Now().UTC()
	job.now = func() time.Time { return today }

	n, err := job.RunOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("first run: n=%d err=%v", n, err)
	}
	days1, month1, _ := snapshotRow(t, db, 1, today)

	later := today.AddDate(0, 0, 45)
	job.now = func() time.Time { return later }
	if _, err := job.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	days2, month2, _ := snapshotRow(t, db, 1, later)
	if days2 != days1+45 || month2 <= month1 {
		t.Fatalf("after 45 days: days %d→%d, month %d→%d", days1, days2, month1, month2)
	}
	status := job.Status()
	if status.LastSuccessAt == "" || status.LastError != "" || status.Students != 2 || status.IntervalSeconds != 86400 {
		t.Fatalf("status = %+v", status)
	}
}

func TestSnapshotJobIsIdempotentPerDay(t *testing.T) {
	db := seededDB(t)
	job := NewSnapshotJob(db, 97, time.Hour)
	job.lockKey = testLockKey()
	for i := 0; i < 3; i++ {
		if _, err := job.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := snapshotCount(t, db); got != 2 {
		t.Fatalf("snapshot rows after three runs on one day = %d, want 2", got)
	}
	// The live cycle's rebuild goes through the same upsert.
	if _, err := RefreshSnapshotsLocked(context.Background(), db, 97, time.Now(), true, job.lockKey); err != nil {
		t.Fatal(err)
	}
	if got := snapshotCount(t, db); got != 2 {
		t.Fatalf("snapshot rows after a live-cycle rebuild = %d, want 2", got)
	}
}

func TestSnapshotJobSkipsWhileAnotherRefreshHoldsTheLock(t *testing.T) {
	db := seededDB(t)
	job := NewSnapshotJob(db, 97, time.Hour)
	job.lockKey = testLockKey()
	inside, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		_ = db.WithAdvisoryLock(context.Background(), job.lockKey, false, func(context.Context) error { close(inside); <-release; return nil })
		close(done)
	}()
	<-inside
	if _, err := job.RunOnce(context.Background()); !errors.Is(err, store.ErrLockHeld) {
		t.Fatalf("err = %v, want ErrLockHeld", err)
	}
	if got := job.Status().LastSkipped; got != SnapshotSkipped {
		t.Fatalf("lastSkipped = %q", got)
	}
	if got := snapshotCount(t, db); got != 0 {
		t.Fatalf("skipped run wrote %d rows", got)
	}
	close(release)
	<-done
	if _, err := job.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if job.Status().LastSkipped != "" {
		t.Fatal("a successful run clears the skip")
	}
}

func testRunner(db *store.DB, cycle func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error), key int64) *Runner {
	r := NewRunner(db, fakeLive{}, nil, pilotConfig, time.Hour)
	r.lockKey, r.cycle = key, cycle
	return r
}

// Two instances (two runners sharing the database) start a cycle at the same
// time: only one runs, the other reports it skipped.
func TestConcurrentLiveCyclesRunOnlyOnce(t *testing.T) {
	db := seededDB(t)
	key := testLockKey()
	started, release := make(chan struct{}), make(chan struct{})
	runs := 0
	blocking := func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error) {
		runs++
		close(started)
		<-release
		return LiveCycleReport{Status: CycleSuccess, CohortWide: true, FinishedAt: time.Now().UTC()}, nil
	}
	quick := func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error) {
		t.Error("the second instance must not run while the first holds the lock")
		return LiveCycleReport{}, nil
	}
	first, second := testRunner(db, blocking, key), testRunner(db, quick, key)

	done := make(chan LiveCycleReport)
	go func() { report, _ := first.RunOnce(context.Background()); done <- report }()
	<-started
	report, err := second.RunOnce(context.Background())
	if err != nil || report.Status != CycleSkipped {
		t.Fatalf("second instance: %+v, %v", report, err)
	}
	status := second.Status()
	if status.LastSkipped != SkippedLocked || status.LastSkippedAt == "" {
		t.Fatalf("second status = %+v", status)
	}
	close(release)
	if got := <-done; got.Status != CycleSuccess || runs != 1 {
		t.Fatalf("first instance: %+v (runs %d)", got, runs)
	}
	if held, _ := db.AdvisoryLockHeld(context.Background(), key); held {
		t.Fatal("lock still held after the cycle")
	}
}

func TestLiveSyncLockIsReleasedAfterAFailedOrPanickingCycle(t *testing.T) {
	db := seededDB(t)
	key := testLockKey()
	failing := testRunner(db, func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error) {
		return LiveCycleReport{Status: CycleFailed}, errors.New("01Edu unreachable")
	}, key)
	if _, err := failing.RunOnce(context.Background()); err == nil {
		t.Fatal("expected the cycle error")
	}
	panicking := testRunner(db, func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error) {
		panic("bad payload")
	}, key)
	if _, err := panicking.RunOnce(context.Background()); err == nil || panicking.Status().Running {
		t.Fatalf("panic should become an error and stop running: %v", err)
	}
	if held, _ := db.AdvisoryLockHeld(context.Background(), key); held {
		t.Fatal("lock still held after a failed cycle")
	}
	ran := false
	next := testRunner(db, func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error) {
		ran = true
		return LiveCycleReport{Status: CycleSuccess, CohortWide: true}, nil
	}, key)
	if report, err := next.RunOnce(context.Background()); err != nil || report.Status == CycleSkipped || !ran {
		t.Fatalf("next cycle after the failure: %+v %v ran=%v", report, err, ran)
	}
	if failing.Status().LastError == "" {
		t.Fatal("the failure should be in sync status")
	}
}
