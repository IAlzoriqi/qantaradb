# FoodTech Database Migration Runbook: Laravel/PHP to PostgreSQL 🍔

Most FoodTech backends (e.g., Delivery Platforms, POS integrations, Kitchen Management systems) are built on PHP/Laravel using MySQL. Transitioning your core database to PostgreSQL unlocks higher concurrency, superior JSON query speeds, and robust geospatial capabilities (PostGIS).

This runbook details specific hazards, precautions, and a step-by-step guide to executing a successful migration using **QantaraDB**.

---

## ⚠️ FoodTech & Laravel Architectural Hazards

### 1. Reserved Identifier Collations (`user`, `order`, `group`)
Laravel schemas often create tables named `users` or columns named `order`, `group`. 
* **The Risk**: In PostgreSQL, `USER`, `ORDER`, and `GROUP` are highly reserved SQL keywords. Writing query statements like `SELECT id, order FROM users` will trigger syntax exceptions in Postgres.
* **The Solution**: Ensure your Laravel ORM (Eloquent) is configured to automatically quote identifiers. Eloquent's Postgres driver does this by default (compiling to `SELECT "id", "order" FROM "users"`). QantaraDB's compiler automatically double-quotes all table and column names in the generated PostgreSQL DDL.

---

### 2. Unsigned Primary Key Conversions
Laravel migrations typically use `$table->id()` which compiles to `BIGINT UNSIGNED AUTO_INCREMENT` in MySQL, whereas PostgreSQL defaults to signed `BIGINT` with sequences.
* **The Risk**: Unsigned bigint values can exceed `9,223,372,036,854,775,807` (signed bigint limit).
* **The Solution**: QantaraDB automatically analyzes the actual maximum values of your primary keys. If they are within the 9.2 Quintillion signed limit, it maps them to Postgres `BIGINT` and synchronizes Postgres sequences. If there's risk of overflow, it falls back to `NUMERIC(20,0)`.

---

### 3. Multi-byte Arabic and Emoji Integrity
Customer reviews, meal descriptions, and delivery address feedback in FoodTech applications frequently utilize **Arabic text** and **Emoji symbols**.
* **The Risk**: MySQL's older default charset was `utf8` (which only supports 3 bytes, causing emojis to crash or get truncated). To support Emojis, MySQL uses `utf8mb4`. If the migration tool does not handle multi-byte encodings properly, text might get transcoded into garbled unicode replacement symbols (e.g., `` or `??`).
* **The Solution**: QantaraDB audits both source and target database encodings. It enforces `UTF-8` client encoding during the copy protocol and verifies that no replacement characters (`\uFFFD`) exist in the migrated target string columns, ensuring Arabic characters and emojis are 100% preserved.

---

## 🏃 Step-by-Step Migration and Cutover Plan

Follow this checklist for a safe, low-downtime cutover:

### Step 1: Pre-Migration Prep
1. Provision a PostgreSQL read-replica or standalone database with identical memory resources as your MySQL primary.
2. Spin up a read-replica for your source MySQL database (QantaraDB will stream from this replica to avoid affecting production web servers).
3. Ensure the PostGIS extension is installed on PostgreSQL if your FoodTech app stores geospatial coordinates (e.g. restaurant locations, delivery geofences).

### Step 2: Compatibility Audit
Run the preflight command to discover compatibility warnings and review remedies:
```bash
qantaradb foodtech-preflight \
  --mysql "root:secret@tcp(mysql-replica.prod:3306)/foodtech_production" \
  --out reports/postgres-readiness.md
```

### Step 3: Run the Dry-run Migration
Execute a sample migration into a staging/test PostgreSQL database to verify timing and compile type mappings:
```bash
qantaradb migrate --config config-staging.yaml
```

### Step 4: Full Integrity Validation
Validate the staging database to confirm counts and check for data drift:
```bash
qantaradb validate --config config-staging.yaml --out reports/validation-staging.md
```

### Step 5: Laravel Configuration Update
In your Laravel application codebase, update `config/database.php` to define the PostgreSQL connection:

```php
'connections' => [
    'pgsql' => [
        'driver' => 'pgsql',
        'host' => env('DB_HOST', '127.0.0.1'),
        'port' => env('DB_PORT', '5432'),
        'database' => env('DB_DATABASE', 'forge'),
        'username' => env('DB_USERNAME', 'forge'),
        'password' => env('DB_PASSWORD', ''),
        'charset' => 'utf8',
        'prefix' => '',
        'schema' => 'public',
        'sslmode' => 'prefer',
    ],
],
```

---

### Step 6: Live Cutover Window
During your off-peak hours (e.g., 3:00 AM - 4:00 AM):
1. **Enable Maintenance Mode** in Laravel:
   ```bash
   php artisan down --secret="cutover-key"
   ```
2. **Execute Final QantaraDB Migration** from MySQL Primary to PostgreSQL Production (with `--force-destructive-production-drop` if schemas are being refreshed):
   ```bash
   qantaradb migrate --config config-production.yaml
   ```
3. **Run Validation & Sequence Sync**:
   ```bash
   qantaradb validate --config config-production.yaml --out reports/validation-prod.md
   ```
4. **Update Production Environment Variables** (`.env`):
   ```env
   DB_CONNECTION=pgsql
   DB_HOST=postgres-prod.internal
   DB_PORT=5432
   DB_DATABASE=foodtech_prod
   DB_USERNAME=laravel_user
   DB_PASSWORD=secure_postgres_pass
   ```
5. **Clear Cache & Warmup**:
   ```bash
   php artisan config:cache
   ```
6. **Bring Application Online**:
   ```bash
   php artisan up
   ```
7. Monitor application error logs for any unrecognized queries or casting issues.
