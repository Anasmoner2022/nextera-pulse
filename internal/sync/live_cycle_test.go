package sync

import (
	"context"
	"errors"
	"fmt"
	"nextera-pulse/internal/store/pgtest"
	"strings"
	"testing"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

type fakeLive struct {
	users        []zeroone.EventUser
	records      []zeroone.LiveRecord
	progress     []zeroone.LiveProgress
	recordsErr   error
	labels       []zeroone.LiveLabel
	members      []zeroone.LiveLabelUser
	profiles     []zeroone.LiveUser
	staff        []zeroone.LiveUserRole
	labelErr     error
	usedCatalog  *bool
	eventProgess []zeroone.LiveProgress
}

func (f fakeLive) AllEventUsers(context.Context, int) ([]zeroone.EventUser, error) {
	return f.users, nil
}
func (f fakeLive) CohortLabels(context.Context, []string) ([]zeroone.LiveLabel, error) {
	return f.labels, f.labelErr
}
func (f fakeLive) LabelUsers(context.Context, []int64) ([]zeroone.LiveLabelUser, error) {
	return f.members, nil
}
func (f fakeLive) UsersByID(_ context.Context, ids []int64) ([]zeroone.LiveUser, error) {
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []zeroone.LiveUser
	for _, p := range f.profiles {
		if want[p.ID] {
			out = append(out, p)
		}
	}
	return out, nil
}
func (f fakeLive) StaffRoles(context.Context, []string) ([]zeroone.LiveUserRole, error) {
	return f.staff, nil
}
func (f fakeLive) CohortProgress(context.Context, []string, []string) ([]zeroone.LiveProgress, error) {
	if f.usedCatalog != nil {
		*f.usedCatalog = true
	}
	return f.progress, nil
}
func (f fakeLive) EventProjectProgress(context.Context, int, []string) ([]zeroone.LiveProgress, error) {
	return f.eventProgess, nil
}
func (f fakeLive) RecordsForUsers(context.Context, []int64) ([]zeroone.LiveRecord, error) {
	return f.records, f.recordsErr
}

var pilotConfig = LiveCycleConfig{EventID: 97, Cohorts: []CohortLabel{{ID: 4, Label: "01 Coding - Second Cohort"}, {ID: 1, Label: "01 Coding - First Cohort"}}}

func seededDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at) VALUES
		(10, 'alpha', 'Alpha', 'One', 'a@example.com', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(11, 'beta', 'Beta', 'Two', 'b@example.com', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	enrolled := time.Now().UTC().AddDate(0, 0, -200).Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at) VALUES
		(1, 97, 4, ?, 1, 0, ?), (2, 97, 4, ?, 1, 0, ?)`), enrolled, enrolled, enrolled, enrolled); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO learning_objects (source_object_id, name, type, path, author_login) VALUES (3, 'ascii-art', 'project', '/cairo/module/ascii-art', '')`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRunLiveCycleAppliesEveryStageAndRebuildsSnapshots(t *testing.T) {
	db := seededDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	grade := 1.2
	source := fakeLive{
		users: []zeroone.EventUser{
			{ID: 1, UserID: 10, UserLogin: "alpha", CreatedAt: now, Level: 30},
			{ID: 2, UserID: 11, UserLogin: "beta", CreatedAt: now, Level: 2},
			{ID: 3, UserID: 999, UserLogin: "stranger", CreatedAt: now, Level: 50},
		},
		records: []zeroone.LiveRecord{
			{ID: 77, UserID: 11, TypeName: "blocked-long", StartAt: now.AddDate(0, 0, -3).Format(time.RFC3339)},
			{ID: 78, UserID: 999, TypeName: "ban", StartAt: now.Format(time.RFC3339)},
		},
		progress: []zeroone.LiveProgress{
			{ID: 5, Path: "/cairo/module/ascii-art", IsDone: true, Grade: &grade, User: &struct {
				Login string `json:"login"`
			}{Login: "alpha"}},
			{ID: 6, Path: "/cairo/module/ascii-art", IsDone: false, User: &struct {
				Login string `json:"login"`
			}{Login: "beta"}},
		},
	}
	report, err := RunLiveCycle(ctx, db, source, pilotConfig)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != CycleSuccess || !report.CohortWide {
		t.Fatalf("report %+v", report)
	}
	if report.Levels.Applied != 2 || report.Records.Applied != 1 || report.Progress.Applied != 2 || report.Snapshots.Applied != 2 {
		t.Fatalf("stages %+v", report)
	}
	var status string
	var level float64
	if err := db.QueryRowContext(ctx, `SELECT pace_status, actual_level FROM student_metric_snapshots WHERE student_id = 1`).Scan(&status, &level); err != nil {
		t.Fatal(err)
	}
	if status != "GREEN" || level != 30 {
		t.Fatalf("alpha snapshot %s %v", status, level)
	}
	var calcVersion string
	if err := db.QueryRowContext(ctx, `SELECT calculation_version FROM student_metric_snapshots WHERE student_id = 1`).Scan(&calcVersion); err != nil || calcVersion != "v0.2+timeline:draft-v0(draft)" {
		t.Fatalf("calculation_version = %q (%v)", calcVersion, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT pace_status FROM student_metric_snapshots WHERE student_id = 2`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "PAUSED" {
		t.Fatalf("beta should be paused by the live blocked-long record, got %s", status)
	}
	var runs int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_runs WHERE source = 'graphql_live_cycle' AND status = 'SUCCESS'`).Scan(&runs)
	if runs != 1 {
		t.Fatalf("sync_runs = %d", runs)
	}
}

func TestRunLiveCycleKeepsGoingWhenOneStageFails(t *testing.T) {
	db := seededDB(t)
	now := time.Now().UTC()
	source := fakeLive{
		users: []zeroone.EventUser{
			{ID: 1, UserID: 10, CreatedAt: now, Level: 3},
			{ID: 2, UserID: 11, CreatedAt: now, Level: 4},
		},
		recordsErr: errors.New("graphql returned 1 error(s)"),
	}
	report, err := RunLiveCycle(context.Background(), db, source, pilotConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Levels.OK || report.Records.OK || report.Records.Error == "" || report.Status != CycleSuccess {
		t.Fatalf("report %+v", report)
	}
}

func TestRunLiveCycleMarksOwnerOnlyTokenAsLimited(t *testing.T) {
	db := seededDB(t)
	source := fakeLive{users: []zeroone.EventUser{{ID: 1, UserID: 10, CreatedAt: time.Now().UTC(), Level: 3}}}
	report, err := RunLiveCycle(context.Background(), db, source, pilotConfig)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != CycleLimited || report.CohortWide {
		t.Fatalf("report %+v", report)
	}
}

func TestRunLiveCycleRefusesAnEmptyRoster(t *testing.T) {
	db, err := store.Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	source := fakeLive{
		users:    []zeroone.EventUser{{ID: 1, UserID: 10, CreatedAt: now, Level: 3}},
		labelErr: errors.New("graphql returned 1 error(s)"),
	}
	report, err := RunLiveCycle(context.Background(), db, source, pilotConfig)
	if err == nil {
		t.Fatal("expected an error for an empty roster")
	}
	if report.Roster.OK || report.Roster.Error == "" {
		t.Fatalf("roster %+v", report.Roster)
	}
}

func TestRunLiveCycleBuildsTheRosterFrom01EduWithoutAnyImport(t *testing.T) {
	db, err := store.Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// A staff account that 01Edu no longer lists as admin must lose its role.
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, source_role) VALUES (500, 'old.admin', 'admin')`); err != nil {
		t.Fatal(err)
	}
	enrolled := time.Now().UTC().AddDate(0, 0, -200)
	event97 := int64(97)
	usedCatalog := false
	source := fakeLive{
		users: []zeroone.EventUser{
			{ID: 1, UserID: 10, CreatedAt: enrolled, Level: 30},
			{ID: 2, UserID: 11, CreatedAt: enrolled, Level: 2},
			{ID: 3, UserID: 12, CreatedAt: enrolled, Level: 5},
			{ID: 4, UserID: 99, CreatedAt: enrolled, Level: 50}, // enrolled, but in no cohort label
		},
		labels: []zeroone.LiveLabel{
			{ID: 41, Name: "01 Coding - Second Cohort", EventID: &event97},
			{ID: 11, Name: "01 Coding - First Cohort"},
		},
		members: []zeroone.LiveLabelUser{
			{ID: 1, LabelID: 41, UserID: 10},
			{ID: 2, LabelID: 41, UserID: 11},
			{ID: 3, LabelID: 11, UserID: 12},
			{ID: 4, LabelID: 11, UserID: 10}, // in both: the first configured cohort wins
			{ID: 5, LabelID: 41, UserID: 13}, // labelled but not enrolled in event 97
		},
		profiles: []zeroone.LiveUser{
			{ID: 10, Login: "alpha", FirstName: "Alpha", LastName: "One", Email: "a@example.com"},
			{ID: 11, Login: "beta", FirstName: "Beta"},
			{ID: 12, Login: "gamma"},
			{ID: 13, Login: "delta"},
			{ID: 200, Login: "mentor.one", FirstName: "Mentor", LastName: "One"},
			{ID: 201, Login: "manager.one", FirstName: "Manager"},
		},
		staff: []zeroone.LiveUserRole{
			{ID: 200, Login: "mentor.one", Role: "admin campus"},
			{ID: 201, Login: "manager.one", Role: "admin"},
		},
		usedCatalog: &usedCatalog,
		eventProgess: []zeroone.LiveProgress{
			{ID: 5, UserID: 10, ObjectID: 3, Path: "/cairo/module/ascii-art", IsDone: true, Object: &zeroone.LiveObject{ID: 3, Name: "ascii-art", Type: "project", Path: "/cairo/module/ascii-art"}, User: &struct {
				Login string `json:"login"`
			}{Login: "alpha"}},
		},
	}
	report, err := RunLiveCycle(ctx, db, source, pilotConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Roster.OK || report.Roster.Applied != 3 || report.Status != CycleSuccess {
		t.Fatalf("report %+v", report)
	}
	if usedCatalog || !report.Progress.OK || report.Progress.Applied != 1 {
		t.Fatalf("an empty catalog must use the event-scoped progress query: %+v", report.Progress)
	}
	// The sync never assigns mentors, even with a campus mentor on the roster:
	// unassigned students wait for a program manager.
	for cycle := 1; cycle <= 2; cycle++ {
		if cycle == 2 {
			if _, err := RunLiveCycle(ctx, db, source, pilotConfig); err != nil {
				t.Fatal(err)
			}
		}
		var assignments int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mentor_assignments`).Scan(&assignments); err != nil || assignments != 0 {
			t.Fatalf("after cycle %d: %d mentor assignments (%v), want none", cycle, assignments, err)
		}
	}
	rows, err := db.QueryContext(ctx, `SELECT s.login, se.cohort_id, se.current_level, s.source_role
		FROM students s JOIN student_events se ON se.student_id = s.id ORDER BY s.login`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var login, role string
		var cohort int
		var level float64
		if err := rows.Scan(&login, &cohort, &level, &role); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s:%d:%.0f:%s", login, cohort, level, role))
	}
	want := "alpha:4:30:user beta:4:2:user gamma:1:5:user"
	if strings.Join(got, " ") != want {
		t.Fatalf("roster = %v, want %s", got, want)
	}
	roles := map[string]string{}
	roleRows, err := db.QueryContext(ctx, `SELECT login, source_role FROM students WHERE login IN ('mentor.one', 'manager.one', 'old.admin')`)
	if err != nil {
		t.Fatal(err)
	}
	defer roleRows.Close()
	for roleRows.Next() {
		var login, role string
		if err := roleRows.Scan(&login, &role); err != nil {
			t.Fatal(err)
		}
		roles[login] = role
	}
	if roles["mentor.one"] != "admin campus" || roles["manager.one"] != "admin" || roles["old.admin"] != "user" {
		t.Fatalf("staff roles = %v", roles)
	}
	var snapshots int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM student_metric_snapshots`).Scan(&snapshots); err != nil || snapshots != 3 {
		t.Fatalf("snapshots %d err %v", snapshots, err)
	}
}

func TestSnapshotsUseTheConfiguredEventAndItsActiveTimeline(t *testing.T) {
	db := seededDB(t) // students 1 and 2 are in event 97
	ctx := context.Background()
	const event = 120
	enrolled := time.Now().UTC().AddDate(0, 0, -150).Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, source_role) VALUES (12, 'gamma', 'user')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at)
		VALUES (3, $1, 4, $2, 8, 0, $2)`, event, enrolled); err != nil {
		t.Fatal(err)
	}
	var versionID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO curriculum_timeline_versions (event_id, label, status) VALUES ($1, 'product-2026-11', 'approved') RETURNING id`, event).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO curriculum_timeline (event_id, version_id, month, min_level, expected_level) VALUES ($1, $2, 4, 6, 8)`, event, versionID); err != nil {
		t.Fatal(err)
	}
	n, err := RefreshSnapshots(ctx, db, event, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("snapshots = %d (%v), want only the event %d student", n, err, event)
	}
	var eventID int
	var calcVersion, status string
	var minLevel float64
	if err := db.QueryRowContext(ctx, `SELECT event_id, calculation_version, minimum_level, pace_status FROM student_metric_snapshots WHERE student_id = 3`).Scan(&eventID, &calcVersion, &minLevel, &status); err != nil {
		t.Fatal(err)
	}
	if eventID != event || calcVersion != "v0.2+timeline:product-2026-11(approved)" || minLevel != 6 {
		t.Fatalf("snapshot event=%d version=%q min=%v status=%s", eventID, calcVersion, minLevel, status)
	}
	var other int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM student_metric_snapshots WHERE event_id <> $1`, event).Scan(&other)
	if other != 0 {
		t.Fatalf("%d snapshots written for another event", other)
	}
}
