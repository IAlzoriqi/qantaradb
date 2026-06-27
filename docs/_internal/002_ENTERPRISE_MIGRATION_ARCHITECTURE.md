# QantaraDB Enterprise Migration Architecture

Status: internal early-development draft.

## Goal

Refactor QantaraDB into a connector-based database transformation engine. MySQL, MariaDB, and PostgreSQL should be connectors behind stable contracts rather than assumptions embedded in the command layer.

## Target package map

```text
cmd/qantaradb/          CLI shell
core/job/               job metadata
core/plan/              neutral migration plans
core/object/            schema objects
core/risk/              compatibility classes
core/secrets/           secret references and masking
connectors/mysql/       MySQL and MariaDB behavior
connectors/postgres/    PostgreSQL behavior
mapper/                 bidirectional mapping registry
engine/                 inspect, plan, copy, resume, validate
state/sqlite/           local durable state
state/postgres/         shared state backend
report/                 JSON and Markdown artifacts
examples/               public examples after readiness
```

## Connector responsibilities

Each database connector should provide:

- schema inspection,
- target DDL rendering,
- bulk row loading,
- validation,
- identifier quoting,
- type mapping metadata,
- source position metadata for future online mode.

## Initial supported directions

- MySQL or MariaDB to PostgreSQL.
- PostgreSQL to MySQL or MariaDB.

## Operating modes

- `inspect-only`: read source metadata.
- `plan-only`: produce a deterministic plan.
- `offline-copy`: copy from a stable source or replica.
- `online-copy`: planned future mode for systems that keep receiving writes.
- `validate-only`: compare source and target without copying.

## Durable state

The current JSON state approach should evolve into a durable state backend.

Recommended defaults:

- SQLite for local CLI jobs.
- PostgreSQL for shared worker jobs.
- JSON and Markdown for exported reports.

State must track:

- job id,
- run id,
- source fingerprint,
- target fingerprint,
- schema hash,
- plan hash,
- table progress,
- chunk progress,
- validation status,
- artifact paths.

## Audit artifacts

Each run should produce:

- inspection JSON,
- plan JSON,
- generated DDL,
- safety gate report,
- migration ledger,
- chunk status report,
- sanitized value report,
- skipped object report,
- validation JSON,
- validation Markdown,
- sequence or auto-increment reconciliation report,
- final summary.

## Safety requirements

- Dry-run remains the default.
- Source operations remain read-only by default.
- Target high-risk actions require explicit approval.
- Secrets must never appear in logs or reports.
- Target fingerprint is stored before execution.
- Resume must verify plan and schema hashes.
- Validation problems block promotion.

## Compatibility rule

Existing commands must continue to work during refactoring:

```bash
qantaradb inspect
qantaradb plan
qantaradb migrate
qantaradb validate
qantaradb resume
qantaradb report
qantaradb foodtech-preflight
```

The command layer should gradually become a thin wrapper over `engine/`.
