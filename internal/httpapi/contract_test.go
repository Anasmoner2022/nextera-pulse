package httpapi

import (
	"testing"
	"time"

	"nextera-pulse/internal/model"
)

func TestResolveMentorCohort(t *testing.T) {
	cohort, event, ok := resolveMentorCohort(97)
	if !ok || cohort != 4 || event != 97 {
		t.Fatalf("event 97 should select the Second Cohort, got cohort=%d event=%d ok=%v", cohort, event, ok)
	}
	if _, _, ok := resolveMentorCohort(99); ok {
		t.Fatal("unknown id should not resolve")
	}
}

func TestMentorStudentsResponseMapsPaceContract(t *testing.T) {
	enrolled := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	payload := mentorStudentsResponse(97, []model.StudentPaceView{{
		ID:         12,
		Login:      "nour.ahmed",
		FirstName:  "Nour",
		LastName:   "Ahmed",
		CohortName: "01 Coding - Second Cohort",
		Pace: model.PaceMetrics{
			EnrolledAt:        enrolled,
			ActiveProgramDays: 40,
			CurriculumMonth:   2,
			ActualLevel:       4,
			Status:            "ONBOARDING",
			Reasons:           []string{"ONBOARDING"},
		},
	}}, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC))

	if payload.EventID != 97 || payload.CohortName != "01 Coding - Second Cohort" {
		t.Fatalf("unexpected envelope: %+v", payload)
	}
	if len(payload.Students) != 1 {
		t.Fatalf("expected one student, got %d", len(payload.Students))
	}
	row := payload.Students[0]
	if row.StudentID != 12 || row.DisplayName != "Nour Ahmed" || row.EnrolledAt != "2026-06-01" {
		t.Fatalf("identity mapping failed: %+v", row)
	}
	if row.PaceStatus != "NOT_MEASURABLE" || row.SourceStatus != "active" {
		t.Fatalf("status mapping failed: %+v", row)
	}
}
