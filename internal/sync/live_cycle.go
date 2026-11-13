package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

// LiveSource is the read-only 01Edu surface a live sync cycle needs.
// *zeroone.Client implements it.
type LiveSource interface {
	AllEventUsers(ctx context.Context, eventID int) ([]zeroone.EventUser, error)
	CohortLabels(ctx context.Context, names []string) ([]zeroone.LiveLabel, error)
	LabelUsers(ctx context.Context, labelIDs []int64) ([]zeroone.LiveLabelUser, error)
	UsersByID(ctx context.Context, ids []int64) ([]zeroone.LiveUser, error)
	StaffRoles(ctx context.Context, roles []string) ([]zeroone.LiveUserRole, error)
	CohortProgress(ctx context.Context, logins, paths []string) ([]zeroone.LiveProgress, error)
	EventProjectProgress(ctx context.Context, eventID int, logins []string) ([]zeroone.LiveProgress, error)
	RecordsForUsers(ctx context.Context, userIDs []int64) ([]zeroone.LiveRecord, error)
}

// CohortLabel maps a Pulse cohort id to the 01Edu label that defines it.
type CohortLabel struct {
	ID    int
	Label string
}

// LiveCycleConfig scopes one live sync cycle.
type LiveCycleConfig struct {
	EventID int
	// Cohorts in priority order: a user in two labels joins the first one.
	Cohorts []CohortLabel
	// SnapshotLockKey overrides store.LockSnapshots (tests only).
	SnapshotLockKey int64
}

// CohortIDs lists the configured Pulse cohort ids.
func (c LiveCycleConfig) CohortIDs() []int {
	ids := make([]int, 0, len(c.Cohorts))
	for _, cohort := range c.Cohorts {
		ids = append(ids, cohort.ID)
	}
	return ids
}

// StaffSourceRoles are the 01Edu user.role values that map to Pulse staff.
var StaffSourceRoles = []string{"admin", "admin campus"}

// Live cycle outcomes stored in sync_runs.status.
const (
	CycleSuccess = "SUCCESS" // at least one stage returned cohort-wide data
	CycleLimited = "LIMITED" // the credential only returned its owner's rows
	CycleFailed  = "FAILED"  // every stage failed
	CycleSkipped = "SKIPPED" // another instance held the live sync lock (not stored in sync_runs)
)

// StageReport describes one stage of a live cycle. Error is a short message
// with no student data or credentials.
type StageReport struct {
	OK       bool   `json:"ok"`
	Rows     int    `json:"rows"`
	Applied  int    `json:"applied"`
	Students int    `json:"students"`
	Error    string `json:"error,omitempty"`
}

// LiveCycleReport is the result of one live sync cycle.
type LiveCycleReport struct {
	Status     string      `json:"status"`
	StartedAt  time.Time   `json:"startedAt"`
	FinishedAt time.Time   `json:"finishedAt"`
	CohortWide bool        `json:"cohortWide"`
	Roster     StageReport `json:"roster"`
	Levels     StageReport `json:"levels"`
	Records    StageReport `json:"records"`
	Progress   StageReport `json:"progress"`
	Snapshots  StageReport `json:"snapshots"`
}

// RunLiveCycle makes Pulse follow 01Edu. It reads the cohort roster (label,
// label_user, user, event_user), levels (event_user), administrative records
// (record) and project progress (progress), then rebuilds the pace snapshots
// the dashboard reads. 01Edu is the only data source; there is no file import.
//
// Each stage is independent: one rejected query does not discard the others.
// Cohort membership comes only from the configured 01Edu labels, never from
// the event-wide event_user feed.
func RunLiveCycle(ctx context.Context, db *store.DB, source LiveSource, cfg LiveCycleConfig) (LiveCycleReport, error) {
	report := LiveCycleReport{StartedAt: time.Now().UTC(), Status: CycleFailed}
	if db == nil || source == nil || cfg.EventID <= 0 || len(cfg.Cohorts) == 0 {
		return report, errors.New("invalid live-sync configuration")
	}
	wide := map[string]bool{}

	// 1. Enrollments (event_user): enrollment date and level per 01Edu user.
	users, usersErr := source.AllEventUsers(ctx, cfg.EventID)
	if usersErr == nil {
		usersErr = ValidateEnrollments(users)
	} else {
		usersErr = errors.New(stageError(source, "event_user", usersErr))
	}

	// 2. Roster: who is in each cohort label, and who is staff.
	if usersErr != nil {
		report.Roster.Error = "needs event_user enrollments: " + usersErr.Error()
	} else {
		report.Roster = syncRoster(ctx, db, source, cfg, users)
		wide["roster"] = report.Roster.Students > 1
	}

	roster, err := loadRoster(ctx, db, cfg)
	if err != nil {
		return report, fmt.Errorf("read Pulse roster: %w", err)
	}
	if len(roster) == 0 {
		report.FinishedAt = time.Now().UTC()
		recordCycle(ctx, db, report)
		reason := report.Roster.Error
		if reason == "" {
			reason = "no learners found in the configured 01Edu cohort labels (check PILOT_COHORTS)"
		}
		return report, errors.New("Pulse roster is empty: " + reason)
	}

	// 3. Levels from event_user.
	if usersErr != nil {
		report.Levels.Error = usersErr.Error()
	} else {
		report.Levels.Rows = len(users)
		applied, err := applyLevels(ctx, db, users, cfg)
		if err != nil {
			report.Levels.Error = "apply levels: " + err.Error()
		} else {
			report.Levels.OK = true
			report.Levels.Applied = applied
			report.Levels.Students = applied
			wide["levels"] = len(users) > 1
		}
	}

	// 4. Administrative records (pause / left-school status).
	userIDs := make([]int64, 0, len(roster))
	byUser := map[int64]int64{}
	for _, person := range roster {
		if person.sourceUserID > 0 {
			userIDs = append(userIDs, person.sourceUserID)
			byUser[person.sourceUserID] = person.studentID
		}
	}
	if records, err := source.RecordsForUsers(ctx, userIDs); err != nil {
		report.Records.Error = stageError(source, "record", err)
	} else {
		report.Records.Rows = len(records)
		applied, students, err := applyRecords(ctx, db, records, byUser)
		if err != nil {
			report.Records.Error = "apply records: " + err.Error()
		} else {
			report.Records.OK = true
			report.Records.Applied = applied
			report.Records.Students = students
			wide["records"] = students > 1
		}
	}

	// 5. Project progress. With no project catalog yet (fresh database) the
	// event-scoped query fills the catalog from the progress rows themselves.
	logins := make([]string, 0, len(roster))
	for _, person := range roster {
		if person.login != "" {
			logins = append(logins, person.login)
		}
	}
	paths, err := ProjectPaths(ctx, db)
	if err != nil {
		report.Progress.Error = "read project catalog: " + err.Error()
	} else {
		var rows []zeroone.LiveProgress
		if len(paths) == 0 {
			rows, err = source.EventProjectProgress(ctx, cfg.EventID, logins)
		} else {
			rows, err = source.CohortProgress(ctx, logins, paths)
		}
		if err != nil {
			report.Progress.Error = stageError(source, "progress", err)
		} else {
			report.Progress.Rows = len(rows)
			applied, err := ApplyLiveProgress(ctx, db, rows)
			if err != nil {
				report.Progress.Error = "apply progress: " + err.Error()
			} else {
				report.Progress.OK = true
				report.Progress.Applied = applied
				report.Progress.Students = distinctLogins(rows)
				wide["progress"] = report.Progress.Students > 1
			}
		}
	}

	// 6. Rebuild pace snapshots (the dashboard reads snapshots, not raw levels)
	// with the same function as the daily snapshot job. It waits for the
	// snapshot lock if that job is running, so fresh data is never skipped.
	if report.Roster.OK || report.Levels.OK || report.Records.OK {
		n, err := RefreshSnapshotsLocked(ctx, db, cfg.EventID, time.Now(), true, cfg.SnapshotLockKey)
		if err != nil {
			report.Snapshots.Error = err.Error()
		} else {
			report.Snapshots.OK = true
			report.Snapshots.Applied = n
		}
	}

	for _, isWide := range wide {
		report.CohortWide = report.CohortWide || isWide
	}
	anyOK := report.Roster.OK || report.Levels.OK || report.Records.OK || report.Progress.OK
	switch {
	case anyOK && report.CohortWide:
		report.Status = CycleSuccess
	case anyOK:
		report.Status = CycleLimited
	default:
		report.Status = CycleFailed
	}
	report.FinishedAt = time.Now().UTC()
	recordCycle(ctx, db, report)
	if report.Status == CycleFailed {
		return report, errors.New(firstError(report))
	}
	return report, nil
}

type rosterEntry struct {
	studentID    int64
	sourceUserID int64
	login        string
}

func loadRoster(ctx context.Context, db *store.DB, cfg LiveCycleConfig) ([]rosterEntry, error) {
	placeholders := make([]string, len(cfg.Cohorts))
	args := []any{cfg.EventID}
	for i, id := range cfg.CohortIDs() {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := db.Rebind(`SELECT DISTINCT s.id, s.source_user_id, COALESCE(s.login, '')
		FROM students s JOIN student_events se ON se.student_id = s.id
		WHERE se.event_id = ? AND se.cohort_id IN (` + strings.Join(placeholders, ",") + `)`)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roster []rosterEntry
	for rows.Next() {
		var entry rosterEntry
		if err := rows.Scan(&entry.studentID, &entry.sourceUserID, &entry.login); err != nil {
			return nil, err
		}
		entry.login = strings.TrimSpace(entry.login)
		roster = append(roster, entry)
	}
	return roster, rows.Err()
}

func applyLevels(ctx context.Context, db *store.DB, users []zeroone.EventUser, cfg LiveCycleConfig) (int, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	placeholders := make([]string, len(cfg.Cohorts))
	for i := range cfg.Cohorts {
		placeholders[i] = "?"
	}
	statement, err := tx.PrepareContext(ctx, db.Rebind(`UPDATE student_events SET current_level = ?, last_synced_at = ?
		WHERE event_id = ? AND cohort_id IN (`+strings.Join(placeholders, ",")+`)
		AND student_id IN (SELECT id FROM students WHERE source_user_id = ?)`))
	if err != nil {
		return 0, err
	}
	defer statement.Close()
	now := store.FormatTime(time.Now().UTC())
	updated := 0
	for _, u := range users {
		args := []any{u.Level, now, cfg.EventID}
		for _, id := range cfg.CohortIDs() {
			args = append(args, id)
		}
		args = append(args, u.UserID)
		res, err := statement.ExecContext(ctx, args...)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		updated += int(n)
	}
	return updated, tx.Commit()
}

func applyRecords(ctx context.Context, db *store.DB, records []zeroone.LiveRecord, byUser map[int64]int64) (applied, students int, err error) {
	seen := map[int64]bool{}
	for _, r := range records {
		studentID, ok := byUser[r.UserID]
		if !ok || r.ID <= 0 || strings.TrimSpace(r.TypeName) == "" {
			continue
		}
		startAt := store.ParseFlexibleTime(r.StartAt)
		if startAt.IsZero() {
			startAt = store.ParseFlexibleTime(r.CreatedAt)
		}
		if startAt.IsZero() {
			continue
		}
		var endAt *time.Time
		if r.EndAt != nil {
			if t := store.ParseFlexibleTime(*r.EndAt); !t.IsZero() {
				endAt = &t
			}
		}
		rec := model.StudentRecord{
			SourceRecordID: r.ID,
			StudentID:      studentID,
			TypeName:       strings.TrimSpace(r.TypeName),
			StartAt:        startAt,
			EndAt:          endAt,
			Message:        r.Message,
			CreatedAt:      store.ParseFlexibleTime(r.CreatedAt),
		}
		if err := db.UpsertStudentRecord(ctx, rec); err != nil {
			return applied, len(seen), err
		}
		applied++
		seen[r.UserID] = true
	}
	return applied, len(seen), nil
}

// ProjectPaths lists every project path in the Pulse catalog plus the live
// /cairo/module/<slug> and archive root/01-edu_module/<slug>/ forms.
func ProjectPaths(ctx context.Context, db *store.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT path FROM learning_objects WHERE lower(type) = 'project' AND TRIM(path) <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		add(path)
		if slug := pathSlug(path); slug != "" {
			add("/cairo/module/" + slug)
			add("root/01-edu_module/" + slug + "/")
		}
	}
	return paths, rows.Err()
}

func distinctLogins(rows []zeroone.LiveProgress) int {
	seen := map[string]struct{}{}
	for _, row := range rows {
		if login := strings.ToLower(strings.TrimSpace(row.Login())); login != "" {
			seen[login] = struct{}{}
		}
	}
	return len(seen)
}

type schemaReporter interface{ SchemaIssue() string }

func stageError(source LiveSource, table string, err error) string {
	msg := table + ": " + err.Error()
	if reporter, ok := source.(schemaReporter); ok {
		if issue := reporter.SchemaIssue(); issue != "" {
			msg += " (" + issue + ")"
		}
	}
	return msg
}

func firstError(report LiveCycleReport) string {
	for _, stage := range []StageReport{report.Roster, report.Levels, report.Records, report.Progress} {
		if stage.Error != "" {
			return stage.Error
		}
	}
	return "no stage returned data"
}

func recordCycle(ctx context.Context, db *store.DB, report LiveCycleReport) {
	var message any
	if report.Status != CycleSuccess {
		message = firstError(report)
	}
	applied := report.Roster.Applied + report.Levels.Applied + report.Records.Applied + report.Progress.Applied
	_, _ = db.ExecContext(ctx, db.Rebind(`INSERT INTO sync_runs (source, status, records_ingested, started_at, completed_at, error_message) VALUES (?, ?, ?, ?, ?, ?)`),
		"graphql_live_cycle", report.Status, applied, store.FormatTime(report.StartedAt), store.FormatTime(report.FinishedAt), message)
}
