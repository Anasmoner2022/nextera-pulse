package store

import (
	"context"
	"database/sql"
	"strings"
)

// StaffMember is a student whose 01Edu role maps to a Pulse workspace role.
type StaffMember struct {
	StudentID    int64  `json:"studentId"`
	SourceUserID int64  `json:"sourceUserId"`
	Login        string `json:"login"`
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	SourceRole   string `json:"sourceRole"`
	PulseRole    string `json:"pulseRole"`
}

// ListStaff returns 01Edu admins mapped onto Pulse roles.
// "admin campus" is a mentor. "admin" is a project manager.
// The export has no student-to-mentor link, so this does not invent assignments.
func (db *DB) ListStaff(ctx context.Context) ([]StaffMember, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, source_user_id, login, COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(email, ''), COALESCE(source_role, '')
		FROM students
		WHERE source_role IN ('admin', 'admin campus')
		ORDER BY source_role, login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var staff []StaffMember
	for rows.Next() {
		var member StaffMember
		var firstName, lastName string
		if err := rows.Scan(&member.StudentID, &member.SourceUserID, &member.Login, &firstName, &lastName, &member.Email, &member.SourceRole); err != nil {
			return nil, err
		}
		member.DisplayName = strings.TrimSpace(firstName + " " + lastName)
		if member.DisplayName == "" {
			member.DisplayName = member.Login
		}
		member.PulseRole = PulseRoleFor(member.SourceRole)
		staff = append(staff, member)
	}
	return staff, rows.Err()
}

// PulseRoleFor maps a 01Edu user.role onto the dashboard role.
// "admin campus" is a mentor, "admin" is a project manager, and every other role is a student.
func PulseRoleFor(sourceRole string) string {
	switch strings.TrimSpace(sourceRole) {
	case "admin campus":
		return "mentor"
	case "admin":
		return "manager"
	default:
		return "student"
	}
}

// FindIdentityByLogin returns the 01Edu person for a login or email.
// The same person is the dashboard account. There is no separate password or permission record.
func (db *DB) FindIdentityByLogin(ctx context.Context, login string) (*StaffMember, error) {
	login = strings.TrimSpace(strings.TrimPrefix(login, "@"))
	if login == "" {
		return nil, nil
	}
	var member StaffMember
	var firstName, lastName string
	query := db.Rebind(`SELECT id, source_user_id, login, COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(email, ''), COALESCE(source_role, '')
		FROM students
		WHERE lower(login) = lower(?) OR lower(COALESCE(email, '')) = lower(?)
		LIMIT 1`)
	err := db.QueryRowContext(ctx, query, login, login).Scan(&member.StudentID, &member.SourceUserID, &member.Login, &firstName, &lastName, &member.Email, &member.SourceRole)
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
