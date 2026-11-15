package pace

import "nextera-pulse/internal/model"

// measurableStatuses are the pace statuses that compare a learner with a
// monthly target. PAUSED, INACTIVE, NOT_MEASURABLE, ONBOARDING and UNKNOWN
// have no comparison, so they are never in the denominator.
var measurableStatuses = map[string]bool{"GREEN": true, "YELLOW": true, "RED": true}

// KPIStudent is what the on-track KPI needs to know about one student.
type KPIStudent struct {
	PaceStatus   string
	SourceStatus string // current 01Edu record type, "active" when none
}

// OnTrack computes the North Star KPI (design definition, review C4):
//
//	on-track rate = GREEN / measurable students
//
// A student is measurable when their pace status is GREEN, YELLOW or RED and
// their source status is not flagged exclude_from_active_kpis in
// status_policy (excludedSources, read from the table by the caller). With
// no measurable students Rate is nil and HasMeasurable false.
func OnTrack(students []KPIStudent, excludedSources map[string]bool) model.OnTrackKPI {
	kpi := model.OnTrackKPI{
		Total:            len(students),
		StatusCounts:     map[string]int{},
		MeasurableCounts: map[string]int{"GREEN": 0, "YELLOW": 0, "RED": 0},
	}
	for _, s := range students {
		status := s.PaceStatus
		if status == "" {
			status = "UNKNOWN"
		}
		kpi.StatusCounts[status]++
		if excludedSources[s.SourceStatus] {
			kpi.ExcludedByPolicy++
			continue
		}
		if !measurableStatuses[status] {
			continue
		}
		kpi.Measurable++
		kpi.MeasurableCounts[status]++
		if status == "GREEN" {
			kpi.OnTrack++
		}
	}
	if kpi.Measurable > 0 {
		rate := float64(kpi.OnTrack) / float64(kpi.Measurable) * 100
		kpi.Rate = &rate
		kpi.HasMeasurable = true
	}
	return kpi
}
