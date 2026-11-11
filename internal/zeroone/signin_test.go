package zeroone

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifySignInAcceptsTheOfficialPasswordCheck(t *testing.T) {
	const login = "learner"
	const password = "correct-password"
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/signin" || r.Method != http.MethodPost {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("eyJhbGciOiJub25lIn0.eyJzdWIiOiIxIn0.sig"))
	}))
	defer srv.Close()

	token, err := VerifySignIn(context.Background(), srv.Client(), srv.URL, login, password)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "eyJ") {
		t.Fatal("missing session token")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sawAuth, "Basic "))
	if err != nil || string(raw) != login+":"+password {
		t.Fatalf("authorization was not the login and password")
	}
}

func TestVerifySignInRejectsAWrongPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"User does not exist or password incorrect"}`))
	}))
	defer srv.Close()

	_, err := VerifySignIn(context.Background(), srv.Client(), srv.URL, "learner", "wrong")
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("got %v", err)
	}
}

func TestVerifySignInRejectsAnEmptyPasswordWithoutCalling(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	_, err := VerifySignIn(context.Background(), srv.Client(), srv.URL, "learner", "")
	if !errors.Is(err, ErrRejected) || called {
		t.Fatalf("err %v called %v", err, called)
	}
}
