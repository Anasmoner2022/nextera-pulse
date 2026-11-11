package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"nextera-pulse/internal/app"
	"nextera-pulse/internal/httpapi"
	"nextera-pulse/internal/store"
)

func main() {
	app.LoadEnvFile(".env")
	log.Println("=== Starting Nextera Pulse API Server ===")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://nextera_pulse:nextera_pulse@127.0.0.1:5432/nextera_pulse?sslmode=disable"
		log.Println("Using local PostgreSQL default configuration")
	} else {
		log.Println("Connecting to configured database")
	}

	db, err := store.Connect(dsn)
	if err != nil {
		log.Fatalf("Fatal: Database connection failed: %v", err)
	}
	defer db.Close()

	// Ensure schema & policies exist
	ctx := context.Background()
	if err := db.Migrate(ctx, ""); err != nil {
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

	server := httpapi.NewServer(db)

	if strings.HasPrefix(addr, ":") {
		log.Printf("Nextera Pulse API listening on http://localhost%s", addr)
	} else {
		log.Printf("Nextera Pulse API listening on http://%s", addr)
	}
	log.Printf("Database ready (%s).", db.DriverName)
	log.Println("Available endpoints:")
	log.Println("  - GET  /health/live")
	log.Println("  - GET  /health/ready")
	log.Println("  - GET  /api/v1/students?cohortId=4&status=RED")
	log.Println("  - GET  /api/v1/students/:id")
	log.Println("  - GET  /api/v1/cohorts")
	log.Println("  - GET  /api/v1/cohorts/:id/students")
	log.Println("  - GET  /api/v1/cohorts/:id/health")
	log.Println("  - GET  /api/v1/interventions")
	log.Println("  - POST /api/v1/interventions")

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
