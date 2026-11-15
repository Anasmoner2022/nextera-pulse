package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nextera-pulse/internal/app"
	"nextera-pulse/internal/httpapi"
	"nextera-pulse/internal/store"
	pulsesync "nextera-pulse/internal/sync"
)

func main() {
	app.LoadEnvFile(".env")
	log.Println("=== Starting Nextera Pulse API Server ===")

	dsn := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		dsn = "postgres://nextera_pulse:nextera_pulse@127.0.0.1:5432/nextera_pulse?sslmode=disable"
		log.Println("Using local PostgreSQL default configuration")
	}
	if err := store.ValidateDSN(dsn); err != nil {
		log.Fatalf("Fatal: %v", err)
	}
	driver, _ := store.DriverFor(dsn)
	log.Printf("Connecting to %s at %s", driver, store.RedactDSN(dsn))

	db, err := store.Connect(dsn)
	if err != nil {
		log.Fatalf("Fatal: Database connection failed: %v", err)
	}
	defer db.Close()

	// Ensure schema & policies exist. ctx ends on Ctrl-C/SIGTERM so background
	// jobs stop and release their advisory locks.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		addr = port
		if !strings.Contains(addr, ":") {
			addr = ":" + addr
		}
	}

	opts, err := httpapi.OptionsFromEnv()
	if err != nil {
		log.Fatalf("Session configuration: %v", err)
	}
	if !opts.SecureCookie {
		log.Println("WARNING: SESSION_COOKIE_SECURE=false; session cookies are sent over plain HTTP (local development only).")
	}

	// Scheduled 01Edu live sync (server-side only; credentials never reach the browser).
	liveSettings, err := app.LiveSyncFromEnv()
	if err != nil {
		log.Fatalf("Live sync configuration: %v", err)
	}
	server := httpapi.NewServerWithOptions(db, opts)
	server.SetProgram(liveSettings.Cycle) // ZEROONE_EVENT_ID and PILOT_COHORTS, also without a credential

	// Placeholder monthly targets (draft-v0) only if this event has none yet;
	// never overwrites targets already in the database.
	if err := db.SeedCurriculumTimeline(ctx, liveSettings.Cycle.EventID); err != nil {
		log.Fatalf("Seeding monthly targets failed: %v", err)
	}
	if version, ok, err := db.ActiveTimelineVersion(ctx, liveSettings.Cycle.EventID); err == nil && ok && !version.Approved {
		log.Printf("WARNING: pace rule for event %d uses timeline %s (%s), not approved by Product. See docs/PACE_TARGETS.md.", version.EventID, version.Label, version.Status)
	}
	// Daily pace snapshots from stored data: runs at startup and every
	// SNAPSHOT_INTERVAL (default 24h), with or without a 01Edu credential.
	if interval, enabled, err := app.SnapshotIntervalFromEnv(); err != nil {
		log.Fatalf("Snapshot job configuration: %v", err)
	} else if enabled {
		job := pulsesync.NewSnapshotJob(db, liveSettings.Cycle.EventID, interval)
		server.SetSnapshotJob(job)
		job.Start(ctx)
		log.Printf("Pace snapshot job enabled (every %s, event %d; no 01Edu calls)", interval, liveSettings.Cycle.EventID)
	} else {
		log.Println("Pace snapshot job disabled (SNAPSHOT_INTERVAL=off); snapshots only change after a live sync.")
	}
	if liveSettings.Enabled {
		runner, err := app.NewLiveRunner(db, liveSettings)
		if err != nil {
			log.Fatalf("Live sync setup failed: %v", err)
		}
		server.SetLiveSync(runner)
		runner.Start(ctx)
		log.Printf("01Edu live sync enabled (credential: %s, every %s, event %d, cohorts %v)",
			liveSettings.Creds.Mode(), liveSettings.Interval, liveSettings.Cycle.EventID, liveSettings.Cycle.CohortIDs())
	} else {
		log.Println("01Edu live sync disabled: set ZEROONE_ACCESS_TOKEN (or ZEROONE_LOGIN/ZEROONE_PASSWORD). Serving the data from the last live sync, if any.")
	}

	if strings.HasPrefix(addr, ":") {
		log.Printf("Nextera Pulse API listening on http://localhost%s", addr)
	} else {
		log.Printf("Nextera Pulse API listening on http://%s", addr)
	}
	log.Println("PostgreSQL schema ready.")
	log.Println("Available endpoints:")
	log.Println("  - POST /api/v1/session (01Edu sign-in; every other /api route needs the session cookie)")
	log.Println("  - GET  /health/live")
	log.Println("  - GET  /health/ready")
	log.Println("  - GET  /api/v1/students?cohortId=4&status=RED")
	log.Println("  - GET  /api/v1/students/:id")
	log.Println("  - GET  /api/v1/program")
	log.Println("  - GET  /api/v1/cohorts")
	log.Println("  - GET  /api/v1/cohorts/:id/students")
	log.Println("  - GET  /api/v1/cohorts/:id/health")
	log.Println("  - GET  /api/v1/kpi/on-track")
	log.Println("  - GET  /api/v1/sync/status")
	log.Println("  - GET  /api/v1/interventions")
	log.Println("  - POST /api/v1/interventions")

	httpServer := &http.Server{Addr: addr, Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		log.Println("Shutting down: stopping background jobs and the HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server stopped: %v", err)
	}
}
