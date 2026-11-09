# Nextera Pulse — Live GraphQL Database Sync implementation (2026-10-27)

## Implemented
- `internal/sync/live.go`: validates a full read-only Event 97 `event_user` response and atomically updates current levels only for existing pilot cohort memberships. Cohort membership is **never inferred** from Event 97 alone. Records `sync_runs` on success.
- `internal/sync/live_test.go`: source-data validation tests (empty/duplicate/missing timestamp/non-finite level).
- `cmd/worker/main.go`: one-shot live refresh using ZEROONE_GRAPHQL_URL, ZEROONE_ACCESS_TOKEN, DATABASE_URL, ZEROONE_EVENT_ID (default 97), PILOT_COHORT_ID (integer, default 4).

## Not yet implemented / not verified
- **NOT a full GraphQL Database Sync**: no live Event 97 curriculum config sync; no live records/progress/transactions, scheduled jobs, or post-sync pace snapshot calculation.
- No live 01Edu connection was tested (endpoint and credentials not available in the environment).
- Project-wide Go tests could not run: local Go 1.23.2 < required 1.26.2 and proxy.golang.org DNS unavailable for dependencies.
- The GraphQL event_user fields must be checked against the live Hasura schema; current client assumes `userLogin` and numeric `level` fields.
- `RefreshPilotLevels` updates levels but existing snapshots remain stale; the API uses snapshots, so this refresh does NOT immediately change mentor dashboard pace values.
- No scheduled worker: run manually only until stale snapshot and full sync behavior are addressed.

## Security and rollout restrictions
- Never commit secrets; rotate credentials if prior .env was exposed.
- Do NOT expose API to users without authentication and RBAC.
- Do NOT run against production without testing on a copy of PostgreSQL with a known valid pilot roster, source schema validation and backup.
- Do not assume unscoped Event 97 students are in Cohort 4.

## Validation commands (with dependencies available)
```
go version
go test ./...
go test -race ./...
```
Check first: `go run ./cmd/worker` will perform a database mutation, not a dry run. Environment must be configured securely.
