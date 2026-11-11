package zeroone

import (
	"encoding/base64"
	"testing"
)

func TestSessionAccessPrefersTheCohortRole(t *testing.T) {
	payload := []byte(`{"sub":"15","exp":2000000000,"https://hasura.io/jwt/claims":{"x-hasura-default-role":"user","x-hasura-allowed-roles":["user","campus-admin"],"x-hasura-user-id":"15"}}`)
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	role, userID, _, ok := SessionAccess(token)
	if !ok || role != "campus-admin" || userID != 15 || !CohortProgressRole(role) {
		t.Fatalf("role %s user %d ok %v", role, userID, ok)
	}
}
