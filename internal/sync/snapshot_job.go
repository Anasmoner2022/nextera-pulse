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

// DefaultSnapshotInterval is how often the API refreshes pace snapshots when
// SNAPSHOT_INTERVAL is not set.
const DefaultSnapshotInterval = 24 * time.Hour

// SnapshotSkipped is the status entry when another instance was refreshing.
const SnapshotSkipped = "skipped: another instance is refreshing snapshots"

// SnapshotStatus is the secret-free state of the daily snapshot job.
type SnapshotStatus struct {
	IntervalSeconds int    `json:"intervalSeconds"`
	Running         bool   `json:"running"`
	LastSuccessAt   string `json:"lastSuccessAt,omitempty"`
	LastAttemptAt   string `json:"lastAttemptAt,omitempty"`
	NextRunAt       string `json:"nextRunAt,omitempty"`
	LastError       string `json:"lastError,omitempty"`
	LastSkipped     string `json:"lastSkipped,omitempty"`
	Students        int    `json:"students"`
}

// SnapshotJob recomputes the daily pace snapshots from stored data on a fixed
// interval. It never calls 01Edu and runs whether or not a 01Edu credential
// is configured, so curriculum month, active days, pace status and KPIs keep
// moving with the calendar even when no sync happens.
type SnapshotJob struct {
	db       *store.DB
	eventID  int
	interval time.Duration
	lockKey  int64
	now      func() time.Time

	mu          gosync.Mutex
	running     bool
	lastAttempt time.Time
	lastSuccess time.Time
	next        time.Time
	lastErr     string
	lastSkipped bool
	students    int
}

// NewSnapshotJob builds the job. interval is clamped to at least one minute.
func NewSnapshotJob(db *store.DB, eventID int, interval time.Duration) *SnapshotJob {
	if interval < time.Minute {
		interval = time.Minute
	}
	return &SnapshotJob{db: db, eventID: eventID, interval: interval, lockKey: store.LockSnapshots, now: time.Now}
}

// Start refreshes once immediately and then every interval until ctx ends.
func (j *SnapshotJob) Start(ctx context.Context) {
	go func() {
		for {
			j.RunOnce(ctx)
			j.mu.Lock()
			j.next = time.Now().Add(j.interval)
			j.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(j.interval):
			}
		}
	}()
}

// RunOnce refreshes today's snapshots unless another refresh holds the
// snapshot lock, in which case it returns store.ErrLockHeld and records a skip.
func (j *SnapshotJob) RunOnce(ctx context.Context) (int, error) {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return 0, store.ErrLockHeld
	}
	j.running = true
	j.lastAttempt = time.Now().UTC()
	j.mu.Unlock()

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	n, err := j.refresh(runCtx)
	cancel()

	j.mu.Lock()
	defer j.mu.Unlock()
	j.running = false
	switch {
	case errors.Is(err, store.ErrLockHeld):
		j.lastSkipped = true
		log.Printf("snapshot job %s; this run was not done", SnapshotSkipped)
	case err != nil:
		j.lastSkipped, j.lastErr = false, err.Error()
		log.Printf("snapshot job failed: %v", err)
	default:
		j.lastSkipped, j.lastErr, j.students = false, "", n
		j.lastSuccess = time.Now().UTC()
		log.Printf("snapshot job: refreshed %d pace snapshots for event %d", n, j.eventID)
	}
	return n, err
}

func (j *SnapshotJob) refresh(ctx context.Context) (n int, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("snapshot job panicked: %v", p)
		}
	}()
	return RefreshSnapshotsLocked(ctx, j.db, j.eventID, j.now(), false, j.lockKey)
}

// Status returns the job's state for GET /api/v1/sync/status.
func (j *SnapshotJob) Status() SnapshotStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	status := SnapshotStatus{IntervalSeconds: int(j.interval / time.Second), Running: j.running, LastError: j.lastErr, Students: j.students}
	if j.lastSkipped {
		status.LastSkipped = SnapshotSkipped
	}
	if !j.lastSuccess.IsZero() {
		status.LastSuccessAt = j.lastSuccess.Format(time.RFC3339)
	}
	if !j.lastAttempt.IsZero() {
		status.LastAttemptAt = j.lastAttempt.Format(time.RFC3339)
	}
	if !j.next.IsZero() {
		status.NextRunAt = j.next.UTC().Format(time.RFC3339)
	}
	return status
}
