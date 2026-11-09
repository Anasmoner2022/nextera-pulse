package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

// EnrollmentSource is implemented by the read-only GraphQL client.
type EnrollmentSource interface {
	AllEventUsers(context.Context, int) ([]zeroone.EventUser, error)
}

// LiveResult describes the scope of a successful safe refresh.
type LiveResult struct {
	SourceRows    int
	UpdatedRows   int
	UnmatchedRows int
}

// RefreshPilotLevels updates only students already known to belong to the pilot
// cohort. The GraphQL event_user feed is EVENT-wide, not cohort-scoped; this
// function must never infer cohort membership from an event enrollment.
// No deletions, enrollment date changes, or automatic reassignments occur.
func RefreshPilotLevels(ctx context.Context, db *store.DB, source EnrollmentSource, eventID, cohortID int) (LiveResult, error) {
	var outcome LiveResult
	if db == nil || source == nil || eventID <= 0 || cohortID <= 0 {
		return outcome, errors.New("invalid live-sync configuration")
	}
	users, err := source.AllEventUsers(ctx, eventID)
	if err != nil {
		return outcome, fmt.Errorf("read 01Edu enrollments: %w", err)
	}
	if err := ValidateEnrollments(users); err != nil {
		return outcome, err
	}
	outcome.SourceRows = len(users)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return outcome, err
	}
	defer tx.Rollback()
	var pilotCount int
	err = tx.QueryRowContext(ctx, db.Rebind(`SELECT COUNT(*) FROM student_events WHERE event_id = ? AND cohort_id = ?`), eventID, cohortID).Scan(&pilotCount)
	if err != nil {
		return outcome, err
	}
	if pilotCount == 0 {
		return outcome, errors.New("pilot roster not seeded; refusing to infer cohort membership from event-wide feed")
	}
	// A full successful source response is required; partial validation failures
	// abort the transaction without changing existing data.
	statement, err := tx.PrepareContext(ctx, db.Rebind(`UPDATE student_events SET current_level = ?, last_synced_at = ?
 WHERE event_id = ? AND cohort_id = ? AND student_id IN
 (SELECT id FROM students WHERE source_user_id = ?)`))
	if err != nil {
		return outcome, err
	}
	defer statement.Close()
	now := time.Now().UTC()
	for _, u := range users {
		res, err := statement.ExecContext(ctx, u.Level, store.FormatTime(now), eventID, cohortID, u.UserID)
		if err != nil {
			return LiveResult{}, fmt.Errorf("apply enrollment refresh: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return LiveResult{}, err
		}
		outcome.UpdatedRows += int(n)
		if n == 0 {
			outcome.UnmatchedRows++
		}
	}
	if outcome.UpdatedRows == 0 {
		return LiveResult{}, errors.New("no matching pilot students; refusing empty live refresh")
	}
	// Store sync history in the same transaction as the level refresh.
	_, err = tx.ExecContext(ctx, db.Rebind(`INSERT INTO sync_runs (source,status,records_ingested,started_at,completed_at) VALUES (?,?,?,?,?)`), "graphql_pilot_levels", "SUCCESS", outcome.UpdatedRows, now, now)
	if err != nil {
		return LiveResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return LiveResult{}, err
	}
	return outcome, nil
}

func ValidateEnrollments(users []zeroone.EventUser) error {
	if len(users) == 0 {
		return errors.New("empty 01Edu enrollment response; refusing sync")
	}
	seen := make(map[int64]bool, len(users))
	for _, u := range users {
		if u.ID <= 0 || u.UserID <= 0 || u.CreatedAt.IsZero() || math.IsInf(u.Level, 0) || math.IsNaN(u.Level) || u.Level < 0 {
			return errors.New("invalid 01Edu enrollment data; refusing sync")
		}
		if seen[u.UserID] {
			return errors.New("duplicate source user in enrollment response; refusing sync")
		}
		seen[u.UserID] = true
	}
	return nil
}
