package store

import (
	"context"
	"fmt"
	"strings"
)

type ProgressProject struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type ProgressStudent struct {
	StudentID   int64  `json:"studentId"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	MentorName  string `json:"mentorName"`
	Done        []bool `json:"done"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
}

type ProgressBoard struct {
	Source   string            `json:"source"`
	SyncedAt string            `json:"syncedAt,omitempty"`
	Projects []ProgressProject `json:"projects"`
	Students []ProgressStudent `json:"students"`
}

// ProgressBoard builds the project matrix from 01Edu progress rows joined to objects.
// A cell is done only when at least one progress row for that student and object has is_done.
func (db *DB) ProgressBoard(ctx context.Context, cohortID int, eventID int, onlyStudent int64) (ProgressBoard, error) {
	board := ProgressBoard{Source: "01edu", Projects: []ProgressProject{}, Students: []ProgressStudent{}}
	projects, err := db.progressProjects(ctx, cohortID, eventID, onlyStudent, "lower(o.type) = 'project'")
	if err != nil {
		return board, err
	}
	if len(projects) == 0 {
		projects, err = db.progressProjects(ctx, cohortID, eventID, onlyStudent, "(lower(o.path) LIKE '%/project%' OR lower(p.path) LIKE '%/project%')")
		if err != nil {
			return board, err
		}
	}
	if len(projects) > 48 {
		projects = projects[:48]
	}
	board.Projects = projects

	studentQuery := `
		SELECT s.id, s.login, TRIM(COALESCE(s.first_name, '') || ' ' || COALESCE(s.last_name, '')), COALESCE(m.mentor_name, '')
		FROM students s
		JOIN student_events se ON se.student_id = s.id
		LEFT JOIN mentor_assignments m ON m.student_id = s.id AND m.is_primary = TRUE
		WHERE se.event_id = ?`
	args := []any{eventID}
	if cohortID > 0 {
		studentQuery += " AND se.cohort_id = ?"
		args = append(args, cohortID)
	}
	if onlyStudent > 0 {
		studentQuery += " AND s.id = ?"
		args = append(args, onlyStudent)
	}
	studentQuery += " ORDER BY s.login"
	rows, err := db.QueryContext(ctx, db.Rebind(studentQuery), args...)
	if err != nil {
		return board, err
	}
	defer rows.Close()
	students := []ProgressStudent{}
	ids := []int64{}
	for rows.Next() {
		var student ProgressStudent
		if err := rows.Scan(&student.StudentID, &student.Login, &student.DisplayName, &student.MentorName); err != nil {
			return board, err
		}
		student.DisplayName = strings.TrimSpace(student.DisplayName)
		if student.DisplayName == "" {
			student.DisplayName = student.Login
		}
		student.Done = make([]bool, len(projects))
		student.Total = len(projects)
		students = append(students, student)
		ids = append(ids, student.StudentID)
	}
	if err := rows.Err(); err != nil {
		return board, err
	}
	if len(projects) == 0 || len(students) == 0 {
		board.Students = students
		return board, nil
	}

	done, err := db.progressDone(ctx, ids, projects)
	if err != nil {
		return board, err
	}
	for i := range students {
		for j, project := range projects {
			if done[students[i].StudentID][project.ID] {
				students[i].Done[j] = true
				students[i].Completed++
			}
		}
	}
	board.Students = students
	return board, nil
}

func (db *DB) progressProjects(ctx context.Context, cohortID int, eventID int, onlyStudent int64, filter string) ([]ProgressProject, error) {
	query := fmt.Sprintf(`
		SELECT o.source_object_id, COALESCE(o.name, ''), COALESCE(o.path, '')
		FROM learning_objects o
		JOIN learning_progress p ON p.object_id = o.source_object_id
		JOIN student_events se ON se.student_id = p.student_id
		WHERE se.event_id = ? AND %s`, filter)
	args := []any{eventID}
	if cohortID > 0 {
		query += " AND se.cohort_id = ?"
		args = append(args, cohortID)
	}
	if onlyStudent > 0 {
		query += " AND p.student_id = ?"
		args = append(args, onlyStudent)
	}
	query += " GROUP BY o.source_object_id, o.name, o.path ORDER BY COUNT(*) DESC, o.name"
	rows, err := db.QueryContext(ctx, db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []ProgressProject
	for rows.Next() {
		var project ProgressProject
		if err := rows.Scan(&project.ID, &project.Name, &project.Path); err != nil {
			return nil, err
		}
		if project.Name == "" {
			project.Name = project.Path
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (db *DB) progressDone(ctx context.Context, studentIDs []int64, projects []ProgressProject) (map[int64]map[int64]bool, error) {
	result := map[int64]map[int64]bool{}
	studentMarks := make([]string, len(studentIDs))
	args := make([]any, 0, len(studentIDs)+len(projects))
	for i, id := range studentIDs {
		studentMarks[i] = "?"
		args = append(args, id)
	}
	projectMarks := make([]string, len(projects))
	for i, project := range projects {
		projectMarks[i] = "?"
		args = append(args, project.ID)
	}
	query := fmt.Sprintf(`SELECT student_id, object_id, MAX(is_done)
		FROM learning_progress
		WHERE student_id IN (%s) AND object_id IN (%s)
		GROUP BY student_id, object_id`, strings.Join(studentMarks, ","), strings.Join(projectMarks, ","))
	rows, err := db.QueryContext(ctx, db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var studentID, objectID int64
		var done int
		if err := rows.Scan(&studentID, &objectID, &done); err != nil {
			return nil, err
		}
		if result[studentID] == nil {
			result[studentID] = map[int64]bool{}
		}
		result[studentID][objectID] = done > 0
	}
	return result, rows.Err()
}
