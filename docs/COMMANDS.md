# QantaraDB CLI Command Reference 💻

This document lists all command-line operations, options, exit codes, and parameters supported by the **QantaraDB** CLI binary.

---

## 🏛️ Command Summary

The CLI uses the following syntax:
```bash
qantaradb <subcommand> [flags]
```

| Subcommand | Purpose | Required Inputs | Output |
| :--- | :--- | :--- | :--- |
| **`foodtech-preflight`** | Audit compatibility with PostgreSQL | MySQL DSN | Markdown Risk Report |
| **`inspect`** | Introspect schema structure and database catalog | MySQL DSN | JSON Catalog Info |
| **`plan`** | Compute dependency sorting and type mappings | JSON Catalog | JSON Migration Plan |
| **`migrate`** | Run complete schema + bulk table streaming copy | YAML Config | PostgreSQL Target DB |
| **`resume`** | Resume from a partially completed migration | YAML Config + State | PostgreSQL Target DB |
| **`validate`** | Post-migration row counts, checksums, and FK audits | YAML Config | Markdown Validation Report |
| **`report`** | Generate combined final migration report | YAML Config | JSON + MD Summary Reports |

---

## 🔍 Subcommands Detail

### 1. `foodtech-preflight`
Runs the preflight analyzer on a source MySQL database. It does not write to either database.

**Flags:**
* `--mysql` (string, required): Data Source Name (DSN) for the source MySQL database.
* `--out` (string, optional, default: `reports/postgres-readiness.md`): Output path for the compatibility report.

**Example:**
```bash
qantaradb foodtech-preflight \
  --mysql "user:password@tcp(127.0.0.1:3306)/foodtech_prod" \
  --out /tmp/preflight.md
```

---

### 2. `inspect`
Queries the MySQL `INFORMATION_SCHEMA` tables to fetch column types, sizes, indexes, and primary keys.

**Flags:**
* `--mysql` (string, required): MySQL connection DSN.
* `--out` (string, optional, default: `reports/inspect.json`): Filepath to write the catalog metadata JSON.

**Example:**
```bash
qantaradb inspect \
  --mysql "user:password@tcp(127.0.0.1:3306)/foodtech_prod" \
  --out /tmp/inspect.json
```

---

### 3. `plan`
Accepts an inspected database JSON file and maps all types to PostgreSQL targets, performing topological sort to compile the execution plan.

**Flags:**
* `--inspect` (string, required): Path to the inspected catalog JSON file.
* `--out` (string, optional, default: `reports/plan.json`): Filepath to save the computed migration plan.

**Example:**
```bash
qantaradb plan --inspect /tmp/inspect.json --out /tmp/plan.json
```

---

### 4. `migrate`
Performs the migration. This creates schemas, indices, and uses high-speed parallel workers to bulk copy rows.

**Flags:**
* `--config` (string, optional, default: `config.yaml`): Path to the YAML configuration file.
* `--force-destructive-production-drop` (boolean, optional): Override and bypass drop protection when migrating to a production database that does not have `test` or `dev` in its DSN name.

**Example:**
```bash
qantaradb migrate --config config.yaml --force-destructive-production-drop
```

---

### 5. `resume`
Checks for a partial checkpoint record inside `reports/migration_state.json`. Resumes copy operations for remaining chunks without repeating tables or rows that have already completed.

**Flags:**
* `--config` (string, optional, default: `config.yaml`): Path to the YAML configuration file.

**Example:**
```bash
qantaradb resume --config config.yaml
```

---

### 6. `validate`
Runs a robust integrity check. Compares MySQL and PostgreSQL row tallies, calculates 100-row sample SHA-256 block checksums, reports orphan foreign-key rows, and checks if multi-byte Arabic and Emoji characters were successfully preserved.

**Flags:**
* `--config` (string, optional, default: `config.yaml`): Path to the YAML configuration file.
* `--out` (string, optional, default: `reports/validation.md`): Markdown auditing filepath.

**Example:**
```bash
qantaradb validate --config config.yaml --out reports/validation.md
```

---

## 🚨 Security & Destructive Drop Safety Gate

To prevent accidents, QantaraDB implements a strict, multi-layer **Drop Protection Gate**:

1. **Default Block**: If the target database DSN does not contain a test/dev indicator (e.g. `test`, `dev`, `sandbox`, `local`, `qa`), QantaraDB blocks execution of tables recreation.
2. **First Override**: Enable the yaml-level configuration property `allow_destructive_production_operations: true` to confirm you are targeting a production instance.
3. **CLI Override**: Run the command with the explicit CLI override: `--force-destructive-production-drop`.

**Failure output when safety gates are breached:**
```text
❌ CRITICAL SAFETY ERROR: Destructive operations (DROP/TRUNCATE) are blocked on non-test target database: postgres://prod-db.amazonaws.com/...
To migrate onto a production target, you must either:
  1. Pass the CLI override flag: --force-destructive-production-drop
  2. Set 'allow_destructive_production_operations: true' in config.yaml
```

---

## 🎯 Exit Codes

| Exit Code | Meaning | Solution |
| :---: | :--- | :--- |
| **`0`** | Success | Command completed successfully. |
| **`1`** | Critical Safety Failure / Config error | Safety gate triggered or config file could not be parsed. |
| **`2`** | Database Connection Error | Verify MySQL or PostgreSQL credentials and ports. |
| **`3`** | Migration Interruption | Network was dropped during copying. Use `resume` to retry. |
