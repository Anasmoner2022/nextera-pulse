package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nextera-pulse/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.Connect(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(t.Context(), "../../migrations/sqlite.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at) VALUES
		(50, 'zzz', 'Zed', 'One', 'zed@example.com', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'aaa', 'Aye', 'Two', 'aye@example.com', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at) VALUES
		(1, 97, 4, '2026-01-01T00:00:00Z', 3, 0, '2026-01-01T00:00:00Z'),
		(2, 97, 4, '2026-01-01T00:00:00Z', 3, 0, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	return NewServer(db)
}

func TestCohortHealthResolvesPilotEvent(t *testing.T) {
	server := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cohorts/97/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			CohortID      int `json:"cohortId"`
			TotalStudents int `json:"totalStudents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.CohortID != 4 || body.Data.TotalStudents != 2 {
		t.Fatalf("health = %+v", body.Data)
	}
}

func TestStudentLookupPrefersInternalID(t *testing.T) {
	server := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/1", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Login string `json:"login"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Login != "zzz" {
		t.Fatalf("id 1 resolved to %s", body.Data.Login)
	}
}
