package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// AssignCampusMentors splits learners evenly across campus admins.
// Ingest and the API do not call it. The program manager assigns students.
func (db *DB) AssignCampusMentors(ctx context.Context) (int, error) {
	var existing int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mentor_assignments`).Scan(&existing); err != nil {
		return 0, err
	}
	if existing > 0 {
		return 0, nil
	}
	mentors, err := db.campusMentors(ctx)
	if err != nil || len(mentors) == 0 {
		return 0, err
	}
	learners, err := db.learnerIDs(ctx)
	if err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM mentor_assignments`); err != nil {
		return 0, err
	}
	query := db.Rebind(`INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, starts_at, is_primary) VALUES (?, ?, ?, ?, ?)`)
	now := time.Now().UTC()
	cohortCount := map[int]int{}
	assigned := 0
	for _, learner := range learners {
		mentor := mentors[cohortCount[learner.cohort]%len(mentors)]
		cohortCount[learner.cohort]++
		if _, err := tx.ExecContext(ctx, query, learner.id, mentor.id, mentor.name, now, true); err != nil {
			return 0, err
		}
		assigned++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return assigned, nil
}

// MoveMentorAssignment moves one learner to a campus admin.
func (db *DB) MoveMentorAssignment(ctx context.Context, studentID, mentorStudentID int64) error {
	if studentID == 0 || mentorStudentID == 0 || studentID == mentorStudentID {
		return fmt.Errorf("student and mentor are required")
	}
	var mentorName, mentorRole, studentRole string
	err := db.QueryRowContext(ctx, db.Rebind(`SELECT TRIM(COALESCE(first_name, '') || ' ' || COALESCE(last_name, '')), COALESCE(source_role, '') FROM students WHERE id = ?`), mentorStudentID).Scan(&mentorName, &mentorRole)
	if err == sql.ErrNoRows || strings.TrimSpace(mentorRole) != "admin campus" {
		return fmt.Errorf("mentor is not a campus admin")
	}
	if err != nil {
		return err
	}
	mentorName = strings.TrimSpace(mentorName)
	if mentorName == "" {
		if err := db.QueryRowContext(ctx, db.Rebind(`SELECT login FROM students WHERE id = ?`), mentorStudentID).Scan(&mentorName); err != nil {
			return err
		}
	}
	err = db.QueryRowContext(ctx, db.Rebind(`SELECT COALESCE(source_role, '') FROM students WHERE id = ?`), studentID).Scan(&studentRole)
	if err == sql.ErrNoRows {
		return fmt.Errorf("student was not found")
	}
	if err != nil {
		return err
	}
	if studentRole == "admin" || studentRole == "admin campus" {
		return fmt.Errorf("staff are not assigned as students")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, db.Rebind(`DELETE FROM mentor_assignments WHERE student_id = ?`), studentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, db.Rebind(`INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, starts_at, is_primary) VALUES (?, ?, ?, ?, ?)`), studentID, mentorStudentID, mentorName, time.Now().UTC(), true); err != nil {
		return err
	}
	return tx.Commit()
}

type campusMentor struct {
	id   int64
	name string
}

type learnerRow struct {
	id     int64
	cohort int
}

func (db *DB) campusMentors(ctx context.Context) ([]campusMentor, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(first_name, ''), COALESCE(last_name, ''), login
		FROM students WHERE source_role = 'admin campus' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mentors []campusMentor
	for rows.Next() {
		var mentor campusMentor
		var first, last, login string
		if err := rows.Scan(&mentor.id, &first, &last, &login); err != nil {
			return nil, err
		}
		mentor.name = strings.TrimSpace(first + " " + last)
		if mentor.name == "" {
			mentor.name = login
		}
		mentors = append(mentors, mentor)
	}
	return mentors, rows.Err()
}

func (db *DB) learnerIDs(ctx context.Context) ([]learnerRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT s.id, se.cohort_id
		FROM students s
		JOIN student_events se ON se.student_id = s.id
		WHERE COALESCE(s.source_role, '') NOT IN ('admin', 'admin campus')
		ORDER BY se.cohort_id, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[int64]bool{}
	var learners []learnerRow
	for rows.Next() {
		var learner learnerRow
		if err := rows.Scan(&learner.id, &learner.cohort); err != nil {
			return nil, err
		}
		if seen[learner.id] {
			continue
		}
		seen[learner.id] = true
		learners = append(learners, learner)
	}
	return learners, rows.Err()
}
