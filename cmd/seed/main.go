package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/sync"
)

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

func main() {
	loadEnvFile(".env")
	log.Println("=== Nextera Pulse: Ingestion & Pace Seeder ===")

	// 1. Determine Database DSN (default to Postgres per PM requirements)
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/nextera_pulse?sslmode=disable"
		log.Printf("Defaulting to PostgreSQL database: %s", dsn)
	} else {
		log.Printf("Connecting to database: %s", dsn)
	}

	// 2. Connect to Database
	db, err := store.Connect(dsn)
	if err != nil {
		log.Fatalf("Fatal: Database connection failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 3. Run Migrations & Seed Default Policies / Timeline
	log.Println("Running schema migrations and policy seeds...")
	if err := db.Migrate(ctx, ""); err != nil {
		log.Fatalf("Fatal: Database migration failed: %v", err)
	}
	log.Println("PostgreSQL schema initialized and policy / Event 97 timeline seeded successfully.")

	// 4. Locate Archive File
	zipPath := "NextEra-All-Cohorts-2026-10-19.zip"
	if len(os.Args) > 1 {
		zipPath = os.Args[1]
	}

	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		// Try parent directory
		parentPath := filepath.Join("..", zipPath)
		if _, err := os.Stat(parentPath); err == nil {
			zipPath = parentPath
		} else {
			desktopPath := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "NextEra-All-Cohorts-2026-10-19.zip")
			if _, err := os.Stat(desktopPath); err == nil {
				zipPath = desktopPath
			} else {
				log.Fatalf("Fatal: Cannot find zip archive at %s, %s, or %s", zipPath, parentPath, desktopPath)
			}
		}
	}

	log.Printf("Loading data from archive: %s", zipPath)

	// 5. Ingest Archive
	res, err := sync.IngestZipArchive(ctx, db, zipPath)
	if err != nil {
		log.Fatalf("Fatal: Ingestion failed: %v", err)
	}

	fmt.Println()
	log.Printf("Ingestion completed in %v!", res.Duration)
	log.Printf("  - Students Ingested: %d", res.StudentsCount)
	log.Printf("  - Status Records Ingested: %d", res.RecordsCount)
	log.Printf("  - Daily Pace Snapshots Generated: %d", res.SnapshotsCount)

	// 6. Print Cohort 4 (Pilot) Health Summary
	summary, err := db.GetCohortHealthSummary(ctx, 4)
	if err == nil {
		fmt.Println()
		log.Printf("=== Pilot Cohort 4 (01 Coding - Second Cohort) Pace Summary ===")
		log.Printf("  Total Students:   %d", summary.TotalStudents)
		log.Printf("  Active Students:  %d", summary.ActiveStudents)
		log.Printf("  Paused Students:  %d", summary.PausedStudents)
		log.Printf("  On-Track (GREEN): %d", summary.OnTrackCount)
		log.Printf("  Near-Track (YELLOW): %d", summary.NearTrackCount)
		log.Printf("  Off-Track (RED):  %d", summary.OffTrackCount)
		log.Printf("  Status Breakdown: %v", summary.StatusCounts)
	}

	log.Println("Event 97 export has no curriculum timeline. Pace still uses the seeded month curve.")
	checkRows, err := db.QueryContext(ctx, db.Rebind(`
		SELECT s.login, se.current_level, snap.curriculum_month, snap.minimum_level, snap.expected_level, snap.pace_status
		FROM students s
		JOIN student_events se ON se.student_id = s.id AND se.cohort_id = 4 AND se.event_id = 97
		JOIN student_metric_snapshots snap ON snap.id = (
			SELECT id FROM student_metric_snapshots WHERE student_id = s.id ORDER BY snapshot_date DESC, id DESC LIMIT 1
		)
		ORDER BY s.login
		LIMIT 5`))
	if err == nil {
		defer checkRows.Close()
		log.Println("Five cohort-4 students checked against the seeded curve:")
		for checkRows.Next() {
			var login, status string
			var level, minimum, expected float64
			var month int
			if scanErr := checkRows.Scan(&login, &level, &month, &minimum, &expected, &status); scanErr == nil {
				log.Printf("  %s level %.0f month %d minimum %.0f expected %.0f %s", login, level, month, minimum, expected, status)
			}
		}
	}

	fmt.Println()
	log.Println("Seeding complete. Run 'go run cmd/api/main.go' to start the REST API.")
}
