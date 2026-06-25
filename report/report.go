package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/OWNER/qantaradb/validator"
)

type MigrationReport struct {
	StartTime          time.Time                   `json:"start_time"`
	EndTime            time.Time                   `json:"end_time"`
	DurationSeconds    float64                     `json:"duration_seconds"`
	SourceDatabase     string                      `json:"source_database"`
	TargetDatabase     string                      `json:"target_database"`
	TotalRowsMigrated  int64                       `json:"total_rows_migrated"`
	AvgRowsPerSecond   float64                     `json:"avg_rows_per_second"`
	TablesCount        int                         `json:"tables_count"`
	TablesPassed       int                         `json:"tables_passed"`
	Validation         *validator.ValidationReport `json:"validation"`
	Warnings           []string                    `json:"warnings"`
}

func GenerateReport(rep *MigrationReport, jsonPath, mdPath string) error {
	rep.DurationSeconds = rep.EndTime.Sub(rep.StartTime).Seconds()
	
	// Write JSON
	jsonData, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json report: %w", err)
	}
	if jsonPath != "" {
		if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write json report: %w", err)
		}
	}

	// Compile Markdown Report
	md := fmt.Sprintf(`# QantaraDB Migration Summary Report 🚀

**Date of Migration:** %s
**Migration Duration:** %.2fs
**Source Database (MySQL/MariaDB):** %s
**Target Database (PostgreSQL):** %s

---

## 📊 Executive Status

| Metric | Value | Status |
| :--- | :--- | :---: |
| **Total Tables Migrated** | %d | - |
| **Integrity Checks Passed** | %d / %d | %s |
| **Total Rows Streamed** | %d | - |
| **Avg Stream Speed** | %.2f rows/sec | - |
| **FK Constraints Integrity** | %s | %s |

`,
		rep.EndTime.Format("2006-01-02 15:04:05 MST"),
		rep.DurationSeconds,
		rep.SourceDatabase,
		rep.TargetDatabase,
		rep.TablesCount,
		rep.TablesPassed, rep.TablesCount,
		getStatusEmoji(rep.TablesPassed == rep.TablesCount),
		rep.TotalRowsMigrated,
		rep.AvgRowsPerSecond,
		getFKStatusText(rep.Validation.FKIntegrityPassed),
		getStatusEmoji(rep.Validation.FKIntegrityPassed),
	)

	// UTF-8 & Arabic/Emoji Integrity Audit Section
	utfAudit := rep.Validation.ArabicEmojiAudit
	md += "## 🌐 UTF-8mb4 / Arabic / Emoji Data Integrity Audit\n\n"
	md += "| Audit Parameter | Details | Status |\n"
	md += "| :--- | :--- | :---: |\n"
	md += fmt.Sprintf("| **Source Charset** | %s | - |\n", utfAudit.SourceCharset)
	md += fmt.Sprintf("| **Target Encoding** | %s | - |\n", utfAudit.TargetEncoding)
	md += fmt.Sprintf("| **Arabic Text Preserved** | Clean multi-byte text (utf8mb4 safe) | %s |\n", getStatusEmoji(utfAudit.ArabicTextMatch))
	md += fmt.Sprintf("| **Emojis Preserved** | High-plane UTF-8 emojis verified | %s |\n", getStatusEmoji(utfAudit.EmojiMatch))
	md += fmt.Sprintf("\n> **Audit Logs:** %s\n\n", utfAudit.Details)

	// Table validation counts & chunk checksum matrix
	md += "## 🏁 Table-by-Table Data Integrity & Checksum Matrix\n\n"
	md += "| Table Name | Source Rows | Target Rows | Count Status | PK Range Match | Chunk Checksum (SHA-256) | Overall Status |\n"
	md += "| :--- | :---: | :---: | :---: | :---: | :---: | :---: |\n"

	for _, tVal := range rep.Validation.TablesValidation {
		countMatchEmoji := "✅ Match"
		if !tVal.CountMatch {
			countMatchEmoji = "❌ Mismatch"
		}

		pkMatchStr := "✅ Match"
		if !tVal.PrimaryKeyMinMatch || !tVal.PrimaryKeyMaxMatch {
			pkMatchStr = "⚠️ Drift"
		}

		checksumStr := "✅ Verified"
		if tVal.SourceCount > 0 && !tVal.ChecksumMatch {
			checksumStr = "❌ Drift"
		} else if tVal.SourceCount == 0 {
			checksumStr = "∅ Empty"
		}

		statusStr := "💚 Passed"
		if !tVal.Passed {
			statusStr = "💔 Failed"
		}

		md += fmt.Sprintf("| `%s` | %d | %d | %s | %s | %s | %s |\n",
			tVal.TableName, tVal.SourceCount, tVal.TargetCount, countMatchEmoji, pkMatchStr, checksumStr, statusStr)
	}

	// Schema & Type Mapping Section
	md += "\n## 🗺️ Physical Schema & Type Mapping Audit\n\n"
	md += "The following type conversion mappings were successfully audited across all table structures:\n\n"
	md += "| Table Name | Column Name | Source Type (MySQL) | Target Type (PostgreSQL) | Constraints / Nullability |\n"
	md += "| :--- | :--- | :--- | :--- | :--- |\n"

	if len(rep.Validation.TypeMappings) == 0 {
		md += "| *No type mappings generated* | - | - | - | - |\n"
	} else {
		for _, m := range rep.Validation.TypeMappings {
			md += fmt.Sprintf("| `%s` | `%s` | `%s` | `%s` | `%s` |\n",
				m.TableName, m.ColumnName, m.SourceType, m.TargetType, m.Constraints)
		}
	}

	// Foreign key constraints failures
	if !rep.Validation.FKIntegrityPassed {
		md += "\n## ⛓️ Foreign Key Integrity Violations\n\n"
		md += "⚠️ **Warning:** The following orphan child rows violate PostgreSQL's strict referential constraints:\n\n"
		md += "| Constraint Name | Child Table | Column | Parent Table | Column | Orphan Rows Count |\n"
		md += "| :--- | :--- | :--- | :--- | :--- | :---: |\n"
		for _, v := range rep.Validation.FKViolations {
			md += fmt.Sprintf("| `%s` | `%s` | `%s` | `%s` | `%s` | %d |\n",
				v.ConstraintName, v.ChildTable, v.ChildColumn, v.ParentTable, v.ParentColumn, v.ViolationCount)
		}
	}

	// Limitations & Known Architectural Gaps
	md += `
## 📖 Architectural Limitations & Known Differences

When migrating from MySQL/MariaDB to PostgreSQL, please note the following behavioral and structural boundaries supported by QantaraDB:

1. **Collations & Sorting Order**: MySQL is typically case-insensitive by default (e.g. ` + "`" + `utf8mb4_unicode_ci` + "`" + `). PostgreSQL uses deterministic, case-sensitive collations by default. Query string comparisons on PostgreSQL will behave as case-sensitive unless explicit ` + "`" + `ILIKE` + "`" + ` or expression indexes are added.
2. **Proprietary Stored Procedures & Triggers**: Native MySQL stored procedures (` + "`" + `CREATE PROCEDURE` + "`" + `) and triggers (` + "`" + `CREATE TRIGGER` + "`" + `) are **not** automatically translated. These must be rewritten manually into PostgreSQL PL/pgSQL dialect.
3. **Fulltext Search Indexes**: MySQL full-text search indexes (` + "`" + `FULLTEXT` + "`" + `) are not natively mapped. It is recommended to replace them with PostgreSQL GIN indexes and ` + "`" + `to_tsvector()` + "`" + ` columns.
4. **Zero Date Handling**: MySQL permits invalid dates (e.g., ` + "`" + `0000-00-00 00:00:00` + "`" + `), which are strictly forbidden in PostgreSQL. QantaraDB automatically transforms zero dates into ` + "`" + `NULL` + "`" + ` or the Unix Epoch (` + "`" + `1970-01-01` + "`" + `) depending on column constraints.
5. **Spatial Geometry Types**: Mapped to PostGIS ` + "`" + `geometry` + "`" + ` where available. In non-PostGIS setups, spatial shapes will fallback to binary format (` + "`" + `bytea` + "`" + `).
`

	// Warnings Section
	if len(rep.Warnings) > 0 {
		md += "\n## ⚠️ Migration Alerts & Schema Warnings\n\n"
		for _, w := range rep.Warnings {
			md += fmt.Sprintf("- ⚠️ %s\n", w)
		}
	}

	md += "\n---\n*Report generated automatically by QantaraDB open-source MySQL-to-PostgreSQL migration library.*"

	if mdPath != "" {
		if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
			return fmt.Errorf("failed to write markdown report: %w", err)
		}
	}

	return nil
}

func getStatusEmoji(passed bool) string {
	if passed {
		return "🟢"
	}
	return "🔴"
}

func getFKStatusText(passed bool) string {
	if passed {
		return "Passed (All relations clean)"
	}
	return "Failed (Violated keys detected)"
}
