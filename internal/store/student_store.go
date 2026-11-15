package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/pace"
)

// Rebind converts '?' placeholders to PostgreSQL's $1, $2, ...
// SQLite keeps the '?' markers.
func (db *DB) Rebind(query string) string {
	if db != nil && db.DriverName == "sqlite" {
		return query
	}
	var b strings.Builder
	paramCount := 1
	for _, r := range query {
		if r == '?' {
			b.WriteString("$" + strconv.Itoa(paramCount))
			paramCount++
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// UpsertStudent inserts or updates student record
func (db *DB) UpsertStudent(ctx context.Context, s model.Student) (int64, error) {
	query := `INSERT INTO students (source_user_id, login, first_name, last_name, email, created_at, updated_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT (source_user_id) DO UPDATE SET
                login = excluded.login,
                first_name = excluded.first_name,
                last_name = excluded.last_name,
                email = excluded.email,
                updated_at = excluded.updated_at`

	query += " RETURNING id"
	var id int64
	err := db.QueryRowContext(ctx, db.Rebind(query), s.SourceUserID, s.Login, s.FirstName, s.LastName, s.Email, s.CreatedAt, s.UpdatedAt).Scan(&id)
	return id, err
}

// UpsertStudentEvent inserts or updates student enrollment & current level in an event
func (db *DB) UpsertStudentEvent(ctx context.Context, se model.StudentEvent) error {
	query := `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT (student_id, event_id) DO UPDATE SET
                cohort_id = excluded.cohort_id,
                current_level = excluded.current_level,
                audit_ratio = excluded.audit_ratio,
                last_synced_at = excluded.last_synced_at;`

	q := db.Rebind(query)
	_, err := db.ExecContext(ctx, q, se.StudentID, se.EventID, se.CohortID, NullableTime(se.EnrolledAt), se.CurrentLevel, se.AuditRatio, NullableTime(se.LastSyncedAt))
	return err
}

// UpsertStudentRecord records status intervals
func (db *DB) UpsertStudentRecord(ctx context.Context, r model.StudentRecord) error {
	query := `INSERT INTO student_source_records (source_record_id, student_id, type_name, start_at, end_at, message, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT (source_record_id) DO UPDATE SET
                type_name = excluded.type_name,
                start_at = excluded.start_at,
                end_at = excluded.end_at,
                message = excluded.message;`

	var endAtStr any
	if r.EndAt != nil {
		endAtStr = NullableTime(*r.EndAt)
	}

	q := db.Rebind(query)
	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := db.ExecContext(ctx, q, r.SourceRecordID, r.StudentID, r.TypeName, NullableTime(r.StartAt), endAtStr, r.Message, createdAt.UTC())
	return err
}

// GetStatusPolicies loads all status policies
func (db *DB) GetStatusPolicies(ctx context.Context) (map[string]model.StatusPolicy, error) {
	rows, err := db.QueryContext(ctx, "SELECT record_type, pause_clock, exclude_from_active_kpis, description FROM status_policy;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make(map[string]model.StatusPolicy)
	for rows.Next() {
		var p model.StatusPolicy
		if err := rows.Scan(&p.RecordType, &p.PauseClock, &p.ExcludeFromActiveKPIs, &p.Description); err != nil {
			return nil, err
		}
		policies[p.RecordType] = p
	}
	return policies, nil
}

// SaveMetricSnapshot stores an immutable daily metric snapshot
func (db *DB) SaveMetricSnapshot(ctx context.Context, snap model.StudentMetricSnapshot) error {
	reasonsJSON, _ := json.Marshal(snap.Pace.Reasons)

	query := `INSERT INTO student_metric_snapshots (
                student_id, event_id, snapshot_date, enrolled_at, active_program_days,
                curriculum_month, actual_level, minimum_level, expected_level,
                checkpoint_target, gap_to_minimum, gap_to_expected, pace_status,
                health_reasons, calculation_version, calculated_at
              ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT (student_id, event_id, snapshot_date) DO UPDATE SET
                active_program_days = excluded.active_program_days,
                curriculum_month = excluded.curriculum_month,
                actual_level = excluded.actual_level,
                minimum_level = excluded.minimum_level,
                expected_level = excluded.expected_level,
                checkpoint_target = excluded.checkpoint_target,
                gap_to_minimum = excluded.gap_to_minimum,
                gap_to_expected = excluded.gap_to_expected,
                pace_status = excluded.pace_status,
                health_reasons = excluded.health_reasons,
                calculated_at = excluded.calculated_at;`

	q := db.Rebind(query)
	_, err := db.ExecContext(ctx, q,
		snap.StudentID, snap.EventID, snap.SnapshotDate, FormatTime(snap.Pace.EnrolledAt), snap.Pace.ActiveProgramDays,
		snap.Pace.CurriculumMonth, snap.Pace.ActualLevel, snap.Pace.MinimumLevel, snap.Pace.ExpectedLevel,
		snap.Pace.CheckpointTarget, snap.Pace.GapToMinimum, snap.Pace.GapToExpected, snap.Pace.Status,
		string(reasonsJSON), snap.CalculationVersion, FormatTime(snap.CalculatedAt),
	)
	return err
}

// GetStudentRecords loads records for a student
func (db *DB) GetStudentRecords(ctx context.Context, studentID int64) ([]model.StudentRecord, error) {
	query := db.Rebind("SELECT id, source_record_id, student_id, type_name, start_at, end_at, COALESCE(message, '') FROM student_source_records WHERE student_id = ? ORDER BY start_at ASC;")
	rows, err := db.QueryContext(ctx, query, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.StudentRecord
	for rows.Next() {
		var r model.StudentRecord
		var startAt, endAt FlexibleTime
		if err := rows.Scan(&r.ID, &r.SourceRecordID, &r.StudentID, &r.TypeName, &startAt, &endAt, &r.Message); err != nil {
			return nil, err
		}
		r.StartAt = startAt.Time
		if !endAt.Time.IsZero() {
			t := endAt.Time
			r.EndAt = &t
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// ListStudentsWithPace returns the students of a curriculum event (eventID 0:
// any event) with their latest metric snapshot. CohortName is left for the
// caller, which knows the configured cohort names.
func (db *DB) ListStudentsWithPace(ctx context.Context, eventID, cohortID int, statusFilter string, search string) ([]model.StudentPaceView, error) {
	baseQuery := `
        SELECT 
            s.id, s.source_user_id, s.login, COALESCE(s.first_name, ''), COALESCE(s.last_name, ''), COALESCE(s.email, ''),
            se.cohort_id, se.audit_ratio,
            COALESCE(m.mentor_id, 0), COALESCE(m.mentor_name, ''),
            COALESCE(snap.enrolled_at, se.enrolled_at),
            COALESCE(snap.active_program_days, 0),
            COALESCE(snap.curriculum_month, 1),
            COALESCE(snap.actual_level, se.current_level),
            COALESCE(snap.minimum_level, 0),
            COALESCE(snap.expected_level, 0),
            COALESCE(snap.checkpoint_target, 0),
            COALESCE(snap.gap_to_minimum, 0),
            COALESCE(snap.gap_to_expected, 0),
            COALESCE(snap.pace_status, 'UNKNOWN'),
            COALESCE(snap.health_reasons, '[]'),
            COALESCE(snap.calculated_at, se.last_synced_at),
            COALESCE((
                SELECT r.type_name FROM student_source_records r
                WHERE r.student_id = s.id AND r.end_at IS NULL
                ORDER BY r.start_at DESC LIMIT 1
            ), 'active')
        FROM students s
        JOIN student_events se ON s.id = se.student_id
        LEFT JOIN mentor_assignments m ON s.id = m.student_id AND m.is_primary = TRUE AND m.ends_at IS NULL
        LEFT JOIN (
            SELECT sms.*
            FROM student_metric_snapshots sms
            INNER JOIN (
                SELECT student_id, event_id, MAX(snapshot_date) as max_date
                FROM student_metric_snapshots
                GROUP BY student_id, event_id
            ) latest ON sms.student_id = latest.student_id
                AND sms.event_id = latest.event_id
                AND sms.snapshot_date = latest.max_date
        ) snap ON s.id = snap.student_id AND snap.event_id = se.event_id
        WHERE 1=1
    `

	var args []any
	if eventID > 0 {
		baseQuery += " AND se.event_id = ?"
		args = append(args, eventID)
	}
	if cohortID > 0 {
		baseQuery += " AND se.cohort_id = ?"
		args = append(args, cohortID)
	}

	if statusFilter != "" {
		baseQuery += " AND snap.pace_status = ?"
		args = append(args, statusFilter)
	}

	if search != "" {
		baseQuery += ` AND (s.login ILIKE ? ESCAPE '\' OR s.first_name ILIKE ? ESCAPE '\' OR s.last_name ILIKE ? ESCAPE '\' OR s.email ILIKE ? ESCAPE '\')`
		searchTerm := "%" + escapeLike(search) + "%"
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
	}

	// Ordering: RED first, then YELLOW, GREEN, PAUSED, others
	baseQuery += ` ORDER BY 
        CASE snap.pace_status
            WHEN 'RED' THEN 1
            WHEN 'YELLOW' THEN 2
            WHEN 'GREEN' THEN 3
            WHEN 'PAUSED' THEN 4
            ELSE 5
        END,
        snap.gap_to_minimum ASC,
        s.login ASC;`

	q := db.Rebind(baseQuery)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed query students: %w", err)
	}
	defer rows.Close()

	var views []model.StudentPaceView
	for rows.Next() {
		var v model.StudentPaceView
		var mentorID int64
		var mentorName string
		var enrolledAt, calculatedAt FlexibleTime
		var reasonsJSON string

		err := rows.Scan(
			&v.ID, &v.SourceUserID, &v.Login, &v.FirstName, &v.LastName, &v.Email,
			&v.CohortID, &v.AuditRatio,
			&mentorID, &mentorName,
			&enrolledAt,
			&v.Pace.ActiveProgramDays,
			&v.Pace.CurriculumMonth,
			&v.Pace.ActualLevel,
			&v.Pace.MinimumLevel,
			&v.Pace.ExpectedLevel,
			&v.Pace.CheckpointTarget,
			&v.Pace.GapToMinimum,
			&v.Pace.GapToExpected,
			&v.Pace.Status,
			&reasonsJSON,
			&calculatedAt,
			&v.SourceStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning student pace row: %w", err)
		}

		if math.IsInf(v.AuditRatio, 0) || math.IsNaN(v.AuditRatio) {
			v.AuditRatio = 0.0
		}

		if mentorID > 0 {
			v.Mentor = &model.Mentor{ID: mentorID, Name: mentorName}
		}

		v.Pace.EnrolledAt = enrolledAt.Time
		v.LastCalculatedAt = calculatedAt.Time
		_ = json.Unmarshal([]byte(reasonsJSON), &v.Pace.Reasons)

		views = append(views, v)
	}

	return views, rows.Err()
}

// GetCohortHealthSummary computes the North Star KPI (GREEN / measurable)
// and status counts for a cohort of a curriculum event, with the same
// formula as OnTrackKPIs. When only is not nil the summary covers just
// those students (a mentor's scope). CohortName is left for the caller.
func (db *DB) GetCohortHealthSummary(ctx context.Context, eventID, cohortID int, only map[int64]bool) (*model.CohortHealthSummary, error) {
	students, err := db.ListStudentsWithPace(ctx, eventID, cohortID, "", "")
	if err != nil {
		return nil, err
	}
	if only != nil {
		visible := students[:0]
		for _, s := range students {
			if only[s.ID] {
				visible = append(visible, s)
			}
		}
		students = visible
	}
	excluded, err := db.KPIExcludedSources(ctx)
	if err != nil {
		return nil, err
	}
	kpi := pace.OnTrack(kpiStudents(students), excluded)
	kpi.Scope, kpi.EventID, kpi.CohortID = "cohort", eventID, cohortID
	return &model.CohortHealthSummary{
		CohortID:           cohortID,
		TotalStudents:      kpi.Total,
		MeasurableStudents: kpi.Measurable,
		OnTrackCount:       kpi.MeasurableCounts["GREEN"],
		NearTrackCount:     kpi.MeasurableCounts["YELLOW"],
		OffTrackCount:      kpi.MeasurableCounts["RED"],
		OnTrackRate:        kpi.Rate,
		StatusCounts:       kpi.StatusCounts,
		KPI:                kpi,
		CalculatedAt:       time.Now(),
	}, nil
}

// KPIExcludedSources reads the source statuses (01Edu record types) that
// status_policy flags exclude_from_active_kpis.
func (db *DB) KPIExcludedSources(ctx context.Context) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT record_type FROM status_policy WHERE exclude_from_active_kpis`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	excluded := map[string]bool{}
	for rows.Next() {
		var recordType string
		if err := rows.Scan(&recordType); err != nil {
			return nil, err
		}
		excluded[recordType] = true
	}
	return excluded, rows.Err()
}

// OnTrackReport is the on-track KPI at every level a dashboard shows.
type OnTrackReport struct {
	Program model.OnTrackKPI
	Cohorts map[int]model.OnTrackKPI
	// Mentors is keyed by the mentor's person id and covers each mentor's
	// current students only.
	Mentors map[int64]model.OnTrackKPI
}

// OnTrackKPIs computes the KPI program-wide (every student of the event), per
// cohort and per current mentor, all with pace.OnTrack.
func (db *DB) OnTrackKPIs(ctx context.Context, eventID int) (OnTrackReport, error) {
	report := OnTrackReport{Cohorts: map[int]model.OnTrackKPI{}, Mentors: map[int64]model.OnTrackKPI{}}
	students, err := db.ListStudentsWithPace(ctx, eventID, 0, "", "")
	if err != nil {
		return report, err
	}
	excluded, err := db.KPIExcludedSources(ctx)
	if err != nil {
		return report, err
	}
	byCohort := map[int][]model.StudentPaceView{}
	byMentor := map[int64][]model.StudentPaceView{}
	for _, s := range students {
		byCohort[s.CohortID] = append(byCohort[s.CohortID], s)
		if s.Mentor != nil && s.Mentor.ID > 0 {
			byMentor[s.Mentor.ID] = append(byMentor[s.Mentor.ID], s)
		}
	}
	report.Program = pace.OnTrack(kpiStudents(students), excluded)
	report.Program.Scope, report.Program.EventID = "program", eventID
	for cohortID, rows := range byCohort {
		kpi := pace.OnTrack(kpiStudents(rows), excluded)
		kpi.Scope, kpi.EventID, kpi.CohortID = "cohort", eventID, cohortID
		report.Cohorts[cohortID] = kpi
	}
	for mentorID, rows := range byMentor {
		kpi := pace.OnTrack(kpiStudents(rows), excluded)
		kpi.Scope, kpi.EventID, kpi.MentorID, kpi.Name = "mentor", eventID, mentorID, rows[0].Mentor.Name
		report.Mentors[mentorID] = kpi
	}
	return report, nil
}

func kpiStudents(students []model.StudentPaceView) []pace.KPIStudent {
	out := make([]pace.KPIStudent, len(students))
	for i, s := range students {
		out[i] = pace.KPIStudent{PaceStatus: s.Pace.Status, SourceStatus: s.SourceStatus}
	}
	return out
}

// escapeLike makes %, _ and \ in user search text match literally.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}
