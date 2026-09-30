BEGIN;

CREATE TABLE app_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    google_sub TEXT UNIQUE,
    display_name TEXT,
    role TEXT NOT NULL CHECK (role IN ('admin','mentor','student')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE students (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_user_id INTEGER NOT NULL UNIQUE,
    login TEXT NOT NULL UNIQUE,
    display_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cohorts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    source_label_id INTEGER UNIQUE,
    source_label_name TEXT NOT NULL,
    is_pilot BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cohort_students (
    cohort_id BIGINT NOT NULL REFERENCES cohorts(id) ON DELETE CASCADE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    source_label_user_id INTEGER,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_current BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (cohort_id, student_id)
);

CREATE TABLE source_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_event_id INTEGER NOT NULL UNIQUE,
    path TEXT NOT NULL,
    name TEXT,
    type TEXT,
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    raw_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE student_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id BIGINT NOT NULL REFERENCES source_events(id) ON DELETE CASCADE,
    source_event_user_id INTEGER NOT NULL,
    enrolled_at TIMESTAMPTZ NOT NULL,
    current_level INTEGER NOT NULL DEFAULT 0,
    audit_ratio DOUBLE PRECISION,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(student_id, event_id),
    UNIQUE(source_event_user_id)
);

CREATE TABLE curriculum_timeline (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES source_events(id) ON DELETE CASCADE,
    month INTEGER NOT NULL CHECK (month > 0),
    min_level INTEGER NOT NULL DEFAULT 0,
    expected_level INTEGER NOT NULL DEFAULT 0,
    checkpoint_level INTEGER NOT NULL DEFAULT 0,
    rank TEXT,
    notes TEXT,
    raw JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE(event_id, month)
);

CREATE TABLE progress_records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_progress_id BIGINT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id BIGINT REFERENCES source_events(id) ON DELETE SET NULL,
    object_id INTEGER NOT NULL,
    group_id INTEGER,
    path TEXT NOT NULL,
    is_done BOOLEAN NOT NULL DEFAULT FALSE,
    grade NUMERIC,
    source_created_at TIMESTAMPTZ NOT NULL,
    source_updated_at TIMESTAMPTZ NOT NULL,
    graded_at TIMESTAMPTZ,
    platform_started_working_at TIMESTAMPTZ,
    group_status TEXT,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_progress_student_updated ON progress_records(student_id, source_updated_at DESC);
CREATE INDEX idx_progress_student_path ON progress_records(student_id, path);
CREATE INDEX idx_progress_active ON progress_records(student_id, is_done) WHERE is_done = FALSE;

CREATE TABLE learning_transactions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_transaction_id BIGINT NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id BIGINT REFERENCES source_events(id) ON DELETE SET NULL,
    object_id INTEGER,
    audit_id BIGINT,
    path TEXT,
    type TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    is_bonus BOOLEAN NOT NULL DEFAULT FALSE,
    source_created_at TIMESTAMPTZ NOT NULL,
    invalidated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tx_student_created ON learning_transactions(student_id, source_created_at DESC);
CREATE INDEX idx_tx_learning_valid ON learning_transactions(student_id, type, source_created_at DESC)
WHERE invalidated_at IS NULL;

CREATE TABLE audits (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_audit_id BIGINT NOT NULL UNIQUE,
    auditor_source_user_id INTEGER,
    auditor_student_id BIGINT REFERENCES students(id) ON DELETE SET NULL,
    group_id INTEGER,
    result_id BIGINT,
    audit_type TEXT,
    source_created_at TIMESTAMPTZ NOT NULL,
    audited_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    grade NUMERIC,
    closure_type TEXT,
    automated BOOLEAN NOT NULL DEFAULT FALSE,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audits_auditor_time ON audits(auditor_source_user_id, audited_at DESC);
CREATE INDEX idx_audits_closure ON audits(closure_type, closed_at DESC);

CREATE TABLE student_source_records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_record_id INTEGER NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    type_name TEXT NOT NULL,
    message TEXT,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ,
    source_created_at TIMESTAMPTZ,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_records_student_window ON student_source_records(student_id, start_at, end_at);
CREATE INDEX idx_records_student_type ON student_source_records(student_id, type_name);

CREATE TABLE object_availability (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_availability_id INTEGER NOT NULL UNIQUE,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id BIGINT REFERENCES source_events(id) ON DELETE SET NULL,
    object_id INTEGER,
    path TEXT NOT NULL,
    source_created_at TIMESTAMPTZ NOT NULL,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_availability_student_time ON object_availability(student_id, source_created_at DESC);

CREATE TABLE status_policy (
    record_type TEXT PRIMARY KEY,
    pause_clock BOOLEAN NOT NULL,
    exclude_from_active_population BOOLEAN NOT NULL,
    terminal BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO status_policy(record_type, pause_clock, exclude_from_active_population, terminal, description) VALUES
('away', FALSE, FALSE, FALSE, 'Does not pause academic clock; remains in active pace KPIs'),
('restricted', TRUE, TRUE, FALSE, 'Pauses academic clock; excluded from active pace KPIs while active'),
('blocked-long', TRUE, TRUE, FALSE, 'Pauses academic clock; excluded from active pace KPIs while active'),
('temporary-notice', FALSE, FALSE, FALSE, 'Annotation only'),
('left-school', TRUE, TRUE, TRUE, 'Terminal/inactive; excluded from active pace KPIs'),
('observation', FALSE, FALSE, FALSE, 'Annotation only'),
('ban', TRUE, TRUE, FALSE, 'Pauses academic clock; excluded from active pace KPIs while active');

CREATE TABLE mentor_assignments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    mentor_user_id BIGINT NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    is_primary BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE UNIQUE INDEX uq_one_active_primary_mentor
ON mentor_assignments(student_id)
WHERE is_primary = TRUE AND ends_at IS NULL;

CREATE TABLE student_metric_snapshots (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id BIGINT NOT NULL REFERENCES source_events(id) ON DELETE CASCADE,
    snapshot_date DATE NOT NULL,
    enrolled_at TIMESTAMPTZ NOT NULL,
    paused_days INTEGER NOT NULL DEFAULT 0 CHECK (paused_days >= 0),
    active_program_days INTEGER NOT NULL CHECK (active_program_days >= 0),
    curriculum_month INTEGER NOT NULL CHECK (curriculum_month > 0),
    actual_level INTEGER NOT NULL,
    minimum_level INTEGER,
    expected_level INTEGER,
    checkpoint_target INTEGER,
    gap_to_minimum INTEGER,
    gap_to_expected INTEGER,
    pace_status TEXT NOT NULL CHECK (pace_status IN ('GREEN','YELLOW','RED','PAUSED','INACTIVE','NOT_MEASURABLE')),
    health_reason_codes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    calculation_version TEXT NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(student_id, event_id, snapshot_date)
);

CREATE INDEX idx_snapshots_date_status ON student_metric_snapshots(snapshot_date DESC, pace_status);
CREATE INDEX idx_snapshots_student_date ON student_metric_snapshots(student_id, snapshot_date DESC);

CREATE TABLE interventions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    mentor_user_id BIGINT NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
    reason_code TEXT NOT NULL,
    type TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','IN_PROGRESS','RESOLVED','CANCELLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    outcome TEXT,
    CHECK (resolved_at IS NULL OR resolved_at >= created_at)
);

CREATE INDEX idx_interventions_student_status ON interventions(student_id, status, due_at);
CREATE INDEX idx_interventions_mentor_status ON interventions(mentor_user_id, status, due_at);

CREATE TABLE intervention_updates (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    intervention_id BIGINT NOT NULL REFERENCES interventions(id) ON DELETE CASCADE,
    author_user_id BIGINT NOT NULL REFERENCES app_users(id) ON DELETE RESTRICT,
    note TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_active ON auth_sessions(user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE sync_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_name TEXT NOT NULL,
    cursor_from TEXT,
    cursor_to TEXT,
    status TEXT NOT NULL CHECK (status IN ('RUNNING','SUCCEEDED','FAILED','PARTIAL')),
    rows_seen INTEGER NOT NULL DEFAULT 0,
    rows_upserted INTEGER NOT NULL DEFAULT 0,
    error_summary TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_sync_runs_job_time ON sync_runs(job_name, started_at DESC);

COMMIT;
