package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"nextera-pulse/internal/model"
)

// Timeline version statuses.
const (
	TimelineDraft    = "draft"
	TimelineApproved = "approved"
	TimelineRetired  = "retired"
)

// PlaceholderTimelineLabel is the version label of the seeded placeholder
// targets.
const PlaceholderTimelineLabel = "draft-v0"

// TimelineVersion identifies the set of monthly targets the pace engine uses.
type TimelineVersion struct {
	ID         int64      `json:"id"`
	EventID    int        `json:"eventId"`
	Label      string     `json:"version"`
	Status     string     `json:"status"`
	Approved   bool       `json:"approved"`
	Notes      string     `json:"notes,omitempty"`
	ApprovedBy string     `json:"approvedBy,omitempty"`
	ApprovedAt *time.Time `json:"approvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// placeholderTimeline are the monthly targets that were hard-coded before
// targets became versioned data. They are PLACEHOLDERS awaiting Product
// sign-off, NOT the official Event timeline (review C2 / XP-004). Do not edit
// them here: Product supplies approved targets as a new version (see
// docs/PACE_TARGETS.md).
var placeholderTimeline = []model.CurriculumMilestone{
	{Month: 2, MinLevel: 5, ExpectedLevel: 7, CheckpointLevel: 0, Rank: 1, Notes: "Month 2 Milestone"},
	{Month: 4, MinLevel: 9, ExpectedLevel: 13, CheckpointLevel: 0, Rank: 2, Notes: "Month 4 Milestone"},
	{Month: 5, MinLevel: 12, ExpectedLevel: 17, CheckpointLevel: 10, Rank: 3, Notes: "Month 5 Milestone"},
	{Month: 6, MinLevel: 15, ExpectedLevel: 18, CheckpointLevel: 20, Rank: 4, Notes: "Month 6 Milestone"},
	{Month: 7, MinLevel: 17, ExpectedLevel: 20, CheckpointLevel: 20, Rank: 5, Notes: "Month 7 Milestone"},
	{Month: 8, MinLevel: 20, ExpectedLevel: 23, CheckpointLevel: 30, Rank: 6, Notes: "Month 8 Milestone"},
	{Month: 10, MinLevel: 28, ExpectedLevel: 34, CheckpointLevel: 30, Rank: 7, Notes: "Month 10 Milestone"},
	{Month: 12, MinLevel: 34, ExpectedLevel: 38, CheckpointLevel: 40, Rank: 8, Notes: "Month 12 Milestone"},
	{Month: 18, MinLevel: 50, ExpectedLevel: 62, CheckpointLevel: 50, Rank: 9, Notes: "Month 18 Graduation Target"},
}

// SeedCurriculumTimeline stores the placeholder targets as version
// "draft-v0" (status draft) for eventID, but only when that event has no
// timeline version at all. It never changes existing versions or rows, so
// targets edited or approved in the database survive every restart.
func (db *DB) SeedCurriculumTimeline(ctx context.Context, eventID int) error {
	if eventID <= 0 {
		return fmt.Errorf("seed timeline: invalid event id %d", eventID)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var versionID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO curriculum_timeline_versions (event_id, label, status, notes)
		SELECT $1, $2, 'draft', 'Pre-existing placeholder targets awaiting Product sign-off. NOT official.'
		WHERE NOT EXISTS (SELECT 1 FROM curriculum_timeline_versions WHERE event_id = $1)
		ON CONFLICT (event_id, label) DO NOTHING
		RETURNING id`, eventID, PlaceholderTimelineLabel).Scan(&versionID)
	if err == sql.ErrNoRows {
		return nil // the event already has targets
	}
	if err != nil {
		return err
	}
	for _, m := range placeholderTimeline {
		if _, err := tx.ExecContext(ctx, `INSERT INTO curriculum_timeline (event_id, version_id, month, min_level, expected_level, checkpoint_level, rank, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, eventID, versionID, m.Month, m.MinLevel, m.ExpectedLevel, m.CheckpointLevel, m.Rank, m.Notes); err != nil {
			return fmt.Errorf("seed timeline month %d: %w", m.Month, err)
		}
	}
	return tx.Commit()
}

// ActiveTimelineVersion returns the version the pace engine uses for
// eventID: the newest approved version, else the newest draft. Retired
// versions are never used. ok is false when the event has no usable version.
func (db *DB) ActiveTimelineVersion(ctx context.Context, eventID int) (TimelineVersion, bool, error) {
	var v TimelineVersion
	var approvedBy sql.NullString
	var approvedAt sql.NullTime
	err := db.QueryRowContext(ctx, `SELECT id, event_id, label, status, notes, approved_by, approved_at, created_at
		FROM curriculum_timeline_versions
		WHERE event_id = $1 AND status IN ('approved', 'draft')
		ORDER BY (status = 'approved') DESC, created_at DESC, id DESC
		LIMIT 1`, eventID).Scan(&v.ID, &v.EventID, &v.Label, &v.Status, &v.Notes, &approvedBy, &approvedAt, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return TimelineVersion{EventID: eventID}, false, nil
	}
	if err != nil {
		return TimelineVersion{}, false, err
	}
	v.Approved = v.Status == TimelineApproved
	v.ApprovedBy = approvedBy.String
	if approvedAt.Valid {
		t := approvedAt.Time
		v.ApprovedAt = &t
	}
	return v, true, nil
}

// ActiveTimeline returns the active version for eventID and its monthly
// targets keyed by month.
func (db *DB) ActiveTimeline(ctx context.Context, eventID int) (TimelineVersion, map[int]model.CurriculumMilestone, error) {
	milestones := map[int]model.CurriculumMilestone{}
	v, ok, err := db.ActiveTimelineVersion(ctx, eventID)
	if err != nil || !ok {
		return v, milestones, err
	}
	rows, err := db.QueryContext(ctx, `SELECT event_id, month, min_level, expected_level, checkpoint_level, COALESCE(rank, 0), COALESCE(notes, '')
		FROM curriculum_timeline WHERE version_id = $1 ORDER BY month`, v.ID)
	if err != nil {
		return v, milestones, err
	}
	defer rows.Close()
	for rows.Next() {
		var m model.CurriculumMilestone
		if err := rows.Scan(&m.EventID, &m.Month, &m.MinLevel, &m.ExpectedLevel, &m.CheckpointLevel, &m.Rank, &m.Notes); err != nil {
			return v, milestones, err
		}
		milestones[m.Month] = m
	}
	return v, milestones, rows.Err()
}
