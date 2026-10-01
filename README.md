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

### Active KPI population

Excluded from active pace KPI denominators:

- `restricted`
- `blocked-long`
- `ban`
- `left-school`

Included:

- `away`
- `temporary-notice`
- `observation`

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

The exact bootstrap commands will be added once the initial Go module and React application are committed.

Expected local dependencies:

- Go
- Node.js
- Docker
- Docker Compose

Never commit real secrets. Start from `.env.example`.

## Important engineering rules

1. `main` is protected.
2. Work happens in short-lived branches.
3. Changes reach `main` through pull requests.
4. Core backend, schema, auth, sync, and pace changes require architecture review.
5. Business-rule changes require Product approval.
6. A pace mismatch against 01Edu blocks Sprint 1 acceptance.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/SPRINT_1.md](docs/SPRINT_1.md).
