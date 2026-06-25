#!/bin/bash
# QantaraDB Docker End-to-End Test Suite 🚀
# This script spins up the test infrastructure, compiles QantaraDB,
# and executes the full migration and validation pipeline.

set -e

echo "========================================================="
echo "🟢 [1/5] Starting E2E Docker Compose Infrastructure"
echo "========================================================="
docker compose up -d --build

# Wait for MySQL to become ready
echo -n "Waiting for MySQL to boot..."
until docker exec $(docker compose ps -q mysql) mysqladmin ping -h 127.0.0.1 -u root --password=secret --silent &>/dev/null; do
    echo -n "."
    sleep 2
done
echo " Ready!"

# Wait for PostgreSQL to become ready
echo -n "Waiting for PostgreSQL to boot..."
until docker exec $(docker compose ps -q postgres) pg_isready -U postgres -h 127.0.0.1 &>/dev/null; do
    echo -n "."
    sleep 2
done
echo " Ready!"

echo "========================================================="
echo "🟢 [2/5] Compiling QantaraDB Go Binary"
echo "========================================================="
go build -o bin/qantaradb cmd/qantaradb/main.go
echo "Compilation successful: ./bin/qantaradb"

echo "========================================================="
echo "🟢 [3/5] Running FoodTech Compatibility Preflight"
echo "========================================================="
./bin/qantaradb foodtech-preflight \
  --mysql "root:secret@tcp(127.0.0.1:3307)/foodtech_production" \
  --out reports/postgres-readiness.md

echo "========================================================="
echo "🟢 [4/5] Executing Migration Schema & Data Load"
echo "========================================================="
./bin/qantaradb migrate --config config.yaml

echo "========================================================="
echo "🟢 [5/5] Conducting Post-Migration Validation Auditor"
echo "========================================================="
./bin/qantaradb validate --config config.yaml --out reports/validation.md

echo "========================================================="
echo "✅ E2E Migration Pipeline Completed Successfully!"
echo "Check output logs and Markdown summaries inside 'reports/'"
echo "========================================================="
