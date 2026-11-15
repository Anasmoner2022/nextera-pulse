package model

import "time"

// Student represents student identity
type Student struct {
	ID           int64     `json:"id"`
	SourceUserID int64     `json:"sourceUserId"`
	Login        string    `json:"login"`
	FirstName    string    `json:"firstName"`
	LastName     string    `json:"lastName"`
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// StudentEvent represents enrollment and level in an event (the configured curriculum event, ZEROONE_EVENT_ID)
type StudentEvent struct {
	ID           int64     `json:"id"`
	StudentID    int64     `json:"studentId"`
	EventID      int       `json:"eventId"`
	CohortID     int       `json:"cohortId"`
	EnrolledAt   time.Time `json:"enrolledAt"`
	CurrentLevel float64   `json:"currentLevel"`
	AuditRatio   float64   `json:"auditRatio"`
	LastSyncedAt time.Time `json:"lastSyncedAt"`
}

// StatusPolicy defines clock pause & KPI exclusion rules for administrative statuses
type StatusPolicy struct {
	RecordType            string `json:"recordType"`
	PauseClock            bool   `json:"pauseClock"`
	ExcludeFromActiveKPIs bool   `json:"excludeFromActiveKpis"`
	Description           string `json:"description"`
}

// CurriculumMilestone represents expected levels by month for an event
type CurriculumMilestone struct {
	EventID         int     `json:"eventId"`
	Month           int     `json:"month"`
	MinLevel        float64 `json:"minLevel"`
	ExpectedLevel   float64 `json:"expectedLevel"`
	CheckpointLevel float64 `json:"checkpointLevel"`
	Rank            int     `json:"rank"`
	Notes           string  `json:"notes"`
}

// StudentRecord represents an administrative record interval (away, blocked-long, etc.)
type StudentRecord struct {
	ID             int64      `json:"id"`
	SourceRecordID int64      `json:"sourceRecordId"`
	StudentID      int64      `json:"studentId"`
	TypeName       string     `json:"typeName"`
	StartAt        time.Time  `json:"startAt"`
	EndAt          *time.Time `json:"endAt,omitempty"`
	Message        string     `json:"message"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// PaceMetrics represents the derived pace evaluation for a student
type PaceMetrics struct {
	EnrolledAt        time.Time `json:"enrolledAt"`
	ActiveProgramDays int       `json:"activeProgramDays"`
	CurriculumMonth   int       `json:"curriculumMonth"`
	ActualLevel       float64   `json:"actualLevel"`
	MinimumLevel      float64   `json:"minimumLevel"`
	ExpectedLevel     float64   `json:"expectedLevel"`
	CheckpointTarget  float64   `json:"checkpointTarget"`
	GapToMinimum      float64   `json:"gapToMinimum"`
	GapToExpected     float64   `json:"gapToExpected"`
	Status            string    `json:"status"` // GREEN, YELLOW, RED, PAUSED, INACTIVE, ONBOARDING
	Reasons           []string  `json:"reasons"`
}

// StudentMetricSnapshot represents an immutable daily snapshot row
type StudentMetricSnapshot struct {
	ID                 int64       `json:"id"`
	StudentID          int64       `json:"studentId"`
	EventID            int         `json:"eventId"`
	SnapshotDate       string      `json:"snapshotDate"` // YYYY-MM-DD
	Pace               PaceMetrics `json:"pace"`
	CalculationVersion string      `json:"calculationVersion"`
	CalculatedAt       time.Time   `json:"calculatedAt"`
}

// Mentor represents assigned mentor info
type Mentor struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StudentPaceView is the full DTO returned to Ziad's frontend
type StudentPaceView struct {
	ID               int64       `json:"id"`
	SourceUserID     int64       `json:"sourceUserId"`
	Login            string      `json:"login"`
	FirstName        string      `json:"firstName"`
	LastName         string      `json:"lastName"`
	Email            string      `json:"email"`
	AuditRatio       float64     `json:"auditRatio"`
	CohortID         int         `json:"cohortId"`
	CohortName       string      `json:"cohortName"`
	Mentor           *Mentor     `json:"mentor,omitempty"`
	SourceStatus     string      `json:"sourceStatus"`
	Pace             PaceMetrics `json:"pace"`
	LastCalculatedAt time.Time   `json:"lastCalculatedAt"`
}

// OnTrackKPI is the North Star KPI: on-track rate = GREEN / measurable
// students. Measurable students have a pace status that compares them with a
// monthly target (GREEN, YELLOW or RED) and a current source status that
// status_policy does not flag exclude_from_active_kpis. Rate is a percentage
// (0–100) and is null when there are no measurable students (HasMeasurable
// false), never 0%.
type OnTrackKPI struct {
	Scope            string         `json:"scope"` // program, cohort or mentor
	EventID          int            `json:"eventId,omitempty"`
	CohortID         int            `json:"cohortId,omitempty"`
	MentorID         int64          `json:"mentorId,omitempty"`
	Name             string         `json:"name,omitempty"`
	OnTrack          int            `json:"onTrack"`    // numerator: measurable GREEN
	Measurable       int            `json:"measurable"` // denominator
	Rate             *float64       `json:"rate"`
	HasMeasurable    bool           `json:"hasMeasurable"`
	Total            int            `json:"total"`            // every student in scope
	ExcludedByPolicy int            `json:"excludedByPolicy"` // source status flagged in status_policy
	StatusCounts     map[string]int `json:"statusCounts"`     // pace status of every student in scope (YELLOW = Watch)
	MeasurableCounts map[string]int `json:"measurableCounts"` // GREEN/YELLOW/RED among measurable students
}

// CohortHealthSummary gives the KPI summary for a cohort.
type CohortHealthSummary struct {
	CohortID           int            `json:"cohortId"`
	CohortName         string         `json:"cohortName"`
	TotalStudents      int            `json:"totalStudents"`
	MeasurableStudents int            `json:"measurableStudents"`
	OnTrackCount       int            `json:"onTrackCount"`   // measurable GREEN
	NearTrackCount     int            `json:"nearTrackCount"` // measurable YELLOW (Watch)
	OffTrackCount      int            `json:"offTrackCount"`  // measurable RED
	OnTrackRate        *float64       `json:"onTrackRate"`    // North Star KPI; null without measurable students
	StatusCounts       map[string]int `json:"statusCounts"`
	KPI                OnTrackKPI     `json:"kpi"`
	CalculatedAt       time.Time      `json:"calculatedAt"`
}
