package zeroone

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fakeJWT(t *testing.T, claims string) string {
	t.Helper()
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"none"}`)) + "." + enc.EncodeToString([]byte(claims)) + ".sig"
}

func TestCredentialModes(t *testing.T) {
	jwt := fakeJWT(t, `{"sub":"1"}`)
	cases := map[string]Credentials{
		ModeJWT:      {AccessToken: jwt},
		ModeAppToken: {AccessToken: "0123456789abcdef0123456789abcdef01234567"},
		ModePassword: {Login: "svc", Password: "pw"},
		"":           {Login: "svc"},
	}
	for want, creds := range cases {
		if got := creds.Mode(); got != want {
			t.Errorf("mode %q, want %q", got, want)
		}
	}
}

func TestTokenSourceExchangesAppTokenAndPicksReadRole(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	jwt := fakeJWT(t, `{"sub":"7","exp":`+itoa(exp)+`,"https://hasura.io/jwt/claims":{"x-hasura-default-role":"user","x-hasura-allowed-roles":["user","admin_read_only"],"x-hasura-user-id":"7"}}`)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/auth/token" || r.URL.Query().Get("token") != "gitea-secret" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`"` + jwt + `"`))
	}))
	defer srv.Close()
	src, err := NewTokenSource(Credentials{BaseURL: srv.URL, AccessToken: "gitea-secret"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		got, err := src.Token(context.Background())
		if err != nil || got != jwt {
			t.Fatalf("token %q err %v", got, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("exchange called %d times, want 1 (cached)", calls.Load())
	}
	if src.Role() != "admin_read_only" || !src.CanReadCohort() {
		t.Fatalf("role %q cohort %v", src.Role(), src.CanReadCohort())
	}
	src.Invalidate()
	if _, err := src.Token(context.Background()); err != nil || calls.Load() != 2 {
		t.Fatalf("invalidate did not renew: calls %d err %v", calls.Load(), err)
	}
}

func TestTokenSourceRejectedAppTokenDoesNotLeakIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Response code 401 (Unauthorized)"}`))
	}))
	defer srv.Close()
	src, _ := NewTokenSource(Credentials{BaseURL: srv.URL, AccessToken: "very-secret-value"}, srv.Client())
	_, err := src.Token(context.Background())
	if err == nil || strings.Contains(err.Error(), "very-secret-value") {
		t.Fatalf("err %v", err)
	}
}

func TestChooseRoleOnlySendsAllowedRoles(t *testing.T) {
	jwt := fakeJWT(t, `{"https://hasura.io/jwt/claims":{"x-hasura-default-role":"user","x-hasura-allowed-roles":["user"]}}`)
	if role := chooseRole("campus-admin", jwt); role != "" {
		t.Fatalf("sent a role the JWT does not allow: %q", role)
	}
	if role := chooseRole("", jwt); role != "" {
		t.Fatalf("role %q, want default", role)
	}
	if role := chooseRole("user", jwt); role != "user" {
		t.Fatalf("role %q", role)
	}
}

func TestExpiredStaticJWTIsRefused(t *testing.T) {
	jwt := fakeJWT(t, `{"exp":1000}`)
	src, _ := NewTokenSource(Credentials{BaseURL: "https://example.org", AccessToken: jwt}, nil)
	if _, err := src.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("err %v", err)
	}
}

func TestClientWithTokensSendsBearerAndRole(t *testing.T) {
	jwt := fakeJWT(t, `{"https://hasura.io/jwt/claims":{"x-hasura-default-role":"user","x-hasura-allowed-roles":["user","campus-admin"]}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+jwt || r.Header.Get("x-hasura-role") != "campus-admin" {
			t.Errorf("auth %q role %q", r.Header.Get("Authorization"), r.Header.Get("x-hasura-role"))
		}
		_, _ = w.Write([]byte(`{"data":{"record":[{"id":1,"userId":2,"typeName":"away","startAt":"2026-10-01T00:00:00Z"}]}}`))
	}))
	defer srv.Close()
	src, _ := NewTokenSource(Credentials{BaseURL: "https://example.org", AccessToken: jwt}, nil)
	client, err := NewClientWithTokens(srv.URL, src, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	records, err := client.RecordsForUsers(context.Background(), []int64{2})
	if err != nil || len(records) != 1 || records[0].TypeName != "away" {
		t.Fatalf("records %+v err %v", records, err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
