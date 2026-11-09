package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"nextera-pulse/internal/store"
)

// Server handles HTTP requests
type Server struct {
	db  *store.DB
	mux *http.ServeMux
}

// NewServer initializes HTTP routes and middleware
func NewServer(db *store.DB) *Server {
	s := &Server{
		db:  db,
		mux: http.NewServeMux(),
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
		// Allow Ziad's frontend and dev origins
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
	s.mux.HandleFunc("GET /api/v1/students/{id}", s.handleGetStudentByID)
	s.mux.HandleFunc("GET /api/v1/cohorts", s.handleGetCohorts)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/health", s.handleGetCohortHealth)
	s.mux.HandleFunc("GET /api/v1/interventions", s.handleGetInterventions)
	s.mux.HandleFunc("POST /api/v1/interventions", s.handleCreateIntervention)
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
		"lastSuccessfulSyncAt": "2026-10-19T11:11:08Z",
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

	for _, s := range students {
		if s.ID == id || s.SourceUserID == id {
			writeJSON(w, http.StatusOK, map[string]any{"data": s})
			return
		}
	}

	writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
}

func (s *Server) handleGetCohorts(w http.ResponseWriter, r *http.Request) {
	cohorts := []map[string]any{
		{
			"id":        4,
			"name":      "01 Coding - Second Cohort",
			"eventId":   97,
			"isPilot":   true,
			"studentCount": 52,
		},
		{
			"id":        1,
			"name":      "01 Coding - First Cohort",
			"eventId":   97,
			"isPilot":   false,
			"studentCount": 94,
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cohorts})
}

func (s *Server) handleGetCohortHealth(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	cohortID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COHORT_ID", "Cohort ID must be an integer")
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

	var list []map[string]any
	for rows.Next() {
		var id, studentID, mentorID int64
		var reasonCode, itype, action, status, createdAt string
		_ = rows.Scan(&id, &studentID, &mentorID, &reasonCode, &itype, &action, &status, &createdAt)
		list = append(list, map[string]any{
			"id":         id,
			"studentId":  studentID,
			"mentorId":   mentorID,
			"reasonCode": reasonCode,
			"type":       itype,
			"action":     action,
			"status":     status,
			"createdAt":  createdAt,
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

	query := s.db.Rebind(`INSERT INTO interventions (student_id, mentor_id, reason_code, type, action, status, due_at, created_at)
                          VALUES (?, ?, ?, ?, ?, 'OPEN', ?, ?)`)

	now := time.Now()
	_, err := s.db.ExecContext(r.Context(), query, req.StudentID, req.MentorID, req.ReasonCode, req.Type, req.Action, req.DueAt, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Intervention recorded successfully",
		"status":  "OPEN",
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
