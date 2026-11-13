package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/pace"
	"nextera-pulse/internal/store"
)

// IngestResult summarizes the results of an ingestion run
type IngestResult struct {
	StudentsCount int
	RecordsCount  int
	SnapshotsCount int
	Duration      time.Duration
}

// IngestZipArchive parses Anas's export zip file and loads everything into the database
func IngestZipArchive(ctx context.Context, db *store.DB, zipFilePath string) (*IngestResult, error) {
	start := time.Now()
	r, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file %s: %w", zipFilePath, err)
	}
	defer r.Close()

	fileMap := make(map[string]*zip.File)
	for _, f := range r.File {
		fileMap[f.Name] = f
	}

	result := &IngestResult{}

	// 1. Ingest Event 97 from json/event.json
	if eventFile, exists := fileMap["json/event.json"]; exists {
		if err := ingestEvents(ctx, db, eventFile); err != nil {
			log.Printf("warning: error ingesting events: %v", err)
		}
	}

	// 2. Ingest Students & Enrollments from All-Cohorts-Roster.csv (or Cohort-4-Roster.csv)
	rosterFile := fileMap["All-Cohorts-Roster.csv"]
	if rosterFile == nil {
		rosterFile = fileMap["Cohort-4-Roster.csv"]
	}
	if rosterFile == nil {
		return nil, fmt.Errorf("no roster CSV found in archive")
	}

	studentIDMap, err := ingestRoster(ctx, db, rosterFile)
	if err != nil {
		return nil, fmt.Errorf("failed ingesting roster: %w", err)
	}
	result.StudentsCount = len(studentIDMap)
	log.Printf("Successfully ingested %d students", result.StudentsCount)

	// 3. Ingest Administrative Records from json/record.json
	if recordFile, exists := fileMap["json/record.json"]; exists {
		count, err := ingestRecords(ctx, db, recordFile, studentIDMap)
		if err != nil {
			log.Printf("warning: error ingesting records: %v", err)
		} else {
			result.RecordsCount = count
			log.Printf("Successfully ingested %d administrative records", count)
		}
	}

	// 4. Seed default mentors if not present
	if err := seedMentors(ctx, db, studentIDMap); err != nil {
		log.Printf("warning: error seeding mentors: %v", err)
	}

	// 5. Run Pace Engine calculation and create daily snapshots
	snapshotCount, err := generateSnapshots(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed generating initial pace snapshots: %w", err)
	}
	result.SnapshotsCount = snapshotCount
	result.Duration = time.Since(start)

	// 6. Record sync audit row
	_, _ = db.ExecContext(ctx, db.Rebind(`INSERT INTO sync_runs (source, status, records_ingested, started_at, completed_at)
                                          VALUES (?, ?, ?, ?, ?)`),
		"archive_zip", "SUCCESS", result.StudentsCount+result.RecordsCount, start, time.Now())

	return result, nil
}

func ingestEvents(ctx context.Context, db *store.DB, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	var events []struct {
		ID      int    `json:"id"`
		Path    string `json:"path"`
		StartAt string `json:"startAt"`
		EndAt   string `json:"endAt"`
	}

	if err := json.NewDecoder(rc).Decode(&events); err != nil {
		return err
	}

	query := db.Rebind(`INSERT INTO source_events (source_event_id, path, name, type, start_at, end_at)
                        VALUES (?, ?, ?, ?, ?, ?)
                        ON CONFLICT (source_event_id) DO UPDATE SET
                           path = excluded.path,
                           start_at = excluded.start_at,
                           end_at = excluded.end_at;`)

	for _, e := range events {
		if e.ID == 97 {
			_, err = db.ExecContext(ctx, query, e.ID, e.Path, "Module #97", "module", e.StartAt, e.EndAt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ingestRoster(ctx context.Context, db *store.DB, f *zip.File) (map[int64]int64, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

	reader := csv.NewReader(bytes.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("roster CSV is empty")
	}

	// Header mapping
	headers := records[0]
	idxMap := make(map[string]int)
	for i, h := range headers {
		clean := strings.Trim(strings.ToLower(h), "\ufeff \"")
		idxMap[clean] = i
	}

	studentIDMap := make(map[int64]int64) // sourceUserId -> dbId

	for _, row := range records[1:] {
		sourceUserID, _ := strconv.ParseInt(row[idxMap["userid"]], 10, 64)
		login := row[idxMap["login"]]
		firstName := row[idxMap["firstname"]]
		lastName := row[idxMap["lastname"]]
		email := row[idxMap["email"]]
		cohortID, _ := strconv.Atoi(row[idxMap["cohortid"]])
		eventID, _ := strconv.Atoi(row[idxMap["eventid"]])
		level, _ := strconv.ParseFloat(row[idxMap["level"]], 64)

		auditRatioStr := row[idxMap["auditratio"]]
		auditRatio, err := strconv.ParseFloat(auditRatioStr, 64)
		if err != nil || math.IsInf(auditRatio, 0) || math.IsNaN(auditRatio) {
			auditRatio = 0.0
		}

		joinedStr := row[idxMap["joinedat"]]
		joinedAt, err := time.Parse(time.RFC3339, joinedStr)
		if err != nil {
			joinedAt = time.Now()
		}

		student := model.Student{
			SourceUserID: sourceUserID,
			Login:        login,
			FirstName:    firstName,
			LastName:     lastName,
			Email:        email,
			CreatedAt:    joinedAt,
			UpdatedAt:    time.Now(),
		}

		dbID, err := db.UpsertStudent(ctx, student)
		if err != nil {
			return nil, fmt.Errorf("failed upserting student %s: %w", login, err)
		}
		studentIDMap[sourceUserID] = dbID

		studentEvent := model.StudentEvent{
			StudentID:    dbID,
			EventID:      eventID,
			CohortID:     cohortID,
			EnrolledAt:   joinedAt,
			CurrentLevel: level,
			AuditRatio:   auditRatio,
			LastSyncedAt: time.Now(),
		}

		if err := db.UpsertStudentEvent(ctx, studentEvent); err != nil {
			return nil, fmt.Errorf("failed upserting student event for %s: %w", login, err)
		}
	}

	return studentIDMap, nil
}

func ingestRecords(ctx context.Context, db *store.DB, f *zip.File, studentIDMap map[int64]int64) (int, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	var rawRecords []struct {
		ID        int64   `json:"id"`
		UserID    int64   `json:"userId"`
		TypeName  string  `json:"typeName"`
		StartAt   string  `json:"startAt"`
		EndAt     *string `json:"endAt"`
		Message   string  `json:"message"`
		CreatedAt string  `json:"createdAt"`
	}

	if err := json.NewDecoder(rc).Decode(&rawRecords); err != nil {
		return 0, err
	}

	count := 0
	for _, r := range rawRecords {
		dbStudentID, exists := studentIDMap[r.UserID]
		if !exists {
			continue
		}

		startAt, _ := time.Parse(time.RFC3339, r.StartAt)
		createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt)
		var endAt *time.Time
		if r.EndAt != nil && *r.EndAt != "" {
			t, err := time.Parse(time.RFC3339, *r.EndAt)
			if err == nil {
				endAt = &t
			}
		}

		rec := model.StudentRecord{
			SourceRecordID: r.ID,
			StudentID:      dbStudentID,
			TypeName:       r.TypeName,
			StartAt:        startAt,
			EndAt:          endAt,
			Message:        r.Message,
			CreatedAt:      createdAt,
		}

		if err := db.UpsertStudentRecord(ctx, rec); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

func seedMentors(ctx context.Context, db *store.DB, studentIDMap map[int64]int64) error {
	mentors := []string{"Sohila (Lead Mentor)", "Anas (Academic Lead)", "Ahmed (Technical Mentor)"}
	// Delete any existing default assignments to keep idempotent
	_, _ = db.ExecContext(ctx, "DELETE FROM mentor_assignments;")

	query := db.Rebind(`INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, starts_at, is_primary)
                        VALUES (?, ?, ?, ?, ?);`)

	i := 0
	for _, dbStudentID := range studentIDMap {
		mName := mentors[i%len(mentors)]
		mID := int64((i % len(mentors)) + 1)
		_, _ = db.ExecContext(ctx, query, dbStudentID, mID, mName, time.Now(), true)
		i++
	}
	return nil
}

func generateSnapshots(ctx context.Context, db *store.DB) (int, error) {
	policies, err := db.GetStatusPolicies(ctx)
	if err != nil {
		return 0, err
	}

	milestones, err := db.GetCurriculumTimeline(ctx, 97)
	if err != nil {
		return 0, err
	}

	engine := pace.NewEngine(policies, milestones)
	now := time.Now()
	todayStr := now.Format("2006-01-02")

	// Get all students enrolled in Event 97
	query := db.Rebind(`SELECT s.id, se.enrolled_at, se.current_level, se.event_id
                        FROM students s
                        JOIN student_events se ON s.id = se.student_id
                        WHERE se.event_id = 97;`)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type studentRow struct {
		id       int64
		enrolled string
		level    float64
		eventID  int
	}

	var students []studentRow
	for rows.Next() {
		var s studentRow
		if err := rows.Scan(&s.id, &s.enrolled, &s.level, &s.eventID); err != nil {
			return 0, err
		}
		students = append(students, s)
	}

	count := 0
	for _, s := range students {
		enrolledAt := store.ParseFlexibleTime(s.enrolled)
		records, _ := db.GetStudentRecords(ctx, s.id)

		metrics := engine.Calculate(enrolledAt, s.level, records, now)

		snap := model.StudentMetricSnapshot{
			StudentID:          s.id,
			EventID:            s.eventID,
			SnapshotDate:       todayStr,
			Pace:               metrics,
			CalculationVersion: "v0.2",
			CalculatedAt:       now,
		}

		if err := db.SaveMetricSnapshot(ctx, snap); err != nil {
			return count, fmt.Errorf("failed saving metric snapshot for student %d: %w", s.id, err)
		}
		count++
	}

	return count, nil
}
