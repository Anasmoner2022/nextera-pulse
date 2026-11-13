package sync

import (
	"context"
	"fmt"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/pace"
	"nextera-pulse/internal/store"
)

// RefreshSnapshots recomputes the pace snapshot dated now (UTC day) for every
// student of the curriculum event from data already in PostgreSQL: active
// program days, curriculum month, monthly targets, gaps and pace status, using
// the event's active timeline version (latest approved, else latest draft).
// It never calls 01Edu. Snapshots are upserted on (student, event, day), so
// running it twice on a day leaves one row per student. Each snapshot records
// the engine and timeline version in calculation_version, e.g.
// "v0.2+timeline:draft-v0(draft)". KPIs are computed from the latest
// snapshots when they are read, so they follow automatically.
//
// This is the only snapshot writer: the daily snapshot job and the live sync
// cycle both call it (through RefreshSnapshotsLocked).
func RefreshSnapshots(ctx context.Context, db *store.DB, eventID int, now time.Time) (int, error) {
	if err := db.SeedCurriculumTimeline(ctx, eventID); err != nil { // no-op when the event has targets
		return 0, err
	}
	policies, err := db.GetStatusPolicies(ctx)
	if err != nil {
		return 0, err
	}

	version, milestones, err := db.ActiveTimeline(ctx, eventID)
	if err != nil {
		return 0, err
	}
	calcVersion := pace.EngineVersion + "+timeline:none"
	if version.Label != "" {
		calcVersion = fmt.Sprintf("%s+timeline:%s(%s)", pace.EngineVersion, version.Label, version.Status)
	}

	engine := pace.NewEngine(policies, milestones)
	now = now.UTC()
	todayStr := now.Format("2006-01-02")

	// Every student enrolled in the curriculum event
	query := db.Rebind(`SELECT s.id, se.enrolled_at, se.current_level, se.event_id
                        FROM students s
                        JOIN student_events se ON s.id = se.student_id
                        WHERE se.event_id = ?;`)

	rows, err := db.QueryContext(ctx, query, eventID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type studentRow struct {
		id       int64
		enrolled store.FlexibleTime
		level    float64
		eventID  int
	}

	var students []studentRow
	for rows.Next() {
		var s studentRow
		if err := rows.Scan(&s.id, &s.enrolled, &s.level, &s.eventID); err != nil {
			return 0, err
		}
		students = append(students, s)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, s := range students {
		enrolledAt := s.enrolled.Time
		records, err := db.GetStudentRecords(ctx, s.id)
		if err != nil {
			return count, fmt.Errorf("read records for student %d: %w", s.id, err)
		}

		metrics := engine.Calculate(enrolledAt, s.level, records, now)

		snap := model.StudentMetricSnapshot{
			StudentID:          s.id,
			EventID:            s.eventID,
			SnapshotDate:       todayStr,
			Pace:               metrics,
			CalculationVersion: calcVersion,
			CalculatedAt:       now,
		}

		if err := db.SaveMetricSnapshot(ctx, snap); err != nil {
			return count, fmt.Errorf("failed saving metric snapshot for student %d: %w", s.id, err)
		}
		count++
	}

	return count, nil
}

// RefreshSnapshotsLocked runs RefreshSnapshots under the snapshot advisory
// lock (store.LockSnapshots unless key is set), so two refreshes never
// overlap across processes. With wait=false a held lock returns
// store.ErrLockHeld at once.
func RefreshSnapshotsLocked(ctx context.Context, db *store.DB, eventID int, now time.Time, wait bool, key int64) (int, error) {
	if key == 0 {
		key = store.LockSnapshots
	}
	n := 0
	err := db.WithAdvisoryLock(ctx, key, wait, func(ctx context.Context) error {
		var err error
		n, err = RefreshSnapshots(ctx, db, eventID, now)
		return err
	})
	return n, err
}
