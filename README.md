# QantaraDB 🚀
> MySQL / MariaDB to PostgreSQL High-Performance, Resumable Migration Library & CLI

QantaraDB is a production-ready, open-source Go library and CLI designed to migrate MySQL and MariaDB databases to PostgreSQL. It features robust, automated schema conversion, PK-based chunk streaming copy, multi-worker parallel loads, resumable progress tracking, data integrity validation reports, and a specialized **FoodTech Preflight Engine** to flag Laravel/PHP and MySQL compatibility risks.

---

## Key Features

- 🔍 **In-Depth Inspector**: Automatically extracts MySQL tables, columns, indexes, foreign keys, auto-increment specs, charsets, and views.
- 🗺️ **Intelligent Type Mapper**: Configurable type conversion (e.g., `tinyint(1)` as `boolean`, `unsigned bigint` as safe `numeric(20,0)` to avoid sign overflows, native ENUM/SET mapping, datetime timezones, and optional PostGIS geometries).
- ⛓️ **Dependency Planner**: Performs Kahn's topological sort on tables based on foreign key relationships, ensuring parents are created and populated before children (zero foreign key constraint failures).
- 🚀 **Blazing Fast Copy Engine**: Uses PostgreSQL `pgx` bulk `CopyFrom` protocol. Avoids loading whole tables into memory by chunking tables using Primary Keys.
- ⏯️ **Resumable Migration State**: Keeps track of migration progress inside a states file. If the migration fails halfway (e.g., network drop), restarting the tool will seamlessly resume exactly from the last processed PK chunk.
- 🧪 **Schema Parity & Validation**: Cross-verifies row counts, min/max PK values, null structures, and constraint compliance. Produces detailed markdown summaries.
- 🍔 **FoodTech Preflight Mode**: Built-in rules engine tailored for Laravel/PHP and generic FoodTech application schemas. Automatically flags reserved PostgreSQL words (like `user`, `group`), MySQL-only view definitions (using CASE/IF), unsigned primary key mismatches, and JSON casting risks.

---

## Project Layout

```text
qantaradb/
├── cmd/
│   └── qantaradb/        # CLI Command Router
├── inspector/            # MySQL schema introspection
├── planner/              # Migration topological order & plan generator
├── mapper/               # Type mapping configurations
├── ddl/                  # PostgreSQL DDL compiler
├── loader/               # Resumable, chunk-based copy engine
├── validator/            # Post-migration data validation auditor
├── preflight/            # FoodTech-specific readiness check
└── report/               # Markdown/JSON report generator
```

---

## Installation

Ensure you have **Go 1.21+** installed:

```bash
cd qantaradb
go build -o bin/qantaradb cmd/qantaradb/main.go
```

---

## Getting Started

### 1. Run FoodTech Preflight Checks
Before doing anything, analyze your FoodTech read-only replica MySQL database for PostgreSQL compatibility hazards:

```bash
./bin/qantaradb foodtech-preflight \
  --mysql "root:secret@tcp(127.0.0.1:3306)/foodtech_production" \
  --out reports/postgres-readiness.md
```

### 2. Inspect Source Schema
Generate a complete JSON breakdown of your source database metadata:

```bash
./bin/qantaradb inspect \
  --mysql "root:secret@tcp(127.0.0.1:3306)/foodtech_production" \
  --out reports/inspect.json
```

### 3. Generate Migration Plan
Generate a customizable type mapping and order list:

```bash
./bin/qantaradb plan \
  --inspect reports/inspect.json \
  --out reports/plan.json
```

### 4. Run Migration
Use a YAML configuration file to run the schema mapping, streaming copying, and sequence synchronization:

```bash
./bin/qantaradb migrate --config config.yaml
```

### 5. Validate Integrity
Check row counts and key drift, reset sequences, and generate markdown auditing proofs:

```bash
./bin/qantaradb validate --config config.yaml --out reports/validation.md
```

---

## Type Mapping Policy

| MySQL / MariaDB Type | PostgreSQL Target Type | Config/Conditions |
| :--- | :--- | :--- |
| `tinyint(1)` | `boolean` or `smallint` | `tinyint1_as_bool: true` |
| `bigint unsigned` | `numeric(20,0)` | Avoids signed-overflow errors |
| `json` | `jsonb` | High performance JSON indexing |
| `enum(...)` | `text` with `CHECK (col IN (...))` | Safe and lightweight mapping |
| `set(...)` | `text[]` or `text` | Configurable |
| `datetime` / `timestamp` | `timestamp with/without time zone` | `datetime_timezone_policy` configuration |
| `geometry` | `geometry` (PostGIS) or `bytea` (WKB) | `geometry_postgis_mode` |

---

## Safety Guidelines & Policy

1. **Read-Only Source**: The library opens connections with read-only operations for the source database. It will NEVER execute writes or alter tables on your source MySQL database.
2. **Target Drop Protection**: Drop target tables only if target database name prefix matches `qantaradb_test_*` AND the `--allow-drop-target` flag is explicitly passed to prevent accidental loss of production postgres instances.
3. **No Hardcoded Credentials**: DSNs and secrets should be provided via OS environment variables or passed securely. Never commit files containing database credentials.

---

## License

This library is open-source and licensed under the **Apache 2.0 License**.
