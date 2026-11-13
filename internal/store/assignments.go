package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Mentor assignments keep their history: a student's current mentor is the
// row with is_primary AND ends_at IS NULL (at most one, enforced by the
// partial unique index uq_mentor_assignments_current). Moving or unassigning
// closes that row; nothing is deleted.

var (
	// ErrInvalidAssignment means the student or mentor cannot take part in an
	// assignment (unknown person, mentor is not a campus admin, student is staff).
	ErrInvalidAssignment = errors.New("invalid mentor assignment")
	// ErrStudentNotFound means no person has that id.
	ErrStudentNotFound = errors.New("student not found")
)

// currentAssignment is the SQL condition for a student's current mentor row.
const currentAssignment = "is_primary = TRUE AND ends_at IS NULL"

// MentorAssignment is one row of a student's assignment history.
type MentorAssignment struct {
	ID         int64      `json:"id"`
	StudentID  int64      `json:"studentId"`
	MentorID   int64      `json:"mentorId"`
	MentorName string     `json:"mentorName"`
	StartsAt   time.Time  `json:"startsAt"`
	EndsAt     *time.Time `json:"endsAt"`
	Reason     string     `json:"reason"`
	AssignedBy *int64     `json:"assignedBy"`
	EndedBy    *int64     `json:"endedBy"`
	EndReason  string     `json:"endReason"`
	Current    bool       `json:"current"`
}

// MoveMentorAssignment makes mentorStudentID the student's current mentor.
// The previous current row is closed (ends_at = now, ended_by, end_reason)
// and a new row is opened with reason and assigned_by. assignedBy is the
// signed-in person's id, or 0 when unknown. Assigning the mentor the student
// already has changes nothing.
func (db *DB) MoveMentorAssignment(ctx context.Context, studentID, mentorStudentID, assignedBy int64, reason string) error {
	if studentID == 0 || mentorStudentID == 0 || studentID == mentorStudentID {
		return fmt.Errorf("%w: student and mentor are required", ErrInvalidAssignment)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual"
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var mentorName, mentorRole, mentorLogin string
	err = tx.QueryRowContext(ctx, db.Rebind(`SELECT TRIM(COALESCE(first_name, '') || ' ' || COALESCE(last_name, '')), COALESCE(source_role, ''), login
		FROM students WHERE id = ?`), mentorStudentID).Scan(&mentorName, &mentorRole, &mentorLogin)
	if err == sql.ErrNoRows || (err == nil && strings.TrimSpace(mentorRole) != "admin campus") {
		return fmt.Errorf("%w: mentor is not a campus admin", ErrInvalidAssignment)
	}
	if err != nil {
		return err
	}
	if mentorName = strings.TrimSpace(mentorName); mentorName == "" {
		mentorName = mentorLogin
	}
	if err := lockLearner(ctx, db, tx, studentID); err != nil {
		return err
	}

	var currentID, currentMentor int64
	err = tx.QueryRowContext(ctx, db.Rebind(`SELECT id, mentor_id FROM mentor_assignments
		WHERE student_id = ? AND `+currentAssignment+` FOR UPDATE`), studentID).Scan(&currentID, &currentMentor)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		return err
	case currentMentor == mentorStudentID:
		return tx.Commit() // already this mentor: keep the open row as it is
	}

	now := time.Now().UTC()
	if currentID != 0 {
		if _, err := tx.ExecContext(ctx, db.Rebind(`UPDATE mentor_assignments SET ends_at = ?, ended_by = ?, end_reason = ? WHERE id = ?`),
			now, nullableID(assignedBy), "moved: "+reason, currentID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, db.Rebind(`INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, starts_at, is_primary, reason, assigned_by)
		VALUES (?, ?, ?, ?, TRUE, ?, ?)`), studentID, mentorStudentID, mentorName, now, reason, nullableID(assignedBy)); err != nil {
		return err
	}
	return tx.Commit()
}

// UnassignMentor closes the student's current mentor row. It reports whether
// there was one to close; the history stays.
func (db *DB) UnassignMentor(ctx context.Context, studentID, endedBy int64, reason string) (bool, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual"
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := lockLearner(ctx, db, tx, studentID); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, db.Rebind(`UPDATE mentor_assignments SET ends_at = ?, ended_by = ?, end_reason = ?
		WHERE student_id = ? AND `+currentAssignment), time.Now().UTC(), nullableID(endedBy), "unassigned: "+reason, studentID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, tx.Commit()
}

// MentorAssignmentHistory lists a student's assignments, newest first.
func (db *DB) MentorAssignmentHistory(ctx context.Context, studentID int64) ([]MentorAssignment, error) {
	rows, err := db.QueryContext(ctx, db.Rebind(`SELECT id, student_id, mentor_id, mentor_name, starts_at, ends_at, reason, assigned_by, ended_by, end_reason, is_primary
		FROM mentor_assignments WHERE student_id = ? ORDER BY starts_at DESC, id DESC`), studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []MentorAssignment{}
	for rows.Next() {
		var a MentorAssignment
		var endsAt sql.NullTime
		var assignedBy, endedBy sql.NullInt64
		var primary bool
		if err := rows.Scan(&a.ID, &a.StudentID, &a.MentorID, &a.MentorName, &a.StartsAt, &endsAt, &a.Reason, &assignedBy, &endedBy, &a.EndReason, &primary); err != nil {
			return nil, err
		}
		if endsAt.Valid {
			t := endsAt.Time
			a.EndsAt = &t
		}
		if assignedBy.Valid {
			a.AssignedBy = &assignedBy.Int64
		}
		if endedBy.Valid {
			a.EndedBy = &endedBy.Int64
		}
		a.Current = primary && a.EndsAt == nil
		history = append(history, a)
	}
	return history, rows.Err()
}

// lockLearner checks that studentID is a learner (not staff) and locks the
// row so concurrent moves of the same student run one after the other.
func lockLearner(ctx context.Context, db *DB, tx *sql.Tx, studentID int64) error {
	var role string
	err := tx.QueryRowContext(ctx, db.Rebind(`SELECT COALESCE(source_role, '') FROM students WHERE id = ? FOR UPDATE`), studentID).Scan(&role)
	if err == sql.ErrNoRows {
		return ErrStudentNotFound
	}
	if err != nil {
		return err
	}
	if role == "admin" || role == "admin campus" {
		return fmt.Errorf("%w: staff are not assigned as students", ErrInvalidAssignment)
	}
	return nil
}

func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}
