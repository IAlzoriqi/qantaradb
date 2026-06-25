# QantaraDB Usage Guide 📖

This guide provides a comprehensive overview of how to integrate the **QantaraDB** library into your Go applications or use it via the command-line interface (CLI) to migrate database schemas and data from MySQL/MariaDB to PostgreSQL.

---

## 🛠️ Step-by-Step Migration Pipeline

Migrating a database in production is a highly delicate task. QantaraDB breaks down this process into five discrete phases to ensure zero data loss, minimal downtime, and complete structural integrity:

```text
+-----------------------+      +-------------------+      +------------------+
| 1. FoodTech Preflight | ---> | 2. Schema Inspect | ---> | 3. Planning      |
+-----------------------+      +-------------------+      +------------------+
                                                                   |
+-----------------------+      +-------------------+               |
| 5. Post-Validation    | <--- | 4. Resume-Stream  | <-------------+
+-----------------------+      +-------------------+
```

---

### Phase 1: PostgreSQL Readiness Preflight Check
The preflight engine analyzes your MySQL read-only replica schema to flag incompatibility risks with PostgreSQL (e.g., PHP/Laravel conventions, reserved words, unsigned auto-increment keys, spatial geometry types, MySQL-specific ENUM structures).

**CLI Command:**
```bash
qantaradb foodtech-preflight \
  --mysql "root:secret@tcp(127.0.0.1:3306)/foodtech_prod" \
  --out reports/postgres-readiness.md
```

**Go Library API:**
```go
package main

import (
	"fmt"
	"os"
	"github.com/OWNER/qantaradb/preflight"
)

func main() {
	mysqlDSN := "root:secret@tcp(127.0.0.1:3306)/foodtech_prod"
	
	report, err := preflight.Run(mysqlDSN)
	if err != nil {
		fmt.Printf("Preflight error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Preflight finished. Found %d compatibility warnings.\n", len(report.Risks))
}
```

---

### Phase 2: Structural Introspection
In this phase, QantaraDB inspects the catalog of your source database, gathering metadata for tables, columns, indexes, foreign keys, and views.

**CLI Command:**
```bash
qantaradb inspect \
  --mysql "root:secret@tcp(127.0.0.1:3306)/foodtech_prod" \
  --out reports/inspect.json
```

---

### Phase 3: Compiling the Migration Plan
The planning module performs **Kahn's Topological Sort** to order your tables based on their foreign key dependencies. This guarantees that parent tables are created and loaded *before* child tables, eliminating relational constraint violations.

**CLI Command:**
```bash
qantaradb plan \
  --inspect reports/inspect.json \
  --out reports/plan.json
```

---

### Phase 4: Bulk Copying & Migration State Tracking
The core data loader runs in multiple concurrent workers using the **PostgreSQL Copy protocol** (`pgx` bulk `CopyFrom`). Instead of pulling complete tables into memory, it streams records in manageable chunks by primary key ranges. Progress is tracked incrementally in a local file so migrations can be resumed if a network interruption occurs.

**CLI Command:**
```bash
qantaradb migrate --config config.yaml
```

If the connection is severed during migration, use the `resume` command to pick up from the last successfully written primary key chunk:

```bash
qantaradb resume --config config.yaml
```

**YAML Configuration File (`config.yaml`):**
```yaml
source_dsn: "root:secret@tcp(127.0.0.1:3306)/foodtech_prod"
target_dsn: "postgres://postgres:secret@127.0.0.1:5432/postgres_target?sslmode=disable"
excludes:
  - failed_jobs
  - password_resets
  - personal_access_tokens

mapper:
  tinyint1_as_bool: true
  datetime_timezone_policy: "utc"
  geometry_postgis_mode: true
  enum_as_domain: true

loader:
  batch_size: 5000
  workers_count: 4
  state_file_path: "reports/migration_state.json"

out_report_json: "reports/migration_report.json"
out_report_md: "reports/migration_report.md"
allow_destructive_production_operations: false
```

---

### Phase 5: Post-Migration Audit & Validation
To confirm a successful migration, the validation auditor performs:
1. **Row-Count Parity check** across all tables.
2. **Min/Max PK matches** to detect any data truncation.
3. **Chunk-based cryptographic checksum audits** (SHA-256) on selected blocks.
4. **Foreign Key referential integrity audits** to identify orphan records.
5. **Arabic and Emoji multi-byte transcoded text checks** to verify characters are not corrupted.
6. **Sequence synchronization** (resetting PK auto-increment identifiers).

**CLI Command:**
```bash
qantaradb validate --config config.yaml --out reports/validation.md
```

---

## 🔒 Production Best Practices

1. **Connect to Read-Only Replica**: Always point QantaraDB's source connection to a read-only MySQL replica. This keeps migration overhead off your primary database.
2. **Disable Non-Essential Indexes**: Target indexes and foreign key constraints are temporarily disabled during bulk-copy operations by QantaraDB and rebuilt upon completion, speeding up loads by up to 10x.
3. **Use Safe Buffers**: Run migrations in dedicated networks or VPCs to limit connection drops and ensure maximum throughput for high-performance chunk streaming.
