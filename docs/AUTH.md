# Sign-in, sessions and access

People sign in to Pulse with their **01Edu login (or email) and password**.
Pulse never stores the password.

1. `POST /api/v1/session` `{login, password}` relays the credentials to
   `POST https://01.nexteraeducation.net/api/auth/signin` (Basic auth).
2. If 01Edu accepts them, Pulse reads the **user id from the token 01Edu
   returned** (`x-hasura-user-id`) and looks that person up by
   `students.source_user_id`. The login text the browser sent is not trusted
   for identity. (Only if the 01Edu token has no user id does Pulse fall back
   to the login it accepted.) The 01Edu token is then discarded: it is never
   used to read 01Edu data.
3. Pulse creates a session row in PostgreSQL (`app_sessions`: SHA-256 of a
   random 256-bit token, person, expiry) and sets the cookie
   `pulse_session` (`HttpOnly`, `SameSite=Lax`, `Secure` unless
   `SESSION_COOKIE_SECURE=false`, `Path=/`, lifetime `SESSION_TTL`, default 12h).
4. `GET /api/v1/session` returns the signed-in person;
   `DELETE /api/v1/session` deletes the session and clears the cookie.

Every `/api/v1` route except `POST /api/v1/session` requires a valid session
(`401 UNAUTHENTICATED` otherwise). `/health/live` and `/health/ready` are public.

## Roles and scopes (enforced on the server)

The role comes from the person's 01Edu `user.role`, refreshed by every live
sync: `admin` → **manager**, `admin campus` → **mentor**, anything else →
**student**. The browser's copy of the role is for display only.

| Route | Manager | Mentor | Student |
|---|---|---|---|
| `GET /students`, `GET /cohorts/{id}/students`, `GET /cohorts/{id}/progress` | everyone | students whose current primary mentor they are | only themselves |
| `GET /students/{id}`, `GET /students/{id}/progress` | any | assigned only, else 403 | self only, else 403 |
| `GET /interventions` | all | about their students, or written by them | about themselves |
| `POST /interventions` | any existing student | assigned students only | 403 |
| `DELETE /interventions/{id}` | any note | only notes they wrote, else 403 | 403 |
| `POST /mentor-assignments` | yes | 403 | 403 |
| `DELETE /mentor-assignments/{studentId}` (unassign) | yes | 403 | 403 |
| `GET /students/{id}/mentor-assignments` (history) | any | assigned only, else 403 | 403 |
| `GET /staff` | yes | yes | 403 |
| `GET /cohorts/{id}/health` | yes | own students only | 403 |
| `GET /kpi/on-track` | program, cohorts, every mentor | own value only | 403 |
| `GET /sync/status` | full | full | freshness fields only |
| `GET /cohorts`, `GET /program`, `GET /session`, `DELETE /session` | yes | yes | yes |

Follow-up authorship comes from the session: `interventions.mentor_id` is the
signed-in author and `author_role` their role; a `mentorId` in the request
body is ignored. `GET /interventions` returns `authorId`, `authorName`,
`authorRole`, and `canDelete` (true for the author and for managers).

Deleting a follow-up is a soft delete: `deleted_at` and `deleted_by` are set
and the note disappears from every list, but the row is kept for the record.
A missing or already deleted note returns 404. A mentor with no assigned
students cannot write follow-ups (the web form says so); their scope is not
widened.

## Sign-in rate limit

In memory, per API process: at most **20 sign-in attempts per IP** and **5
failed attempts per login** in 15 minutes; then `429 TOO_MANY_ATTEMPTS` with
`Retry-After`, and the attempt never reaches 01Edu. A successful sign-in
clears the login's counter. Behind a reverse proxy set
`TRUST_PROXY_HEADERS=true` so the client IP comes from `X-Forwarded-For`.

## Local development

The Vite dev server proxies `/api` to the API, so the cookie is same-origin.
Over plain `http://` set `SESSION_COOKIE_SECURE=false` in `.env` (browsers
generally accept Secure cookies on `localhost`, but not on other HTTP hosts).
Signing in needs a real 01Edu account that the live sync has put in the
database. For UI work without the backend use `VITE_STUDENTS_SOURCE=mock`
(demo data and demo workspaces only).
