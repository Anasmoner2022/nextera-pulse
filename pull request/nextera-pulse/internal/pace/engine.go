package pace

import (
	"sort"
	"time"

	"nextera-pulse/internal/model"
)

// Engine calculates pace metrics deterministically
type Engine struct {
	Policies     map[string]model.StatusPolicy
	Milestones   map[int]model.CurriculumMilestone
	sortedMonths []int
}

// NewEngine initializes a Pace Engine with policies and milestone expectations
func NewEngine(policies map[string]model.StatusPolicy, milestones map[int]model.CurriculumMilestone) *Engine {
	var months []int
	for m := range milestones {
		months = append(months, m)
	}
	sort.Ints(months)

	return &Engine{
		Policies:     policies,
		Milestones:   milestones,
		sortedMonths: months,
	}
}

// Calculate computes the PaceMetrics for a student given their records and level
func (e *Engine) Calculate(enrolledAt time.Time, actualLevel float64, records []model.StudentRecord, now time.Time) model.PaceMetrics {
	if enrolledAt.After(now) {
		enrolledAt = now
	}

	// Count the union of approved pause intervals, not the sum of overlapping records.
	// A future record must never mark a student as currently paused.
	type interval struct{ start, end time.Time }
	var intervals []interval
	isCurrentlyPaused := false
	isLeftSchool := false
	for _, r := range records {
		policy, exists := e.Policies[r.TypeName]
		if !exists || r.StartAt.After(now) {
			continue
		}
		if r.TypeName == "left-school" {
			isLeftSchool = true
		}
		if !policy.PauseClock {
			continue
		}
		end := now
		if r.EndAt != nil && r.EndAt.Before(now) {
			end = *r.EndAt
		} else if r.TypeName != "left-school" {
			isCurrentlyPaused = true
		}
		start := r.StartAt
		if start.Before(enrolledAt) {
			start = enrolledAt
		}
		if end.After(start) {
			intervals = append(intervals, interval{start, end})
		}
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start.Before(intervals[j].start) })
	var pauseDuration time.Duration
	if len(intervals) > 0 {
		start, end := intervals[0].start, intervals[0].end
		for _, next := range intervals[1:] {
			if !next.start.After(end) {
				if next.end.After(end) {
					end = next.end
				}
			} else {
				pauseDuration += end.Sub(start)
				start, end = next.start, next.end
			}
		}
		pauseDuration += end.Sub(start)
	}
	pauseDays := int(pauseDuration.Hours() / 24)

	// 2. Calculate active days & curriculum month
	totalDays := int(now.Sub(enrolledAt).Hours() / 24)
	if totalDays < 0 {
		totalDays = 0
	}

	activeDays := totalDays - pauseDays
	if activeDays < 0 {
		activeDays = 0
	}

	curriculumMonth := (activeDays / 30) + 1

	// 3. Resolve target expectations for curriculum month
	target := e.resolveTarget(curriculumMonth)

	// 4. Calculate gaps
	gapToMin := actualLevel - target.MinLevel
	gapToExp := actualLevel - target.ExpectedLevel

	// 5. Determine pace status and reason codes
	var status string
	var reasons []string

	if isLeftSchool {
		status = "INACTIVE"
		reasons = append(reasons, "STUDENT_LEFT_SCHOOL")
	} else if isCurrentlyPaused {
		status = "PAUSED"
		reasons = append(reasons, "ADMINISTRATIVE_PAUSE")
	} else if curriculumMonth == 1 {
		// First 30 days are onboarding period
		if actualLevel >= target.ExpectedLevel && target.ExpectedLevel > 0 {
			status = "GREEN"
		} else {
			status = "ONBOARDING"
		}
	} else if len(e.sortedMonths) == 0 {
		status = "NOT_MEASURABLE"
	} else if actualLevel >= target.ExpectedLevel {
		status = "GREEN"
	} else if actualLevel >= target.MinLevel {
		status = "YELLOW"
		reasons = append(reasons, "PACE_BELOW_TARGET")
	} else {
		status = "RED"
		reasons = append(reasons, "PACE_BELOW_MINIMUM")
	}

	return model.PaceMetrics{
		EnrolledAt:        enrolledAt,
		ActiveProgramDays: activeDays,
		CurriculumMonth:   curriculumMonth,
		ActualLevel:       actualLevel,
		MinimumLevel:      target.MinLevel,
		ExpectedLevel:     target.ExpectedLevel,
		CheckpointTarget:  target.CheckpointLevel,
		GapToMinimum:      gapToMin,
		GapToExpected:     gapToExp,
		Status:            status,
		Reasons:           reasons,
	}
}

// resolveTarget matches the milestone targets for a given curriculum month
func (e *Engine) resolveTarget(month int) model.CurriculumMilestone {
	if m, exists := e.Milestones[month]; exists {
		return m
	}

	// Month 1 before month 2 milestone
	if month < 2 {
		return model.CurriculumMilestone{
			Month:           month,
			MinLevel:        0.0,
			ExpectedLevel:   0.0,
			CheckpointLevel: 0.0,
		}
	}

	// If between milestones, use the highest defined milestone <= month
	var closestMonth int
	for _, m := range e.sortedMonths {
		if m <= month {
			closestMonth = m
		} else {
			break
		}
	}

	if closestMonth > 0 {
		return e.Milestones[closestMonth]
	}

	// Fallback to highest milestone if past month 18
	if len(e.sortedMonths) > 0 {
		return e.Milestones[e.sortedMonths[len(e.sortedMonths)-1]]
	}

	return model.CurriculumMilestone{Month: month}
}
