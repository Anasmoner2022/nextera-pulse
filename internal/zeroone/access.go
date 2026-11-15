package zeroone

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// SessionAccess reads the role and user id from a 01Edu session token.
// The token is not verified here; it was just returned by the school over HTTPS.
func SessionAccess(token string) (role string, userID int64, expires time.Time, ok bool) {
	payload := tokenPayload(token)
	if payload == nil {
		return "", 0, time.Time{}, false
	}
	role = highestRole(payload)
	userID = claimUserID(payload)
	if exp, ok := numberClaim(payload["exp"]); ok {
		expires = time.Unix(int64(exp), 0).UTC()
	}
	return role, userID, expires, role != "" && userID > 0
}

// AllowedRoles lists the Hasura roles carried by a 01Edu session token.
func AllowedRoles(token string) []string {
	payload := tokenPayload(token)
	if payload == nil {
		return nil
	}
	return roleNames(payload)
}

// CohortProgressRole reports whether this 01Edu role can read other students' progress.
func CohortProgressRole(role string) bool {
	switch strings.ReplaceAll(strings.TrimSpace(role), "_", "-") {
	case "admin", "admin-read-only", "campus-admin", "campus-admin-read-only":
		return true
	default:
		return false
	}
}

func tokenPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func highestRole(payload map[string]any) string {
	roles := roleNames(payload)
	order := []string{"admin", "admin-read-only", "admin_read_only", "campus-admin", "campus_admin", "campus-admin-read-only", "campus_admin_read_only", "user"}
	for _, candidate := range order {
		for _, role := range roles {
			if role == candidate || strings.ReplaceAll(role, "_", "-") == strings.ReplaceAll(candidate, "_", "-") {
				return role
			}
		}
	}
	if len(roles) > 0 {
		return roles[0]
	}
	return ""
}

func roleNames(payload map[string]any) []string {
	claims, _ := payload["https://hasura.io/jwt/claims"].(map[string]any)
	if claims == nil {
		return nil
	}
	seen := map[string]bool{}
	var roles []string
	add := func(role string) {
		role = strings.TrimSpace(role)
		if role == "" || seen[role] {
			return
		}
		seen[role] = true
		roles = append(roles, role)
	}
	if value, ok := claims["x-hasura-default-role"].(string); ok {
		add(value)
	}
	if list, ok := claims["x-hasura-allowed-roles"].([]any); ok {
		for _, item := range list {
			if role, ok := item.(string); ok {
				add(role)
			}
		}
	}
	return roles
}

func claimUserID(payload map[string]any) int64 {
	claims, _ := payload["https://hasura.io/jwt/claims"].(map[string]any)
	if claims != nil {
		if id, ok := numberClaim(claims["x-hasura-user-id"]); ok {
			return int64(id)
		}
		if text, ok := claims["x-hasura-user-id"].(string); ok {
			if id, err := strconv.ParseInt(text, 10, 64); err == nil {
				return id
			}
		}
	}
	if id, ok := numberClaim(payload["sub"]); ok {
		return int64(id)
	}
	if text, ok := payload["sub"].(string); ok {
		if id, err := strconv.ParseInt(text, 10, 64); err == nil {
			return id
		}
	}
	return 0
}

func numberClaim(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}
