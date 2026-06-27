# QantaraDB Global Library Readiness Audit

Status: internal early-development draft  
Scope: technical and product gap analysis  
Do not publish as marketing material yet.

## 1. Current positioning

QantaraDB currently presents itself as a high-performance, resumable MySQL/MariaDB to PostgreSQL migration library and CLI.

The current public README already claims these implemented or intended capability areas:

- MySQL/MariaDB to PostgreSQL migration.
- Schema inspection.
- Type mapping.
- Dependency planning.
- PK-based chunk streaming copy.
- Resumable state tracking.
- Validation reports.
- FoodTech-specific preflight checks.

This is a strong foundation, but it is not yet enough to call the project a global database transformation library.

## 2. Target global positioning

The future public positioning should be:

> QantaraDB is a Go library and CLI for safe, resumable, auditable database transformation between relational databases, starting with MySQL/MariaDB and PostgreSQL, with schema conversion, bulk copy, validation, restart safety, and planned CDC/live cutover support.

Do not claim live billion-row enterprise migration readiness until the CDC, snapshot, validation, and rollback gates in this document are implemented and tested.

## 3. Capability matrix

| Capability | Current state | Required global-grade state |
|---|---:|---|
| MySQL/MariaDB to PostgreSQL | implemented/partial | production hardening, broader type coverage, large fixture testing |
| PostgreSQL to MySQL/MariaDB | missing | reverse inspector, reverse mapper, reverse DDL compiler, reverse loader |
| Schema inspection | implemented for MySQL | unified connector model for source/target databases |
| Type mapping | implemented for MySQL to PostgreSQL | bidirectional mapping registry with explicit loss-risk classifications |
| Dependency planning | implemented | cycle handling, deferred constraints, partition-aware ordering |
| Bulk loading | implemented with PostgreSQL CopyFrom | pluggable loaders for both PostgreSQL COPY and MySQL LOAD DATA / prepared batch inserts |
| Resume | partially implemented with state file | durable checkpoint store, atomic chunk commits, idempotency keys, recovery tests |
| Validation | implemented/partial | row count, checksums, sampled diff, null/encoding drift, FK/index/sequence verification |
| High-write live migration | missing | initial snapshot + CDC + lag monitor + cutover gate |
| Billion-row scale | not proven | streaming benchmarks, memory caps, parallel partitioning, chunk leasing, backpressure |
| Disaster safety | partial | source read-only enforcement, target guardrails, reversible phases, stop/resume/rollback runbooks |
| Developer library API | partial | stable public Go interfaces, examples, godoc package comments, semantic versioning |
| CLI UX | partial | interactive `init`, DSN builder, config validation, secrets masking, clear dry-run/execute split |
| Reports | implemented/partial | immutable job ledger, machine-readable JSON, human markdown, audit trail, skipped object registry |
| Discoverability | partial | correct module path, tags, docs, examples, topics, website, pkg.go.dev, llms.txt after public readiness |

## 4. Critical technical gaps

### 4.1 Bidirectional database support

The current design is direction-specific. A global library must separate database-specific code behind interfaces:

- `Inspector`: reads tables, columns, indexes, constraints, sequences, views, triggers, routines, partitions.
- `Mapper`: maps source objects to target objects and emits risk classifications.
- `DDLCompiler`: renders safe target DDL.
- `Loader`: copies rows with resumable chunks.
- `Validator`: verifies data and structure after load.
- `ChangeCapture`: streams source changes after snapshot.
- `CutoverCoordinator`: controls final consistency and switchover.

Required initial connectors:

- `mysql` source connector.
- `postgres` target connector.
- `postgres` source connector.
- `mysql` target connector.

The library must never treat MySQL->PostgreSQL logic as the core model. It must treat each database as a connector implementing contracts.

### 4.2 Live migration under write pressure

Bulk copy alone is not enough for active systems with continuous writes. The enterprise sequence must be:

1. Preflight and compatibility scan.
2. Consistent source snapshot.
3. Initial schema transform.
4. Bulk table load.
5. CDC catch-up from snapshot position.
6. Lag monitoring.
7. Validation gate.
8. Read-only or dual-write cutover window.
9. Final checksum and sequence reconciliation.
10. Rollback or promote decision.

Database-specific CDC paths:

- MySQL/MariaDB: binlog position or GTID.
- PostgreSQL: logical replication slot / WAL LSN.

Until CDC exists, public docs must say `offline/bulk migration` or `local migration`, not `live migration`.

### 4.3 Resume and disaster recovery

A JSON state file is acceptable for a prototype, but not for global-grade operations.

Required state model:

- `job_id`
- `run_id`
- source fingerprint
- target fingerprint
- schema hash
- table plan hash
- chunk key range
- chunk status: `pending`, `claimed`, `copying`, `copied`, `validated`, `failed`, `skipped`
- retry count
- last error
- source low/high watermark
- CDC position
- checksum status
- sanitized row counters
- created/updated timestamps

Durable state storage options:

- embedded SQLite state database for local CLI jobs,
- PostgreSQL state schema for distributed workers,
- JSON export only as report output, not as the primary state database.

The resume contract must be:

> After process crash, network interruption, database disconnect, or host restart, rerunning the same job must continue from the last verified chunk without duplicating committed target rows or skipping unverified source rows.

### 4.4 Idempotent target writes

Resume is unsafe unless writes are idempotent.

Required options:

- staging tables per job,
- deterministic chunk identifiers,
- target-side unique job/chunk ledger,
- `INSERT ... ON CONFLICT` for PostgreSQL where possible,
- MySQL equivalent upsert strategy for reverse direction,
- chunk-level transaction boundaries,
- checksum-before-promote gates,
- no destructive target operation without explicit approval.

### 4.5 Validation strength

Validation must graduate from basic parity to audit-grade proof.

Required validation layers:

1. Table row count.
2. Primary key min/max.
3. Per-chunk checksum.
4. Per-column null count.
5. Type coercion drift report.
6. Sanitized-value report.
7. Skipped-object report.
8. FK validation.
9. Index existence validation.
10. Sequence/auto-increment reconciliation.
11. View/function/trigger compatibility report.
12. Sampled row diff for high-volume tables.

Every validation report must classify each table/object as:

- `passed`
- `warning`
- `normalized_equivalent`
- `sanitized_equivalent`
- `unsupported_comparison`
- `skipped_by_policy`
- `failed`

### 4.6 Safety gates

Default behavior must remain preview-first.

Mandatory gates:

- source connection must be read-only by default,
- target writes blocked unless `--execute` is provided,
- destructive target operations blocked unless explicitly approved,
- production-like DSNs require an approval token or signed approval file,
- all secrets masked in logs and reports,
- target database fingerprint stored before migration,
- plan hash must match at resume time,
- source schema drift must be detected before resume,
- CDC lag must be below threshold before cutover,
- failed validation must block promote.

### 4.7 Credential and database selection UX

Global CLI users must not be forced to write DSNs manually.

Required command:

```bash
qantaradb init
```

It should ask:

- source database engine: `mysql`, `mariadb`, `postgres`
- source host
- source port
- source database name
- source username
- source password, hidden input
- target database engine
- target host
- target port
- target database name
- target username
- target password, hidden input
- migration mode: `inspect`, `plan`, `offline-copy`, `cdc`, `validate-only`
- state backend: `sqlite`, `postgres`, `file-report-only`
- output directory

Generated config must support environment references:

```yaml
source:
  driver: mysql
  host: 127.0.0.1
  port: 3306
  database: ${QANTARA_SOURCE_DB}
  username: ${QANTARA_SOURCE_USER}
  password_env: QANTARA_SOURCE_PASSWORD

target:
  driver: postgres
  host: 127.0.0.1
  port: 5432
  database: ${QANTARA_TARGET_DB}
  username: ${QANTARA_TARGET_USER}
  password_env: QANTARA_TARGET_PASSWORD
```

Rules:

- Never write passwords to config files by default.
- Never print raw DSNs.
- Support DSN input for advanced users.
- Support `--source-dsn-env` and `--target-dsn-env`.

## 5. Required repository readiness before public push

Before public marketing, the repository must have:

- correct Go module path: `github.com/IAlzoriqi/qantaradb`, not placeholder owner path,
- stable package comments for pkg.go.dev,
- CLI examples with fake credentials only,
- `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`,
- GitHub Actions matrix for Go tests,
- integration tests using ephemeral MySQL/MariaDB/PostgreSQL containers,
- benchmark fixtures,
- release tags,
- versioned docs,
- public examples under `examples/`,
- hidden/internal docs not linked until stable.

## 6. Public-readiness gate

QantaraDB can be called global-grade only after all of these pass:

```text
go fmt ./...
go test ./...
go vet ./...
go build ./...
integration:mysql-to-postgres
integration:postgres-to-mysql
resume-crash-test
checksum-drift-test
schema-drift-test
cdc-catchup-test
validation-report-test
secrets-redaction-test
large-fixture-benchmark
```

Until then, use this public wording:

> QantaraDB is an early-stage Go CLI and library focused on safe, resumable MySQL/MariaDB to PostgreSQL migration, with planned bidirectional and live migration support.
