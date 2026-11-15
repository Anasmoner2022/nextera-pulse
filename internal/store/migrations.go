package store

import (
	"context"
	"fmt"

	"nextera-pulse/migrations"
)

// Migrate applies the embedded PostgreSQL schema and adds any missing
// default status policies. Every statement is idempotent, and nothing an
// operator changed in the database is overwritten. Monthly targets are seeded
// per curriculum event with SeedCurriculumTimeline.
func (db *DB) Migrate(ctx context.Context) error {
	schema := migrations.Postgres
	if db.DriverName == "sqlite" {
		schema = migrations.SQLite
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}
	if err := db.SeedStatusPolicy(ctx); err != nil {
		return fmt.Errorf("failed to seed status policy: %w", err)
	}
	return nil
}

// SeedStatusPolicy inserts the default status policies that are missing.
// Existing rows are never changed, so edits made in the database survive a
// restart.
func (db *DB) SeedStatusPolicy(ctx context.Context) error {
	policies := []struct {
		recordType            string
		pauseClock            bool
		excludeFromActiveKPIs bool
		description           string
	}{
		{"away", false, false, "Temporary approved absence; academic clock runs"},
		{"restricted", true, true, "Temporary restriction; pauses academic clock and excluded from active KPIs"},
		{"blocked-long", true, true, "Extended administrative block; pauses academic clock and excluded from active KPIs"},
		{"temporary-notice", false, false, "Informational annotation; clock runs"},
		{"left-school", true, true, "Terminal withdrawal; inactive"},
		{"observation", false, false, "Academic observation; clock runs"},
		{"ban", true, true, "Disciplinary ban; pauses clock and excluded from active KPIs"},
	}

	for _, p := range policies {
		query := db.Rebind(`INSERT INTO status_policy (record_type, pause_clock, exclude_from_active_kpis, description)
                     VALUES (?, ?, ?, ?)
                     ON CONFLICT (record_type) DO NOTHING`)

		_, err := db.ExecContext(ctx, query, p.recordType, p.pauseClock, p.excludeFromActiveKPIs, p.description)
		if err != nil {
			return fmt.Errorf("failed inserting status policy %s: %w", p.recordType, err)
		}
	}

	return nil
}
