# QantaraDB Type Mapping & Translation Engine 🗺️

When migrating from MySQL/MariaDB to PostgreSQL, mapping types accurately is crucial. Differences in numerical signs, date behaviors, geometries, and array structures require a comprehensive, automated translation engine.

---

## 📋 Comprehensive Type Translation Matrix

The table below outlines QantaraDB's default mapping strategies, rules, and optional configurations:

| MySQL / MariaDB Type | PostgreSQL Target Type | Default Translation Strategy / Options |
| :--- | :--- | :--- |
| **`tinyint(1)`** | `boolean` | Translates to PostgreSQL `boolean`. Configure with `tinyint1_as_bool: false` to map to `smallint` instead. |
| **`tinyint` / `smallint`** | `smallint` | Straightforward mapping. |
| **`mediumint` / `int`** | `integer` | Fits within PostgreSQL 32-bit signed integer. |
| **`bigint`** | `bigint` | Fits within PostgreSQL 64-bit signed integer. |
| **`bigint unsigned`** | `numeric(20,0)` | **Crucial Safety Mapping**: MySQL unsigned bigint caps at `18,446,744,073,709,551,615`, which exceeds PostgreSQL signed bigint limit of `9,223,372,036,854,775,807`. Maps to `numeric(20,0)` to prevent sign overflows. |
| **`decimal(p, s)`** | `numeric(p, s)` | Exact numeric representation, preserved perfectly. |
| **`float`** | `real` | Single-precision floating point. |
| **`double`** | `double precision` | Double-precision floating point. |
| **`varchar(N)`** | `varchar(N)` | Character-varying text. |
| **`char(N)`** | `char(N)` or `text` | Preserved as character string. |
| **`text` / `mediumtext` / `longtext`** | `text` | PostgreSQL supports unbounded `text` columns efficiently. |
| **`json`** | `jsonb` | **High Performance Optimization**: Converted to `jsonb` (binary JSON) in PostgreSQL to support fast GIN indexing and rich query functions. |
| **`enum('a','b')`** | `text` with `CHECK (col IN ('a','b'))` | Converted to `text` with inline CHECK constraints. If `enum_as_domain` is enabled, compiles a custom PostgreSQL `CREATE DOMAIN` type. |
| **`set('x','y')`** | `text[]` or `text` | Configurable to map to native Postgres arrays or comma-separated text. |
| **`blob` / `mediumblob` / `longblob`** | `bytea` | Binary string format. |
| **`datetime` / `timestamp`** | `timestamp [tz]` | Configurable via `datetime_timezone_policy`. Mapped to `timestamp with time zone` (timestamptz) or `timestamp without time zone` based on config. |
| **`geometry` / `point` / `polygon`** | `geometry` or `bytea` | Configurable via `geometry_postgis_mode`. Maps to native PostGIS geometries if enabled; falls back to WKB binary (`bytea`) otherwise. |

---

## 🛡️ Critical Edge Cases & Workarounds

### 1. Unsigned Integers vs. Signed Integers
PostgreSQL does not natively support `unsigned` integer types. An `int unsigned` column in MySQL can hold values up to `4,294,967,295`. 
* **MySQL `int unsigned`** -> Maps to **PostgreSQL `bigint`** (since standard 32-bit signed integer only goes to `2,147,483,647`).
* **MySQL `bigint unsigned`** -> Maps to **PostgreSQL `numeric(20,0)`** (since standard 64-bit signed bigint only goes to `9,223,372,036,854,775,807`, whereas unsigned bigints can go up to `18,446,744,073,709,551,615`). This prevents numeric overflows.

---

### 2. Zero-Date Handling (`0000-00-00 00:00:00`)
MySQL allows dates with zero values, which are strictly illegal in PostgreSQL. If a row contains `'0000-00-00 00:00:00'`, PostgreSQL will throw an out-of-range error.

**QantaraDB Remediation**:
* If the column permits `NULL` values, the zero-date is converted to **`NULL`**.
* If the column is defined as `NOT NULL`, the date falls back to the **Unix Epoch (`1970-01-01 00:00:00 UTC`)** to prevent copy crashes.

---

### 3. Enum Constraints
MySQL native `ENUM` values are lightweight inside MySQL tables, but adding/removing items requires heavy ALTER TABLE table copy.

**QantaraDB Remediation**:
QantaraDB offers two mapping styles:
1. **CHECK Constraints (Default)**: Columns are mapped as `text` with a check constraint, e.g., `CHECK (status IN ('pending', 'processing', 'completed'))`. This is highly compatible and allows adding values easily.
2. **Domain/Postgres Enum Types (`enum_as_domain: true`)**: Generates a reusable user-defined enum or domain, e.g., `CREATE TYPE t_status AS ENUM ('pending', 'processing', 'completed')`.

---

### 4. Spatial Coordinates and Geometries
FoodTech applications often store delivery locations using MySQL's native `POINT` or `GEOMETRY` classes.

**QantaraDB Remediation**:
* **PostGIS Mode Enabled (`geometry_postgis_mode: true`)**: Compiles into PostgreSQL `geometry(Point, 4326)` columns.
* **PostGIS Mode Disabled (`geometry_postgis_mode: false`)**: Converts the spatial structures into Well-Known Binary (WKB) and stores them inside `bytea` columns to ensure compatibility even without extension support.
