#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  SOURCE_DSN='mysql://root@127.0.0.1:3306/foodtech_test' \
  TARGET_DSN='postgresql://qantara_local@127.0.0.1:55432/foodtech_qantara_console_test' \
  ./scripts/pgloader_local_benchmark.sh

Options:
  --source-dsn VALUE       MySQL source DSN. Defaults to local foodtech_test.
  --target-dsn VALUE       PostgreSQL target DSN. Defaults to local 55432 test DB.
  --source-schema VALUE    MySQL schema name to rename to public. Defaults to foodtech_test.
  --report-dir VALUE       Report directory. Defaults to storage/reports.
  --allow-non-local        Permit non-local DSN hosts. Do not use without approval.
  -h, --help               Show this help.

The script does not print DSNs or passwords. Runtime files are written to the
report directory, which is git-ignored by this repository.
USAGE
}

allow_non_local=0
report_dir="${REPORT_DIR:-storage/reports}"
source_dsn="${SOURCE_DSN:-mysql://root@127.0.0.1:3306/foodtech_test}"
target_dsn="${TARGET_DSN:-postgresql://qantara_local@127.0.0.1:55432/foodtech_qantara_console_test}"
source_schema="${SOURCE_SCHEMA:-foodtech_test}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-dsn)
      source_dsn="${2:?Missing value for --source-dsn}"
      shift 2
      ;;
    --target-dsn)
      target_dsn="${2:?Missing value for --target-dsn}"
      shift 2
      ;;
    --source-schema)
      source_schema="${2:?Missing value for --source-schema}"
      shift 2
      ;;
    --report-dir)
      report_dir="${2:?Missing value for --report-dir}"
      shift 2
      ;;
    --allow-non-local)
      allow_non_local=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command not found: $1" >&2
    exit 127
  fi
}

extract_host() {
  local dsn="$1"
  local host=""

  if [[ "$dsn" =~ ^[A-Za-z][A-Za-z0-9+.-]*://([^/@]+@)?(\[[^]]+\]|[^/:?]+) ]]; then
    host="${BASH_REMATCH[2]}"
    host="${host#[}"
    host="${host%]}"
    printf '%s\n' "$host"
    return 0
  fi

  if [[ "$dsn" =~ (^|[[:space:]])host=([^[:space:]]+) ]]; then
    printf '%s\n' "${BASH_REMATCH[2]}"
    return 0
  fi

  printf '\n'
}

is_local_host() {
  case "$1" in
    ""|"localhost"|"127.0.0.1"|"::1")
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

guard_local_dsn() {
  local label="$1"
  local dsn="$2"
  local host
  host="$(extract_host "$dsn")"

  if [[ "$allow_non_local" -eq 1 ]]; then
    return 0
  fi

  if ! is_local_host "$host"; then
    echo "Refusing non-local ${label} DSN host. Use --allow-non-local only with explicit approval." >&2
    exit 3
  fi
}

require_command pgloader
require_command psql

guard_local_dsn "source" "$source_dsn"
guard_local_dsn "target" "$target_dsn"

if [[ ! "$source_schema" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo "Invalid source schema name." >&2
  exit 4
fi

mkdir -p "$report_dir"
chmod 700 "$report_dir"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
runtime_load="${report_dir}/pgloader-runtime-${timestamp}.load"
log_file="${report_dir}/pgloader-runtime-${timestamp}.log"
summary_file="${report_dir}/pgloader-runtime-${timestamp}.summary.md"

cat > "$runtime_load" <<EOF_LOAD
LOAD DATABASE
     FROM ${source_dsn}
     INTO ${target_dsn}

WITH include drop,
     create tables,
     create indexes,
     reset sequences,
     foreign keys,
     workers = 4,
     concurrency = 2,
     batch rows = 1000,
     prefetch rows = 1000

EXCLUDING TABLE NAMES MATCHING
     ~/^sessions$/,
     ~/^failed_jobs$/,
     ~/^temp_debug_log$/

ALTER SCHEMA '${source_schema}' RENAME TO 'public'

CAST
     type datetime to timestamptz drop default drop not null using zero-dates-to-null,
     type date drop default drop not null using zero-dates-to-null,
     type bigint when unsigned to bigint drop typemod,
     type bigint to bigint drop typemod,
     type int when unsigned to bigint drop typemod,
     type int to integer drop typemod,
     type tinyint when (= precision 1) to smallint drop typemod

SET PostgreSQL PARAMETERS
     maintenance_work_mem to '512MB',
     work_mem to '64MB';
EOF_LOAD
chmod 600 "$runtime_load"

start_epoch="$(date +%s)"
set +e
pgloader "$runtime_load" >"$log_file" 2>&1
status=$?
set -e
end_epoch="$(date +%s)"
elapsed="$((end_epoch - start_epoch))"

SOURCE_DSN="$source_dsn" TARGET_DSN="$target_dsn" perl -0pi -e '
  s/\Q$ENV{SOURCE_DSN}\E/[REDACTED_SOURCE_DSN]/g if length $ENV{SOURCE_DSN};
  s/\Q$ENV{TARGET_DSN}\E/[REDACTED_TARGET_DSN]/g if length $ENV{TARGET_DSN};
' "$log_file"

if grep -Eq '(^|[[:space:]])(FATAL|ERROR)([[:space:]]|$)' "$log_file"; then
  status=1
fi

{
  echo "# pgloader Local Benchmark Summary"
  echo
  echo "- status: ${status}"
  echo "- elapsed_seconds: ${elapsed}"
  echo "- source_host: $(extract_host "$source_dsn")"
  echo "- target_host: $(extract_host "$target_dsn")"
  echo "- source_schema: ${source_schema}"
  echo "- runtime_load: ${runtime_load}"
  echo "- log_file: ${log_file}"
  echo
  echo "QantaraDB validation must be run after this migration before any staging decision."
} > "$summary_file"

echo "pgloader local benchmark finished with status ${status}."
echo "Summary: ${summary_file}"
echo "Log: ${log_file}"

exit "$status"
