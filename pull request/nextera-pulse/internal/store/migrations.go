package store

import (
	"context"
	"fmt"
	"os"
)

// Migrate runs schema migrations and seeds default policies & timeline
func (db *DB) Migrate(ctx context.Context, migrationPath string) error {
	var sqlContent []byte
	var err error

	if migrationPath != "" {
		sqlContent, err = os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
		}
	} else {
		// Default to embedded DDL string based on driver
		if db.DriverName == "postgres" {
			sqlContent, err = os.ReadFile("migrations/postgres.sql")
		} else {
			sqlContent, err = os.ReadFile("migrations/sqlite.sql")
		}
		if err != nil {
			return fmt.Errorf("failed to load migration: %w", err)
		}
	}

	_, err = db.ExecContext(ctx, string(sqlContent))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Seed status policies
	if err := db.SeedStatusPolicy(ctx); err != nil {
		return fmt.Errorf("failed to seed status policy: %w", err)
	}

	// Seed Event 97 official curriculum timeline
	if err := db.SeedCurriculumTimeline(ctx); err != nil {
		return fmt.Errorf("failed to seed curriculum timeline: %w", err)
	}

	return nil
}

// SeedStatusPolicy inserts standard status policy if not present
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
		var query string
		if db.DriverName == "postgres" {
			query = `INSERT INTO status_policy (record_type, pause_clock, exclude_from_active_kpis, description)
                     VALUES ($1, $2, $3, $4)
                     ON CONFLICT (record_type) DO UPDATE SET
                        pause_clock = EXCLUDED.pause_clock,
                        exclude_from_active_kpis = EXCLUDED.exclude_from_active_kpis,
                        description = EXCLUDED.description;`
		} else {
			query = `INSERT INTO status_policy (record_type, pause_clock, exclude_from_active_kpis, description)
                     VALUES (?, ?, ?, ?)
                     ON CONFLICT (record_type) DO UPDATE SET
                        pause_clock = excluded.pause_clock,
                        exclude_from_active_kpis = excluded.exclude_from_active_kpis,
                        description = excluded.description;`
		}

		_, err := db.ExecContext(ctx, query, p.recordType, p.pauseClock, p.excludeFromActiveKPIs, p.description)
		if err != nil {
			return fmt.Errorf("failed inserting status policy %s: %w", p.recordType, err)
		}
	}

	return nil
}

// SeedCurriculumTimeline inserts official Event 97 monthly expectations
func (db *DB) SeedCurriculumTimeline(ctx context.Context) error {
	timeline := []struct {
		month           int
		minLevel        float64
		expectedLevel   float64
		checkpointLevel float64
		rank            int
		notes           string
	}{
		{2, 5.0, 7.0, 0.0, 1, "Month 2 Milestone"},
		{4, 9.0, 13.0, 0.0, 2, "Month 4 Milestone"},
		{5, 12.0, 17.0, 10.0, 3, "Month 5 Milestone"},
		{6, 15.0, 18.0, 20.0, 4, "Month 6 Milestone"},
		{7, 17.0, 20.0, 20.0, 5, "Month 7 Milestone"},
		{8, 20.0, 23.0, 30.0, 6, "Month 8 Milestone"},
		{10, 28.0, 34.0, 30.0, 7, "Month 10 Milestone"},
		{12, 34.0, 38.0, 40.0, 8, "Month 12 Milestone"},
		{18, 50.0, 62.0, 50.0, 9, "Month 18 Graduation Target"},
	}

	eventID := 97
	for _, m := range timeline {
		var query string
		if db.DriverName == "postgres" {
			query = `INSERT INTO curriculum_timeline (event_id, month, min_level, expected_level, checkpoint_level, rank, notes)
                     VALUES ($1, $2, $3, $4, $5, $6, $7)
                     ON CONFLICT (event_id, month) DO UPDATE SET
                        min_level = EXCLUDED.min_level,
                        expected_level = EXCLUDED.expected_level,
                        checkpoint_level = EXCLUDED.checkpoint_level,
                        rank = EXCLUDED.rank,
                        notes = EXCLUDED.notes;`
		} else {
			query = `INSERT INTO curriculum_timeline (event_id, month, min_level, expected_level, checkpoint_level, rank, notes)
                     VALUES (?, ?, ?, ?, ?, ?, ?)
                     ON CONFLICT (event_id, month) DO UPDATE SET
                        min_level = excluded.min_level,
                        expected_level = excluded.expected_level,
                        checkpoint_level = excluded.checkpoint_level,
                        rank = excluded.rank,
                        notes = excluded.notes;`
		}

		_, err := db.ExecContext(ctx, query, eventID, m.month, m.minLevel, m.expectedLevel, m.checkpointLevel, m.rank, m.notes)
		if err != nil {
			return fmt.Errorf("failed inserting timeline for month %d: %w", m.month, err)
		}
	}

	return nil
}
