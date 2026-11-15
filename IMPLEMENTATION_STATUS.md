# Nextera Pulse implementation handoff (2026-10-27)

> **Update (2026-11-13):** the environment notes below are historical.
> - The project now builds and tests with Go 1.26.2+ (go.mod).
> - `go test ./...` runs against PostgreSQL 17 in CI and locally with `PULSE_REQUIRE_PG=1`.
> - The web app uses Node.js 22 LTS (`.nvmrc`).
> - The unused `migrations/001_init.sql` was removed; `migrations/postgres.sql` is the only schema.

## Implemented in this package
- Retains PostgreSQL storage, REST API, and data model. (2026-11-13: the zip archive loader and SQLite support were removed; 01Edu live sync is the only data source.)
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
- (Oct 27, historical) Requested Go version from go.mod: 1.26.2. That environment had Go 1.23.2 and `go.dev` DNS resolution failed, so full `go test ./...` was not run then. It now runs on Go 1.26.2+.
- Never copy `.env`, database files, ZIP exports, or student information into source control or release archives.

## Run once correct Go is available
```sh
go version
go test ./...
go run ./cmd/worker # requires ZEROONE_GRAPHQL_URL and ZEROONE_ACCESS_TOKEN
```
