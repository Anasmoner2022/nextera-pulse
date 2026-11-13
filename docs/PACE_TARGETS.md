# Monthly pace targets (curriculum timeline)

The pace engine compares a learner's level with the targets for their
curriculum month. Targets are **versioned data** in PostgreSQL:

* `curriculum_timeline_versions`: one row per version: `event_id`, `label`,
  `status` (`draft`, `approved`, `retired`), `notes`, `approved_by`,
  `approved_at`, `created_at`.
* `curriculum_timeline`: the monthly rows of a version (`version_id`,
  `month`, `min_level`, `expected_level`, `checkpoint_level`, `rank`, `notes`).

**Which version is used:** for the configured event (`ZEROONE_EVENT_ID`,
default 97), the newest `approved` version; if there is none, the newest
`draft`. `retired` versions are never used. Every snapshot stores the version
in `calculation_version`, for example `v0.2+timeline:draft-v0(draft)`, and
the API reports it as `paceRule` (`GET /api/v1/program`, the cohort students
response and the cohort health response), so the UI can say when the rule is
not approved.

## The placeholder version `draft-v0`

On start, if the configured event has **no** version at all, Pulse stores
the nine monthly values that used to be hard-coded as version `draft-v0`,
status `draft`. **These are pre-existing placeholder values awaiting Product
sign-off; they are not the official Event timeline** (review C2, ticket
XP-004). Startup never overwrites or re-seeds targets once an event has any
version, so edits made in the database survive restarts. The same applies to
`status_policy`: missing default policies are added, existing rows are never
changed.

## How Product supplies approved targets

Add a new version (never edit an approved one in place), in one transaction:

```sql
BEGIN;
INSERT INTO curriculum_timeline_versions (event_id, label, status, notes, approved_by, approved_at)
VALUES (97, 'event97-2026-11', 'approved', 'Signed off by Product on 2026-11-20 (ticket XP-004)', 'Product owner name', NOW())
RETURNING id;   -- use this id below as :version_id

INSERT INTO curriculum_timeline (event_id, version_id, month, min_level, expected_level, checkpoint_level, rank, notes) VALUES
  (97, :version_id, 2, ..., ..., ..., 1, 'Month 2'),
  (97, :version_id, 4, ..., ..., ..., 2, 'Month 4');
  -- one row per milestone month
COMMIT;
```

The next sync (or `go run ./cmd/worker`) rebuilds today's snapshots with the
new version. To withdraw a version, set `status = 'retired'`; Pulse falls
back to the newest remaining approved version, else the newest draft. A
draft for review can be added the same way with `status = 'draft'`; it is
used only while no approved version exists.
