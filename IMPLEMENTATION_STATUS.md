# Nextera Pulse implementation handoff (2026-10-27)

## Implemented in this package
- Retains existing archive loader, PostgreSQL/SQLite storage, REST API, and data model.
- Pace engine patch: merge overlapping pause intervals; future intervals don't pause students; missing timeline returns NOT_MEASURABLE beyond onboarding. New regression tests included.
- Read-only `internal/zeroone` GraphQL HTTP client with bounded response, request timeouts, HTTPS policy, no raw error/data logging, typed event enrollment pages, pagination, and HTTP mock tests.
- `cmd/worker` **connectivity probe only**: retrieves event-user enrollment pages and logs only row count. Does NOT persist fetched records, does NOT filter Cohort-4 label, and is NOT a scheduled sync worker yet.
- Removed database DSN (which may contain credentials) from API startup logs; migration errors now prevent startup.

## NOT completed / no claims of readiness
- Full 01Edu GraphQL live schema validation, label_user mapping, incremental progress, transaction and administrative record sync, database transactions/upsert wiring, scheduling/reconciliation.
- OAuth/OIDC, session management, role-based authorization. Do not deploy API publicly before these are implemented; current REST endpoints may expose student data and allow writes.
- Existing seeded timeline must be validated from the actual official 01Edu configuration rather than treated as an automatically refreshed source.
- CI/CD and complete database integration tests.

## Testing and environment
- Requested Go version from go.mod: 1.26.2. This environment has Go 1.23.2, and `go.dev` DNS resolution failed on Oct 27. Full `go test ./...` was therefore **not run successfully** here.
- The standalone standard-library-only GraphQL client tests can be run in a temporary GOPATH-mode location under Go 1.23; this does not verify integration with the whole repository.
- Never copy `.env`, database files, ZIP exports, or student information into source control or release archives.

## Run once correct Go is available
```sh
go version
go test ./...
go run ./cmd/worker # requires ZEROONE_GRAPHQL_URL and ZEROONE_ACCESS_TOKEN
```
