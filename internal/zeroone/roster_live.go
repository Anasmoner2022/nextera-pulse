package zeroone

import (
	"context"
	"errors"
)

// The roster queries replace the old zip export. Field names follow the
// columns of that export (csv/label.csv, csv/label_user.csv, csv/user.csv),
// which was itself pulled from the 01Edu GraphQL console. They still need to be
// confirmed once against the live schema with a staff token; a mismatch shows
// up as a roster stage error in /api/v1/sync/status, not as a crash.

// LiveLabel is a 01Edu label such as "01 Coding - Second Cohort".
type LiveLabel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	EventID     *int64 `json:"eventId"`
	Description string `json:"description"`
}

// LiveLabelUser links a 01Edu user to a label (cohort membership).
type LiveLabelUser struct {
	ID      int64 `json:"id"`
	LabelID int64 `json:"labelId"`
	UserID  int64 `json:"userId"`
}

// LiveUser is the profile Pulse keeps for a learner or staff member.
type LiveUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

// LiveUserRole is the 01Edu user.role ("admin", "admin campus", "user", ...).
type LiveUserRole struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Role  string `json:"role"`
}

const rosterPageSize = 200

// CohortLabels returns the labels with these exact names.
func (c *Client) CohortLabels(ctx context.Context, names []string) ([]LiveLabel, error) {
	if len(names) == 0 {
		return nil, errors.New("no cohort labels configured")
	}
	const query = `query PulseCohortLabels($names: [String!]!) {
 label(where: {name: {_in: $names}}, order_by: {id: asc}) { id name eventId description }
}`
	var result struct {
		Label []LiveLabel `json:"label"`
	}
	if err := c.Query(ctx, query, map[string]any{"names": names}, &result); err != nil {
		return nil, err
	}
	return result.Label, nil
}

// LabelUsers returns every membership of these labels.
func (c *Client) LabelUsers(ctx context.Context, labelIDs []int64) ([]LiveLabelUser, error) {
	if len(labelIDs) == 0 {
		return nil, errors.New("no cohort labels to read")
	}
	const query = `query PulseLabelUsers($labelIds: [Int!]!, $limit: Int!, $offset: Int!) {
 label_user(where: {labelId: {_in: $labelIds}}, order_by: {id: asc}, limit: $limit, offset: $offset) { id labelId userId }
}`
	var all []LiveLabelUser
	for offset := 0; offset < 50000; offset += rosterPageSize {
		var result struct {
			LabelUser []LiveLabelUser `json:"label_user"`
		}
		vars := map[string]any{"labelIds": labelIDs, "limit": rosterPageSize, "offset": offset}
		if err := c.Query(ctx, query, vars, &result); err != nil {
			return nil, err
		}
		all = append(all, result.LabelUser...)
		if len(result.LabelUser) < rosterPageSize {
			break
		}
	}
	return all, nil
}

// UsersByID returns the profiles of these 01Edu users.
func (c *Client) UsersByID(ctx context.Context, ids []int64) ([]LiveUser, error) {
	const query = `query PulseUsers($ids: [Int!]!) {
 user(where: {id: {_in: $ids}}, order_by: {id: asc}) { id login firstName lastName email }
}`
	var all []LiveUser
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		var result struct {
			User []LiveUser `json:"user"`
		}
		if err := c.Query(ctx, query, map[string]any{"ids": ids[start:end]}, &result); err != nil {
			return nil, err
		}
		all = append(all, result.User...)
	}
	return all, nil
}

// StaffRoles returns users whose 01Edu role is one of roles (staff accounts).
func (c *Client) StaffRoles(ctx context.Context, roles []string) ([]LiveUserRole, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	const query = `query PulseStaff($roles: [String!]!) {
 user(where: {role: {_in: $roles}}, order_by: {id: asc}) { id login role }
}`
	var result struct {
		User []LiveUserRole `json:"user"`
	}
	if err := c.Query(ctx, query, map[string]any{"roles": roles}, &result); err != nil {
		return nil, err
	}
	return result.User, nil
}

// EventProjectProgress reads project progress for these logins in one event.
// It is used when Pulse has no project catalog yet (fresh database): the
// object relation fills learning_objects as rows arrive.
func (c *Client) EventProjectProgress(ctx context.Context, eventID int, logins []string) ([]LiveProgress, error) {
	if eventID <= 0 || len(logins) == 0 {
		return nil, errors.New("no 01Edu users to refresh")
	}
	const query = `query PulseEventProgress($eventId: Int!, $logins: [String!]!, $limit: Int!, $offset: Int!) {
 progress(where: {eventId: {_eq: $eventId}, user: {login: {_in: $logins}}, object: {type: {_eq: "project"}}}, order_by: {id: asc}, limit: $limit, offset: $offset) {
 id userId objectId path isDone grade
 object { id name type }
 user { login }
 }
}`
	var all []LiveProgress
	for start := 0; start < len(logins); start += 40 {
		end := min(start+40, len(logins))
		for offset := 0; offset < 10000; offset += rosterPageSize {
			var result struct {
				Progress []LiveProgress `json:"progress"`
			}
			vars := map[string]any{"eventId": eventID, "logins": logins[start:end], "limit": rosterPageSize, "offset": offset}
			if err := c.Query(ctx, query, vars, &result); err != nil {
				return nil, err
			}
			for i := range result.Progress {
				if obj := result.Progress[i].Object; obj != nil && obj.Path == "" {
					obj.Path = result.Progress[i].Path
				}
			}
			all = append(all, result.Progress...)
			if len(result.Progress) < rosterPageSize {
				break
			}
		}
	}
	return all, nil
}
