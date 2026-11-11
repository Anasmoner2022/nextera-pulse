-- Nextera Pulse SQLite DDL
-- Sprint 1 Core Schema & Operational Tables

CREATE TABLE IF NOT EXISTS students (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_user_id INTEGER UNIQUE NOT NULL,
    login TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    email TEXT,
    source_role TEXT NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS learning_objects (
    source_object_id INTEGER PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    author_login TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS learning_progress (
    source_progress_id INTEGER PRIMARY KEY,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    source_user_id INTEGER NOT NULL,
    event_id INTEGER,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    is_done INTEGER NOT NULL DEFAULT 0,
    grade REAL,
    group_id INTEGER,
    updated_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_learning_progress_student ON learning_progress(student_id, is_done);
CREATE INDEX IF NOT EXISTS idx_learning_progress_object ON learning_progress(object_id);

CREATE TABLE IF NOT EXISTS learning_results (
    source_result_id INTEGER PRIMARY KEY,
    student_id INTEGER,
    source_user_id INTEGER NOT NULL,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    grade REAL,
    type TEXT NOT NULL DEFAULT '',
    is_last INTEGER NOT NULL DEFAULT 0,
    created_at TEXT
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
    grade REAL,
    closure_type TEXT NOT NULL DEFAULT '',
    audited_at TEXT
);

CREATE TABLE IF NOT EXISTS learning_transactions (
    source_transaction_id INTEGER PRIMARY KEY,
    student_id INTEGER,
    source_user_id INTEGER NOT NULL,
    object_id INTEGER,
    path TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    amount REAL NOT NULL DEFAULT 0,
    is_bonus INTEGER NOT NULL DEFAULT 0,
    created_at TEXT
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
    source_user_id INTEGER NOT NULL,
    label_name TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_label_users_user ON cohort_label_users(source_user_id, label_id);
