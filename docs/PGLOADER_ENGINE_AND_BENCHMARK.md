# pgloader Engine and Benchmark

## purpose

- pgloader is an optional fast migration and benchmark engine for local MySQL to PostgreSQL trials.
- QantaraDB remains the governance and validation engine for reports, checksums, sequence reset checks, sanitized row evidence, and staging gates.
- pgloader output is never trusted by itself.

## local use

Use the local runner only against local databases:

```bash
SOURCE_DSN='mysql://root@127.0.0.1:3306/foodtech_test' \
TARGET_DSN='postgresql://qantara_local@127.0.0.1:55432/foodtech_qantara_console_test' \
SOURCE_SCHEMA='foodtech_test' \
./scripts/pgloader_local_benchmark.sh
```

The runner writes local generated files and logs under `storage/reports`, which is git-ignored.

## comparison_metrics

- elapsed time
- tables migrated
- rows migrated
- rejected rows
- sequence reset result
- FK integrity
- QantaraDB checksum validation after pgloader migration
- Console PostgreSQL validation result

## decision_rule

- pgloader output is not trusted alone.
- staging remains blocked until QantaraDB validation passes and Console PostgreSQL validation passes.
- If pgloader is faster but QantaraDB validation reports real mismatches, the migration is considered failed.
- If QantaraDB validation passes but Console PostgreSQL validation fails, staging remains blocked.

## current default mapping

The example load file maps MySQL `tinyint(1)` to PostgreSQL `smallint` by default. It does not apply a global boolean mapping because FoodTech legacy tables may use `tinyint(1)` as numeric state, flags, or compatibility fields. Boolean conversion requires a separate documented mapping decision.
