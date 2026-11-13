package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nextera-pulse/internal/model"
	"nextera-pulse/internal/pace"
	"nextera-pulse/internal/store"
	pulsesync "nextera-pulse/internal/sync"
	"nextera-pulse/internal/zeroone"
)

// Server handles HTTP requests
type Server struct {
	db           *store.DB
	mux          *http.ServeMux
	verify       func(ctx context.Context, login, password string) (string, error)
	live         *pulsesync.Runner
	snapshots    *pulsesync.SnapshotJob
	program      pulsesync.LiveCycleConfig
	opts         Options
	ipLimiter    *attemptLimiter
	loginLimiter *attemptLimiter
}

// SetLiveSync attaches the scheduled 01Edu live sync. When set, dashboards
// report its freshness and the per-request password-session pull is disabled.
func (s *Server) SetLiveSync(runner *pulsesync.Runner) {
	s.live = runner
}

// SetSnapshotJob attaches the daily pace snapshot job so sync status reports it.
func (s *Server) SetSnapshotJob(job *pulsesync.SnapshotJob) {
	s.snapshots = job
}

// SetProgram sets the curriculum event and cohorts (ZEROONE_EVENT_ID,
// PILOT_COHORTS). Without it the server uses pulsesync.DefaultProgram().
func (s *Server) SetProgram(program pulsesync.LiveCycleConfig) {
	if program.EventID > 0 && len(program.Cohorts) > 0 {
		s.program = program
	}
}

// NewServer initializes HTTP routes and middleware with DefaultOptions.
func NewServer(db *store.DB) *Server {
	return NewServerWithOptions(db, DefaultOptions())
}

// NewServerWithOptions initializes HTTP routes and middleware.
func NewServerWithOptions(db *store.DB, opts Options) *Server {
	defaults := DefaultOptions()
	if opts.CookieName == "" {
		opts.CookieName = defaults.CookieName
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = defaults.SessionTTL
	}
	if opts.SignInPerIP <= 0 {
		opts.SignInPerIP = defaults.SignInPerIP
	}
	if opts.SignInPerLogin <= 0 {
		opts.SignInPerLogin = defaults.SignInPerLogin
	}
	if opts.SignInRateWindow <= 0 {
		opts.SignInRateWindow = defaults.SignInRateWindow
	}
	s := &Server{
		db:           db,
		mux:          http.NewServeMux(),
		opts:         opts,
		program:      pulsesync.DefaultProgram(),
		ipLimiter:    newAttemptLimiter(opts.SignInPerIP, opts.SignInRateWindow),
		loginLimiter: newAttemptLimiter(opts.SignInPerLogin, opts.SignInRateWindow),
	}
	s.verify = func(ctx context.Context, login, password string) (string, error) {
		return zeroone.VerifySignIn(ctx, nil, zeroone.SchoolBaseURL(), login, password)
	}

	s.routes()
	return s
}

// Handler returns the HTTP handler: CORS, then the session check on /api.
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.requireSession(s.mux))
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

	// Session. POST (sign-in) is the only public /api route; requireSession
	// guards every other one. Student lists are filtered to the caller's scope.
	s.mux.HandleFunc("POST /api/v1/session", s.handleSignIn)
	s.mux.HandleFunc("GET /api/v1/session", s.handleGetSession)
	s.mux.HandleFunc("DELETE /api/v1/session", s.handleSignOut)

	// API v1
	s.mux.HandleFunc("GET /api/v1/students", s.handleGetStudents)
	s.mux.HandleFunc("GET /api/v1/students/{id}/progress", s.handleGetStudentProgress)
	s.mux.HandleFunc("GET /api/v1/students/{id}", s.handleGetStudentByID)
	s.mux.HandleFunc("GET /api/v1/staff", requireRole(s.handleGetStaff, roleManager, roleMentor))
	s.mux.HandleFunc("GET /api/v1/sync/status", s.handleSyncStatus)
	s.mux.HandleFunc("GET /api/v1/program", s.handleGetProgram)
	s.mux.HandleFunc("GET /api/v1/cohorts", s.handleGetCohorts)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/progress", s.handleGetCohortProgress)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/students", s.handleGetCohortStudents)
	s.mux.HandleFunc("GET /api/v1/cohorts/{id}/health", requireRole(s.handleGetCohortHealth, roleManager, roleMentor))
	s.mux.HandleFunc("GET /api/v1/kpi/on-track", requireRole(s.handleGetOnTrackKPI, roleManager, roleMentor))
	s.mux.HandleFunc("GET /api/v1/interventions", s.handleGetInterventions)
	s.mux.HandleFunc("POST /api/v1/interventions", requireRole(s.handleCreateIntervention, roleManager, roleMentor))
	s.mux.HandleFunc("DELETE /api/v1/interventions/{id}", s.handleDeleteIntervention)
	s.mux.HandleFunc("POST /api/v1/mentor-assignments", requireRole(s.handleAssignMentor, roleManager))
	s.mux.HandleFunc("DELETE /api/v1/mentor-assignments/{studentId}", requireRole(s.handleUnassignMentor, roleManager))
	s.mux.HandleFunc("GET /api/v1/students/{id}/mentor-assignments", requireRole(s.handleGetAssignmentHistory, roleManager, roleMentor))
}

func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "UP", "timestamp": time.Now()})
}

func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		log.Printf("readiness: database ping failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "DOWN"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "READY", "timestamp": time.Now()})
}

func (s *Server) handleGetStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	cohortID, _ := strconv.Atoi(query.Get("cohortId"))
	if cohortID == 0 {
		cohortID = s.program.DefaultCohortID() // the pilot (first configured) cohort
	}

	statusFilter := query.Get("status")
	search := query.Get("search")

	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	students, err := s.db.ListStudentsWithPace(ctx, s.program.EventID, cohortID, statusFilter, search)
	if err != nil {
		internalError(w, "list students", err)
		return
	}
	students = s.nameCohorts(filterStudents(students, scope))

	meta := map[string]any{
		"totalCount":           len(students),
		"cohortId":             cohortID,
		"statusFilter":         statusFilter,
		"lastSuccessfulSyncAt": s.lastSuccessfulSyncAt(r.Context()),
	}

	meta["eventId"] = s.program.EventID
	if name, ok := s.program.CohortName(cohortID); ok {
		meta["cohortName"] = name
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

	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	students, err := s.db.ListStudentsWithPace(r.Context(), s.program.EventID, 0, "", "")
	if err != nil {
		internalError(w, "get student", err)
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
			if pilot == nil && row.CohortID == s.program.DefaultCohortID() {
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
		if !scope.all {
			forbidden(w) // do not reveal which ids exist
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
		return
	}
	if !scope.allows(exact.ID) {
		forbidden(w)
		return
	}
	exact.CohortName, _ = s.program.CohortName(exact.CohortID)
	writeJSON(w, http.StatusOK, map[string]any{"data": exact})
}

func (s *Server) handleAssignMentor(w http.ResponseWriter, r *http.Request) {
	p, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req struct {
		StudentID int64  `json:"studentId"`
		MentorID  int64  `json:"mentorId"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Send the student and mentor.")
		return
	}
	if len(req.Reason) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "The reason is too long.")
		return
	}
	if !scope.allows(req.StudentID) {
		forbidden(w)
		return
	}
	// The history records who made the change from the session, never the body.
	if err := s.db.MoveMentorAssignment(r.Context(), req.StudentID, req.MentorID, p.StudentID, req.Reason); err != nil {
		switch {
		case errors.Is(err, store.ErrStudentNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
		case errors.Is(err, store.ErrInvalidAssignment):
			writeError(w, http.StatusBadRequest, "INVALID_ASSIGNMENT", "That student could not be assigned to that mentor.")
		default:
			internalError(w, "move mentor assignment", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"studentId": req.StudentID, "mentorId": req.MentorID})
}

// handleUnassignMentor closes the student's current mentor row (manager
// only). The row stays in the history. An optional ?reason= is recorded.
func (s *Server) handleUnassignMentor(w http.ResponseWriter, r *http.Request) {
	p, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	studentID, err := strconv.ParseInt(r.PathValue("studentId"), 10, 64)
	if err != nil || studentID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Student id must be a number")
		return
	}
	reason := r.URL.Query().Get("reason")
	if len(reason) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "The reason is too long.")
		return
	}
	if !scope.allows(studentID) {
		forbidden(w)
		return
	}
	closed, err := s.db.UnassignMentor(r.Context(), studentID, p.StudentID, reason)
	switch {
	case errors.Is(err, store.ErrStudentNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
		return
	case errors.Is(err, store.ErrInvalidAssignment):
		writeError(w, http.StatusBadRequest, "INVALID_ASSIGNMENT", "Staff do not have mentors.")
		return
	case err != nil:
		internalError(w, "unassign mentor", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"studentId": studentID, "unassigned": closed})
}

// handleGetAssignmentHistory returns a student's mentor assignments, newest
// first. Managers see any student, mentors only students currently theirs.
func (s *Server) handleGetAssignmentHistory(w http.ResponseWriter, r *http.Request) {
	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	studentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || studentID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Student id must be a number")
		return
	}
	if !scope.allows(studentID) {
		forbidden(w)
		return
	}
	exists, err := s.db.StudentExists(r.Context(), studentID)
	if err != nil {
		internalError(w, "check student", err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
		return
	}
	history, err := s.db.MentorAssignmentHistory(r.Context(), studentID)
	if err != nil {
		internalError(w, "assignment history", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": history})
}

func (s *Server) handleGetStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := s.db.ListStaff(r.Context())
	if err != nil {
		internalError(w, "list staff", err)
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

func (s *Server) handleGetCohorts(w http.ResponseWriter, r *http.Request) {
	counts := map[int]int{}
	rows, err := s.db.QueryContext(r.Context(), "SELECT cohort_id, COUNT(*) FROM student_events WHERE event_id = $1 GROUP BY cohort_id", s.program.EventID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cohortID, count int
			if rows.Scan(&cohortID, &count) == nil {
				counts[cohortID] = count
			}
		}
	}
	cohorts := make([]map[string]any, 0, len(s.program.Cohorts))
	for i, cohort := range s.program.Cohorts {
		cohorts = append(cohorts, map[string]any{
			"id":           cohort.ID,
			"name":         cohort.Label,
			"eventId":      s.program.EventID,
			"isPilot":      i == 0,
			"studentCount": counts[cohort.ID],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cohorts})
}

// handleGetProgram tells the web app which curriculum event and cohorts this
// server is configured for, and which pace-rule version is in use, so the
// client never hard-codes them.
func (s *Server) handleGetProgram(w http.ResponseWriter, r *http.Request) {
	cohorts := make([]map[string]any, 0, len(s.program.Cohorts))
	for _, cohort := range s.program.Cohorts {
		cohorts = append(cohorts, map[string]any{"id": cohort.ID, "name": cohort.Label})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"eventId":         s.program.EventID,
		"defaultCohortId": s.program.DefaultCohortID(),
		"cohorts":         cohorts,
		"paceRule":        s.paceRule(r.Context()),
	})
}

// paceRule reports the active monthly-target version for the configured
// event, or nil when it cannot be read.
func (s *Server) paceRule(ctx context.Context) *paceRule {
	version, ok, err := s.db.ActiveTimelineVersion(ctx, s.program.EventID)
	if err != nil {
		log.Printf("read pace rule version: %v", err)
		return nil
	}
	if !ok {
		return &paceRule{EventID: s.program.EventID, Status: "missing"}
	}
	return &paceRule{EventID: version.EventID, Version: version.Label, Status: version.Status, Approved: version.Approved}
}

// nameCohorts fills CohortName from the configured cohort labels.
func (s *Server) nameCohorts(students []model.StudentPaceView) []model.StudentPaceView {
	for i := range students {
		students[i].CohortName, _ = s.program.CohortName(students[i].CohortID)
	}
	return students
}

func (s *Server) handleGetCohortProgress(w http.ResponseWriter, r *http.Request) {
	pathID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COHORT_ID", "Cohort or event ID must be an integer")
		return
	}
	cohortID, eventID, ok := s.program.ResolveCohort(pathID)
	if !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_COHORT", "Unknown cohort or event id")
		return
	}
	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	board, err := s.liveProgressBoard(r.Context(), cohortID, eventID, 0)
	if err != nil {
		internalError(w, "cohort progress", err)
		return
	}
	if !scope.all {
		visible := board.Students[:0]
		for _, student := range board.Students {
			if scope.allows(student.StudentID) {
				visible = append(visible, student)
			}
		}
		board.Students = visible
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) handleGetStudentProgress(w http.ResponseWriter, r *http.Request) {
	studentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_STUDENT_ID", "Student ID must be an integer")
		return
	}
	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	if !scope.allows(studentID) {
		forbidden(w)
		return
	}
	board, err := s.liveProgressBoard(r.Context(), 0, s.program.EventID, studentID)
	if err != nil {
		internalError(w, "student progress", err)
		return
	}
	if len(board.Students) == 0 {
		writeError(w, http.StatusNotFound, "STUDENT_NOT_FOUND", "Student was not found in the curriculum event")
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
	cohortID, eventID, ok := s.program.ResolveCohort(pathID)
	if !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_COHORT", "Unknown cohort or event id")
		return
	}

	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	students, err := s.db.ListStudentsWithPace(r.Context(), eventID, cohortID, r.URL.Query().Get("status"), r.URL.Query().Get("search"))
	if err != nil {
		internalError(w, "cohort students", err)
		return
	}
	students = filterStudents(students, scope)

	cohortName, _ := s.program.CohortName(cohortID)
	payload := mentorStudentsResponse(eventID, cohortID, cohortName, students, time.Now())
	payload.PaceRule = s.paceRule(r.Context())
	payload.DataSource, payload.SyncedAt = s.dataFreshness(r.Context())
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleGetCohortHealth(w http.ResponseWriter, r *http.Request) {
	pathID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COHORT_ID", "Cohort ID must be an integer")
		return
	}
	cohortID, eventID, ok := s.program.ResolveCohort(pathID)
	if !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_COHORT", "Unknown cohort or event id")
		return
	}

	// A mentor's summary covers only their current students.
	_, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	var only map[int64]bool
	if !scope.all {
		only = scope.ids
		if only == nil {
			only = map[int64]bool{}
		}
	}
	summary, err := s.db.GetCohortHealthSummary(r.Context(), eventID, cohortID, only)
	if err != nil {
		internalError(w, "cohort health", err)
		return
	}
	summary.CohortName, _ = s.program.CohortName(cohortID)

	writeJSON(w, http.StatusOK, map[string]any{"data": summary, "eventId": eventID, "paceRule": s.paceRule(r.Context())})
}

// handleGetOnTrackKPI returns the North Star KPI (GREEN / measurable). A
// manager gets the program-wide value, every configured cohort and every
// mentor (each mentor's current students). A mentor gets only their own
// value; program and cohorts are omitted. Students may not call it.
func (s *Server) handleGetOnTrackKPI(w http.ResponseWriter, r *http.Request) {
	p, ok := principalOf(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in with your 01Edu login.")
		return
	}
	report, err := s.db.OnTrackKPIs(r.Context(), s.program.EventID)
	if err != nil {
		internalError(w, "on-track kpi", err)
		return
	}
	mentorKPI := func(id int64, name string) model.OnTrackKPI {
		kpi, found := report.Mentors[id]
		if !found {
			kpi = pace.OnTrack(nil, nil)
			kpi.Scope, kpi.EventID, kpi.MentorID = "mentor", s.program.EventID, id
		}
		kpi.Name = name
		return kpi
	}
	resp := map[string]any{
		"eventId":    s.program.EventID,
		"definition": "on-track rate = GREEN / measurable students (pace GREEN, YELLOW or RED, source status not excluded by status_policy)",
		"paceRule":   s.paceRule(r.Context()),
	}
	if p.Role == roleMentor {
		resp["program"] = nil
		resp["cohorts"] = []model.OnTrackKPI{}
		resp["mentors"] = []model.OnTrackKPI{mentorKPI(p.StudentID, p.DisplayName)}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	cohorts := make([]model.OnTrackKPI, 0, len(s.program.Cohorts))
	for _, cohort := range s.program.Cohorts {
		kpi, found := report.Cohorts[cohort.ID]
		if !found {
			kpi = pace.OnTrack(nil, nil)
			kpi.Scope, kpi.EventID, kpi.CohortID = "cohort", s.program.EventID, cohort.ID
		}
		kpi.Name = cohort.Label
		cohorts = append(cohorts, kpi)
	}
	staff, err := s.db.ListStaff(r.Context())
	if err != nil {
		internalError(w, "on-track kpi staff", err)
		return
	}
	mentors := []model.OnTrackKPI{}
	for _, member := range staff {
		if member.PulseRole == roleMentor {
			mentors = append(mentors, mentorKPI(member.StudentID, member.DisplayName))
		}
	}
	resp["program"] = report.Program
	resp["cohorts"] = cohorts
	resp["mentors"] = mentors
	writeJSON(w, http.StatusOK, resp)
}

// handleGetInterventions lists follow-ups the caller may see: a manager sees
// all, a mentor those about their students or written by them, a student
// those about themselves. Each row carries its author.
func (s *Server) handleGetInterventions(w http.ResponseWriter, r *http.Request) {
	p, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT i.id, i.student_id, i.mentor_id, i.reason_code, i.type, i.action, i.status, i.created_at,
			COALESCE(NULLIF(TRIM(COALESCE(a.first_name, '') || ' ' || COALESCE(a.last_name, '')), ''), a.login, ''), i.author_role
		FROM interventions i LEFT JOIN students a ON a.id = i.mentor_id
		WHERE i.deleted_at IS NULL
		ORDER BY i.created_at DESC, i.id DESC`)
	if err != nil {
		internalError(w, "list interventions", err)
		return
	}
	defer rows.Close()

	list := []map[string]any{}
	for rows.Next() {
		var id, studentID, authorID int64
		var reasonCode, itype, action, status, authorName, authorRole string
		var createdAt store.FlexibleTime
		if err := rows.Scan(&id, &studentID, &authorID, &reasonCode, &itype, &action, &status, &createdAt, &authorName, &authorRole); err != nil {
			internalError(w, "scan intervention", err)
			return
		}
		if !scope.allows(studentID) && !(p.Role == roleMentor && authorID == p.StudentID) {
			continue
		}
		list = append(list, map[string]any{
			"id":         id,
			"studentId":  studentID,
			"mentorId":   authorID,
			"authorId":   authorID,
			"authorName": authorName,
			"authorRole": authorRole,
			"canDelete":  canDeleteIntervention(p, authorID),
			"reasonCode": reasonCode,
			"type":       itype,
			"action":     action,
			"status":     status,
			"createdAt":  formatAPITime(createdAt.Time.UTC().Format(time.RFC3339)),
		})
	}
	if err := rows.Err(); err != nil {
		internalError(w, "list interventions", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// canDeleteIntervention: a follow-up may be deleted by its author or a manager.
func canDeleteIntervention(p Principal, authorID int64) bool {
	return p.Role == roleManager || (authorID != 0 && authorID == p.StudentID)
}

// handleDeleteIntervention soft-deletes a follow-up (deleted_at, deleted_by).
// Only its author or a manager may delete it: 403 otherwise, 404 when there is
// no such (visible) note.
func (s *Server) handleDeleteIntervention(w http.ResponseWriter, r *http.Request) {
	p, ok := principalOf(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in with your 01Edu login.")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Follow-up id must be a number")
		return
	}
	author, err := s.db.InterventionAuthor(r.Context(), id)
	if errors.Is(err, store.ErrInterventionNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Follow-up not found")
		return
	}
	if err != nil {
		internalError(w, "read follow-up", err)
		return
	}
	if !canDeleteIntervention(p, author) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the author or a program manager can delete this follow-up.")
		return
	}
	if err := s.db.SoftDeleteIntervention(r.Context(), id, p.StudentID); err != nil {
		if errors.Is(err, store.ErrInterventionNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Follow-up not found")
			return
		}
		internalError(w, "delete follow-up", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCreateIntervention records a follow-up. The author is the signed-in
// mentor or manager; any mentorId in the body is ignored.
func (s *Server) handleCreateIntervention(w http.ResponseWriter, r *http.Request) {
	p, scope, ok := s.requestScope(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req struct {
		StudentID  int64  `json:"studentId"`
		ReasonCode string `json:"reasonCode"`
		Type       string `json:"type"`
		Action     string `json:"action"`
		DueAt      string `json:"dueAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Send studentId and action as JSON.")
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.StudentID == 0 || req.Action == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "studentId and action are required")
		return
	}
	if len(req.Action) > 4000 || len(req.ReasonCode) > 100 || len(req.Type) > 100 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "The follow-up is too long.")
		return
	}
	if !scope.allows(req.StudentID) {
		forbidden(w)
		return
	}
	exists, err := s.db.StudentExists(r.Context(), req.StudentID)
	if err != nil {
		internalError(w, "check student", err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Student not found")
		return
	}
	if req.ReasonCode == "" {
		req.ReasonCode = "MENTOR_FOLLOW_UP"
	}
	if req.Type == "" {
		req.Type = "follow-up"
	}
	var dueAt any
	if strings.TrimSpace(req.DueAt) != "" {
		parsed := store.ParseFlexibleTime(req.DueAt)
		if parsed.IsZero() {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "dueAt must be a date")
			return
		}
		dueAt = parsed.UTC()
	}

	createdAt := time.Now().UTC()
	var id int64
	query := s.db.Rebind(`INSERT INTO interventions (student_id, mentor_id, author_role, reason_code, type, action, status, due_at, created_at)
                          VALUES (?, ?, ?, ?, ?, ?, 'OPEN', ?, ?) RETURNING id`)
	if err := s.db.QueryRowContext(r.Context(), query, req.StudentID, p.StudentID, p.Role, req.ReasonCode, req.Type, req.Action, dueAt, createdAt).Scan(&id); err != nil {
		internalError(w, "create intervention", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Intervention recorded successfully",
		"status":  "OPEN",
		"data": map[string]any{
			"id":         id,
			"studentId":  req.StudentID,
			"mentorId":   p.StudentID,
			"authorId":   p.StudentID,
			"authorName": p.DisplayName,
			"authorRole": p.Role,
			"canDelete":  true,
			"reasonCode": req.ReasonCode,
			"type":       req.Type,
			"action":     req.Action,
			"status":     "OPEN",
			"createdAt":  createdAt.Format(time.RFC3339),
		},
	})
}

// liveProgressBoard reads the project board from PostgreSQL. Only the
// scheduled live sync writes progress; no user's own 01Edu session is used.
func (s *Server) liveProgressBoard(ctx context.Context, cohortID, eventID int, studentID int64) (store.ProgressBoard, error) {
	board, err := s.db.ProgressBoard(ctx, cohortID, eventID, studentID)
	if err != nil {
		return board, err
	}
	if s.live != nil {
		if last := s.live.LastSuccess(); !last.IsZero() {
			board.SyncedAt = last.UTC().Format(time.RFC3339)
		}
	} else if last := s.lastLiveCycleAt(ctx); last != "" {
		board.SyncedAt = last
	}
	return board, nil
}

func filterStudents(students []model.StudentPaceView, scope studentScope) []model.StudentPaceView {
	if scope.all {
		return students
	}
	visible := make([]model.StudentPaceView, 0, len(students))
	for _, student := range students {
		if scope.allows(student.ID) {
			visible = append(visible, student)
		}
	}
	return visible
}

// internalError logs the detail and returns a generic message.
func internalError(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
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
