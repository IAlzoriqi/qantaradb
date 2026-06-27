# QantaraDB Hardening 002 Report

status:
partially_completed

baseline:
5e2f6f65f1cce50e76ae3e183e362ceaf099bf8a

scope:
- sanitized-row reporting
- sequence reset hardening
- checksum normalization
- validation status behavior
- staging readiness gate

sanitized_rows_report:
created

outputs:
- `storage/reports/qantaradb_sanitized_rows_report.json`
- `storage/reports/qantaradb_sanitized_rows_report.md`

sequence_reset_report:
created

outputs:
- `storage/reports/qantaradb_sequence_reset_report.json`
- `storage/reports/qantaradb_sequence_reset_report.md`

checksum_normalization:
implemented

classification:
- `passed`
- `normalized_equivalent`
- `sanitized_equivalent`
- `real_mismatch`
- `unsupported_comparison`

validation_status:
implemented through `go run ./cmd/qantaradb validate`

local_validation_result:
completed_failed

local_validation_blocker:
Initial default config validation was blocked by local credentials, then rerun with local-only MySQL root/no-password against `foodtech_test` and a temporary PostgreSQL target on `127.0.0.1:55432`.

local_validation_status:
VALIDATION_FAILED

local_validation_summary:
- source: local MySQL `foodtech_test`
- target: temporary local PostgreSQL `foodtech_qantara_validation_test`
- total_tables: 26
- passed_tables: 16
- row_counts: passed
- foreign_keys: passed
- sequences: reset_passed
- sanitized_rows: clean
- failed_reason: real checksum mismatches in 10 tables

checksum_failed_tables:
- activity_log
- branches
- brands
- business_settings
- manager_permission_audit_logs
- product_by_branches
- products
- users
- variations
- variations_values

statuses:
- `VALIDATION_PASSED`
- `VALIDATION_PARTIAL`
- `VALIDATION_FAILED`

staging_readiness:
blocked because local validation reported real checksum mismatches.

remaining_blockers:
- checksum mismatch investigation for the 10 failed local validation tables.
- staging execution still requires owner-approved staging dry-run.
- server database migration remains forbidden until separate approval.

server:
not touched

staging_upload:
not attempted

server_db_migration:
not executed

secrets:
not committed
