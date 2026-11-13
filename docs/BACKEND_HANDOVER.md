# Backend handover: Pulse follows 01Edu

Audience: backend. PostgreSQL is the store. 01Edu is the only data source. The scheduled live sync fills the roster, roles, levels, records, and project progress, then rebuilds pace snapshots. Detail is in [LIVE_DATA.md](LIVE_DATA.md). Sign-in is in [AUTH.md](AUTH.md).

The archive import and the old per-request progress pull are gone. Do not look for a seed command or an archive loader. A fresh database is filled by the first successful live cycle.

## What is running

- API: `go run ./cmd/api`. PostgreSQL via `DATABASE_URL` (local default in `.env.example`). `HTTP_ADDR=:8080`.
- Live sync starts when `ZEROONE_ACCESS_TOKEN` or `ZEROONE_LOGIN` + `ZEROONE_PASSWORD` is set, unless `LIVE_SYNC=off`. Interval `SYNC_INTERVAL` (default 30m). Event `ZEROONE_EVENT_ID` (default 97). Cohorts `PILOT_COHORTS`.
- Web: Vite on `http://127.0.0.1:4181/`, proxying `/api` to the API. The browser never calls 01Edu.
- Pace snapshots: rebuilt at the end of a successful roster, level, or record stage, and again on `SNAPSHOT_INTERVAL` from data already in PostgreSQL.

## Where the work lives

- `cmd/api/main.go` — starts the API, the live sync, and the snapshot job
- `cmd/worker/main.go` — one cycle, or `-watch`, or one snapshot refresh with `-snapshots`
- `internal/app/live.go` — reads the sync settings
- `internal/sync/runner.go` — schedule
- `internal/sync/live_cycle.go` — one cycle: roster, levels, records, progress, then snapshots
- `internal/sync/roster_live.go` — creates students from the configured 01Edu labels
- `internal/sync/snapshots.go` — rebuilds `student_metric_snapshots`
- `internal/zeroone/credentials.go` — Gitea token exchange and role choice
- `internal/zeroone/roster_live.go` — label, user, and staff queries
- `internal/zeroone/records_live.go` — status records
- `internal/zeroone/progress_live.go` — project progress query
- `internal/sync/progress_live.go` — writes progress into `learning_progress`
- `internal/httpapi/auth.go` — 01Edu password check and the Pulse session cookie
- `internal/store/sessions.go` — sessions in PostgreSQL
- `internal/store/assignments.go` — mentor assign, move, and unassign

## Still separate from 01Edu

01Edu has no student-to-mentor link. A program manager assigns mentors with `POST /api/v1/mentor-assignments`. Follow-ups stay in Pulse `interventions`.

Month minimum, expected, and checkpoint stay the timeline stored in PostgreSQL until 01Edu provides one. The active version is in `internal/store/timeline.go`.

## Do not

- Do not put the Gitea token, a JWT, or a password in the web app, logs, or git.
- Do not treat a one-login `user` response as a cohort refresh. That run is reported as `LIMITED`.
- Do not invent cohort membership from the event-wide enrollment list. Membership comes from `PILOT_COHORTS`.
- Do not delete learners who leave a label. The sync keeps their history.
