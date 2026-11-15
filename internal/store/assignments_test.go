package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nextera-pulse/internal/store/pgtest"
)

// Fixture person ids: 1 manager, 2 and 3 campus mentors, 4–6 learners.
func newAssignmentDB(t *testing.T) *DB {
	t.Helper()
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (id, source_user_id, login, first_name, last_name, email, source_role) VALUES
		(1, 101, 'manager', 'Program', 'Manager', 'pm@example.com', 'admin'),
		(2, 102, 'mentor.a', 'Mentor', 'A', 'a@example.com', 'admin campus'),
		(3, 103, 'mentor.b', 'Mentor', 'B', 'b@example.com', 'admin campus'),
		(4, 104, 'learner.1', 'Learner', 'One', 'l1@example.com', 'user'),
		(5, 105, 'learner.2', 'Learner', 'Two', 'l2@example.com', 'user'),
		(6, 106, 'learner.3', 'Learner', 'Three', 'l3@example.com', 'user')`); err != nil {
		t.Fatalf("students: %v", err)
	}
	if _, err := db.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('students', 'id'), 100)`); err != nil {
		t.Fatalf("sequence: %v", err)
	}
	for _, id := range []int{4, 5, 6} {
		if _, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level) VALUES (?, 97, 4, '2026-01-01', 0)`), id); err != nil {
			t.Fatalf("events: %v", err)
		}
	}
	return db
}

func currentMentor(t *testing.T, db *DB, studentID int64) int64 {
	t.Helper()
	var mentor int64
	err := db.QueryRowContext(context.Background(), `SELECT COALESCE(MAX(mentor_id), 0) FROM mentor_assignments WHERE student_id = $1 AND is_primary AND ends_at IS NULL`, studentID).Scan(&mentor)
	if err != nil {
		t.Fatal(err)
	}
	return mentor
}

func TestMoveMentorAssignmentKeepsHistory(t *testing.T) {
	db := newAssignmentDB(t)
	ctx := context.Background()
	if err := db.MoveMentorAssignment(ctx, 4, 2, 1, "first mentor"); err != nil {
		t.Fatal(err)
	}
	if err := db.MoveMentorAssignment(ctx, 4, 2, 1, "same again"); err != nil {
		t.Fatal(err)
	}
	if err := db.MoveMentorAssignment(ctx, 4, 3, 1, "workload"); err != nil {
		t.Fatal(err)
	}
	history, err := db.MentorAssignmentHistory(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %+v, want 2 rows (re-assigning the same mentor adds nothing)", history)
	}
	now, before := history[0], history[1]
	if now.MentorID != 3 || !now.Current || now.EndsAt != nil || now.Reason != "workload" || now.AssignedBy == nil || *now.AssignedBy != 1 {
		t.Fatalf("current row = %+v", now)
	}
	if before.MentorID != 2 || before.Current || before.EndsAt == nil || before.Reason != "first mentor" ||
		before.EndedBy == nil || *before.EndedBy != 1 || !strings.Contains(before.EndReason, "workload") {
		t.Fatalf("closed row = %+v", before)
	}
	if !before.EndsAt.Equal(now.StartsAt) {
		t.Fatalf("history has a gap: ended %v, next started %v", before.EndsAt, now.StartsAt)
	}
	if got := currentMentor(t, db, 4); got != 3 {
		t.Fatalf("current mentor = %d", got)
	}
}

func TestMoveMentorAssignmentRejectsBadPeople(t *testing.T) {
	db := newAssignmentDB(t)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		student, mentor int64
		want            error
	}{
		"mentor is a learner":   {4, 5, ErrInvalidAssignment},
		"mentor is a manager":   {4, 1, ErrInvalidAssignment},
		"student is staff":      {3, 2, ErrInvalidAssignment},
		"unknown student":       {999, 2, ErrStudentNotFound},
		"unknown mentor":        {4, 999, ErrInvalidAssignment},
		"student is the mentor": {2, 2, ErrInvalidAssignment},
	} {
		if err := db.MoveMentorAssignment(ctx, tc.student, tc.mentor, 1, ""); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestUnassignMentorClosesTheCurrentRow(t *testing.T) {
	db := newAssignmentDB(t)
	ctx := context.Background()
	if err := db.MoveMentorAssignment(ctx, 5, 2, 1, ""); err != nil {
		t.Fatal(err)
	}
	closed, err := db.UnassignMentor(ctx, 5, 1, "left the program")
	if err != nil || !closed {
		t.Fatalf("unassign = %v %v", closed, err)
	}
	if again, err := db.UnassignMentor(ctx, 5, 1, ""); err != nil || again {
		t.Fatalf("second unassign = %v %v, want nothing to close", again, err)
	}
	if got := currentMentor(t, db, 5); got != 0 {
		t.Fatalf("still assigned to %d", got)
	}
	history, _ := db.MentorAssignmentHistory(ctx, 5)
	if len(history) != 1 || history[0].Current || history[0].EndsAt == nil || !strings.Contains(history[0].EndReason, "left the program") {
		t.Fatalf("history = %+v", history)
	}
	if _, err := db.UnassignMentor(ctx, 999, 1, ""); !errors.Is(err, ErrStudentNotFound) {
		t.Fatalf("unknown student err = %v", err)
	}
}

func TestOnlyOneCurrentMentorPerStudent(t *testing.T) {
	db := newAssignmentDB(t)
	ctx := context.Background()
	if err := db.MoveMentorAssignment(ctx, 6, 2, 1, ""); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, is_primary) VALUES (6, 3, 'Mentor B', TRUE)`)
	if err == nil || !strings.Contains(err.Error(), "uq_mentor_assignments_current") {
		t.Fatalf("second open row err = %v, want the unique index to refuse it", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, is_primary) VALUES (6, 999, 'Nobody', FALSE)`); err == nil {
		t.Fatal("assignment to an unknown mentor id was accepted; want the FK to refuse it")
	}
	// Closed rows do not count: history can hold many.
	if _, err := db.ExecContext(ctx, `INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, starts_at, ends_at) VALUES
		(6, 3, 'Mentor B', '2026-01-01', '2026-02-01'), (6, 3, 'Mentor B', '2026-02-01', '2026-03-01')`); err != nil {
		t.Fatalf("closed history rows: %v", err)
	}
}

func TestReadsUseOnlyTheCurrentAssignment(t *testing.T) {
	db := newAssignmentDB(t)
	ctx := context.Background()
	for _, step := range []struct{ student, mentor int64 }{{4, 2}, {4, 3}, {4, 2}, {5, 3}} {
		if err := db.MoveMentorAssignment(ctx, step.student, step.mentor, 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.UnassignMentor(ctx, 5, 1, ""); err != nil {
		t.Fatal(err)
	}

	students, err := db.ListStudentsWithPace(ctx, 0, 4, "", "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]int{}
	mentorOf := map[int64]int64{}
	for _, s := range students {
		seen[s.ID]++
		if s.Mentor != nil {
			mentorOf[s.ID] = s.Mentor.ID
		}
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("student %d listed %d times", id, n)
		}
	}
	if len(seen) != 3 || mentorOf[4] != 2 || mentorOf[5] != 0 || mentorOf[6] != 0 {
		t.Fatalf("list mentors = %v (count %d)", mentorOf, len(seen))
	}

	board, err := db.ProgressBoard(ctx, 4, 97, 0)
	if err != nil {
		t.Fatal(err)
	}
	names := map[int64]string{}
	for _, s := range board.Students {
		if _, dup := names[s.StudentID]; dup {
			t.Fatalf("progress lists student %d twice", s.StudentID)
		}
		names[s.StudentID] = s.MentorName
	}
	if names[4] != "Mentor A" || names[5] != "" {
		t.Fatalf("progress mentors = %v", names)
	}

	assigned, err := db.AssignedStudentIDs(ctx, 2)
	if err != nil || len(assigned) != 1 || !assigned[4] {
		t.Fatalf("mentor A scope = %v %v", assigned, err)
	}
	if assigned, _ := db.AssignedStudentIDs(ctx, 3); len(assigned) != 0 {
		t.Fatalf("mentor B keeps closed students in scope: %v", assigned)
	}
}

func TestMigrateClosesDuplicateCurrentRowsAndIsRepeatable(t *testing.T) {
	db := newAssignmentDB(t)
	ctx := context.Background()
	// Simulate a database from before the unique index existed.
	if _, err := db.ExecContext(ctx, `DROP INDEX uq_mentor_assignments_current`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, starts_at) VALUES
		(4, 2, 'Mentor A', '2026-01-01'), (4, 3, 'Mentor B', '2026-02-01')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Migrate(ctx); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}
	if got := currentMentor(t, db, 4); got != 3 {
		t.Fatalf("current mentor after dedup = %d, want the newest (3)", got)
	}
	history, _ := db.MentorAssignmentHistory(ctx, 4)
	if len(history) != 2 || history[1].EndsAt == nil {
		t.Fatalf("history after dedup = %+v, want the older row closed, not deleted", history)
	}
}
