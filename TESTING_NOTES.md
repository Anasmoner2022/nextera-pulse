# Test instructions (2026-11-11)

Standard: `go test ./... -count=1`.

Optional integration tests require `-tags=integration`. PostgreSQL snapshot test writes to **disposable** `nextera_pulse_test` only and expects a synthetic `test_student` (Event 97, Cohort 4). It does NOT test live 01Edu. Live read test does not write but needs an *approved* server-side read-only service token. A SKIP is not live success.

Do not run `cmd/worker` against production: it updates levels without recalculating the API's latest snapshots. Do not publicly expose API: staff authentication and RBAC are unfinished.

See `HANDOVER_2026-11-11.txt` for detailed evidence, work remaining and safety notes.
