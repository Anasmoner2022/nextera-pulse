package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

// Server handles HTTP requests
type Server struct {
	db       *store.DB
	mux      *http.ServeMux
	verify   func(ctx context.Context, login, password string) (string, error)
	progress progressGate
}

type progressGate struct {
	mu           sync.Mutex
	token        string
	role         string
	userID       int64
	expires      time.Time
	refreshed    time.Time
	cohortSynced time.Time
	generation   int
	inflight     chan struct{}
}

// NewServer initializes HTTP routes and middleware
func NewServer(db *store.DB) *Server {
	s := &Server{
		db:  db,
		mux: http.NewServeMux(),
	}
	s.verify = func(ctx context.Context, login, password string) (string, error) {
		return zeroone.VerifySignIn(ctx, nil, zeroone.SchoolBaseURL(), login, password)
	}

	s.routes()
	return s
}

// Handler returns the HTTP handler with CORS enabled
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); isLocalDevOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	// Health probes
	s.mux.HandleFunc("GET /health/live", s.handleHealthLive)
	s.mux.HandleFunc("GET /health/ready", s.handleHealthReady)

	// API v1
	s.mux.HandleFunc("GET /api/v1/students", s.handleGetStudents)
	s.mux.HandleFunc("GET /api/v1/students/{id}/progress", s.handleGetStudentProgress)
	s.mux.HandleFunc("GET /api/v1/students/{id}", s.handleGetStudentByID)
	s.mux.HandleFunc("GET /api/v1/staff", s.handleGetStaff)
	s.mux.HandleFunc("POST /api/v1/session", s.handleSignIn)
	s.mux.HandleFunc("GET /api/v1/cohorts", s.handleGetCohorts)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/progress", s.handleGetCohortProgress)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/students", s.handleGetCohortStudents)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/health", s.handleGetCohortHealth)
	s.mux.HandleFunc("GET /api/v1/interventions", s.handleGetInterventions)
	s.mux.HandleFunc("POST /api/v1/interventions", s.handleCreateIntervention)
	s.mux.HandleFunc("POST /api/v1/mentor-assignments", s.handleAssignMentor)
}

func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "UP", "timestamp": time.Now()})
}

func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "DOWN", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "READY", "timestamp": time.Now()})
}

func (s *Server) handleGetStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	cohortID, _ := strconv.Atoi(query.Get("cohortId"))
	if cohortID == 0 {
		cohortID = 4 // default to pilot Second Cohort if not specified
	}

	statusFilter := query.Get("status")
	search := query.Get("search")

	students, err := s.db.ListStudentsWithPace(ctx, cohortID, statusFilter, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	meta := map[string]any{
		"totalCount":           len(students),
		"cohortId":             cohortID,
		"statusFilter":         statusFilter,
		"lastSuccessfulSyncAt": s.lastSuccessfulSyncAt(r.Context()),
	}

	if cohortID == 4 {
		meta["cohortName"] = "01 Coding - Second Cohort"
	} else if cohortID == 1 {
		meta["cohortName"] = "01 Coding - First Cohort"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": students,
		"meta": meta,
	})
}

func (s *Server) lastSuccessfulSyncAt(ctx context.Context) string {
	var completed store.FlexibleTime
	err := s.db.QueryRowContext(ctx, `SELECT MAX(completed_at) FROM sync_runs WHERE status = 'SUCCESS' AND completed_at IS NOT NULL`).Scan(&completed)
	if err != nil || completed.Time.IsZero() {
		return ""
	}
	return completed.Time.UTC().Format(time.RFC3339)
}

func (s *Server) handleGetStudentByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Student ID must be an integer")
		return
	}

	students, err := s.db.ListStudentsWithPace(r.Context(), 0, "", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	var exact *model.StudentPaceView
	var pilot *model.StudentPaceView
	var bySource *model.StudentPaceView
	for i := range students {
		row := students[i]
		if row.ID == id {
			if exact == nil {
				match := row
				exact = &match
			}
			if pilot == nil && row.CohortID == secondCohortID {
				match := row
				pilot = &match
			}
			continue
		}
		if bySource == nil && row.SourceUserID == id {
			match := row
			bySource = &match
		}
	}
	if pilot != nil {
		exact = pilot
	}
	if exact == nil {
		exact = bySource
	}
	if exact == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": exact})
}

func (s *Server) handleAssignMentor(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req struct {
		StudentID int64 `json:"studentId"`
		MentorID  int64 `json:"mentorId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Send the student and mentor.")
		return
	}
	if err := s.db.MoveMentorAssignment(r.Context(), req.StudentID, req.MentorID); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ASSIGNMENT", "That student could not be assigned to that mentor.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"studentId": req.StudentID, "mentorId": req.MentorID})
}

func (s *Server) handleGetStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := s.db.ListStaff(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if staff == nil {
		staff = []store.StaffMember{}
	}
	managers := []store.StaffMember{}
	mentors := []store.StaffMember{}
	for _, member := range staff {
		if member.PulseRole == "manager" {
			managers = append(managers, member)
		} else if member.PulseRole == "mentor" {
			mentors = append(mentors, member)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"managers": managers, "mentors": mentors})
}

func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Send the 01Edu login and password.")
		return
	}
	token, err := s.verify(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, zeroone.ErrRejected) {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "The 01Edu login or password is incorrect.")
			return
		}
		writeError(w, http.StatusBadGateway, "SCHOOL_UNAVAILABLE", "The 01Edu sign-in service did not respond.")
		return
	}
	person, err := s.db.FindIdentityByLogin(r.Context(), req.Login)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if person == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "This 01Edu account is not in the loaded cohort.")
		return
	}
	s.rememberProgressSession(token)
	writeJSON(w, http.StatusOK, person)
}

func (s *Server) handleGetCohorts(w http.ResponseWriter, r *http.Request) {
	counts := map[int]int{4: 0, 1: 0}
	rows, err := s.db.QueryContext(r.Context(), "SELECT cohort_id, COUNT(*) FROM student_events WHERE event_id = 97 GROUP BY cohort_id")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cohortID, count int
			if rows.Scan(&cohortID, &count) == nil {
				counts[cohortID] = count
			}
		}
	}
	cohorts := []map[string]any{
		{
			"id":           4,
			"name":         "01 Coding - Second Cohort",
			"eventId":      97,
			"isPilot":      true,
			"studentCount": counts[4],
		},
		{
			"id":           1,
			"name":         "01 Coding - First Cohort",
			"eventId":      97,
			"isPilot":      false,
			"studentCount": counts[1],
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cohorts})
}

func (s *Server) handleGetCohortProgress(w http.ResponseWriter, r *http.Request) {
	pathID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COHORT_ID", "Cohort or event ID must be an integer")
		return
	}
	cohortID, eventID, ok := resolveMentorCohort(pathID)
	if !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_COHORT", "Unknown cohort or event id")
		return
	}
	board, err := s.liveProgressBoard(r.Context(), cohortID, eventID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) handleGetStudentProgress(w http.ResponseWriter, r *http.Request) {
	studentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_STUDENT_ID", "Student ID must be an integer")
		return
	}
	board, err := s.liveProgressBoard(r.Context(), 0, 97, studentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if len(board.Students) == 0 {
		writeError(w, http.StatusNotFound, "STUDENT_NOT_FOUND", "Student was not found in event 97")
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) handleGetCohortStudents(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	pathID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COHORT_ID", "Cohort or event ID must be an integer")
		return
	}
	cohortID, eventID, ok := resolveMentorCohort(pathID)
	if !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_COHORT", "Unknown cohort or event id")
		return
	}

	students, err := s.db.ListStudentsWithPace(r.Context(), cohortID, r.URL.Query().Get("status"), r.URL.Query().Get("search"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mentorStudentsResponse(eventID, students, time.Now()))
}

func (s *Server) handleGetCohortHealth(w http.ResponseWriter, r *http.Request) {
	pathID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COHORT_ID", "Cohort ID must be an integer")
		return
	}
	cohortID, _, ok := resolveMentorCohort(pathID)
	if !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_COHORT", "Unknown cohort or event id")
		return
	}

	summary, err := s.db.GetCohortHealthSummary(r.Context(), cohortID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": summary})
}

func (s *Server) handleGetInterventions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT id, student_id, mentor_id, reason_code, type, action, status, created_at FROM interventions ORDER BY created_at DESC;")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := []map[string]any{}
	for rows.Next() {
		var id, studentID, mentorID int64
		var reasonCode, itype, action, status string
		var createdAt store.FlexibleTime
		if err := rows.Scan(&id, &studentID, &mentorID, &reasonCode, &itype, &action, &status, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		list = append(list, map[string]any{
			"id":         id,
			"studentId":  studentID,
			"mentorId":   mentorID,
			"reasonCode": reasonCode,
			"type":       itype,
			"action":     action,
			"status":     status,
			"createdAt":  formatAPITime(createdAt.Time.UTC().Format(time.RFC3339)),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (s *Server) handleCreateIntervention(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StudentID  int64  `json:"studentId"`
		MentorID   int64  `json:"mentorId"`
		ReasonCode string `json:"reasonCode"`
		Type       string `json:"type"`
		Action     string `json:"action"`
		DueAt      string `json:"dueAt"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error())
		return
	}

	if req.StudentID == 0 || req.Action == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "studentId and action are required")
		return
	}

	if req.MentorID == 0 {
		req.MentorID = 1 // default mentor
	}
	if req.ReasonCode == "" {
		req.ReasonCode = "MENTOR_FOLLOW_UP"
	}
	if req.Type == "" {
		req.Type = "follow-up"
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	var dueAt any
	if strings.TrimSpace(req.DueAt) != "" {
		dueAt = req.DueAt
	}
	var id int64
	if s.db.DriverName == "postgres" {
		query := s.db.Rebind(`INSERT INTO interventions (student_id, mentor_id, reason_code, type, action, status, due_at, created_at)
                          VALUES (?, ?, ?, ?, ?, 'OPEN', ?, ?) RETURNING id`)
		err := s.db.QueryRowContext(r.Context(), query, req.StudentID, req.MentorID, req.ReasonCode, req.Type, req.Action, dueAt, createdAt).Scan(&id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	} else {
		query := s.db.Rebind(`INSERT INTO interventions (student_id, mentor_id, reason_code, type, action, status, due_at, created_at)
                          VALUES (?, ?, ?, ?, ?, 'OPEN', ?, ?)`)
		result, err := s.db.ExecContext(r.Context(), query, req.StudentID, req.MentorID, req.ReasonCode, req.Type, req.Action, dueAt, createdAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		id, err = result.LastInsertId()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Intervention recorded successfully",
		"status":  "OPEN",
		"data": map[string]any{
			"id":         id,
			"studentId":  req.StudentID,
			"mentorId":   req.MentorID,
			"reasonCode": req.ReasonCode,
			"type":       req.Type,
			"action":     req.Action,
			"status":     "OPEN",
			"createdAt":  createdAt,
		},
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	buf, err := json.Marshal(data)
	if err != nil {
		http.Error(w, `{"error":{"code":"ENCODE_ERROR","message":"failed to encode response"}}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf)
}

func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
