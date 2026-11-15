package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	gosync "sync"
	"time"

	"nextera-pulse/internal/store"
)

// LiveStatus is the public, secret-free view of the live sync.
type LiveStatus struct {
	// Mode is "live" when a credential is configured, otherwise "offline".
	Mode            string           `json:"mode"`
	CredentialMode  string           `json:"credentialMode,omitempty"`
	Role            string           `json:"role,omitempty"`
	CanReadCohort   bool             `json:"canReadCohort"`
	IntervalSeconds int              `json:"intervalSeconds,omitempty"`
	Running         bool             `json:"running"`
	LastSuccessAt   string           `json:"lastSuccessAt,omitempty"`
	LastAttemptAt   string           `json:"lastAttemptAt,omitempty"`
	NextRunAt       string           `json:"nextRunAt,omitempty"`
	LastError       string           `json:"lastError,omitempty"`
	LastReport      *LiveCycleReport `json:"lastReport,omitempty"`
	// LastSkipped is set when the latest scheduled cycle did not run because
	// another instance held the live sync lock.
	LastSkipped   string `json:"lastSkipped,omitempty"`
	LastSkippedAt string `json:"lastSkippedAt,omitempty"`
	// Snapshots is the daily pace snapshot job (runs without a credential too).
	Snapshots *SnapshotStatus `json:"snapshots,omitempty"`
}

// SkippedLocked is the sync status entry for a cycle another instance was running.
const SkippedLocked = "skipped: another instance is syncing"

// CredentialInfo exposes non-secret facts about the configured credential.
type CredentialInfo interface {
	Mode() string
	Role() string
	CanReadCohort() bool
}

// Runner runs RunLiveCycle on a fixed interval inside the API process.
type Runner struct {
	db       *store.DB
	source   LiveSource
	creds    CredentialInfo
	cfg      LiveCycleConfig
	interval time.Duration
	// lockKey is the advisory lock that keeps one live cycle at a time across
	// instances (store.LockLiveSync; tests override it).
	lockKey int64
	// cycle is RunLiveCycle (tests replace it to block, fail or panic).
	cycle func(context.Context, *store.DB, LiveSource, LiveCycleConfig) (LiveCycleReport, error)

	mu          gosync.Mutex
	lastSkipped time.Time
	running     bool
	lastAttempt time.Time
	lastSuccess time.Time
	next        time.Time
	lastErr     string
	lastReport  *LiveCycleReport
}

// NewRunner builds a scheduler. interval is clamped to at least one minute.
func NewRunner(db *store.DB, source LiveSource, creds CredentialInfo, cfg LiveCycleConfig, interval time.Duration) *Runner {
	if interval < time.Minute {
		interval = time.Minute
	}
	return &Runner{db: db, source: source, creds: creds, cfg: cfg, interval: interval, lockKey: store.LockLiveSync, cycle: RunLiveCycle}
}

// Start runs one cycle immediately and then every interval until ctx ends.
func (r *Runner) Start(ctx context.Context) {
	go func() {
		for {
			r.RunOnce(ctx)
			r.mu.Lock()
			r.next = time.Now().Add(r.interval)
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.interval):
			}
		}
	}()
}

// RunOnce runs a single cycle unless one is already running.
func (r *Runner) RunOnce(ctx context.Context) (LiveCycleReport, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return LiveCycleReport{}, nil
	}
	r.running = true
	r.lastAttempt = time.Now().UTC()
	r.mu.Unlock()

	// One live cycle at a time across every API replica and cmd/worker: the
	// cycle runs inside a PostgreSQL advisory lock, released on return, error,
	// panic or cancellation (see store.WithAdvisoryLock).
	cycleCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	var report LiveCycleReport
	err := r.db.WithAdvisoryLock(cycleCtx, r.lockKey, false, func(lockedCtx context.Context) (cycleErr error) {
		defer func() {
			if p := recover(); p != nil {
				cycleErr = fmt.Errorf("live sync cycle panicked: %v", p)
			}
		}()
		report, cycleErr = r.cycle(lockedCtx, r.db, r.source, r.cfg)
		return cycleErr
	})
	cancel()

	if errors.Is(err, store.ErrLockHeld) {
		r.mu.Lock()
		r.running = false
		r.lastSkipped = time.Now().UTC()
		r.mu.Unlock()
		log.Printf("live sync %s; this cycle was not run", SkippedLocked)
		return LiveCycleReport{Status: CycleSkipped}, nil
	}

	r.mu.Lock()
	r.running = false
	r.lastSkipped = time.Time{}
	r.lastReport = &report
	switch {
	case err != nil:
		r.lastErr = err.Error()
	case report.Status == CycleLimited:
		r.lastErr = "the 01Edu credential only returned its own rows; use a campus admin / admin_read_only token to sync the cohort"
	default:
		r.lastErr = ""
		r.lastSuccess = report.FinishedAt
	}
	r.mu.Unlock()

	if err != nil {
		log.Printf("live sync failed: %v", err)
	} else {
		log.Printf("live sync %s: roster=%d levels=%d records=%d progress=%d snapshots=%d", report.Status,
			report.Roster.Applied, report.Levels.Applied, report.Records.Applied, report.Progress.Applied, report.Snapshots.Applied)
		for _, stage := range []StageReport{report.Roster, report.Levels, report.Records, report.Progress, report.Snapshots} {
			if stage.Error != "" {
				log.Printf("live sync stage warning: %s", stage.Error)
			}
		}
	}
	return report, err
}

// LastSuccess is the finish time of the last cohort-wide successful cycle.
func (r *Runner) LastSuccess() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastSuccess
}

// Status returns the secret-free state of the scheduler.
func (r *Runner) Status() LiveStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := LiveStatus{
		Mode:            "live",
		IntervalSeconds: int(r.interval / time.Second),
		Running:         r.running,
		LastError:       r.lastErr,
		LastReport:      r.lastReport,
	}
	if r.creds != nil {
		status.CredentialMode = r.creds.Mode()
		status.Role = r.creds.Role()
		status.CanReadCohort = r.creds.CanReadCohort()
	}
	if !r.lastSuccess.IsZero() {
		status.LastSuccessAt = r.lastSuccess.UTC().Format(time.RFC3339)
	}
	if !r.lastAttempt.IsZero() {
		status.LastAttemptAt = r.lastAttempt.UTC().Format(time.RFC3339)
	}
	if !r.lastSkipped.IsZero() {
		status.LastSkipped = SkippedLocked
		status.LastSkippedAt = r.lastSkipped.Format(time.RFC3339)
	}
	if !r.next.IsZero() {
		status.NextRunAt = r.next.UTC().Format(time.RFC3339)
	}
	return status
}
