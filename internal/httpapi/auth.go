package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

// Pulse roles, derived on the server from the person's 01Edu role
// (store.PulseRoleFor): "admin" → manager, "admin campus" → mentor, else student.
const (
	roleManager = "manager"
	roleMentor  = "mentor"
	roleStudent = "student"
)

// Options configures sessions and sign-in protection.
type Options struct {
	// CookieName is the session cookie (HttpOnly, SameSite=Lax).
	CookieName string
	// SecureCookie sets the Secure flag. Turn it off only for plain-HTTP local dev.
	SecureCookie bool
	// SessionTTL is how long a session lasts after sign-in.
	SessionTTL time.Duration
	// TrustProxyHeaders uses X-Forwarded-For for the client IP (behind a proxy only).
	TrustProxyHeaders bool
	// Sign-in limits: attempts per IP and failures per login, per window.
	SignInPerIP      int
	SignInPerLogin   int
	SignInRateWindow time.Duration
}

// DefaultOptions are safe production defaults.
func DefaultOptions() Options {
	return Options{
		CookieName:       "pulse_session",
		SecureCookie:     true,
		SessionTTL:       12 * time.Hour,
		SignInPerIP:      20,
		SignInPerLogin:   5,
		SignInRateWindow: 15 * time.Minute,
	}
}

// OptionsFromEnv reads SESSION_COOKIE_SECURE, SESSION_TTL and TRUST_PROXY_HEADERS.
func OptionsFromEnv() (Options, error) {
	opts := DefaultOptions()
	if raw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return opts, errors.New("invalid SESSION_COOKIE_SECURE (true/false)")
		}
		opts.SecureCookie = secure
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_TTL")); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl < 5*time.Minute || ttl > 30*24*time.Hour {
			return opts, errors.New("invalid SESSION_TTL (Go duration between 5m and 720h)")
		}
		opts.SessionTTL = ttl
	}
	if raw := strings.TrimSpace(os.Getenv("TRUST_PROXY_HEADERS")); raw != "" {
		trust, err := strconv.ParseBool(raw)
		if err != nil {
			return opts, errors.New("invalid TRUST_PROXY_HEADERS (true/false)")
		}
		opts.TrustProxyHeaders = trust
	}
	return opts, nil
}

// Principal is the signed-in person. It comes only from the server session.
type Principal struct {
	StudentID    int64  `json:"studentId"`
	SourceUserID int64  `json:"sourceUserId"`
	Login        string `json:"login"`
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	SourceRole   string `json:"sourceRole"`
	Role         string `json:"pulseRole"`
}

func principalFrom(member *store.StaffMember) Principal {
	return Principal{
		StudentID:    member.StudentID,
		SourceUserID: member.SourceUserID,
		Login:        member.Login,
		DisplayName:  member.DisplayName,
		Email:        member.Email,
		SourceRole:   member.SourceRole,
		Role:         store.PulseRoleFor(member.SourceRole),
	}
}

type principalKey struct{}

func withPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// principalOf returns the signed-in person. Every /api/v1 handler except
// sign-in runs behind requireSession, so it is always present there.
func principalOf(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// isPublic lists the only routes reachable without a session.
func isPublic(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return true // /health/live, /health/ready
	}
	return r.Method == http.MethodPost && r.URL.Path == "/api/v1/session"
}

// requireSession rejects /api requests without a valid session cookie and
// attaches the Principal to the request context.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(s.opts.CookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in with your 01Edu login.")
			return
		}
		member, _, err := s.db.SessionPerson(r.Context(), cookie.Value)
		if errors.Is(err, store.ErrNoSession) {
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Your session has ended. Sign in again.")
			return
		}
		if err != nil {
			log.Printf("session lookup failed: %v", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not check the session.")
			return
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principalFrom(member))))
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.opts.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(math.Max(1, time.Until(expires).Seconds())),
		HttpOnly: true,
		Secure:   s.opts.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.opts.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.opts.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

type sessionResponse struct {
	Principal
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// handleSignIn checks the login and password with 01Edu, then issues a Pulse
// session. The identity comes from the verified 01Edu token's user id, not
// from the login text the browser sent.
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
	ip := clientIP(r, s.opts.TrustProxyHeaders)
	loginKey := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.Login), "@")))
	if blocked, wait := s.ipLimiter.blocked(ip); blocked {
		tooManyAttempts(w, wait)
		return
	}
	if blocked, wait := s.loginLimiter.blocked(loginKey); blocked {
		tooManyAttempts(w, wait)
		return
	}
	s.ipLimiter.add(ip)

	token, err := s.verify(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, zeroone.ErrRejected) {
			s.loginLimiter.add(loginKey)
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "The 01Edu login or password is incorrect.")
			return
		}
		log.Printf("01Edu sign-in unavailable: %v", err)
		writeError(w, http.StatusBadGateway, "SCHOOL_UNAVAILABLE", "The 01Edu sign-in service did not respond.")
		return
	}

	var person *store.StaffMember
	if _, userID, _, ok := zeroone.SessionAccess(token); ok && userID > 0 {
		person, err = s.db.FindIdentityBySourceUserID(r.Context(), userID)
	} else {
		// 01Edu accepted the password but its token carries no user id; the
		// login it accepted is then the only identity available.
		person, err = s.db.FindIdentityByLogin(r.Context(), req.Login)
	}
	if err != nil {
		log.Printf("sign-in identity lookup failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not sign in. Please try again.")
		return
	}
	if person == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "This 01Edu account is not in the synced cohorts.")
		return
	}
	s.loginLimiter.reset(loginKey)
	sessionToken, expires, err := s.db.CreateSession(r.Context(), person.StudentID, s.opts.SessionTTL, r.UserAgent(), ip)
	if err != nil {
		log.Printf("create session failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not sign in. Please try again.")
		return
	}
	s.setSessionCookie(w, sessionToken, expires)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, sessionResponse{Principal: principalFrom(person), ExpiresAt: expires.UTC().Format(time.RFC3339)})
}

func tooManyAttempts(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	writeError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Too many sign-in attempts. Try again later.")
}

// handleGetSession returns the signed-in person (GET /api/v1/session).
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	p, _ := principalOf(r.Context())
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, sessionResponse{Principal: p})
}

// handleSignOut ends the session (DELETE /api/v1/session).
func (s *Server) handleSignOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.opts.CookieName); err == nil {
		if err := s.db.DeleteSession(r.Context(), cookie.Value); err != nil {
			log.Printf("delete session failed: %v", err)
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// requireRole wraps a handler so only these Pulse roles reach it.
func requireRole(handler http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalOf(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in with your 01Edu login.")
			return
		}
		for _, role := range roles {
			if p.Role == role {
				handler(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Your role cannot do this.")
	}
}

// studentScope is the set of learners a principal may see.
type studentScope struct {
	all bool
	ids map[int64]bool
}

func (sc studentScope) allows(studentID int64) bool {
	return sc.all || sc.ids[studentID]
}

// scopeFor: a manager sees everyone, a mentor their assigned students, a
// student only themselves.
func (s *Server) scopeFor(ctx context.Context, p Principal) (studentScope, error) {
	switch p.Role {
	case roleManager:
		return studentScope{all: true}, nil
	case roleMentor:
		ids, err := s.db.AssignedStudentIDs(ctx, p.StudentID)
		return studentScope{ids: ids}, err
	default:
		return studentScope{ids: map[int64]bool{p.StudentID: true}}, nil
	}
}

// requestScope resolves the caller's scope or writes an error.
func (s *Server) requestScope(w http.ResponseWriter, r *http.Request) (Principal, studentScope, bool) {
	p, ok := principalOf(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in with your 01Edu login.")
		return p, studentScope{}, false
	}
	scope, err := s.scopeFor(r.Context(), p)
	if err != nil {
		log.Printf("scope lookup failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not check access.")
		return p, studentScope{}, false
	}
	return p, scope, true
}

func forbidden(w http.ResponseWriter) {
	writeError(w, http.StatusForbidden, "FORBIDDEN", "You do not have access to this student.")
}
