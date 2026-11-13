// One-shot (default) or scheduled (-watch) 01Edu live sync: levels, administrative
// records and project progress for students already on the Pulse roster, followed
// by a pace snapshot rebuild. The API runs the same cycle on SYNC_INTERVAL when
// credentials are set; use this command for cron jobs or a manual refresh.
//
// -snapshots refreshes today's pace snapshots from stored data once and exits.
// It needs no 01Edu credential and never calls 01Edu. The worker does not
// schedule snapshots itself: the API does that (SNAPSHOT_INTERVAL). Both the
// sync cycle and the snapshot refresh hold PostgreSQL advisory locks, so the
// worker can run next to the API without two cycles or refreshes overlapping.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nextera-pulse/internal/app"
	"nextera-pulse/internal/store"
	pulsesync "nextera-pulse/internal/sync"
)

func main() {
	watch := flag.Bool("watch", false, "keep running and sync every SYNC_INTERVAL")
	snapshotsOnly := flag.Bool("snapshots", false, "refresh today's pace snapshots from stored data once (no 01Edu calls, no credential needed) and exit")
	flag.Parse()
	app.LoadEnvFile(".env")

	settings, err := app.LiveSyncFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if !settings.Enabled && !*snapshotsOnly {
		log.Fatal("live sync needs ZEROONE_ACCESS_TOKEN (Gitea app token or JWT) or ZEROONE_LOGIN + ZEROONE_PASSWORD")
	}
	dsn := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		dsn = "postgres://nextera_pulse:nextera_pulse@127.0.0.1:5432/nextera_pulse?sslmode=disable"
	}
	if err := store.ValidateDSN(dsn); err != nil {
		log.Fatal(err)
	}
	db, err := store.Connect(dsn)
	if err != nil {
		log.Fatal("database connection failed")
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
	if err := db.SeedCurriculumTimeline(ctx, settings.Cycle.EventID); err != nil {
		log.Fatalf("seeding monthly targets failed: %v", err)
	}
	if *snapshotsOnly {
		n, err := pulsesync.RefreshSnapshotsLocked(ctx, db, settings.Cycle.EventID, time.Now(), false, 0)
		if errors.Is(err, store.ErrLockHeld) {
			log.Printf("snapshot refresh %s", pulsesync.SnapshotSkipped)
			return
		}
		if err != nil {
			log.Fatalf("snapshot refresh failed: %v", err)
		}
		log.Printf("refreshed %d pace snapshots for event %d", n, settings.Cycle.EventID)
		return
	}
	runner, err := app.NewLiveRunner(db, settings)
	if err != nil {
		log.Fatal(err)
	}
	if *watch {
		runner.Start(ctx)
		<-ctx.Done()
		return
	}
	report, err := runner.RunOnce(ctx)
	if report.Status == pulsesync.CycleSkipped {
		log.Printf("live sync %s", pulsesync.SkippedLocked)
		return
	}
	summary, _ := json.MarshalIndent(report, "", "  ")
	log.Printf("live sync report:\n%s", summary)
	if err != nil {
		log.Fatalf("live sync failed: %v", err)
	}
	status := runner.Status()
	if status.LastError != "" {
		log.Printf("warning: %s", status.LastError)
	}
}
