package zeroone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRejectsUnsafeURLs(t *testing.T) {
	for _, endpoint := range []string{"", "file:///tmp/data", "http://example.org/graphql"} {
		if _, err := NewClient(endpoint, "token", nil); err == nil {
			t.Errorf("accepted unsafe endpoint %q", endpoint)
		}
	}
}
func TestClientHandlesAuthorizationPaginationAndError(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("incorrect request")
		}
		if calls == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":{"event_user":[{"id":2,"userId":3,"userLogin":"example","createdAt":"2026-10-01T00:00:00Z","level":4}]}}`))
			return
		}
		w.Write([]byte(`{"errors":[{"message":"private source detail"}]}`))
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.EventUsers(context.Background(), 97, 100, 0)
	if err != nil || len(got) != 1 || got[0].Level != 4 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	_, err = c.EventUsers(context.Background(), 97, 100, 0)
	if err == nil || strings.Contains(err.Error(), "private source detail") {
		t.Fatalf("error detail leaked: %v", err)
	}
}
func TestResponseLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("a", maxResponseBytes+1))) }))
	defer srv.Close()
	c, _ := NewClient(srv.URL, "secret", nil)
	var out any
	if err := c.Query(context.Background(), "query { x }", nil, &out); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}
