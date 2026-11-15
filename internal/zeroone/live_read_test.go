//go:build integration

package zeroone

import (
	"context"
	"os"
	"testing"
	"time"
)

// Read-only live smoke test; use only approved read-only service credentials.
func TestLiveEventUsers(t *testing.T) {
	endpoint := os.Getenv("ZEROONE_GRAPHQL_URL")
	token := os.Getenv("ZEROONE_ACCESS_TOKEN")
	if endpoint == "" || token == "" {
		t.Skip("approved live 01Edu credentials not configured")
	}
	client, err := NewClient(endpoint, token, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users, err := client.EventUsers(ctx, 97, 5, 0)
	if err != nil {
		t.Fatalf("live GraphQL read failed (do not print credentials): %v", err)
	}
	if len(users) == 0 {
		t.Fatal("live GraphQL returned no enrollments")
	}
	t.Logf("live read succeeded: %d enrollment records", len(users))
}
