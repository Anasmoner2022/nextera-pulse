package httpapi

import (
	"context"
	"net/http"
	"time"

	"nextera-pulse/internal/store"
	pulsesync "nextera-pulse/internal/sync"
)

// handleSyncStatus reports where dashboard data comes from. It never returns
// tokens, logins, or student data. Students get only the freshness fields;
// credential mode, role, errors and stage reports are for staff.
func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	var status pulsesync.LiveStatus
	if s.live != nil {
		status = s.live.Status()
	} else {
		// No 01Edu credential on this server: the data is whatever the last live
		// sync stored (there is no file import).
		status = pulsesync.LiveStatus{Mode: "offline", LastSuccessAt: s.lastLiveCycleAt(r.Context())}
	}
	if s.snapshots != nil {
		snap := s.snapshots.Status()
		status.Snapshots = &snap
	}
	if p, _ := principalOf(r.Context()); p.Role == roleStudent {
		status = pulsesync.LiveStatus{
			Mode:            status.Mode,
			IntervalSeconds: status.IntervalSeconds,
			Running:         status.Running,
			LastSuccessAt:   status.LastSuccessAt,
			NextRunAt:       status.NextRunAt,
		}
	}
	writeJSON(w, http.StatusOK, status)
}

// dataFreshness labels student responses as live (with the last cohort-wide
// sync time) or offline (no 01Edu credential configured on this server).
func (s *Server) dataFreshness(ctx context.Context) (source, syncedAt string) {
	if s.live != nil {
		if last := s.live.LastSuccess(); !last.IsZero() {
			return "live", last.UTC().Format(time.RFC3339)
		}
		if last := s.lastLiveCycleAt(ctx); last != "" {
			return "live", last
		}
	}
	return "offline", s.lastLiveCycleAt(ctx)
}

func (s *Server) lastLiveCycleAt(ctx context.Context) string {
	return s.lastSyncRun(ctx, `source = 'graphql_live_cycle' AND status = 'SUCCESS'`)
}

func (s *Server) lastSyncRun(ctx context.Context, where string) string {
	var completed store.FlexibleTime
	err := s.db.QueryRowContext(ctx, `SELECT MAX(completed_at) FROM sync_runs WHERE completed_at IS NOT NULL AND `+where).Scan(&completed)
	if err != nil || completed.Time.IsZero() {
		return ""
	}
	return completed.Time.UTC().Format(time.RFC3339)
}
