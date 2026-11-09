package httpapi

import (
	"strings"
	"time"

	"nextera-pulse/internal/model"
)

const (
	pilotEventID   = 97
	secondCohortID = 4
	firstCohortID  = 1
)

// resolveMentorCohort maps the id the mentor UI puts in
// /api/v1/cohorts/{id}/students. Event 97 is the pilot event and selects
// the Second Cohort. Cohort ids 4 and 1 select those cohorts directly.
func resolveMentorCohort(pathID int) (cohortID, eventID int, ok bool) {
	switch pathID {
	case pilotEventID, secondCohortID:
		return secondCohortID, pilotEventID, true
	case firstCohortID:
		return firstCohortID, pilotEventID, true
	default:
		return 0, 0, false
	}
}

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
}

type mentorStudentsPayload struct {
	CohortName   string             `json:"cohortName"`
	EventID      int                `json:"eventId"`
	SnapshotDate string             `json:"snapshotDate"`
	Students     []mentorStudentRow `json:"students"`
}

func mentorStudentsResponse(eventID int, students []model.StudentPaceView, now time.Time) mentorStudentsPayload {
	rows := make([]mentorStudentRow, 0, len(students))
	snapshot := now.UTC()
	cohortName := "01 Coding - Second Cohort"
	for _, student := range students {
		if student.CohortName != "" {
			cohortName = student.CohortName
		}
		if !student.LastCalculatedAt.IsZero() && student.LastCalculatedAt.After(snapshot) {
			snapshot = student.LastCalculatedAt.UTC()
		}
		name := strings.TrimSpace(student.FirstName + " " + student.LastName)
		if name == "" {
			name = student.Login
		}
		reasons := student.Pace.Reasons
		if reasons == nil {
			reasons = []string{}
		}
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
			SourceStatus:       mentorSourceStatus(student.Pace.Status),
			HealthReasonCodes:  reasons,
		})
	}
	return mentorStudentsPayload{
		CohortName:   cohortName,
		EventID:      eventID,
		SnapshotDate: snapshot.Format("2006-01-02"),
		Students:     rows,
	}
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

func mentorSourceStatus(paceStatus string) string {
	switch paceStatus {
	case "INACTIVE":
		return "left-school"
	case "PAUSED":
		return "restricted"
	default:
		return "active"
	}
}
