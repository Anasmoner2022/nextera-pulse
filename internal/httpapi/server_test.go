package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/store/pgtest"
	pulsesync "nextera-pulse/internal/sync"
	"nextera-pulse/internal/zeroone"
)

// Fixture person ids (students.id). Staff are rows in students too.
const (
	managerID   int64 = 1 // 01Edu role "admin"        → manager
	mentorAID   int64 = 2 // 01Edu role "admin campus" → mentor of learnerA
	mentorBID   int64 = 3 // 01Edu role "admin campus" → mentor of learnerB
	learnerAID  int64 = 4
	learnerBID  int64 = 5
	learnerCID  int64 = 6 // no mentor
	learnerASrc int64 = 104
)

type testAPI struct {
	t      *testing.T
	db     *store.DB
	server *Server
	h      http.Handler
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	db, err := store.Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role) VALUES
		(101, 'manager', 'Program', 'Manager', 'pm@example.com', 'admin'),
		(102, 'mentor.a', 'Mentor', 'A', 'ma@example.com', 'admin campus'),
		(103, 'mentor.b', 'Mentor', 'B', 'mb@example.com', 'admin campus'),
		(104, 'learner.a', 'Learner', 'A', 'la@example.com', 'user'),
		(105, 'learner.b', 'Learner', 'B', 'lb@example.com', 'user'),
		(106, 'learner.c', 'Learner', 'C', 'lc@example.com', 'user')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at) VALUES
		(4, 97, 4, '2026-01-01T00:00:00Z', 3, 0, '2026-01-01T00:00:00Z'),
		(5, 97, 4, '2026-01-01T00:00:00Z', 3, 0, '2026-01-01T00:00:00Z'),
		(6, 97, 4, '2026-01-01T00:00:00Z', 3, 0, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mentor_assignments (student_id, mentor_id, mentor_name, is_primary) VALUES
		(4, 2, 'Mentor A', TRUE), (5, 3, 'Mentor B', TRUE)`); err != nil {
		t.Fatal(err)
	}
	server := NewServer(db)
	server.verify = func(context.Context, string, string) (string, error) { return "", zeroone.ErrRejected }
	return &testAPI{t: t, db: db, server: server, h: server.Handler()}
}

// cookieFor signs a person in directly through the session store.
func (a *testAPI) cookieFor(studentID int64) *http.Cookie {
	a.t.Helper()
	token, _, err := a.db.CreateSession(context.Background(), studentID, time.Hour, "test", "127.0.0.1")
	if err != nil {
		a.t.Fatal(err)
	}
	return &http.Cookie{Name: "pulse_session", Value: token}
}

func (a *testAPI) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

// fakeJWT is an unsigned 01Edu-shaped token carrying a Hasura user id.
func fakeJWT(userID int64) string {
	claims, _ := json.Marshal(map[string]any{
		"sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
		"https://hasura.io/jwt/claims": map[string]any{
			"x-hasura-user-id":       userID,
			"x-hasura-default-role":  "user",
			"x-hasura-allowed-roles": []string{"user"},
		},
	})
	return "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(claims) + ".sig"
}

func TestEveryAPIRouteRequiresASession(t *testing.T) {
	api := newTestAPI(t)
	routes := []struct{ method, path string }{
		{"GET", "/api/v1/session"},
		{"DELETE", "/api/v1/session"},
		{"GET", "/api/v1/students"},
		{"GET", "/api/v1/students/4"},
		{"GET", "/api/v1/students/4/progress"},
		{"GET", "/api/v1/staff"},
		{"GET", "/api/v1/sync/status"},
		{"GET", "/api/v1/cohorts"},
		{"GET", "/api/v1/program"},
		{"GET", "/api/v1/cohorts/97/students"},
		{"GET", "/api/v1/cohorts/97/progress"},
		{"GET", "/api/v1/cohorts/97/health"},
		{"GET", "/api/v1/kpi/on-track"},
		{"GET", "/api/v1/interventions"},
		{"POST", "/api/v1/interventions"},
		{"DELETE", "/api/v1/interventions/1"},
		{"POST", "/api/v1/mentor-assignments"},
		{"DELETE", "/api/v1/mentor-assignments/4"},
		{"GET", "/api/v1/students/4/mentor-assignments"},
	}
	for _, route := range routes {
		if rec := api.do(route.method, route.path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without session = %d", route.method, route.path, rec.Code)
		}
		bogus := &http.Cookie{Name: "pulse_session", Value: "not-a-real-session"}
		if rec := api.do(route.method, route.path, "", bogus); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with unknown session = %d", route.method, route.path, rec.Code)
		}
	}
	if rec := api.do("GET", "/health/live", "", nil); rec.Code != http.StatusOK {
		t.Errorf("health/live = %d", rec.Code)
	}
	if rec := api.do("POST", "/api/v1/session", "{", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("sign-in must be reachable without a session, got %d", rec.Code)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	api := newTestAPI(t)
	cookie := api.cookieFor(learnerAID)
	if _, err := api.db.ExecContext(context.Background(), `UPDATE app_sessions SET expires_at = NOW() - INTERVAL '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if rec := api.do("GET", "/api/v1/session", "", cookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired session = %d", rec.Code)
	}
}

func TestRosterIsScopedByRole(t *testing.T) {
	api := newTestAPI(t)
	cases := []struct {
		name   string
		person int64
		want   string
	}{
		{"manager sees everyone", managerID, "learner.a,learner.b,learner.c"},
		{"mentor sees assigned students", mentorAID, "learner.a"},
		{"other mentor", mentorBID, "learner.b"},
		{"student sees only themselves", learnerCID, "learner.c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cookie := api.cookieFor(tc.person)
			for _, path := range []string{"/api/v1/cohorts/97/students", "/api/v1/students?cohortId=4"} {
				rec := api.do("GET", path, "", cookie)
				if rec.Code != http.StatusOK {
					t.Fatalf("%s = %d %s", path, rec.Code, rec.Body.String())
				}
				var logins []string
				if strings.HasPrefix(path, "/api/v1/cohorts") {
					body := decode[struct {
						Students []struct{ Login string } `json:"students"`
					}](t, rec)
					for _, s := range body.Students {
						logins = append(logins, s.Login)
					}
				} else {
					body := decode[struct {
						Data []struct{ Login string } `json:"data"`
					}](t, rec)
					for _, s := range body.Data {
						logins = append(logins, s.Login)
					}
				}
				if got := sortedJoin(logins); got != tc.want {
					t.Fatalf("%s logins = %s, want %s", path, got, tc.want)
				}
			}
			board := decode[struct {
				Students []struct{ Login string } `json:"students"`
			}](t, api.do("GET", "/api/v1/cohorts/97/progress", "", cookie))
			var logins []string
			for _, s := range board.Students {
				logins = append(logins, s.Login)
			}
			if got := sortedJoin(logins); got != tc.want {
				t.Fatalf("progress board logins = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSingleStudentEndpointsEnforceScope(t *testing.T) {
	api := newTestAPI(t)
	cases := []struct {
		person  int64
		student string
		want    int
	}{
		{managerID, "5", http.StatusOK},
		{mentorAID, "4", http.StatusOK},
		{mentorAID, "5", http.StatusForbidden},
		{mentorAID, "6", http.StatusForbidden},
		{learnerAID, "4", http.StatusOK},
		{learnerAID, "5", http.StatusForbidden},
		{learnerAID, "999", http.StatusForbidden},
	}
	for _, tc := range cases {
		cookie := api.cookieFor(tc.person)
		if rec := api.do("GET", "/api/v1/students/"+tc.student, "", cookie); rec.Code != tc.want {
			t.Errorf("person %d GET /students/%s = %d, want %d", tc.person, tc.student, rec.Code, tc.want)
		}
		if rec := api.do("GET", "/api/v1/students/"+tc.student+"/progress", "", cookie); rec.Code != tc.want {
			t.Errorf("person %d GET /students/%s/progress = %d, want %d", tc.person, tc.student, rec.Code, tc.want)
		}
	}
}

func TestStaffOnlyRoutes(t *testing.T) {
	api := newTestAPI(t)
	student, mentor, manager := api.cookieFor(learnerAID), api.cookieFor(mentorAID), api.cookieFor(managerID)
	for _, path := range []string{"/api/v1/staff", "/api/v1/cohorts/97/health", "/api/v1/kpi/on-track"} {
		if rec := api.do("GET", path, "", student); rec.Code != http.StatusForbidden {
			t.Errorf("student %s = %d", path, rec.Code)
		}
		if rec := api.do("GET", path, "", mentor); rec.Code != http.StatusOK {
			t.Errorf("mentor %s = %d", path, rec.Code)
		}
	}
	move := `{"studentId":6,"mentorId":2}`
	if rec := api.do("POST", "/api/v1/mentor-assignments", move, mentor); rec.Code != http.StatusForbidden {
		t.Errorf("mentor reassigning = %d", rec.Code)
	}
	if rec := api.do("POST", "/api/v1/mentor-assignments", move, student); rec.Code != http.StatusForbidden {
		t.Errorf("student reassigning = %d", rec.Code)
	}
	if rec := api.do("POST", "/api/v1/mentor-assignments", move, manager); rec.Code != http.StatusOK {
		t.Errorf("manager reassigning = %d %s", rec.Code, rec.Body.String())
	}
	// The move takes effect in the mentor's scope immediately.
	if rec := api.do("GET", "/api/v1/students/6", "", mentor); rec.Code != http.StatusOK {
		t.Errorf("mentor after assignment = %d", rec.Code)
	}
}

func TestFollowUpAuthorComesFromTheSession(t *testing.T) {
	api := newTestAPI(t)
	mentor := api.cookieFor(mentorAID)
	rec := api.do("POST", "/api/v1/interventions", `{"studentId":4,"mentorId":999,"action":"Talk about the next project"}`, mentor)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	var authorID int64
	var authorRole string
	if err := api.db.QueryRowContext(context.Background(), `SELECT mentor_id, author_role FROM interventions`).Scan(&authorID, &authorRole); err != nil {
		t.Fatal(err)
	}
	if authorID != mentorAID || authorRole != "mentor" {
		t.Fatalf("author = %d %q, want the signed-in mentor", authorID, authorRole)
	}
	if rec := api.do("POST", "/api/v1/interventions", `{"studentId":5,"action":"not my student"}`, mentor); rec.Code != http.StatusForbidden {
		t.Fatalf("mentor writing about another mentor's student = %d", rec.Code)
	}
	if rec := api.do("POST", "/api/v1/interventions", `{"studentId":4,"action":"self note"}`, api.cookieFor(learnerAID)); rec.Code != http.StatusForbidden {
		t.Fatalf("student writing a follow-up = %d", rec.Code)
	}
	if rec := api.do("POST", "/api/v1/interventions", `{"studentId":4040,"action":"x"}`, api.cookieFor(managerID)); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown student = %d", rec.Code)
	}

	type list struct {
		Data []struct {
			StudentID  int64  `json:"studentId"`
			AuthorName string `json:"authorName"`
			AuthorRole string `json:"authorRole"`
		} `json:"data"`
	}
	own := decode[list](t, api.do("GET", "/api/v1/interventions", "", api.cookieFor(learnerAID)))
	if len(own.Data) != 1 || own.Data[0].AuthorName != "Mentor A" || own.Data[0].AuthorRole != "mentor" {
		t.Fatalf("learner A notes = %+v", own.Data)
	}
	if other := decode[list](t, api.do("GET", "/api/v1/interventions", "", api.cookieFor(learnerBID))); len(other.Data) != 0 {
		t.Fatalf("learner B sees someone else's notes: %+v", other.Data)
	}
	if otherMentor := decode[list](t, api.do("GET", "/api/v1/interventions", "", api.cookieFor(mentorBID))); len(otherMentor.Data) != 0 {
		t.Fatalf("mentor B sees mentor A's student notes: %+v", otherMentor.Data)
	}
	if all := decode[list](t, api.do("GET", "/api/v1/interventions", "", api.cookieFor(managerID))); len(all.Data) != 1 {
		t.Fatalf("manager notes = %+v", all.Data)
	}
}

func TestSignInIssuesAnHttpOnlySessionForThe01EduUser(t *testing.T) {
	api := newTestAPI(t)
	var gotLogin string
	api.server.verify = func(_ context.Context, login, password string) (string, error) {
		gotLogin = login
		if password != "right" {
			return "", zeroone.ErrRejected
		}
		return fakeJWT(learnerASrc), nil
	}
	// The identity comes from the verified token's user id, not from the
	// login text, so a stale or different login string cannot pick an account.
	rec := api.do("POST", "/api/v1/session", `{"login":"Learner.A@example.com","password":"right"}`, nil)
	if rec.Code != http.StatusOK || gotLogin != "Learner.A@example.com" {
		t.Fatalf("sign-in = %d %s", rec.Code, rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_session" {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Value == "" {
		t.Fatalf("cookie = %+v", cookie)
	}
	var stored int
	_ = api.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM app_sessions WHERE token_hash = $1`, cookie.Value).Scan(&stored)
	if stored != 0 {
		t.Fatal("the raw session token must not be stored")
	}

	me := decode[struct {
		Login     string `json:"login"`
		PulseRole string `json:"pulseRole"`
	}](t, api.do("GET", "/api/v1/session", "", cookie))
	if me.Login != "learner.a" || me.PulseRole != "student" {
		t.Fatalf("session = %+v", me)
	}
	if rec := api.do("DELETE", "/api/v1/session", "", cookie); rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}
	if rec := api.do("GET", "/api/v1/session", "", cookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout = %d", rec.Code)
	}

	if rec := api.do("POST", "/api/v1/session", `{"login":"learner.a","password":"wrong"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d", rec.Code)
	}
	api.server.verify = func(context.Context, string, string) (string, error) { return fakeJWT(777), nil }
	if rec := api.do("POST", "/api/v1/session", `{"login":"someone","password":"x"}`, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("01Edu user outside the synced cohorts = %d", rec.Code)
	}
}

func TestSessionCookieSecureFlagIsConfigurable(t *testing.T) {
	api := newTestAPI(t)
	opts := DefaultOptions()
	opts.SecureCookie = false
	server := NewServerWithOptions(api.db, opts)
	server.verify = func(context.Context, string, string) (string, error) { return fakeJWT(learnerASrc), nil }
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/session", strings.NewReader(`{"login":"learner.a","password":"x"}`))
	server.Handler().ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()
	if rec.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("status %d cookies %+v", rec.Code, cookies)
	}
}

func TestSignInIsRateLimitedPerLoginAndPerIP(t *testing.T) {
	api := newTestAPI(t)
	calls := 0
	api.server.verify = func(context.Context, string, string) (string, error) {
		calls++
		return "", zeroone.ErrRejected
	}
	for i := 0; i < 5; i++ {
		if rec := api.do("POST", "/api/v1/session", `{"login":"learner.a","password":"guess"}`, nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d", i+1, rec.Code)
		}
	}
	rec := api.do("POST", "/api/v1/session", `{"login":"@Learner.A","password":"guess"}`, nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("6th failure for one login = %d", rec.Code)
	}
	if calls != 5 {
		t.Fatalf("a blocked attempt must not reach 01Edu (calls=%d)", calls)
	}
	// Per IP: many different logins from one address.
	for i := 6; i < 21; i++ { // 5 counted above + 15 = the 20-attempt IP limit
		api.do("POST", "/api/v1/session", `{"login":"user`+string(rune('a'+i))+`","password":"x"}`, nil)
	}
	if rec := api.do("POST", "/api/v1/session", `{"login":"fresh.login","password":"x"}`, nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP over its limit = %d", rec.Code)
	}
}

func TestCohortHealthResolvesPilotEvent(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/cohorts/97/health", "", api.cookieFor(managerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := decode[struct {
		Data struct {
			CohortID      int `json:"cohortId"`
			TotalStudents int `json:"totalStudents"`
		} `json:"data"`
	}](t, rec)
	if body.Data.CohortID != 4 || body.Data.TotalStudents != 3 {
		t.Fatalf("health = %+v", body.Data)
	}
}

func TestStudentLookupPrefersInternalID(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/students/4", "", api.cookieFor(managerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := decode[struct {
		Data struct {
			Login string `json:"login"`
		} `json:"data"`
	}](t, rec)
	if body.Data.Login != "learner.a" {
		t.Fatalf("id 4 resolved to %s", body.Data.Login)
	}
}

func sortedJoin(values []string) string {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
	return strings.Join(values, ",")
}

func TestUnassignAndHistoryEndpoints(t *testing.T) {
	api := newTestAPI(t)
	student, mentorA, mentorB, manager := api.cookieFor(learnerAID), api.cookieFor(mentorAID), api.cookieFor(mentorBID), api.cookieFor(managerID)

	// Only the program manager may unassign.
	for name, cookie := range map[string]*http.Cookie{"student": student, "mentor": mentorA} {
		if rec := api.do("DELETE", "/api/v1/mentor-assignments/4", "", cookie); rec.Code != http.StatusForbidden {
			t.Errorf("%s unassigning = %d", name, rec.Code)
		}
	}
	if rec := api.do("DELETE", "/api/v1/mentor-assignments/999", "", manager); rec.Code != http.StatusNotFound {
		t.Errorf("unassign unknown student = %d", rec.Code)
	}
	if rec := api.do("DELETE", "/api/v1/mentor-assignments/2", "", manager); rec.Code != http.StatusBadRequest {
		t.Errorf("unassign a staff member = %d", rec.Code)
	}

	// Move learner A from mentor A to mentor B, then unassign.
	if rec := api.do("POST", "/api/v1/mentor-assignments", `{"studentId":4,"mentorId":3,"reason":"workload","assignedBy":999}`, manager); rec.Code != http.StatusOK {
		t.Fatalf("move = %d %s", rec.Code, rec.Body.String())
	}
	if rec := api.do("GET", "/api/v1/students/4", "", mentorA); rec.Code != http.StatusForbidden {
		t.Errorf("previous mentor still sees the student: %d", rec.Code)
	}
	type history struct {
		Data []store.MentorAssignment `json:"data"`
	}
	h := decode[history](t, api.do("GET", "/api/v1/students/4/mentor-assignments", "", mentorB))
	if len(h.Data) != 2 || !h.Data[0].Current || h.Data[0].MentorID != mentorBID || h.Data[0].Reason != "workload" ||
		h.Data[0].AssignedBy == nil || *h.Data[0].AssignedBy != managerID || h.Data[1].Current || h.Data[1].MentorID != mentorAID {
		t.Fatalf("history after move = %+v", h.Data)
	}

	rec := api.do("DELETE", "/api/v1/mentor-assignments/4?reason=graduated", "", manager)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"unassigned":true`) {
		t.Fatalf("unassign = %d %s", rec.Code, rec.Body.String())
	}
	if rec := api.do("DELETE", "/api/v1/mentor-assignments/4", "", manager); !strings.Contains(rec.Body.String(), `"unassigned":false`) {
		t.Errorf("second unassign = %s", rec.Body.String())
	}
	if rec := api.do("GET", "/api/v1/students/4", "", mentorB); rec.Code != http.StatusForbidden {
		t.Errorf("unassigned mentor still sees the student: %d", rec.Code)
	}
	h = decode[history](t, api.do("GET", "/api/v1/students/4/mentor-assignments", "", manager))
	if len(h.Data) != 2 || h.Data[0].Current || h.Data[0].EndedBy == nil || *h.Data[0].EndedBy != managerID ||
		!strings.Contains(h.Data[0].EndReason, "graduated") {
		t.Fatalf("history after unassign = %+v", h.Data)
	}

	// History is staff-only and scoped.
	if rec := api.do("GET", "/api/v1/students/4/mentor-assignments", "", student); rec.Code != http.StatusForbidden {
		t.Errorf("student reading history = %d", rec.Code)
	}
	if rec := api.do("GET", "/api/v1/students/5/mentor-assignments", "", mentorA); rec.Code != http.StatusForbidden {
		t.Errorf("mentor A reading mentor B's student history = %d", rec.Code)
	}
}

func TestDeleteFollowUp(t *testing.T) {
	api := newTestAPI(t)
	mentorA, mentorB, manager := api.cookieFor(mentorAID), api.cookieFor(mentorBID), api.cookieFor(managerID)
	learnerA := api.cookieFor(learnerAID)
	create := func(cookie *http.Cookie, studentID int64) int64 {
		t.Helper()
		rec := api.do("POST", "/api/v1/interventions", fmt.Sprintf(`{"studentId":%d,"action":"note"}`, studentID), cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
		}
		return decode[struct {
			Data struct {
				ID        int64 `json:"id"`
				CanDelete bool  `json:"canDelete"`
			} `json:"data"`
		}](t, rec).Data.ID
	}
	type note struct {
		ID        int64 `json:"id"`
		CanDelete bool  `json:"canDelete"`
	}
	list := func(cookie *http.Cookie) map[int64]bool {
		t.Helper()
		out := map[int64]bool{}
		for _, n := range decode[struct {
			Data []note `json:"data"`
		}](t, api.do("GET", "/api/v1/interventions", "", cookie)).Data {
			out[n.ID] = n.CanDelete
		}
		return out
	}
	byA := create(mentorA, learnerAID)
	byA2 := create(mentorA, learnerAID)
	byManager := create(manager, learnerAID)
	byB := create(mentorB, learnerBID)

	// canDelete tells the web app when to show the ✕.
	if got := list(mentorA); !got[byA] || got[byManager] {
		t.Fatalf("mentor A canDelete = %v (own note yes, manager's note no)", got)
	}
	if got := list(learnerA); got[byA] || got[byManager] || len(got) != 3 {
		t.Fatalf("learner canDelete = %v, want 3 notes, none deletable", got)
	}
	if got := list(manager); !got[byA] || !got[byB] {
		t.Fatalf("manager canDelete = %v", got)
	}

	// Another mentor and a student may not delete.
	if rec := api.do("DELETE", fmt.Sprintf("/api/v1/interventions/%d", byA), "", mentorB); rec.Code != http.StatusForbidden {
		t.Errorf("other mentor delete = %d", rec.Code)
	}
	if rec := api.do("DELETE", fmt.Sprintf("/api/v1/interventions/%d", byA), "", learnerA); rec.Code != http.StatusForbidden {
		t.Errorf("student delete = %d", rec.Code)
	}
	if rec := api.do("DELETE", fmt.Sprintf("/api/v1/interventions/%d", byManager), "", mentorA); rec.Code != http.StatusForbidden {
		t.Errorf("mentor deleting the manager's note = %d", rec.Code)
	}
	// The author may delete their own note; a manager may delete anyone's.
	if rec := api.do("DELETE", fmt.Sprintf("/api/v1/interventions/%d", byA), "", mentorA); rec.Code != http.StatusNoContent {
		t.Fatalf("author delete = %d %s", rec.Code, rec.Body.String())
	}
	if rec := api.do("DELETE", fmt.Sprintf("/api/v1/interventions/%d", byA2), "", manager); rec.Code != http.StatusNoContent {
		t.Fatalf("manager delete = %d %s", rec.Code, rec.Body.String())
	}
	// Missing or already deleted: 404.
	for _, path := range []string{fmt.Sprintf("/api/v1/interventions/%d", byA), "/api/v1/interventions/99999"} {
		if rec := api.do("DELETE", path, "", manager); rec.Code != http.StatusNotFound {
			t.Errorf("DELETE %s = %d, want 404", path, rec.Code)
		}
	}
	if rec := api.do("DELETE", "/api/v1/interventions/abc", "", manager); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id = %d", rec.Code)
	}

	// Deleted notes are hidden from every list but kept with who deleted them.
	for name, cookie := range map[string]*http.Cookie{"manager": manager, "mentor A": mentorA, "learner A": learnerA} {
		got := list(cookie)
		if _, ok := got[byA]; ok {
			t.Errorf("%s still lists the author-deleted note", name)
		}
		if _, ok := got[byA2]; ok {
			t.Errorf("%s still lists the manager-deleted note", name)
		}
	}
	var deletedBy int64
	var deletedAt time.Time
	if err := api.db.QueryRowContext(context.Background(), `SELECT deleted_by, deleted_at FROM interventions WHERE id = $1`, byA2).Scan(&deletedBy, &deletedAt); err != nil || deletedBy != managerID || deletedAt.IsZero() {
		t.Fatalf("soft delete record = %d %v %v", deletedBy, deletedAt, err)
	}
}

func TestConfiguredEventIsRespected(t *testing.T) {
	api := newTestAPI(t) // learners 4, 5, 6 are in the default event, cohort 4
	ctx := context.Background()
	const event = 120
	if _, err := api.db.ExecContext(ctx, `INSERT INTO students (id, source_user_id, login, first_name, source_role) VALUES (7, 107, 'learner.d', 'Learner', 'user');
		INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, audit_ratio, last_synced_at)
		VALUES (7, 120, 7, '2026-01-01T00:00:00Z', 2, 0, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := api.db.SeedCurriculumTimeline(ctx, event); err != nil {
		t.Fatal(err)
	}
	api.server.SetProgram(pulsesync.LiveCycleConfig{EventID: event, Cohorts: []pulsesync.CohortLabel{{ID: 7, Label: "Cohort Seven"}}})
	manager := api.cookieFor(managerID)

	program := decode[struct {
		EventID         int `json:"eventId"`
		DefaultCohortID int `json:"defaultCohortId"`
		Cohorts         []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"cohorts"`
		PaceRule paceRule `json:"paceRule"`
	}](t, api.do("GET", "/api/v1/program", "", manager))
	if program.EventID != event || program.DefaultCohortID != 7 || len(program.Cohorts) != 1 || program.Cohorts[0].Name != "Cohort Seven" {
		t.Fatalf("program = %+v", program)
	}
	if program.PaceRule.EventID != event || program.PaceRule.Version != "draft-v0" || program.PaceRule.Status != "draft" || program.PaceRule.Approved {
		t.Fatalf("pace rule = %+v", program.PaceRule)
	}

	cohort := decode[struct {
		EventID    int    `json:"eventId"`
		CohortID   int    `json:"cohortId"`
		CohortName string `json:"cohortName"`
		PaceRule   *paceRule
		Students   []struct {
			StudentID int64 `json:"studentId"`
		} `json:"students"`
	}](t, api.do("GET", "/api/v1/cohorts/120/students", "", manager))
	if cohort.EventID != event || cohort.CohortID != 7 || cohort.CohortName != "Cohort Seven" || len(cohort.Students) != 1 || cohort.Students[0].StudentID != 7 {
		t.Fatalf("cohort students = %+v", cohort)
	}
	if rec := api.do("GET", "/api/v1/cohorts/97/students", "", manager); rec.Code != http.StatusNotFound {
		t.Errorf("the default event still resolves: %d", rec.Code)
	}
	all := decode[struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}](t, api.do("GET", "/api/v1/students", "", manager))
	if len(all.Data) != 1 || all.Data[0].ID != 7 || all.Meta["eventId"] != float64(event) {
		t.Fatalf("students of the configured event = %+v", all)
	}
	health := decode[struct {
		EventID  int      `json:"eventId"`
		PaceRule paceRule `json:"paceRule"`
		Data     struct {
			CohortName    string `json:"cohortName"`
			TotalStudents int    `json:"totalStudents"`
		} `json:"data"`
	}](t, api.do("GET", "/api/v1/cohorts/7/health", "", manager))
	if health.EventID != event || health.Data.CohortName != "Cohort Seven" || health.Data.TotalStudents != 1 || health.PaceRule.Version != "draft-v0" {
		t.Fatalf("health = %+v", health)
	}
	cohorts := decode[struct {
		Data []map[string]any `json:"data"`
	}](t, api.do("GET", "/api/v1/cohorts", "", manager))
	if len(cohorts.Data) != 1 || cohorts.Data[0]["eventId"] != float64(event) || cohorts.Data[0]["studentCount"] != float64(1) {
		t.Fatalf("cohorts = %+v", cohorts.Data)
	}
	if rec := api.do("GET", "/api/v1/students/4/progress", "", manager); rec.Code != http.StatusNotFound {
		t.Errorf("progress for a student outside the configured event = %d", rec.Code)
	}
}

type kpiBody struct {
	OnTrack       int            `json:"onTrack"`
	Measurable    int            `json:"measurable"`
	Rate          *float64       `json:"rate"`
	HasMeasurable bool           `json:"hasMeasurable"`
	MentorID      int64          `json:"mentorId"`
	CohortID      int            `json:"cohortId"`
	StatusCounts  map[string]int `json:"statusCounts"`
}

func (a *testAPI) setPace(studentID int64, status string) {
	a.t.Helper()
	if _, err := a.db.ExecContext(context.Background(), `INSERT INTO student_metric_snapshots (student_id, event_id, snapshot_date, enrolled_at,
		active_program_days, curriculum_month, actual_level, minimum_level, expected_level, checkpoint_target, gap_to_minimum, gap_to_expected, pace_status)
		VALUES ($1, 97, '2026-11-01', '2026-01-01', 300, 10, 5, 5, 6, 0, 0, -1, $2)`, studentID, status); err != nil {
		a.t.Fatal(err)
	}
}

func TestOnTrackKPIIsScopedByRole(t *testing.T) {
	api := newTestAPI(t)
	api.setPace(learnerAID, "GREEN")  // mentor A
	api.setPace(learnerBID, "YELLOW") // mentor B
	api.setPace(learnerCID, "RED")    // no mentor

	type response struct {
		Program *kpiBody  `json:"program"`
		Cohorts []kpiBody `json:"cohorts"`
		Mentors []kpiBody `json:"mentors"`
	}
	rec := api.do("GET", "/api/v1/kpi/on-track", "", api.cookieFor(managerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("manager = %d %s", rec.Code, rec.Body.String())
	}
	manager := decode[response](t, rec)
	// Program-wide: GREEN A, YELLOW B, RED C → 1 of 3 measurable on pace.
	if p := manager.Program; p == nil || p.OnTrack != 1 || p.Measurable != 3 || p.Rate == nil || p.StatusCounts["YELLOW"] != 1 {
		t.Fatalf("program = %+v", p)
	}
	if len(manager.Mentors) != 2 || len(manager.Cohorts) == 0 || manager.Cohorts[0].CohortID != 4 || manager.Cohorts[0].Measurable != 3 {
		t.Fatalf("manager mentors %+v cohorts %+v", manager.Mentors, manager.Cohorts)
	}
	for _, m := range manager.Mentors {
		want := map[int64]float64{mentorAID: 100, mentorBID: 0}[m.MentorID]
		if m.Measurable != 1 || m.Rate == nil || *m.Rate != want {
			t.Fatalf("mentor %d = %+v, want %v%%", m.MentorID, m, want)
		}
	}
	// A configured cohort with no students has a null rate, not 0%.
	for _, c := range manager.Cohorts[1:] {
		if c.Rate != nil || c.HasMeasurable {
			t.Fatalf("empty cohort = %+v", c)
		}
	}

	mentor := decode[response](t, api.do("GET", "/api/v1/kpi/on-track", "", api.cookieFor(mentorBID)))
	if mentor.Program != nil || len(mentor.Cohorts) != 0 || len(mentor.Mentors) != 1 || mentor.Mentors[0].MentorID != mentorBID ||
		mentor.Mentors[0].OnTrack != 0 || mentor.Mentors[0].Measurable != 1 || *mentor.Mentors[0].Rate != 0 {
		t.Fatalf("mentor B sees %+v", mentor)
	}

	if rec := api.do("GET", "/api/v1/kpi/on-track", "", api.cookieFor(learnerAID)); rec.Code != http.StatusForbidden {
		t.Fatalf("student = %d", rec.Code)
	}

	// Cohort health for a mentor covers only their students.
	health := decode[struct {
		Data struct {
			TotalStudents int      `json:"totalStudents"`
			OnTrackRate   *float64 `json:"onTrackRate"`
			KPI           kpiBody  `json:"kpi"`
		} `json:"data"`
	}](t, api.do("GET", "/api/v1/cohorts/97/health", "", api.cookieFor(mentorAID)))
	if health.Data.TotalStudents != 1 || health.Data.OnTrackRate == nil || *health.Data.OnTrackRate != 100 || health.Data.KPI.OnTrack != 1 {
		t.Fatalf("mentor A cohort health = %+v", health.Data)
	}
}

func TestOnTrackKPIForAMentorWithoutStudentsIsNull(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.db.ExecContext(context.Background(), `UPDATE mentor_assignments SET ends_at = NOW() WHERE mentor_id = $1`, mentorAID); err != nil {
		t.Fatal(err)
	}
	body := decode[struct {
		Mentors []kpiBody `json:"mentors"`
	}](t, api.do("GET", "/api/v1/kpi/on-track", "", api.cookieFor(mentorAID)))
	if len(body.Mentors) != 1 || body.Mentors[0].Rate != nil || body.Mentors[0].HasMeasurable || body.Mentors[0].Measurable != 0 {
		t.Fatalf("mentor without students = %+v", body.Mentors)
	}
	if !strings.Contains(api.do("GET", "/api/v1/kpi/on-track", "", api.cookieFor(mentorAID)).Body.String(), `"rate":null`) {
		t.Fatal("rate should be JSON null")
	}
}

func TestSyncStatusReportsTheSnapshotJobWithoutACredential(t *testing.T) {
	api := newTestAPI(t)
	job := pulsesync.NewSnapshotJob(api.db, 97, 24*time.Hour)
	api.server.SetSnapshotJob(job)
	body := decode[struct {
		Mode      string `json:"mode"`
		Snapshots *struct {
			IntervalSeconds int `json:"intervalSeconds"`
		} `json:"snapshots"`
	}](t, api.do("GET", "/api/v1/sync/status", "", api.cookieFor(managerID)))
	if body.Mode != "offline" || body.Snapshots == nil || body.Snapshots.IntervalSeconds != 86400 {
		t.Fatalf("sync status = %+v", body)
	}
}
