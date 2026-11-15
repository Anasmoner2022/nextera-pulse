package sync

// DefaultEventID is the 01Edu curriculum event used when ZEROONE_EVENT_ID is
// not set. This is the only place the default event id is written down;
// everything else reads the configured LiveCycleConfig.EventID.
const DefaultEventID = 97

// defaultCohorts are the Pulse cohorts used when PILOT_COHORTS is not set.
var defaultCohorts = []CohortLabel{
	{ID: 4, Label: "01 Coding - Second Cohort"},
	{ID: 1, Label: "01 Coding - First Cohort"},
}

// DefaultProgram is the event and cohorts used when nothing is configured.
func DefaultProgram() LiveCycleConfig {
	return LiveCycleConfig{EventID: DefaultEventID, Cohorts: DefaultCohorts()}
}

// DefaultCohorts returns a copy of the default cohort list.
func DefaultCohorts() []CohortLabel {
	return append([]CohortLabel(nil), defaultCohorts...)
}

// DefaultCohortID is the pilot cohort: the first configured one.
func (c LiveCycleConfig) DefaultCohortID() int {
	if len(c.Cohorts) == 0 {
		return 0
	}
	return c.Cohorts[0].ID
}

// CohortName returns the 01Edu label name of a configured cohort.
func (c LiveCycleConfig) CohortName(id int) (string, bool) {
	for _, cohort := range c.Cohorts {
		if cohort.ID == id {
			return cohort.Label, true
		}
	}
	return "", false
}

// ResolveCohort maps the id in /api/v1/cohorts/{id}/...: the configured event
// id selects the pilot (first) cohort; a configured cohort id selects that
// cohort. Both return the configured event.
func (c LiveCycleConfig) ResolveCohort(pathID int) (cohortID, eventID int, ok bool) {
	if pathID <= 0 || c.EventID <= 0 {
		return 0, 0, false
	}
	if pathID == c.EventID {
		if id := c.DefaultCohortID(); id > 0 {
			return id, c.EventID, true
		}
		return 0, 0, false
	}
	if _, found := c.CohortName(pathID); found {
		return pathID, c.EventID, true
	}
	return 0, 0, false
}
