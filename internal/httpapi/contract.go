package httpapi

import (
	"strings"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/store"
)

type mentorStudentRow struct {
	StudentID          int64    `json:"studentId"`
	Login              string   `json:"login"`
	DisplayName        string   `json:"displayName"`
	Email              string   `json:"email"`
	EnrolledAt         string   `json:"enrolledAt"`
	ActiveAcademicDays int      `json:"activeAcademicDays"`
	CurriculumMonth    int      `json:"curriculumMonth"`
	ActualLevel        float64  `json:"actualLevel"`
	MinimumLevel       float64  `json:"minimumLevel"`
	ExpectedLevel      float64  `json:"expectedLevel"`
	CheckpointTarget   float64  `json:"checkpointTarget"`
	GapToMinimum       float64  `json:"gapToMinimum"`
	GapToExpected      float64  `json:"gapToExpected"`
	PaceStatus         string   `json:"paceStatus"`
	SourceStatus       string   `json:"sourceStatus"`
	HealthReasonCodes  []string `json:"healthReasonCodes"`
	MentorID           int64    `json:"mentorId,omitempty"`
	MentorName         string   `json:"mentorName,omitempty"`
}

type mentorStudentsPayload struct {
	CohortID   int    `json:"cohortId"`
	CohortName string `json:"cohortName"`
	EventID    int    `json:"eventId"`
	// PaceRule is the monthly-target version behind the pace statuses.
	PaceRule     *paceRule `json:"paceRule,omitempty"`
	SnapshotDate string    `json:"snapshotDate"`
	// DataSource is "live" when this server syncs 01Edu, otherwise "offline".
	DataSource string             `json:"dataSource"`
	SyncedAt   string             `json:"syncedAt,omitempty"`
	Students   []mentorStudentRow `json:"students"`
}

func mentorStudentsResponse(eventID, cohortID int, cohortName string, students []model.StudentPaceView, now time.Time) mentorStudentsPayload {
	rows := make([]mentorStudentRow, 0, len(students))
	snapshot := now.UTC()
	for _, student := range students {
		if !student.LastCalculatedAt.IsZero() && student.LastCalculatedAt.After(snapshot) {
			snapshot = student.LastCalculatedAt.UTC()
		}
		name := strings.TrimSpace(student.FirstName + " " + student.LastName)
		if name == "" {
			name = student.Login
		}
		reasons := mentorReasonCodes(student.Pace.Reasons)
		rows = append(rows, mentorStudentRow{
			StudentID:          student.ID,
			Login:              student.Login,
			DisplayName:        name,
			Email:              student.Email,
			EnrolledAt:         student.Pace.EnrolledAt.UTC().Format("2006-01-02"),
			ActiveAcademicDays: student.Pace.ActiveProgramDays,
			CurriculumMonth:    student.Pace.CurriculumMonth,
			ActualLevel:        student.Pace.ActualLevel,
			MinimumLevel:       student.Pace.MinimumLevel,
			ExpectedLevel:      student.Pace.ExpectedLevel,
			CheckpointTarget:   student.Pace.CheckpointTarget,
			GapToMinimum:       student.Pace.GapToMinimum,
			GapToExpected:      student.Pace.GapToExpected,
			PaceStatus:         mentorPaceStatus(student.Pace.Status),
			SourceStatus:       sourceStatusOrActive(student.SourceStatus),
			HealthReasonCodes:  reasons,
			MentorID:           mentorID(student),
			MentorName:         mentorName(student),
		})
	}
	return mentorStudentsPayload{
		CohortID:     cohortID,
		CohortName:   cohortName,
		EventID:      eventID,
		SnapshotDate: snapshot.Format("2006-01-02"),
		DataSource:   "offline",
		Students:     rows,
	}
}

func mentorID(student model.StudentPaceView) int64 {
	if student.Mentor == nil {
		return 0
	}
	return student.Mentor.ID
}

func mentorName(student model.StudentPaceView) string {
	if student.Mentor == nil {
		return ""
	}
	return student.Mentor.Name
}

func mentorPaceStatus(status string) string {
	switch status {
	case "GREEN", "YELLOW", "RED", "PAUSED", "INACTIVE", "NOT_MEASURABLE":
		return status
	default:
		// ONBOARDING and UNKNOWN are not part of the mentor UI contract.
		return "NOT_MEASURABLE"
	}
}

func mentorReasonCodes(reasons []string) []string {
	if len(reasons) == 0 {
		return []string{}
	}
	mapped := make([]string, len(reasons))
	for i, reason := range reasons {
		switch reason {
		case "PACE_BELOW_TARGET":
			mapped[i] = "BELOW_EXPECTED_LEVEL"
		case "PACE_BELOW_MINIMUM":
			mapped[i] = "BELOW_MINIMUM_LEVEL"
		case "ADMINISTRATIVE_PAUSE":
			mapped[i] = "CURRENTLY_PAUSED"
		case "STUDENT_LEFT_SCHOOL":
			mapped[i] = "LEFT_SCHOOL"
		case "ONBOARDING":
			mapped[i] = "NO_TIMELINE"
		default:
			mapped[i] = reason
		}
	}
	return mapped
}

func formatAPITime(value string) string {
	parsed := store.ParseFlexibleTime(value)
	if parsed.IsZero() {
		return value
	}
	return parsed.UTC().Format(time.RFC3339)
}

func isLocalDevOrigin(origin string) bool {
	switch origin {
	case "http://127.0.0.1:4181", "http://localhost:4181", "http://127.0.0.1:5173", "http://localhost:5173":
		return true
	default:
		return false
	}
}

func sourceStatusOrActive(recordType string) string {
	status := strings.TrimSpace(recordType)
	if status == "" {
		return "active"
	}
	return status
}

// paceRule tells clients which monthly-target version the pace statuses use
// and whether Product approved it.
type paceRule struct {
	EventID  int    `json:"eventId"`
	Version  string `json:"version"`
	Status   string `json:"status"`
	Approved bool   `json:"approved"`
}
