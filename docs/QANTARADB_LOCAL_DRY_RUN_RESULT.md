# QantaraDB Local Dry Run Result

status:
passed

source:
local mysql

target:
local postgres

tables_detected:
946

tables_supported:
944

tables_blocked:
- `sessions`
- `failed_jobs`

type_mapping_issues:
- `tinyint(1)` to boolean is not safe globally for FoodTech legacy data because Go MySQL values arrive as numeric values during CopyFrom.
- `enum` inline CHECK constraints are unsafe globally because legacy MySQL data may contain empty or non-canonical values.

foreign_key_issues:
- 114 foreign-key groups detected for PostgreSQL DDL.

estimated_rows:
770074 copied in the local execution that followed dry-run.

ready_for_local_execute:
yes

notes:
- Dry-run generated `reports/qantaradb_dry_run_report.json`.
- Generated reports are ignored by Git because they are local environment artifacts.
