# QantaraDB Go Local Audit

status:
ready_with_caveats

language:
Go

go_module:
github.com/OWNER/qantaradb

entrypoints:
- `cmd/qantaradb/main.go`

commands:
- `go run ./cmd/qantaradb inspect --dry-run --mysql <local-mysql-dsn>`
- `go run ./cmd/qantaradb migrate --dry-run --config config.yaml`
- `go run ./cmd/qantaradb migrate --execute --local-only --config config.yaml`
- `go run ./cmd/qantaradb validate --dry-run --config config.yaml`
- `go run ./cmd/qantaradb validate --config config.yaml`
- `go run ./cmd/qantaradb report --config config.yaml`

config:
- env:
  - `.env.example` documents local-only connection variables without secrets.
- flags:
  - `--dry-run` is accepted and defaults to safe preview behavior.
  - `--execute` is required for real copy.
  - `--local-only` is required with `--execute`.
- config files:
  - `config.yaml`
  - `examples/config.yaml`

source_mysql:
- driver: `github.com/go-sql-driver/mysql`
- connection: MySQL DSN from config/flags.
- schema inspection: `inspector.Inspect` reads `information_schema`.

target_postgres:
- driver: `github.com/jackc/pgx/v5`
- connection: PostgreSQL DSN from config.
- schema_creation: generated through `ddl.GenerateDDL` and applied only during `migrate --execute --local-only`.

capabilities:
- dry_run: yes
- schema_mapping: yes
- data_copy: yes, via pgx CopyFrom
- batch_copy: yes
- resume_retry: partial, through loader state file
- row_count_validation: yes
- checksum_validation: sample checksum support exists
- failed_rows_report: partial
- foreign_keys: DDL generation and validation support exist
- indexes: DDL generation support exists
- sequences: documented as required; full reset still needs a focused pass
- json: maps to `jsonb`
- decimal: maps to `numeric(p,s)`
- datetime: maps by configured policy
- enum: maps to text/check policy
- unsigned_int: maps to wider PostgreSQL numeric/signed types with checks
- blob: maps to `bytea`
- reports_json: yes
- reports_markdown: yes

blockers:
- Server DB execution is forbidden for this task.
- Staging upload is blocked until Console PostgreSQL smoke-test blockers are addressed.
- Sanitized-row reporting and sequence reset need focused hardening before staging DB rehearsal.

recommended_fixes:
- Add a sanitized-row report for removed NUL bytes and invalid UTF-8 sequences.
- Add a focused sequence-reset implementation before staging migration rehearsal.
- Add safe boolean allow-listing for `tinyint(1)` columns instead of global boolean conversion.
- Keep server migration blocked until staging dry-run/migration/rollback are proven.
