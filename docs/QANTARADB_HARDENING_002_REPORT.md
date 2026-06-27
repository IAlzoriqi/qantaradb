# QantaraDB Hardening 002 Report

status:
completed

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
VALIDATION_PARTIAL

local_validation_summary:
- source: local MySQL `foodtech_test`
- target: temporary local PostgreSQL `foodtech_qantara_validation_test`
- total_tables: 26
- passed_tables: 26
- row_counts: passed
- foreign_keys: passed
- sequences: reset_passed
- sanitized_rows: clean
- checksum_drilldown: clean
- result_reason: normalized-equivalent checksums in the 10 tables that originally failed.

checksum_mismatch_resolution:
- original_real_mismatch_tables:
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
- root_cause: PostgreSQL `numeric` values returned by pgx as `pgtype.Numeric` plus a decimal normalizer bug that stripped trailing zeros from integer strings, turning values such as `10` into `1`.
- fix: normalize finite `pgtype.Numeric` using integer and exponent, and only strip trailing zeros after a decimal point.
- resolved_as_normalized_equivalent:
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
- resolved_as_sanitized_equivalent: []
- remaining_real_mismatch: []
- unsupported_comparison: []
- validation_status: VALIDATION_PARTIAL
- staging_readiness: ready

original_real_mismatch_tables:
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
ready; validation remains `VALIDATION_PARTIAL` because normalized-equivalent warnings are recorded for review, but there are no row-count failures, FK failures, sequence failures, real mismatches, or missing sanitized-row reports.

remaining_blockers:
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
