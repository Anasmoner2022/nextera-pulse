package httpapi

import (
	"testing"
	"time"

	"nextera-pulse/internal/model"
	pulsesync "nextera-pulse/internal/sync"
)

func TestResolveCohortUsesTheConfiguredProgram(t *testing.T) {
	def := pulsesync.DefaultProgram()
	cohort, event, ok := def.ResolveCohort(pulsesync.DefaultEventID)
	if !ok || cohort != def.DefaultCohortID() || event != pulsesync.DefaultEventID {
		t.Fatalf("default event should select the pilot cohort, got cohort=%d event=%d ok=%v", cohort, event, ok)
	}
	if _, _, ok := def.ResolveCohort(99); ok {
		t.Fatal("unknown id should not resolve")
	}
	custom := pulsesync.LiveCycleConfig{EventID: 120, Cohorts: []pulsesync.CohortLabel{{ID: 7, Label: "Cohort Seven"}, {ID: 8, Label: "Cohort Eight"}}}
	for path, want := range map[int]int{120: 7, 7: 7, 8: 8} {
		cohort, event, ok := custom.ResolveCohort(path)
		if !ok || cohort != want || event != 120 {
			t.Errorf("custom %d -> cohort=%d event=%d ok=%v, want %d/120", path, cohort, event, ok, want)
		}
	}
	if _, _, ok := custom.ResolveCohort(pulsesync.DefaultEventID); ok {
		t.Error("the default event must not resolve when another event is configured")
	}
}

func TestMentorStudentsResponseMapsPaceContract(t *testing.T) {
	enrolled := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	payload := mentorStudentsResponse(120, 7, "Cohort Seven", []model.StudentPaceView{{
		ID:        12,
		Login:     "nour.ahmed",
		FirstName: "Nour",
		LastName:  "Ahmed",
		Pace: model.PaceMetrics{
			EnrolledAt:        enrolled,
			ActiveProgramDays: 40,
			CurriculumMonth:   2,
			ActualLevel:       4,
			Status:            "ONBOARDING",
			Reasons:           []string{"ONBOARDING", "PACE_BELOW_MINIMUM"},
		},
	}}, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC))

	if payload.EventID != 120 || payload.CohortID != 7 || payload.CohortName != "Cohort Seven" {
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
	if len(row.HealthReasonCodes) != 2 || row.HealthReasonCodes[0] != "NO_TIMELINE" || row.HealthReasonCodes[1] != "BELOW_MINIMUM_LEVEL" {
		t.Fatalf("reason mapping failed: %+v", row.HealthReasonCodes)
	}
}
