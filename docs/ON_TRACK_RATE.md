# On-track rate (North Star KPI)

```text
on-track rate = GREEN students / measurable students
```

- **GREEN** means the student's level is at or above the *expected* level for
  their curriculum month (see `docs/SPRINT_1.md`). YELLOW ("Watch", between the
  minimum and the expected level) is **not** on track; it is reported
  separately.
- **Measurable** students have a pace status that compares them with a
  monthly target (GREEN, YELLOW or RED) **and** a current 01Edu source status
  (open record type) that the `status_policy` table does not flag
  `exclude_from_active_kpis`. The flag is read from the table on every
  request; there is no list in code. Seeded defaults: `restricted`,
  `blocked-long`, `left-school` and `ban` are excluded.
- PAUSED, INACTIVE, NOT_MEASURABLE, ONBOARDING and UNKNOWN have no target
  comparison and are never in the denominator.
- With no measurable students the rate is `null` and `hasMeasurable` is
  `false`. The dashboard shows "—" and "No measurable students", never 0%.

The formula lives in one place on the server, `pace.OnTrack`
(`internal/pace/kpi.go`). `GetCohortHealthSummary` and `OnTrackKPIs` in
`internal/store/student_store.go` both call it.

## API

`GET /api/v1/kpi/on-track` (manager or mentor; students get 403)

- Manager: `program` (every student of the configured event), `cohorts` (one
  per configured cohort) and `mentors` (one per mentor, covering that mentor's
  *current* students only).
- Mentor: only their own entry in `mentors`; `program` is `null` and
  `cohorts` is empty.

Each value:

```json
{
  "scope": "mentor", "eventId": 97, "mentorId": 12, "name": "Mentor A",
  "onTrack": 3, "measurable": 8, "rate": 37.5, "hasMeasurable": true,
  "total": 10, "excludedByPolicy": 1,
  "statusCounts": {"GREEN": 3, "YELLOW": 2, "RED": 3, "PAUSED": 1, "INACTIVE": 1},
  "measurableCounts": {"GREEN": 3, "YELLOW": 2, "RED": 3}
}
```

`rate` is a percentage (0–100). `statusCounts` covers every student in scope,
and `measurableCounts` only the denominator.

`GET /api/v1/cohorts/{id}/health` returns the same figures for a cohort
(`onTrackRate`, which is null when nobody is measurable, plus
`measurableStudents`, `onTrackCount`, `nearTrackCount`, `offTrackCount`,
`statusCounts` and the full `kpi`). For a mentor it covers only their
students in that cohort.

## Web

In live mode the overview, the mentor cards, the mentor workspace and the
stats grid show the server value ("X of N measurable on pace"), with a
loading state and an error state instead of 0. Demo mode runs the same
formula (`web/src/studentMetrics.mjs`) on mock rows and a mock copy of the
policy (`mockStatusPolicy`).

## Change note (review C4, November 2026)

- Before:
  - The handover spec described the KPI as students "at or above the
    minimum level".
  - `GET /cohorts/{id}/health` counted GREEN + YELLOW over all non-paused,
    non-inactive students.
  - The dashboard computed GREEN over a hardcoded exclusion list in the
    browser.
- Now every place uses GREEN / measurable students, and the exclusion comes
  from `status_policy`.
- Removed response fields: `activeStudents`, `pausedStudents` and
  `expectedLevelRate` (the latter is now the same number as `onTrackRate`).
- Added fields: `measurableStudents` and `kpi`.
- `onTrackRate` can now be `null`.
