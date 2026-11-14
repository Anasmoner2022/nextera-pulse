# Nextera Pulse — Backend & Pace Engine

Operational Student Success and Learning Intelligence system for NextEra / 01Edu.

This repository provides the core backend API, PostgreSQL data models, archive ingestion pipeline, and deterministic Pace Engine for Sprint 1 (Pilot: **Event 97 / Cohort 4: 01 Coding - Second Cohort**).

---

## 1. Quick Start

### Prerequisites
* Go 1.22+
* PostgreSQL 16 (or Docker)

### Environment Configuration
Copy `.env.example` to `.env`:
```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/nextera_pulse?sslmode=disable
PORT=8080
APP_ENV=development
PILOT_COHORT_ID=4
ZEROONE_EVENT_ID=97
```

### Ingest Data & Generate Pace Snapshots
Run the seeder to migrate tables, seed policies & Event 97 curriculum timeline, ingest students, and compute initial pace snapshots:
```bash
go run ./cmd/seed ../NextEra-All-Cohorts-2026-10-19.zip
```

### Start the REST API Server
```bash
go run ./cmd/api
```
The API starts on `http://localhost:8080` with CORS enabled for frontend clients (`localhost:5173`).

---

## 2. Docker Deployment

To spin up both PostgreSQL and the API server in Docker:
```bash
docker compose up -d
```

---

## 3. Sprint 1 Deliverables (Sohila's Scope)

| Ticket | Description | Implementation Status |
| :--- | :--- | :---: |
| **NP-002** | PostgreSQL schema + migrations | `migrations/postgres.sql` |
| **NP-004** | Event 97 timeline & milestones | `internal/store/migrations.go` |
| **NP-005** | Second Cohort sync (from archive) | `internal/sync/loader.go` |
| **NP-006** | Status-policy seed (pause/exclusion) | `internal/store/migrations.go` |
| **NP-007** | Active program age calculation | `internal/pace/engine.go` |
| **NP-008** | Pace engine & deterministic rules | `internal/pace/engine.go` |
| **NP-009** | Daily metric snapshots | `internal/store/student_store.go` |
| **NP-010** | Student pace REST API | `internal/httpapi/server.go` |

---

## 4. REST API Surface

### 1. Health & Readiness Probes
* `GET /health/live` — Returns process status (`{"status": "UP"}`).
* `GET /health/ready` — Verifies database connection (`{"status": "READY"}`).

### 2. Students Pace View
`GET /api/v1/students`
* **Query Parameters**:
  * `cohortId` (optional, default `4`): `1` (First Cohort) or `4` (Second Cohort).
  * `status` (optional): Filter by `RED`, `YELLOW`, `GREEN`, `PAUSED`, `ONBOARDING`.
  * `search` (optional): Case-insensitive search on login, name, or email.

**Sample Response**:
```json
{
  "data": [
    {
      "id": 93,
      "sourceUserId": 622,
      "login": "ahali",
      "firstName": "Ahmed",
      "lastName": "Ali",
      "email": "a.soliman1989@yahoo.com",
      "auditRatio": 0,
      "cohortId": 4,
      "cohortName": "01 Coding - Second Cohort",
      "mentor": {
        "id": 1,
        "name": "Anas (Academic Lead)"
      },
      "pace": {
        "enrolledAt": "2026-04-04T15:33:18+02:00",
        "activeProgramDays": 200,
        "curriculumMonth": 7,
        "actualLevel": 0,
        "minimumLevel": 17,
        "expectedLevel": 20,
        "checkpointTarget": 20,
        "gapToMinimum": -17,
        "gapToExpected": -20,
        "status": "RED",
        "reasons": ["PACE_BELOW_MINIMUM"]
      },
      "lastCalculatedAt": "2026-10-21T20:33:49+03:00"
    }
  ],
  "meta": {
    "cohortId": 4,
    "cohortName": "01 Coding - Second Cohort",
    "totalCount": 52,
    "lastSuccessfulSyncAt": "2026-10-19T11:11:08Z"
  }
}
```

### 3. Cohorts & North Star KPI
* `GET /api/v1/cohorts` — List tracked cohorts.
* `GET /api/v1/cohorts/:id/health` — Returns North Star on-track rates and status counts:
```json
{
  "data": {
    "cohortId": 4,
    "cohortName": "01 Coding - Second Cohort",
    "totalStudents": 52,
    "activeStudents": 51,
    "pausedStudents": 1,
    "onTrackCount": 0,
    "nearTrackCount": 0,
    "offTrackCount": 40,
    "onTrackRate": 0,
    "expectedLevelRate": 0,
    "statusCounts": {
      "ONBOARDING": 11,
      "PAUSED": 1,
      "RED": 40
    }
  }
}
```

### 4. Mentor Interventions
* `GET /api/v1/interventions` — List recorded interventions.
* `POST /api/v1/interventions` — Record an intervention:
```json
{
  "studentId": 93,
  "mentorId": 1,
  "reasonCode": "PACE_BELOW_MINIMUM",
  "type": "CHECK_IN",
  "action": "Conduct 1-on-1 session to diagnose curriculum blockage and set 7-day milestone.",
  "dueAt": "2026-10-28T18:00:00Z"
}
```

---

## 5. Verification & Tests

### Run Pace Engine Unit Tests
```bash
go test ./internal/pace/... -v
```

### Run End-to-End API Integration Suite
```bash
python test_api.py
```


## 2026-11-11 reviewer handover
Read `HANDOVER_2026-11-11.txt` and `TESTING_NOTES.md` before running live integration.
