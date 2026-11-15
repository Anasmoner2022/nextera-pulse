package zeroone

import (
	"context"
	"fmt"
)

// LiveProgress is one 01Edu progress row for a module project.
type LiveProgress struct {
	ID       int64       `json:"id"`
	UserID   int64       `json:"userId"`
	ObjectID int64       `json:"objectId"`
	Path     string      `json:"path"`
	IsDone   bool        `json:"isDone"`
	Grade    *float64    `json:"grade"`
	Object   *LiveObject `json:"object"`
	User     *struct {
		Login string `json:"login"`
	} `json:"user"`
}

// Login is the 01Edu username on the progress row.
func (row LiveProgress) Login() string {
	if row.User == nil {
		return ""
	}
	return row.User.Login
}

// LiveObject is the project behind a progress row.
type LiveObject struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// CohortProgress reads current module progress for these 01Edu logins.
// The query matches the official progress shape: user login plus path.
func (c *Client) CohortProgress(ctx context.Context, logins, paths []string) ([]LiveProgress, error) {
	if len(logins) == 0 || len(paths) == 0 {
		return nil, fmt.Errorf("no 01Edu users to refresh")
	}
	var all []LiveProgress
	for start := 0; start < len(logins); start += 40 {
		end := start + 40
		if end > len(logins) {
			end = len(logins)
		}
		page, err := c.progressChunk(ctx, logins[start:end], paths)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
	}
	return all, nil
}

func (c *Client) progressChunk(ctx context.Context, logins, paths []string) ([]LiveProgress, error) {
	const pageSize = 200
	const query = `query PulseProgress($logins: [String!]!, $paths: [String!]!, $limit: Int!, $offset: Int!) {
 progress(where: {user: {login: {_in: $logins}}, path: {_in: $paths}}, order_by: {id: asc}, limit: $limit, offset: $offset) {
 id path isDone grade
 user { login }
 }
}`
	var all []LiveProgress
	for offset := 0; offset < 10000; offset += pageSize {
		var result struct {
			Progress []LiveProgress `json:"progress"`
		}
		err := c.Query(ctx, query, map[string]any{"logins": logins, "paths": paths, "limit": pageSize, "offset": offset}, &result)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Progress...)
		if len(result.Progress) < pageSize {
			return all, nil
		}
	}
	return all, nil
}
