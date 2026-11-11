# Backend handover: make Pulse follow 01Edu

Audience: backend. This is the current local state of `nextera-pulse` and the next sync work. `LIVE_SYNC_STATUS.md` is older and does not describe the project-progress pull that is now in the API.

The product requirement is that a change on `https://01.nexteraeducation.net` shows up in Pulse without another zip import. That is not true today.

## What is running

- API: `go run ./cmd/api` with `DATABASE_URL=file:dev-pulse.sqlite`, `PORT=8080`, `ZEROONE_BASE_URL=https://01.nexteraeducation.net`.
- Web: Vite on `http://127.0.0.1:4181/`, proxying `/api` to the API. `VITE_STUDENTS_SOURCE=api`. The browser never calls 01Edu.
- Loaded data: `NextEra-All-Cohorts-2026-10-19.zip`, event 97. Cohort 4 is 01 Coding, Second Cohort. The zip stays gitignored.

## What the screen is using

| Shown data | Where it comes from now |
|---|---|
| Login, name, email, enrollment, cohort | Archive roster, imported once |
| Level | Archive roster `level`, copied into `student_metric_snapshots` at import |
| 01Edu status | Archive `json/record.json`. No open record becomes `active` |
| Program manager / mentor / student | Archive `csv/user.csv` `role`: `admin` → manager, `admin campus` → mentor, anything else → student |
| Project names and filled squares | Archive `csv/object.csv` (`type=project`) and `csv/progress.csv` (`is_done`) |
| Pace color, month, minimum, expected, checkpoint, gaps | Calculated by `internal/pace` from the archived level and the month table in `internal/store/migrations.go`. `event.json` has no timeline |
| Mentor on a student | `mentor_assignments` in Pulse. 01Edu has no student-to-mentor link. The program manager changes it with `POST /api/v1/mentor-assignments` |
| Follow-ups | Pulse `interventions`. Not from 01Edu |
| Password check | Live. `POST https://01.nexteraeducation.net/api/auth/signin` |

Also imported and not shown: `result.csv`, `group.csv`, `audit.csv`, `transaction.csv`, `label.csv`, `label_user.csv`. XP, audit ratio, project grade, and group status are in SQLite and are not on the dashboard.

`web/src/data/mockStudents.ts` and the invented project catalog are unused while `VITE_STUDENTS_SOURCE=api`.

## Why it is not staying up to date

01Edu does not push changes. Pulse has one live read, and it cannot see the cohort.

1. The read runs only inside `GET /api/v1/cohorts/97/progress` and `GET /api/v1/students/{id}/progress`, and only while the last sign-in JWT is still in the API process. A restart drops it. It is not stored. A successful cohort pull is throttled to 2 minutes.
2. That read asks only for project `is_done` and `grade` on the 9 project paths. It does not refresh level, records, enrollment, or pace.
3. The password JWT is not allowed to read other students. The last sign-in against the school returned “Your requested role is not in allowed roles” for `campus-admin`, `campus-admin-read-only`, `admin`, `admin-read-only`, and the underscore forms. Role `user` succeeded and returned that account’s own rows. Hasura will not accept an `x-hasura-role` that is absent from the JWT `x-hasura-allowed-roles`.
4. Because that pull was one login, the API does not set `syncedAt`. The board stays labeled as the archive. `internal/httpapi/progress_sync.go` only marks the board live when the response contains more than one login.
5. `ZEROONE_ACCESS_TOKEN` is empty. `cmd/worker` can refresh levels from GraphQL, but it has never been run against this school, and it does not rebuild the snapshots the dashboard reads.

Do not keep retrying role headers that the token does not list. That does not grant access.

## Next work, in order

### 1. Get a school read token

A campus admin or program manager creates a Gitea application token at `https://01.nexteraeducation.net/user/settings/applications`. Put it in `ZEROONE_ACCESS_TOKEN` on the API host. Never commit it, log it, or send it to the browser.

Exchange it once:

```http
GET https://01.nexteraeducation.net/api/auth/token?token=<gitea sha1>
```

Confirm the JWT claim `https://hasura.io/jwt/claims` includes `admin_read_only` or `campus-admin` in `x-hasura-allowed-roles`. The official client’s example allows `user` and `admin_read_only`, with default role `admin_read_only`. If those roles are absent, stop. A password sign-in cannot be upgraded in Pulse.

Set `ZEROONE_GRAPHQL_URL` to `https://01.nexteraeducation.net/api/graphql-engine/v1/graphql`.

### 2. Use that JWT for the cohort progress pull

`internal/httpapi/progress_sync.go` already calls `zeroone.Client.CohortProgress` and `sync.ApplyLiveProgress`. Point that pull at the exchanged admin token instead of the password session.

The query already in `internal/zeroone/progress_live.go` is the one to keep:

```graphql
query PulseProgress($logins: [String!]!, $paths: [String!]!, $limit: Int!, $offset: Int!) {
  progress(where: {user: {login: {_in: $logins}}, path: {_in: $paths}}, order_by: {id: asc}, limit: $limit, offset: $offset) {
    id path isDone grade
    user { login }
  }
}
```

Send `x-hasura-role` only when that role is in the token. Prefer the token’s default role when it is already `admin_read_only`.

Acceptance: after one poll, a student other than the token owner has a `learning_progress.updated_at` newer than 19 October, and `GET /api/v1/cohorts/97/progress` returns `syncedAt`.

### 3. Poll the rest of the visible fields

01Edu will not notify Pulse. Run the same token on a timer (`SYNC_INTERVAL` is already in `.env.example`; nothing schedules it). Each cycle should:

- Progress: the query above, for every loaded login and the project paths. `ApplyLiveProgress` updates `is_done` and `grade` and matches paths by slug because archive object paths (`root/01-edu_module/.../`) and live progress paths (`/cairo/module/...`) differ.
- Level: `event_user` for event 97, through `sync.RefreshPilotLevels`. Update only students already in cohort 4 or cohort 1. Do not invent cohort membership from the event.
- Administrative records: open `record` rows, so source status is not stuck on the archive.
- Pace: rebuild `student_metric_snapshots` after level and record updates. `RefreshPilotLevels` currently updates `student_events.current_level` and leaves the snapshots alone, so the dashboard would not change. The API reads the snapshot.

Month minimum, expected, and checkpoint stay the seeded table until 01Edu actually provides a timeline. `event.json` has `startAt` and `endAt` only.

### 4. Keep the password session separate

`POST /api/v1/session` checks the official password and returns the Pulse identity. It must not become the cohort read credential unless that JWT’s allowed roles include a cohort role. Role on the dashboard stays `students.source_role` from the roster until a live `user.role` query is added.

## Files

- `internal/httpapi/progress_sync.go` — in-memory token, role attempts, live board flag
- `internal/zeroone/progress_live.go` — project progress query
- `internal/zeroone/signin.go` — password check
- `internal/zeroone/access.go` — JWT role and user id, no signature check
- `internal/sync/progress_live.go` — write live progress into `learning_progress`
- `internal/sync/live.go` — level refresh that does not rebuild snapshots
- `cmd/worker/main.go` — one-shot level refresh, not scheduled, not progress
- `internal/store/progress.go` — board: project catalog, `is_done`, mentor name
- `internal/store/assignments.go` — program-manager assignment. Do not call `AssignCampusMentors` on startup

## Do not

- Do not put the Gitea token or a JWT in the web app, logs, or git.
- Do not treat a one-login `user` response as a cohort refresh.
- Do not infer mentors from the archive. Assignments are a Pulse action.
- Do not show XP, grade, or group status until the field mapping already agreed is implemented: XP is `transaction.type = "xp"` and not invalidated; project grade is `progress.grade`, and null is not zero; “still working” is group status `working` only.
