-- Nextera Pulse PostgreSQL DDL
-- Sprint 1 Core Schema & Operational Tables

CREATE TABLE IF NOT EXISTS students (
    id BIGSERIAL PRIMARY KEY,
    source_user_id BIGINT UNIQUE NOT NULL,
    login TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    email TEXT,
    source_role TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Databases created before source_role existed.
ALTER TABLE students ADD COLUMN IF NOT EXISTS source_role TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS source_events (
    id SERIAL PRIMARY KEY,
    source_event_id INTEGER UNIQUE NOT NULL,
    path TEXT,
    name TEXT,
    type TEXT,
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS student_events (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id INTEGER NOT NULL,
    cohort_id INTEGER NOT NULL,
    enrolled_at TIMESTAMPTZ NOT NULL,
    current_level DOUBLE PRECISION NOT NULL DEFAULT 0,
    audit_ratio DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_student_event UNIQUE (student_id, event_id)
);

CREATE TABLE IF NOT EXISTS status_policy (
    record_type TEXT PRIMARY KEY,
    pause_clock BOOLEAN NOT NULL DEFAULT FALSE,
    exclude_from_active_kpis BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS curriculum_timeline (
    id SERIAL PRIMARY KEY,
    event_id INTEGER NOT NULL,
    month INTEGER NOT NULL,
    min_level DOUBLE PRECISION NOT NULL,
    expected_level DOUBLE PRECISION NOT NULL,
    checkpoint_level DOUBLE PRECISION NOT NULL DEFAULT 0,
    rank INTEGER DEFAULT 0,
    notes TEXT
);

CREATE TABLE IF NOT EXISTS student_source_records (
    id BIGSERIAL PRIMARY KEY,
    source_record_id BIGINT UNIQUE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    type_name TEXT NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ,
    message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS student_metric_snapshots (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id INTEGER NOT NULL,
    snapshot_date DATE NOT NULL,
    enrolled_at TIMESTAMPTZ NOT NULL,
    active_program_days INTEGER NOT NULL,
    curriculum_month INTEGER NOT NULL,
    actual_level DOUBLE PRECISION NOT NULL,
    minimum_level DOUBLE PRECISION NOT NULL,
    expected_level DOUBLE PRECISION NOT NULL,
    checkpoint_target DOUBLE PRECISION NOT NULL,
    gap_to_minimum DOUBLE PRECISION NOT NULL,
    gap_to_expected DOUBLE PRECISION NOT NULL,
    pace_status TEXT NOT NULL,
    health_reasons TEXT,
    calculation_version TEXT NOT NULL DEFAULT 'v0.2',
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_student_snapshot UNIQUE (student_id, event_id, snapshot_date)
);

CREATE TABLE IF NOT EXISTS mentor_assignments (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    mentor_id BIGINT NOT NULL,
    mentor_name TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ,
    is_primary BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS interventions (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    mentor_id BIGINT NOT NULL,
    reason_code TEXT NOT NULL,
    type TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    due_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    outcome TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sync_runs (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    records_ingested INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_student_events_cohort ON student_events(cohort_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_date ON student_metric_snapshots(snapshot_date);
CREATE INDEX IF NOT EXISTS idx_snapshots_status ON student_metric_snapshots(pace_status);

CREATE TABLE IF NOT EXISTS learning_objects (
    source_object_id INTEGER PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    author_login TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS learning_progress (
    source_progress_id INTEGER PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    source_user_id BIGINT NOT NULL,
    event_id INTEGER,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    is_done INTEGER NOT NULL DEFAULT 0,
    grade DOUBLE PRECISION,
    group_id INTEGER,
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_learning_progress_student ON learning_progress(student_id, is_done);
CREATE INDEX IF NOT EXISTS idx_learning_progress_object ON learning_progress(object_id);

CREATE TABLE IF NOT EXISTS learning_results (
    source_result_id INTEGER PRIMARY KEY,
    student_id BIGINT,
    source_user_id BIGINT NOT NULL,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    grade DOUBLE PRECISION,
    type TEXT NOT NULL DEFAULT '',
    is_last INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_learning_results_student ON learning_results(student_id);

CREATE TABLE IF NOT EXISTS learning_groups (
    source_group_id INTEGER PRIMARY KEY,
    event_id INTEGER,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    captain_login TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS learning_audits (
    source_audit_id INTEGER PRIMARY KEY,
    group_id INTEGER,
    auditor_login TEXT NOT NULL DEFAULT '',
    grade DOUBLE PRECISION,
    closure_type TEXT NOT NULL DEFAULT '',
    audited_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS learning_transactions (
    source_transaction_id INTEGER PRIMARY KEY,
    student_id BIGINT,
    source_user_id BIGINT NOT NULL,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_bonus INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_learning_tx_student ON learning_transactions(student_id);

CREATE TABLE IF NOT EXISTS cohort_labels (
    source_label_id INTEGER PRIMARY KEY,
    event_id INTEGER,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS cohort_label_users (
    source_id INTEGER PRIMARY KEY,
    label_id INTEGER NOT NULL,
    source_user_id BIGINT NOT NULL,
    label_name TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_label_users_user ON cohort_label_users(source_user_id, label_id);

-- Server-side sessions issued after a successful 01Edu sign-in. Only a
-- SHA-256 hash of the cookie token is stored.
CREATE TABLE IF NOT EXISTS app_sessions (
    id BIGSERIAL PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_agent TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_app_sessions_expires ON app_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_app_sessions_student ON app_sessions(student_id);

-- Follow-up author: interventions.mentor_id is the signed-in author's person id
-- (taken from the session); author_role is their Pulse role at the time.
ALTER TABLE interventions ADD COLUMN IF NOT EXISTS author_role TEXT NOT NULL DEFAULT '';

-- Mentor-assignment history (review C1, D2). A move or unassign closes the
-- current row (ends_at) instead of deleting it, so the history is kept.
-- assigned_by / ended_by are the signed-in manager's person id (NULL for the
-- automatic assignment after a live sync).
ALTER TABLE mentor_assignments ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';
ALTER TABLE mentor_assignments ADD COLUMN IF NOT EXISTS assigned_by BIGINT REFERENCES students(id) ON DELETE SET NULL;
ALTER TABLE mentor_assignments ADD COLUMN IF NOT EXISTS ended_by BIGINT REFERENCES students(id) ON DELETE SET NULL;
ALTER TABLE mentor_assignments ADD COLUMN IF NOT EXISTS end_reason TEXT NOT NULL DEFAULT '';

-- Older databases could hold several open rows for one student: keep the
-- newest open, close the rest, so the unique index below can be built.
UPDATE mentor_assignments m
SET ends_at = NOW(), end_reason = 'closed: duplicate current assignment'
WHERE m.ends_at IS NULL AND m.is_primary AND EXISTS (
    SELECT 1 FROM mentor_assignments n
    WHERE n.student_id = m.student_id AND n.ends_at IS NULL AND n.is_primary
      AND (n.starts_at > m.starts_at OR (n.starts_at = m.starts_at AND n.id > m.id))
);

-- At most one current primary mentor per student.
CREATE UNIQUE INDEX IF NOT EXISTS uq_mentor_assignments_current
    ON mentor_assignments(student_id) WHERE is_primary AND ends_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_mentor_assignments_mentor_current
    ON mentor_assignments(mentor_id) WHERE ends_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_mentor_assignments_student_history
    ON mentor_assignments(student_id, starts_at DESC);

-- mentor_id is the mentor's students.id. The FK is added NOT VALID (enforced
-- for new rows) and validated when the existing rows allow it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_mentor_assignments_mentor'
                   AND conrelid = 'mentor_assignments'::regclass) THEN
        ALTER TABLE mentor_assignments ADD CONSTRAINT fk_mentor_assignments_mentor
            FOREIGN KEY (mentor_id) REFERENCES students(id) NOT VALID;
    END IF;
    BEGIN
        ALTER TABLE mentor_assignments VALIDATE CONSTRAINT fk_mentor_assignments_mentor;
    EXCEPTION WHEN foreign_key_violation THEN
        RAISE NOTICE 'mentor_assignments has rows whose mentor is not a known person; FK stays NOT VALID';
    END;
END $$;

-- Follow-up deletion is a soft delete: the note is hidden from every list but
-- kept for the record, with who removed it and when.
ALTER TABLE interventions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE interventions ADD COLUMN IF NOT EXISTS deleted_by BIGINT REFERENCES students(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_interventions_visible ON interventions(created_at DESC) WHERE deleted_at IS NULL;

-- Versioned monthly targets. Each version belongs to one curriculum event and
-- is a draft until Product approves it. The pace engine uses the latest
-- approved version of the configured event, else the latest draft. Startup
-- never overwrites targets (see store.SeedCurriculumTimeline).
CREATE TABLE IF NOT EXISTS curriculum_timeline_versions (
    id BIGSERIAL PRIMARY KEY,
    event_id INTEGER NOT NULL,
    label TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'retired')),
    notes TEXT NOT NULL DEFAULT '',
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_timeline_version_label UNIQUE (event_id, label)
);

ALTER TABLE curriculum_timeline ADD COLUMN IF NOT EXISTS version_id BIGINT REFERENCES curriculum_timeline_versions(id) ON DELETE CASCADE;
ALTER TABLE curriculum_timeline DROP CONSTRAINT IF EXISTS uq_event_month;

-- Rows from before versioning are the old placeholder seed: file them under
-- "draft-v0" for their event.
INSERT INTO curriculum_timeline_versions (event_id, label, status, notes)
SELECT DISTINCT event_id, 'draft-v0', 'draft',
       'Pre-existing placeholder targets awaiting Product sign-off. NOT official.'
FROM curriculum_timeline WHERE version_id IS NULL
ON CONFLICT (event_id, label) DO NOTHING;
UPDATE curriculum_timeline t SET version_id = v.id
FROM curriculum_timeline_versions v
WHERE t.version_id IS NULL AND v.event_id = t.event_id AND v.label = 'draft-v0';
ALTER TABLE curriculum_timeline ALTER COLUMN version_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_timeline_version_month ON curriculum_timeline(version_id, month);
CREATE INDEX IF NOT EXISTS idx_timeline_versions_event ON curriculum_timeline_versions(event_id, status, created_at DESC);
