package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// ErrNoSession means the cookie token is unknown or expired.
var ErrNoSession = errors.New("no valid session")

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession stores a new session for a person and returns the raw token
// for the cookie. Only its SHA-256 hash is stored.
func (db *DB) CreateSession(ctx context.Context, studentID int64, ttl time.Duration, userAgent, ip string) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	expires := time.Now().UTC().Add(ttl)
	if len(userAgent) > 300 {
		userAgent = userAgent[:300]
	}
	_, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO app_sessions (token_hash, student_id, expires_at, user_agent, ip) VALUES (?, ?, ?, ?, ?)`),
		hashToken(token), studentID, expires, userAgent, ip)
	if err != nil {
		return "", time.Time{}, err
	}
	// Opportunistic cleanup; failures do not affect the new session.
	_, _ = db.ExecContext(ctx, `DELETE FROM app_sessions WHERE expires_at < NOW() - INTERVAL '1 day'`)
	return token, expires, nil
}

// SessionPerson returns the person behind a valid, unexpired session token.
func (db *DB) SessionPerson(ctx context.Context, token string) (*StaffMember, time.Time, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 200 {
		return nil, time.Time{}, ErrNoSession
	}
	hash := hashToken(token)
	var member StaffMember
	var firstName, lastName string
	var expires time.Time
	err := db.QueryRowContext(ctx, db.Rebind(`SELECT s.id, s.source_user_id, s.login, COALESCE(s.first_name, ''), COALESCE(s.last_name, ''), COALESCE(s.email, ''), COALESCE(s.source_role, ''), a.expires_at
		FROM app_sessions a JOIN students s ON s.id = a.student_id
		WHERE a.token_hash = ? AND a.expires_at > NOW()`), hash).
		Scan(&member.StudentID, &member.SourceUserID, &member.Login, &firstName, &lastName, &member.Email, &member.SourceRole, &expires)
	if err == sql.ErrNoRows {
		return nil, time.Time{}, ErrNoSession
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	member.DisplayName = strings.TrimSpace(firstName + " " + lastName)
	if member.DisplayName == "" {
		member.DisplayName = member.Login
	}
	member.PulseRole = PulseRoleFor(member.SourceRole)
	_, _ = db.ExecContext(ctx, db.Rebind(`UPDATE app_sessions SET last_seen_at = NOW() WHERE token_hash = ? AND last_seen_at < NOW() - INTERVAL '1 minute'`), hash)
	return &member, expires, nil
}

// DeleteSession ends a session (logout). Unknown tokens are ignored.
func (db *DB) DeleteSession(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, db.Rebind(`DELETE FROM app_sessions WHERE token_hash = ?`), hashToken(token))
	return err
}

// FindIdentityBySourceUserID returns the person with this 01Edu user id.
func (db *DB) FindIdentityBySourceUserID(ctx context.Context, sourceUserID int64) (*StaffMember, error) {
	if sourceUserID <= 0 {
		return nil, nil
	}
	var member StaffMember
	var firstName, lastName string
	err := db.QueryRowContext(ctx, db.Rebind(`SELECT id, source_user_id, login, COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(email, ''), COALESCE(source_role, '')
		FROM students WHERE source_user_id = ?`), sourceUserID).
		Scan(&member.StudentID, &member.SourceUserID, &member.Login, &firstName, &lastName, &member.Email, &member.SourceRole)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	member.DisplayName = strings.TrimSpace(firstName + " " + lastName)
	if member.DisplayName == "" {
		member.DisplayName = member.Login
	}
	member.PulseRole = PulseRoleFor(member.SourceRole)
	return &member, nil
}

// AssignedStudentIDs lists the learners whose current primary mentor is mentorID.
func (db *DB) AssignedStudentIDs(ctx context.Context, mentorID int64) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, db.Rebind(`SELECT DISTINCT student_id FROM mentor_assignments
		WHERE mentor_id = ? AND is_primary = TRUE AND ends_at IS NULL`), mentorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// StudentExists reports whether a person row exists.
func (db *DB) StudentExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, db.Rebind(`SELECT EXISTS (SELECT 1 FROM students WHERE id = ?)`), id).Scan(&exists)
	return exists, err
}
