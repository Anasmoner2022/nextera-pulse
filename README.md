# Nextera Pulse

Nextera Pulse is a Student Success and Learning Intelligence platform for X Academy.

The first release focuses on one question:

> For every active student in the pilot cohort, are they on pace according to the official 01Edu curriculum timeline?

## Sprint 1

**Duration:** 2 weeks  
**Kickoff:** Sunday at 8:00 PM  
**Pilot cohort:** `01 Coding - Second Cohort`  
**01Edu event:** `97`

Sprint 1 should reliably provide:

- enrollment date
- active academic days
- curriculum month
- current level
- minimum level
- expected level
- checkpoint target
- gap to minimum
- gap to expected
- pace status: `GREEN`, `YELLOW`, `RED`, `PAUSED`, `INACTIVE`, `NOT_MEASURABLE`

## Team

| Person | Primary ownership | Secondary |
|---|---|---|
| Anas | Product Owner / PM | QA acceptance, business rules |
| Shahd | Tech Lead / Architecture | Backend support, code review |
| Sohila | Backend Engineer | Data and 01Edu integration |
| Zeyad | Frontend Engineer | Optional backend support |
| Intern 1 | TBD | Scoped tasks only |
| Intern 2 | TBD | Scoped tasks only |

## Planned stack

- **Frontend:** React + Vite + TypeScript
- **Backend:** Go
- **Database:** PostgreSQL
- **Source integration:** 01Edu / Hasura GraphQL
- **Auth:** Google OAuth / OIDC + Nextera Pulse RBAC
- **Local development:** Docker Compose

## High-level architecture

```text
01Edu / Hasura
      |
      v
 Sync Worker
      |
      v
 PostgreSQL
  |        |
  v        v
Pace     Snapshots
Engine
   \       /
     Go API
        |
        v
React + Vite UI
```

The frontend must never query 01Edu directly. All 01Edu credentials remain server-side.

## Locked product rules

### Academic clock

Clock anchor:

```text
event_user.createdAt
```

Statuses that pause the clock:

- `restricted`
- `blocked-long`
- `ban`
- `left-school` is terminal/inactive rather than a temporary pause

Statuses that do **not** pause the clock:

- `away`
- `temporary-notice`
- `observation`

### On-track rate (North Star KPI)

```text
on-track rate = GREEN students / measurable students
```

Measurable students have a pace status of GREEN, YELLOW or RED, and a source
status that `status_policy.exclude_from_active_kpis` does not flag. The flag is
read from the table, not from code; by default it excludes:

- `restricted`
- `blocked-long`
- `ban`
- `left-school`

`away`, `temporary-notice` and `observation` stay in the population.

- YELLOW ("Watch") is measurable but not on track, and it is reported
  separately.
- With no measurable students the rate is null and the dashboard shows "—",
  not 0%.
- The server computes the rate per cohort, per mentor and program-wide
  (`GET /api/v1/kpi/on-track`); see `docs/ON_TRACK_RATE.md`.

### Mentor assignment

- one mentor can own many students
- a student can have at most one active primary mentor
- mentor assignment history must be preserved

## Repository direction

```text
cmd/
  api/
  worker/

internal/
  app/
  auth/
  zeroone/
  sync/
  student/
  cohort/
  curriculum/
  pace/
  health/
  mentor/
  intervention/
  snapshot/
  store/
  httpapi/
  clock/

migrations/
web/
docs/
```

Do not create catch-all `services` or `utils` packages for unrelated business logic.

## Development

Local dependencies: Go 1.26.2+ (go.mod), Node.js 22 LTS (`.nvmrc`; CI uses the same), and PostgreSQL 17 (Docker
Compose is the easiest way to get it). Pulse runs on **PostgreSQL only**;
`DATABASE_URL` must be a `postgres://` URL and the API refuses to start
otherwise. The schema is embedded in the binaries and applied on start.

```sh
cp .env.example .env     # never commit .env
make dev-up              # postgres:17 on 127.0.0.1:5432 (+ nextera_pulse_test DB)
make test                # go test ./... against TEST_DATABASE_URL
```

Never commit real secrets. Start from `.env.example`.

### Live 01Edu data

01Edu is the only data source; there is no zip/file import. On a schedule the
API reads the cohort roster (labels), staff roles, levels, status records,
project progress, and rebuilds pace snapshots from
`https://01.nexteraeducation.net`, using a server-side credential
(`ZEROONE_ACCESS_TOKEN`, a Gitea application token from a campus admin, or
`ZEROONE_LOGIN`/`ZEROONE_PASSWORD`). Without one the API serves whatever the
last sync stored, and the web app can still run on synthetic data with
`VITE_STUDENTS_SOURCE=mock`.

With or without a credential, the API also refreshes the daily pace snapshots
from stored data at startup and every `SNAPSHOT_INTERVAL` (default `24h`), so
curriculum month, pace status and KPIs keep moving with the calendar. This
job never calls 01Edu. PostgreSQL advisory locks keep a single live sync
and a single snapshot refresh running at a time across instances; see
docs/LIVE_DATA.md.

```sh
cp .env.example .env                 # fill ZEROONE_ACCESS_TOKEN
make dev-up                          # PostgreSQL 17
go run ./cmd/api                     # uses DATABASE_URL from .env; first sync fills the roster
curl -s localhost:8080/api/v1/sync/status
cd web && npm install && npm run dev
```

See [docs/LIVE_DATA.md](docs/LIVE_DATA.md) for the endpoints, env vars, and
what still needs checking against the live schema.

### Sign-in

Everyone signs in with their 01Edu login and password; the API checks them
with 01Edu, then sets its own HttpOnly session cookie (stored in PostgreSQL).
Every `/api/v1` route except sign-in needs that cookie, and the server limits
what each role sees (manager: everyone, mentor: assigned students, student:
themselves). For local plain-HTTP development set `SESSION_COOKIE_SECURE=false`
in `.env`. See [docs/AUTH.md](docs/AUTH.md).

## Important engineering rules

1. `main` is protected.
2. Work happens in short-lived branches.
3. Changes reach `main` through pull requests.
4. Core backend, schema, auth, sync, and pace changes require architecture review.
5. Business-rule changes require Product approval.
6. A pace mismatch against 01Edu blocks Sprint 1 acceptance.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/SPRINT_1.md](docs/SPRINT_1.md).
