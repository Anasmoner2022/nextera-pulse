package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"nextera-pulse/internal/model"
)

// Rebind converts '?' placeholders to '$1, $2' if using Postgres
func (db *DB) Rebind(query string) string {
	if db.DriverName != "postgres" {
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

	if db.DriverName == "postgres" {
		query += " RETURNING id;"
		q := db.Rebind(query)
		var id int64
		err := db.QueryRowContext(ctx, q, s.SourceUserID, s.Login, s.FirstName, s.LastName, s.Email, s.CreatedAt, s.UpdatedAt).Scan(&id)
		return id, err
	}

	q := db.Rebind(query)
	_, err := db.ExecContext(ctx, q, s.SourceUserID, s.Login, s.FirstName, s.LastName, s.Email, FormatTime(s.CreatedAt), FormatTime(s.UpdatedAt))
	if err != nil {
		return 0, err
	}

	var id int64
	err = db.QueryRowContext(ctx, "SELECT id FROM students WHERE source_user_id = ?", s.SourceUserID).Scan(&id)
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
	_, err := db.ExecContext(ctx, q, se.StudentID, se.EventID, se.CohortID, FormatTime(se.EnrolledAt), se.CurrentLevel, se.AuditRatio, FormatTime(se.LastSyncedAt))
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

	var endAtStr any = nil
	if r.EndAt != nil {
		endAtStr = FormatTime(*r.EndAt)
	}

	q := db.Rebind(query)
	_, err := db.ExecContext(ctx, q, r.SourceRecordID, r.StudentID, r.TypeName, FormatTime(r.StartAt), endAtStr, r.Message, FormatTime(r.CreatedAt))
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

// GetCurriculumTimeline loads all monthly milestones for an event
func (db *DB) GetCurriculumTimeline(ctx context.Context, eventID int) (map[int]model.CurriculumMilestone, error) {
	query := db.Rebind("SELECT event_id, month, min_level, expected_level, checkpoint_level, rank, COALESCE(notes, '') FROM curriculum_timeline WHERE event_id = ? ORDER BY month ASC;")
	rows, err := db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	milestones := make(map[int]model.CurriculumMilestone)
	for rows.Next() {
		var m model.CurriculumMilestone
		if err := rows.Scan(&m.EventID, &m.Month, &m.MinLevel, &m.ExpectedLevel, &m.CheckpointLevel, &m.Rank, &m.Notes); err != nil {
			return nil, err
		}
		milestones[m.Month] = m
	}
	return milestones, nil
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
		var startStr string
		var endStr sql.NullString
		if err := rows.Scan(&r.ID, &r.SourceRecordID, &r.StudentID, &r.TypeName, &startStr, &endStr, &r.Message); err != nil {
			return nil, err
		}
		r.StartAt = ParseFlexibleTime(startStr)
		if endStr.Valid && endStr.String != "" {
			t := ParseFlexibleTime(endStr.String)
			if !t.IsZero() {
				r.EndAt = &t
			}
		}
		records = append(records, r)
	}
	return records, nil
}

// ListStudentsWithPace returns all students with their latest metric snapshot
func (db *DB) ListStudentsWithPace(ctx context.Context, cohortID int, statusFilter string, search string) ([]model.StudentPaceView, error) {
	baseQuery := `
        SELECT 
            s.id, s.source_user_id, s.login, COALESCE(s.first_name, ''), COALESCE(s.last_name, ''), COALESCE(s.email, ''),
            se.cohort_id, se.audit_ratio,
            COALESCE(m.id, 0), COALESCE(m.mentor_name, ''),
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
            COALESCE(snap.calculated_at, se.last_synced_at)
        FROM students s
        JOIN student_events se ON s.id = se.student_id
        LEFT JOIN mentor_assignments m ON s.id = m.student_id AND m.is_primary = TRUE
        LEFT JOIN (
            SELECT sms.*
            FROM student_metric_snapshots sms
            INNER JOIN (
                SELECT student_id, MAX(snapshot_date) as max_date
                FROM student_metric_snapshots
                GROUP BY student_id
            ) latest ON sms.student_id = latest.student_id AND sms.snapshot_date = latest.max_date
        ) snap ON s.id = snap.student_id
        WHERE 1=1
    `

	var args []any
	if cohortID > 0 {
		baseQuery += " AND se.cohort_id = ?"
		args = append(args, cohortID)
	}

	if statusFilter != "" {
		baseQuery += " AND snap.pace_status = ?"
		args = append(args, statusFilter)
	}

	if search != "" {
		baseQuery += " AND (s.login LIKE ? OR s.first_name LIKE ? OR s.last_name LIKE ? OR s.email LIKE ?)"
		searchTerm := "%" + search + "%"
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
		var enrolledStr, calculatedStr, reasonsJSON string

		err := rows.Scan(
			&v.ID, &v.SourceUserID, &v.Login, &v.FirstName, &v.LastName, &v.Email,
			&v.CohortID, &v.AuditRatio,
			&mentorID, &mentorName,
			&enrolledStr,
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
			&calculatedStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning student pace row: %w", err)
		}

		if math.IsInf(v.AuditRatio, 0) || math.IsNaN(v.AuditRatio) {
			v.AuditRatio = 0.0
		}

		if v.CohortID == 4 {
			v.CohortName = "01 Coding - Second Cohort"
		} else if v.CohortID == 1 {
			v.CohortName = "01 Coding - First Cohort"
		}

		if mentorID > 0 {
			v.Mentor = &model.Mentor{ID: mentorID, Name: mentorName}
		}

		v.Pace.EnrolledAt = ParseFlexibleTime(enrolledStr)
		v.LastCalculatedAt = ParseFlexibleTime(calculatedStr)
		_ = json.Unmarshal([]byte(reasonsJSON), &v.Pace.Reasons)

		views = append(views, v)
	}

	return views, nil
}

// GetCohortHealthSummary computes North Star KPI and status counts for a cohort
func (db *DB) GetCohortHealthSummary(ctx context.Context, cohortID int) (*model.CohortHealthSummary, error) {
	students, err := db.ListStudentsWithPace(ctx, cohortID, "", "")
	if err != nil {
		return nil, err
	}

	summary := &model.CohortHealthSummary{
		CohortID:      cohortID,
		TotalStudents: len(students),
		StatusCounts:  make(map[string]int),
		CalculatedAt:  time.Now(),
	}

	if cohortID == 4 {
		summary.CohortName = "01 Coding - Second Cohort"
	} else if cohortID == 1 {
		summary.CohortName = "01 Coding - First Cohort"
	}

	for _, s := range students {
		st := s.Pace.Status
		summary.StatusCounts[st]++

		if st == "PAUSED" || st == "INACTIVE" {
			summary.PausedStudents++
		} else {
			summary.ActiveStudents++
			switch st {
			case "GREEN":
				summary.OnTrackCount++
			case "YELLOW":
				summary.NearTrackCount++
			case "RED":
				summary.OffTrackCount++
			}
		}
	}

	// North Star: OnTrackRate = (GREEN + YELLOW) / ActiveStudents (or GREEN / ActiveStudents)
	// In PRD Section 4: On-Track = at or above configured minimum level (which includes Green & Yellow!)
	if summary.ActiveStudents > 0 {
		summary.OnTrackRate = float64(summary.OnTrackCount+summary.NearTrackCount) / float64(summary.ActiveStudents) * 100.0
		summary.ExpectedLevelRate = float64(summary.OnTrackCount) / float64(summary.ActiveStudents) * 100.0
	}

	return summary, nil
}
