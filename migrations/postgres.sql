-- Nextera Pulse PostgreSQL DDL
-- Sprint 1 Core Schema & Operational Tables

CREATE TABLE IF NOT EXISTS students (
    id BIGSERIAL PRIMARY KEY,
    source_user_id BIGINT UNIQUE NOT NULL,
    login TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    email TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
    notes TEXT,
    CONSTRAINT uq_event_month UNIQUE (event_id, month)
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
