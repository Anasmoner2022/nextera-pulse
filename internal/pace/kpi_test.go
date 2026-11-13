package pace

import "testing"

func TestOnTrackIsGreenOverMeasurable(t *testing.T) {
	excluded := map[string]bool{"left-school": true}
	students := []KPIStudent{
		{"GREEN", "active"}, {"GREEN", "active"}, {"GREEN", "active"},
		{"YELLOW", "active"}, {"YELLOW", "active"}, // Watch: measurable, not on track
		{"RED", "active"},
		{"PAUSED", "away"}, {"INACTIVE", "active"}, {"NOT_MEASURABLE", "active"},
		{"ONBOARDING", "active"}, {"UNKNOWN", "active"}, {"", "active"},
		{"GREEN", "left-school"}, // flagged by policy: out of the denominator
	}
	kpi := OnTrack(students, excluded)
	if kpi.OnTrack != 3 || kpi.Measurable != 6 || !kpi.HasMeasurable || kpi.Rate == nil || *kpi.Rate != 50 {
		t.Fatalf("kpi = %+v rate %v, want 3 / 6 = 50%%", kpi, kpi.Rate)
	}
	if kpi.Total != 13 || kpi.ExcludedByPolicy != 1 {
		t.Fatalf("total %d excluded %d", kpi.Total, kpi.ExcludedByPolicy)
	}
	if kpi.StatusCounts["YELLOW"] != 2 || kpi.MeasurableCounts["YELLOW"] != 2 || kpi.StatusCounts["GREEN"] != 4 || kpi.MeasurableCounts["GREEN"] != 3 {
		t.Fatalf("counts %v measurable %v", kpi.StatusCounts, kpi.MeasurableCounts)
	}
	if kpi.StatusCounts["UNKNOWN"] != 2 {
		t.Fatalf("empty status should count as UNKNOWN: %v", kpi.StatusCounts)
	}
}

func TestOnTrackWithNoMeasurableStudentsIsNullNotZero(t *testing.T) {
	for name, students := range map[string][]KPIStudent{
		"empty":         nil,
		"only paused":   {{"PAUSED", "away"}, {"ONBOARDING", "active"}},
		"only excluded": {{"GREEN", "ban"}, {"RED", "ban"}},
	} {
		kpi := OnTrack(students, map[string]bool{"ban": true})
		if kpi.Rate != nil || kpi.HasMeasurable || kpi.Measurable != 0 {
			t.Fatalf("%s: kpi = %+v, want null rate", name, kpi)
		}
	}
}

func TestOnTrackExclusionFollowsThePolicyFlags(t *testing.T) {
	students := []KPIStudent{{"GREEN", "active"}, {"RED", "observation"}}
	if kpi := OnTrack(students, map[string]bool{}); kpi.Measurable != 2 || *kpi.Rate != 50 {
		t.Fatalf("no flags: %+v", kpi)
	}
	if kpi := OnTrack(students, map[string]bool{"observation": true}); kpi.Measurable != 1 || *kpi.Rate != 100 {
		t.Fatalf("observation flagged: %+v", kpi)
	}
}
