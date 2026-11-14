-- Nextera Pulse SQLite DDL
-- Sprint 1 Core Schema & Operational Tables

CREATE TABLE IF NOT EXISTS students (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_user_id INTEGER UNIQUE NOT NULL,
    login TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    email TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS source_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_event_id INTEGER UNIQUE NOT NULL,
    path TEXT,
    name TEXT,
    type TEXT,
    start_at TEXT,
    end_at TEXT,
    last_synced_at TEXT
);

CREATE TABLE IF NOT EXISTS student_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id INTEGER NOT NULL,
    cohort_id INTEGER NOT NULL,
    enrolled_at TEXT NOT NULL,
    current_level REAL NOT NULL DEFAULT 0,
    audit_ratio REAL NOT NULL DEFAULT 0,
    last_synced_at TEXT,
    UNIQUE (student_id, event_id)
);

CREATE TABLE IF NOT EXISTS status_policy (
    record_type TEXT PRIMARY KEY,
    pause_clock INTEGER NOT NULL DEFAULT 0,
    exclude_from_active_kpis INTEGER NOT NULL DEFAULT 0,
    description TEXT
);

CREATE TABLE IF NOT EXISTS curriculum_timeline (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL,
    month INTEGER NOT NULL,
    min_level REAL NOT NULL,
    expected_level REAL NOT NULL,
    checkpoint_level REAL NOT NULL DEFAULT 0,
    rank INTEGER DEFAULT 0,
    notes TEXT,
    UNIQUE (event_id, month)
);

CREATE TABLE IF NOT EXISTS student_source_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_record_id INTEGER UNIQUE,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    type_name TEXT NOT NULL,
    start_at TEXT NOT NULL,
    end_at TEXT,
    message TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS student_metric_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_id INTEGER NOT NULL,
    snapshot_date TEXT NOT NULL,
    enrolled_at TEXT NOT NULL,
    active_program_days INTEGER NOT NULL,
    curriculum_month INTEGER NOT NULL,
    actual_level REAL NOT NULL,
    minimum_level REAL NOT NULL,
    expected_level REAL NOT NULL,
    checkpoint_target REAL NOT NULL,
    gap_to_minimum REAL NOT NULL,
    gap_to_expected REAL NOT NULL,
    pace_status TEXT NOT NULL,
    health_reasons TEXT,
    calculation_version TEXT NOT NULL DEFAULT 'v0.2',
    calculated_at TEXT NOT NULL,
    UNIQUE (student_id, event_id, snapshot_date)
);

CREATE TABLE IF NOT EXISTS mentor_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    mentor_id INTEGER NOT NULL,
    mentor_name TEXT NOT NULL,
    starts_at TEXT NOT NULL,
    ends_at TEXT,
    is_primary INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS interventions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    mentor_id INTEGER NOT NULL,
    reason_code TEXT NOT NULL,
    type TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    due_at TEXT,
    resolved_at TEXT,
    outcome TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    records_ingested INTEGER NOT NULL DEFAULT 0,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_student_events_cohort ON student_events(cohort_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_date ON student_metric_snapshots(snapshot_date);
CREATE INDEX IF NOT EXISTS idx_snapshots_status ON student_metric_snapshots(pace_status);
