package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"nextera-pulse/internal/store"
	pulsesync "nextera-pulse/internal/sync"
	"nextera-pulse/internal/zeroone"
)

// LiveSyncSettings is the non-secret live-sync configuration read from env.
type LiveSyncSettings struct {
	Enabled  bool
	Interval time.Duration
	Cycle    pulsesync.LiveCycleConfig
	Creds    zeroone.Credentials
}

// SnapshotIntervalFromEnv reads SNAPSHOT_INTERVAL: a Go duration between pace
// snapshot refreshes (default 24h, minimum 1m), or "off" to disable the job.
// The job needs no 01Edu credential.
func SnapshotIntervalFromEnv() (interval time.Duration, enabled bool, err error) {
	raw := strings.TrimSpace(os.Getenv("SNAPSHOT_INTERVAL"))
	switch {
	case raw == "":
		return pulsesync.DefaultSnapshotInterval, true, nil
	case strings.EqualFold(raw, "off"):
		return 0, false, nil
	}
	interval, err = time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		return 0, false, fmt.Errorf("invalid SNAPSHOT_INTERVAL %q (use a duration such as 24h, or off)", raw)
	}
	if interval < time.Minute {
		interval = time.Minute
	}
	return interval, true, nil
}

// LiveSyncFromEnv reads the live-sync settings.
//
//	LIVE_SYNC         "off" disables the scheduler even when credentials are set
//	SYNC_INTERVAL     Go duration between cycles (default 30m, minimum 1m)
//	ZEROONE_EVENT_ID  curriculum event (default pulsesync.DefaultEventID)
//	PILOT_COHORTS     Pulse cohort id = 01Edu label name, comma-separated, in priority
//	                  order (default "4=01 Coding - Second Cohort,1=01 Coding - First Cohort")
func LiveSyncFromEnv() (LiveSyncSettings, error) {
	settings := LiveSyncSettings{Creds: zeroone.CredentialsFromEnv(), Interval: 30 * time.Minute}
	settings.Enabled = settings.Creds.Configured() && !strings.EqualFold(strings.TrimSpace(os.Getenv("LIVE_SYNC")), "off")
	if raw := strings.TrimSpace(os.Getenv("SYNC_INTERVAL")); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil || interval <= 0 {
			return settings, fmt.Errorf("invalid SYNC_INTERVAL %q", raw)
		}
		settings.Interval = interval
	}
	settings.Cycle.EventID = pulsesync.DefaultEventID
	if raw := strings.TrimSpace(os.Getenv("ZEROONE_EVENT_ID")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return settings, fmt.Errorf("invalid ZEROONE_EVENT_ID %q", raw)
		}
		settings.Cycle.EventID = id
	}
	cohorts, err := ParseCohorts(os.Getenv("PILOT_COHORTS"))
	if err != nil {
		return settings, err
	}
	settings.Cycle.Cohorts = cohorts
	return settings, nil
}

// NewLiveRunner wires the token source, GraphQL client and scheduler.
func NewLiveRunner(db *store.DB, settings LiveSyncSettings) (*pulsesync.Runner, error) {
	if !settings.Enabled {
		return nil, errors.New("live sync is not enabled")
	}
	tokens, err := zeroone.NewTokenSource(settings.Creds, nil)
	if err != nil {
		return nil, err
	}
	client, err := zeroone.NewClientWithTokens(settings.Creds.GraphQLURL, tokens, &http.Client{Timeout: 60 * time.Second})
	if err != nil {
		return nil, err
	}
	return pulsesync.NewRunner(db, client, tokens, settings.Cycle, settings.Interval), nil
}

// DefaultCohorts is used when PILOT_COHORTS is empty.
const DefaultCohorts = "4=01 Coding - Second Cohort,1=01 Coding - First Cohort"

// ParseCohorts reads "id=label name,id=label name". Label names are the exact
// 01Edu label names that define each Pulse cohort.
func ParseCohorts(raw string) ([]pulsesync.CohortLabel, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pulsesync.DefaultCohorts(), nil
	}
	var cohorts []pulsesync.CohortLabel
	seen := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		idText, label, ok := strings.Cut(part, "=")
		id, err := strconv.Atoi(strings.TrimSpace(idText))
		label = strings.TrimSpace(label)
		if !ok || err != nil || id <= 0 || label == "" || seen[id] {
			return nil, fmt.Errorf("invalid PILOT_COHORTS %q (want \"4=01 Coding - Second Cohort,1=01 Coding - First Cohort\")", raw)
		}
		seen[id] = true
		cohorts = append(cohorts, pulsesync.CohortLabel{ID: id, Label: label})
	}
	return cohorts, nil
}
