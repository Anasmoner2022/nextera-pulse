// Package zeroone provides a read-only, server-side GraphQL client for 01Edu.
package zeroone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 16 << 20

// Client performs authenticated read-only GraphQL calls.
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

func NewClient(endpoint, token string, transport *http.Client) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("invalid 01Edu GraphQL URL")
	}
	if u.Scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return nil, errors.New("01Edu GraphQL requires HTTPS outside localhost")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("missing 01Edu read-only token")
	}
	if transport == nil {
		transport = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{endpoint: u.String(), token: token, httpClient: transport}, nil
}

type graphQLRequest struct {
	Query     string `json:"query"`
	Variables any    `json:"variables,omitempty"`
}
type graphQLError struct {
	Message string `json:"message"`
}
type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

// Query uses POST and returns GraphQL's data field. Callers must not log raw
// responses, which can contain student personal information.
func (c *Client) Query(ctx context.Context, query string, vars any, target any) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("empty query")
	}
	encoded, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("encode graphql request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("graphql request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql upstream HTTP status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxResponseBytes {
		return errors.New("graphql response exceeds size limit")
	}
	var envelope graphQLResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql returned %d error(s)", len(envelope.Errors))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("graphql response missing data")
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode graphql data: %w", err)
	}
	return nil
}

type EventUser struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	UserLogin string    `json:"userLogin"`
	CreatedAt time.Time `json:"createdAt"`
	Level     float64   `json:"level"`
}

// EventUsers retrieves a page of enrollment/level records. The 01Edu Hasura
// field names must be validated against the live schema before production use.
func (c *Client) EventUsers(ctx context.Context, eventID, limit, offset int) ([]EventUser, error) {
	if eventID <= 0 || limit < 1 || limit > 500 || offset < 0 {
		return nil, errors.New("invalid pagination or event ID")
	}
	const q = `query PulseEventUsers($eventId: Int!, $limit: Int!, $offset: Int!) {
 event_user(where: {eventId: {_eq: $eventId}}, order_by: {id: asc}, limit: $limit, offset: $offset) {
 id userId userLogin createdAt level
 }
}`
	var result struct {
		EventUsers []EventUser `json:"event_user"`
	}
	err := c.Query(ctx, q, map[string]int{"eventId": eventID, "limit": limit, "offset": offset}, &result)
	return result.EventUsers, err
}

// AllEventUsers fetches every page, without conflating an empty page with an error.
func (c *Client) AllEventUsers(ctx context.Context, eventID int) ([]EventUser, error) {
	const size = 200
	var all []EventUser
	for offset := 0; ; offset += size {
		page, err := c.EventUsers(ctx, eventID, size, offset)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < size {
			return all, nil
		}
	}
}
