package pace

import (
	"testing"
	"time"

	"nextera-pulse/internal/model"
)

func getTestSetup() *Engine {
	policies := map[string]model.StatusPolicy{
		"away":             {RecordType: "away", PauseClock: false, ExcludeFromActiveKPIs: false},
		"restricted":       {RecordType: "restricted", PauseClock: true, ExcludeFromActiveKPIs: true},
		"blocked-long":     {RecordType: "blocked-long", PauseClock: true, ExcludeFromActiveKPIs: true},
		"temporary-notice": {RecordType: "temporary-notice", PauseClock: false, ExcludeFromActiveKPIs: false},
		"left-school":      {RecordType: "left-school", PauseClock: true, ExcludeFromActiveKPIs: true},
		"ban":              {RecordType: "ban", PauseClock: true, ExcludeFromActiveKPIs: true},
	}

	milestones := map[int]model.CurriculumMilestone{
		2:  {Month: 2, MinLevel: 5.0, ExpectedLevel: 7.0, CheckpointLevel: 0.0},
		4:  {Month: 4, MinLevel: 9.0, ExpectedLevel: 13.0, CheckpointLevel: 0.0},
		5:  {Month: 5, MinLevel: 12.0, ExpectedLevel: 17.0, CheckpointLevel: 10.0},
		6:  {Month: 6, MinLevel: 15.0, ExpectedLevel: 18.0, CheckpointLevel: 20.0},
		7:  {Month: 7, MinLevel: 17.0, ExpectedLevel: 20.0, CheckpointLevel: 20.0},
		8:  {Month: 8, MinLevel: 20.0, ExpectedLevel: 23.0, CheckpointLevel: 30.0},
		10: {Month: 10, MinLevel: 28.0, ExpectedLevel: 34.0, CheckpointLevel: 30.0},
		12: {Month: 12, MinLevel: 34.0, ExpectedLevel: 38.0, CheckpointLevel: 40.0},
		18: {Month: 18, MinLevel: 50.0, ExpectedLevel: 62.0, CheckpointLevel: 50.0},
	}

	return NewEngine(policies, milestones)
}

func TestPaceEngine_GreenStatus(t *testing.T) {
	engine := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	// 200 days ago -> Month 7 (200 / 30 = 6, 6 + 1 = 7). Month 7 expected is 20.0
	enrolledAt := now.AddDate(0, 0, -200)

	metrics := engine.Calculate(enrolledAt, 21.0, nil, now)

	if metrics.CurriculumMonth != 7 {
		t.Fatalf("expected month 7, got %d", metrics.CurriculumMonth)
	}
	if metrics.Status != "GREEN" {
		t.Fatalf("expected status GREEN, got %s", metrics.Status)
	}
	if metrics.GapToExpected != 1.0 {
		t.Fatalf("expected gap to expected 1.0, got %f", metrics.GapToExpected)
	}
}

func TestPaceEngine_YellowStatus(t *testing.T) {
	engine := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	// Month 7: min 17, expected 20. Actual 18.0
	enrolledAt := now.AddDate(0, 0, -200)

	metrics := engine.Calculate(enrolledAt, 18.0, nil, now)

	if metrics.Status != "YELLOW" {
		t.Fatalf("expected status YELLOW, got %s", metrics.Status)
	}
	if len(metrics.Reasons) == 0 || metrics.Reasons[0] != "PACE_BELOW_TARGET" {
		t.Fatalf("expected reason PACE_BELOW_TARGET, got %v", metrics.Reasons)
	}
}

func TestPaceEngine_RedStatus(t *testing.T) {
	engine := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	// Month 7: min 17. Actual 7.0
	enrolledAt := now.AddDate(0, 0, -200)

	metrics := engine.Calculate(enrolledAt, 7.0, nil, now)

	if metrics.Status != "RED" {
		t.Fatalf("expected status RED, got %s", metrics.Status)
	}
	if metrics.GapToMinimum != -10.0 {
		t.Fatalf("expected gap to min -10.0, got %f", metrics.GapToMinimum)
	}
	if len(metrics.Reasons) == 0 || metrics.Reasons[0] != "PACE_BELOW_MINIMUM" {
		t.Fatalf("expected reason PACE_BELOW_MINIMUM, got %v", metrics.Reasons)
	}
}

func TestPaceEngine_PausedStatus(t *testing.T) {
	engine := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	enrolledAt := now.AddDate(0, 0, -200)

	records := []model.StudentRecord{
		{
			TypeName: "blocked-long",
			StartAt:  now.AddDate(0, 0, -6),
			EndAt:    nil, // currently active pause
		},
	}

	metrics := engine.Calculate(enrolledAt, 5.0, records, now)

	if metrics.Status != "PAUSED" {
		t.Fatalf("expected status PAUSED, got %s", metrics.Status)
	}
}

func TestPaceEngine_PastPauseAdjustment(t *testing.T) {
	engine := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	enrolledAt := now.AddDate(0, 0, -200) // 200 calendar days

	// Paused for 60 days in the past
	pauseStart := now.AddDate(0, 0, -100)
	pauseEnd := now.AddDate(0, 0, -40)
	records := []model.StudentRecord{
		{
			TypeName: "restricted",
			StartAt:  pauseStart,
			EndAt:    &pauseEnd,
		},
	}

	metrics := engine.Calculate(enrolledAt, 15.0, records, now)

	// Active days: 200 - 60 = 140 days
	// Month: 140 / 30 + 1 = 5 (Month 5: min 12, expected 17)
	if metrics.ActiveProgramDays != 140 {
		t.Fatalf("expected 140 active days, got %d", metrics.ActiveProgramDays)
	}
	if metrics.CurriculumMonth != 5 {
		t.Fatalf("expected month 5, got %d", metrics.CurriculumMonth)
	}
	// Level 15 in Month 5 (min 12, exp 17) -> YELLOW
	if metrics.Status != "YELLOW" {
		t.Fatalf("expected status YELLOW, got %s", metrics.Status)
	}
}

func TestPaceEngine_OnboardingPeriod(t *testing.T) {
	engine := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	// Enrolled 15 days ago -> Month 1
	enrolledAt := now.AddDate(0, 0, -15)

	metrics := engine.Calculate(enrolledAt, 0.0, nil, now)

	if metrics.CurriculumMonth != 1 {
		t.Fatalf("expected month 1, got %d", metrics.CurriculumMonth)
	}
	if metrics.Status != "ONBOARDING" {
		t.Fatalf("expected status ONBOARDING, got %s", metrics.Status)
	}
}

func TestPaceEngine_OverlappingPausesCountOnce(t *testing.T) {
	e := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	endA := now.AddDate(0, 0, -50)
	endB := now.AddDate(0, 0, -40)
	records := []model.StudentRecord{
		{TypeName: "restricted", StartAt: now.AddDate(0, 0, -100), EndAt: &endA},
		{TypeName: "ban", StartAt: now.AddDate(0, 0, -80), EndAt: &endB},
	}
	got := e.Calculate(now.AddDate(0, 0, -200), 15, records, now)
	if got.ActiveProgramDays != 140 {
		t.Fatalf("expected 140 active days, got %d", got.ActiveProgramDays)
	}
	if got.Status == "PAUSED" {
		t.Fatalf("past overlapping pauses should not mark student paused")
	}
}

func TestPaceEngine_FuturePauseNotActive(t *testing.T) {
	e := getTestSetup()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	records := []model.StudentRecord{{TypeName: "restricted", StartAt: now.AddDate(0, 0, 2)}}
	got := e.Calculate(now.AddDate(0, 0, -200), 18, records, now)
	if got.ActiveProgramDays != 200 || got.Status != "YELLOW" {
		t.Fatalf("future pause affected present pace: %+v", got)
	}
}

func TestPaceEngine_NoTimelineIsNotMeasurable(t *testing.T) {
	e := NewEngine(getTestSetup().Policies, map[int]model.CurriculumMilestone{})
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	got := e.Calculate(now.AddDate(0, 0, -200), 0, nil, now)
	if got.Status != "NOT_MEASURABLE" {
		t.Fatalf("missing timeline should not be GREEN: %+v", got)
	}
}
