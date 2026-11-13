package zeroone

import (
	"context"
	"errors"
)

// LiveRecord is one 01Edu administrative record (away, restricted, blocked-long, ...).
// Field names follow the GraphQL export already ingested from json/record.json
// and queries.graphql (PulseRecordsForUsers); confirm them against the live schema.
type LiveRecord struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"userId"`
	TypeName  string  `json:"typeName"`
	Message   string  `json:"message"`
	StartAt   string  `json:"startAt"`
	EndAt     *string `json:"endAt"`
	CreatedAt string  `json:"createdAt"`
}

// RecordsForUsers reads administrative records for these 01Edu user ids.
func (c *Client) RecordsForUsers(ctx context.Context, userIDs []int64) ([]LiveRecord, error) {
	if len(userIDs) == 0 {
		return nil, errors.New("no 01Edu users to refresh")
	}
	const pageSize = 200
	const query = `query PulseRecordsForUsers($userIds: [Int!]!, $limit: Int!, $offset: Int!) {
 record(where: {userId: {_in: $userIds}}, order_by: {id: asc}, limit: $limit, offset: $offset) {
 id userId typeName message startAt endAt createdAt
 }
}`
	var all []LiveRecord
	for start := 0; start < len(userIDs); start += 100 {
		end := min(start+100, len(userIDs))
		for offset := 0; offset < 20000; offset += pageSize {
			var result struct {
				Record []LiveRecord `json:"record"`
			}
			vars := map[string]any{"userIds": userIDs[start:end], "limit": pageSize, "offset": offset}
			if err := c.Query(ctx, query, vars, &result); err != nil {
				return nil, err
			}
			all = append(all, result.Record...)
			if len(result.Record) < pageSize {
				break
			}
		}
	}
	return all, nil
}
