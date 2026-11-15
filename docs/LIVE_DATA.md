# Live 01Edu data

01Edu (`https://01.nexteraeducation.net`) is Pulse's **only** data source. The
Go API reads it **on the server** on a schedule and stores the result in
PostgreSQL. There is no file import: the old `NextEra-All-Cohorts-2026-10-19.zip`
loader (`cmd/seed`, `internal/sync/loader.go`, `internal/sync/ingest_archive.go`)
was removed on 2026-11-13. The browser only talks to the Pulse API; 01Edu
credentials never reach the web app.

## What one sync cycle does

| Stage | 01Edu query | Pulse table |
|---|---|---|
| Enrollments | `event_user(where:{eventId})` → `id userId userLogin createdAt level` | (input to roster and levels) |
| Roster | `label(where:{name:{_in}})` → `id name eventId description`; `label_user(where:{labelId:{_in}})` → `id labelId userId`; `user(where:{id:{_in}})` → `id login firstName lastName email` | `students`, `student_events` (cohort, enrollment date = `event_user.createdAt`, level), `cohort_labels`, `cohort_label_users` |
| Staff | `user(where:{role:{_in:["admin","admin campus"]}})` → `id login role` | `students.source_role` (`admin` → manager, `admin campus` → mentor) |
| Levels | `event_user` (above) | `student_events.current_level` |
| Status records | `record(where:{userId:{_in}})` → `typeName startAt endAt message` | `student_source_records` |
| Project progress | `progress(where:{user:{login:{_in}}, path:{_in}})`, or on a fresh database `progress(where:{eventId, user:{login:{_in}}, object:{type:"project"}})` with `object { id name type }` | `learning_objects`, `learning_progress` |
| Pace | (computed) | `student_metric_snapshots`, rebuilt after every cycle |

Rules:

* Cohort membership comes **only** from the configured 01Edu labels
  (`PILOT_COHORTS`), never from the event-wide `event_user` feed. A user in
  two labels joins the first cohort listed.
* A labelled user who is not enrolled in the event is skipped (no clock anchor).
* Nothing is deleted. A learner who leaves a label keeps their history.
* Staff roles are revoked (set back to `user`) only when the staff query
  succeeds and returns at least one admin, so a failed or empty response never
  removes everyone's access.
* Each stage is independent: if 01Edu rejects one query (for example a field
  name), the others still apply and the error shows in `/api/v1/sync/status`.

### Mentor assignments

01Edu has no student-to-mentor link, and the sync never assigns mentors.
A student with no mentor stays unassigned until a program manager assigns
them (`POST /api/v1/mentor-assignments`). Assignments keep their history: a
move or unassign (`DELETE /api/v1/mentor-assignments/{studentId}`) closes the
current row (`ends_at`, `ended_by`, `end_reason`) and never deletes it, and a
partial unique index allows at most one current mentor per student. The
history is at `GET /api/v1/students/{id}/mentor-assignments`.

## Settings

| Env var | Meaning |
|---|---|
| `ZEROONE_ACCESS_TOKEN` | **Preferred.** A Gitea application token created by a campus admin at `/user/settings/applications`. Exchanged at `GET /api/auth/token?token=…` for a Hasura JWT and renewed 5 minutes before expiry. A raw JWT (`eyJ…`) is also accepted but cannot be renewed. |
| `ZEROONE_LOGIN` + `ZEROONE_PASSWORD` | Service account, used only if no access token is set. Must be a staff account; a student account only syncs its own rows (reported as `LIMITED`). |
| `ZEROONE_HASURA_ROLE` | Optional role header. Sent only if the JWT lists it; otherwise the first of `admin_read_only`, `campus_admin_read_only`, `campus-admin`, `admin` in the JWT is used. |
| `PILOT_COHORTS` | Pulse cohort id = exact 01Edu label name, comma-separated, in priority order. Default `4=01 Coding - Second Cohort,1=01 Coding - First Cohort`. |
| `ZEROONE_EVENT_ID` | Curriculum event, default `97` (`pulsesync.DefaultEventID`, the only place it is written). The API, snapshots, monthly targets and the web app (via `GET /api/v1/program`) all use it. |
| `SYNC_INTERVAL` | Go duration between cycles, default `30m`, minimum `1m`. |
| `LIVE_SYNC=off` | Keep the credential but pause the scheduler. |
| `SNAPSHOT_INTERVAL` | Go duration between pace snapshot refreshes, default `24h`, minimum `1m`; `off` disables the job. Runs with or without a 01Edu credential. |
| `ZEROONE_BASE_URL` / `ZEROONE_GRAPHQL_URL` | Defaults `https://01.nexteraeducation.net` and `<base>/api/graphql-engine/v1/graphql`. |

Put them in the repo-root `.env` (gitignored) or the API host's environment.
**Never** in `web/.env`, git, logs, or chat.

## Run it (PostgreSQL only)

```sh
cp .env.example .env              # then fill ZEROONE_ACCESS_TOKEN
make dev-up                       # postgres:17 on 127.0.0.1:5432
go run ./cmd/api                  # migrates, refreshes snapshots and syncs at startup, then on SNAPSHOT_INTERVAL / SYNC_INTERVAL
cd web && npm install && npm run dev   # http://127.0.0.1:4181
```

The first cycle builds the whole roster from 01Edu, so a fresh database is
filled within one cycle. One-off or cron refresh without the API:
`go run ./cmd/worker` (prints a report) or `go run ./cmd/worker -watch`.

## Daily pace snapshots (no 01Edu calls)

The dashboard reads `student_metric_snapshots`. Without a refresh, a student's
curriculum month, active days and pace status would freeze whenever no sync
happens (no credential, 01Edu down, `LIVE_SYNC=off`). So the API runs a
**snapshot job** at startup and then every `SNAPSHOT_INTERVAL` (default 24h),
whether or not a credential is set. It:

- recomputes active program days, curriculum month, the monthly targets
  (from the event's active timeline version), the gaps and the pace status
  from data already in PostgreSQL (`student_events`, `student_source_records`,
  `status_policy`, `curriculum_timeline`);
- upserts one snapshot per student per event per day (`ON CONFLICT
  (student_id, event_id, snapshot_date) DO UPDATE`), so reruns on the same day
  never add rows;
- never calls 01Edu.

KPIs (`GET /api/v1/kpi/on-track`, cohort health) are computed from the latest
snapshots when read, so they follow automatically.

The live sync cycle rebuilds snapshots after a sync with the **same function**
(`sync.RefreshSnapshots`, via `RefreshSnapshotsLocked`). The job's state is in
`GET /api/v1/sync/status` under `snapshots` (`lastSuccessAt`, `nextRunAt`,
`lastError`, `lastSkipped`).

**cmd/worker does not schedule snapshots.** The API owns the schedule, so
there is one schedule per deployment. `go run ./cmd/worker -snapshots` runs a
single refresh and exits; it needs no credential, which suits a cron job
on a host without the API. The worker's sync cycle rebuilds snapshots
through the same function.

## One sync and one snapshot refresh at a time (advisory locks)

Several API replicas, or the API plus `cmd/worker`, can share one database.
Each job holds a PostgreSQL session-level advisory lock on a dedicated
connection while it runs (`store.WithAdvisoryLock`):

| Job | Lock key (`store`) | If held |
|---|---|---|
| Live 01Edu sync cycle | `LockLiveSync` | The cycle is skipped. The log and `GET /api/v1/sync/status` show `lastSkipped: "skipped: another instance is syncing"` and `lastSkippedAt`. |
| Snapshot job / `worker -snapshots` | `LockSnapshots` | The run is skipped (`snapshots.lastSkipped`). |
| Snapshot rebuild inside a live cycle | `LockSnapshots` | It waits for the running refresh to finish (bounded by the cycle's 10-minute timeout), so fresh data is never dropped. |

The lock is released:

- when the job returns, fails or panics;
- when its context is cancelled. The unlock uses a fresh context, and a
  panic in a cycle is turned into a sync error.

If the unlock itself fails, the connection is discarded rather than returned
to the pool, which ends the session and so releases the lock in PostgreSQL.
The API stops its jobs on SIGINT or SIGTERM.

## Check it

`/api/v1/sync/status` needs a signed-in session (see `docs/AUTH.md`). In the
browser the dashboard header badge shows **Live 01Edu · N min ago**,
**Syncing 01Edu…**, **Live sync issue**, **Offline · last 01Edu sync**, or
**Demo**.

* `mode: "offline"` – no credential configured on this server; the data is
  what the last live sync stored (empty on a fresh database).
* `mode: "live"`, `lastReport.status: "SUCCESS"` – cohort-wide data applied.
* `"LIMITED"` – the credential only returned its owner's rows (not a staff token).
* `"FAILED"` / `lastError` – read the message, e.g. a rejected token or a
  GraphQL field error from one stage.

## Not verified without a staff credential

* The `/api/graphql-engine/v1/graphql` endpoint answers anonymously with
  `no_queries_available`, so the schema could not be introspected. Field names
  come from `queries.graphql` and the column names of the old GraphQL-console
  export. The most likely to need adjusting: `record.typeName`,
  `event_user.level`/`userLogin`, `label.eventId`, `label_user.labelId`,
  `user.firstName`/`lastName`/`email`, `user.role`, and the
  `progress.object` relation filter.
* Whether a Gitea token from this school yields `admin_read_only` /
  `campus-admin` in `x-hasura-allowed-roles`. If it does not, the sync runs as
  `LIMITED`; ask the platform admins for a read-only staff token.
* The exact 01Edu label names for the cohorts (`PILOT_COHORTS`).
