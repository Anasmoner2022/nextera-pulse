package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"nextera-pulse/internal/httpapi"
	"nextera-pulse/internal/store"
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
	log.Println("=== Starting Nextera Pulse API Server ===")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/nextera_pulse?sslmode=disable"
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	server := httpapi.NewServer(db)

	log.Printf("Nextera Pulse API listening on http://localhost%s", addr)
	log.Println("Connected to PostgreSQL (nextera_pulse). Ready for Ziad's Frontend.")
	log.Println("Available endpoints:")
	log.Println("  - GET  /health/live")
	log.Println("  - GET  /health/ready")
	log.Println("  - GET  /api/v1/students?cohortId=4&status=RED")
	log.Println("  - GET  /api/v1/students/:id")
	log.Println("  - GET  /api/v1/cohorts")
	log.Println("  - GET  /api/v1/cohorts/:id/health")
	log.Println("  - GET  /api/v1/interventions")
	log.Println("  - POST /api/v1/interventions")

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
