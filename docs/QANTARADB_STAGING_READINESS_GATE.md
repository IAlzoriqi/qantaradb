# QantaraDB Staging Readiness Gate

status:
active

latest_local_validation:
VALIDATION_FAILED

latest_local_validation_summary:
- local-only validation ran against MySQL `foodtech_test` and temporary PostgreSQL `foodtech_qantara_validation_test`.
- row counts passed.
- FK validation passed.
- sequence reset passed.
- sanitized rows report was clean.
- staging readiness remains blocked by real checksum mismatches in 10 tables.

purpose:
Block unsafe staging upload or staging database migration until local validation proves that the MySQL to PostgreSQL migration is structurally safe.

required_local_gates:
- `go fmt ./...`
- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `go run ./cmd/qantaradb validate`

validation_status_rules:
- `VALIDATION_PASSED`: row counts, FK checks, sequence reset, and checksums passed, with only safe normalized or sanitized equivalence where reported.
- `VALIDATION_PARTIAL`: data copied, but unsupported comparisons or sanitized/normalized equivalents require manual review.
- `VALIDATION_FAILED`: row count failure, FK failure, real checksum mismatch, or sequence reset failure.

blocking_conditions:
- row count failed
- FK validation failed
- sequence reset failed
- real checksum mismatches exist
- failed rows exist
- sanitized rows exist without `qantaradb_sanitized_rows_report`
- source/target safety is not documented

required_reports:
- `storage/reports/qantaradb_validation_report.json`
- `storage/reports/qantaradb_validation_report.md`
- `storage/reports/qantaradb_sanitized_rows_report.json`
- `storage/reports/qantaradb_sanitized_rows_report.md`
- `storage/reports/qantaradb_sequence_reset_report.json`
- `storage/reports/qantaradb_sequence_reset_report.md`

server_policy:
- no Server SSH during local hardening
- no staging upload until owner approval
- no server database migration until staging dry-run, backup, rollback, and owner approval are complete

secrets_policy:
No DSN passwords, tokens, SSH keys, or server credentials may be committed.
