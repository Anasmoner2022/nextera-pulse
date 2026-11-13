-- Runs once when the dev Postgres volume is created: a separate database for
-- `go test ./...` (TEST_DATABASE_URL). Each test creates and drops its own schema.
CREATE DATABASE nextera_pulse_test OWNER nextera_pulse;
