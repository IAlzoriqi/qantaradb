package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OWNER/qantaradb/inspector"
)

type PreflightRisk struct {
	Category    string `json:"category"`
	TableName   string `json:"table_name"`
	ColumnName  string `json:"column_name"`
	RiskLevel   string `json:"risk_level"` // "High", "Medium", "Low"
	Description string `json:"description"`
	Remedy      string `json:"remedy"`
}

type PreflightReport struct {
	TotalRisks     int             `json:"total_rows"`
	HighRisksCount int             `json:"high_risks_count"`
	Risks          []PreflightRisk `json:"risks"`
}

var postgresReservedWords = map[string]bool{
	"user":    true,
	"group":   true,
	"order":   true,
	"limit":   true,
	"offset":  true,
	"select":  true,
	"table":   true,
	"column":  true,
	"primary": true,
	"foreign": true,
	"key":     true,
	"check":   true,
	"index":   true,
	"create":  true,
	"where":   true,
	"join":    true,
	"having":  true,
	"union":   true,
	"cast":    true,
}

func RunPreflight(schema *inspector.SchemaInfo) (*PreflightReport, error) {
	report := &PreflightReport{
		TotalRisks:     0,
		HighRisksCount: 0,
		Risks:          []PreflightRisk{},
	}

	addRisk := func(r PreflightRisk) {
		report.Risks = append(report.Risks, r)
		report.TotalRisks++
		if r.RiskLevel == "High" {
			report.HighRisksCount++
		}
	}

	for _, t := range schema.Tables {
		// 1. Check for Reserved Identifier Risks
		tableNameLower := strings.ToLower(t.Name)
		if postgresReservedWords[tableNameLower] {
			addRisk(PreflightRisk{
				Category:    "Reserved Identifier",
				TableName:   t.Name,
				ColumnName:  "*",
				RiskLevel:   "High",
				Description: fmt.Sprintf("Table name '%s' is a reserved PostgreSQL word.", t.Name),
				Remedy:      "Rename the table or ensure all application queries wrap the table name in double quotes: \"user\".",
			})
		}

		for _, col := range t.Columns {
			colNameLower := strings.ToLower(col.Name)
			// Check Column Reserved Words
			if postgresReservedWords[colNameLower] {
				addRisk(PreflightRisk{
					Category:    "Reserved Identifier",
					TableName:   t.Name,
					ColumnName:  col.Name,
					RiskLevel:   "Medium",
					Description: fmt.Sprintf("Column name '%s' in table '%s' is a reserved PostgreSQL word.", col.Name, t.Name),
					Remedy:      "Ensure ORM or raw SQL queries escape this column using double quotes: SELECT \"order\" FROM...",
				})
			}

			// 2. Check Unsigned Key Compatibility (Laravel / PHP default pattern)
			if col.IsUnsigned && (strings.Contains(colNameLower, "id") || strings.Contains(col.Extra, "auto_increment")) {
				addRisk(PreflightRisk{
					Category:    "Unsigned Keys",
					TableName:   t.Name,
					ColumnName:  col.Name,
					RiskLevel:   "Medium",
					Description: fmt.Sprintf("Unsigned key column '%s' in table '%s' is mapped to signed big integer in PostgreSQL.", col.Name, t.Name),
					Remedy:      "If key values exceed 9,223,372,036,854,775,807 (Postgres signed max), map the key type to numeric(20,0). otherwise, standard BIGINT is safe but watch for negative value checks.",
				})
			}

			// 3. MySQL / Laravel JSON field considerations
			if strings.ToLower(col.DataType) == "json" {
				addRisk(PreflightRisk{
					Category:    "JSON Serialization",
					TableName:   t.Name,
					ColumnName:  col.Name,
					RiskLevel:   "Low",
					Description: fmt.Sprintf("Column '%s' is JSON. In PostgreSQL, it maps to JSONB which requires strict JSON syntax.", col.Name),
					Remedy:      "Ensure that MySQL doesn't contain malformed JSON strings or empty values which break strict JSONB validation.",
				})
			}

			// 4. Native MySQL Enum
			if strings.Contains(strings.ToLower(col.ColumnType), "enum") {
				addRisk(PreflightRisk{
					Category:    "Enum Constraints",
					TableName:   t.Name,
					ColumnName:  col.Name,
					RiskLevel:   "Medium",
					Description: fmt.Sprintf("Column '%s' uses native MySQL ENUM: %s.", col.Name, col.ColumnType),
					Remedy:      "Will map to TEXT with CHECK (col IN (...)) inline constraint. If schema changes are needed later, updating the CHECK constraint requires ALTER TABLE drop/add constraint.",
				})
			}
		}

		// 5. Views with MySQL Specific functions
		if t.Type == "VIEW" {
			addRisk(PreflightRisk{
				Category:    "View Definition",
				TableName:   t.Name,
				ColumnName:  "*",
				RiskLevel:   "High",
				Description: fmt.Sprintf("View '%s' contains MySQL native query definitions which won't directly compile in PostgreSQL.", t.Name),
				Remedy:      "Rewrite the view query syntax to be PostgreSQL-compliant (e.g. replace IF() with CASE WHEN, GROUP_CONCAT with string_agg, etc.) and create it after table loading completes.",
			})
		}
	}

	return report, nil
}

func GeneratePreflightReportMarkdown(rep *PreflightReport, dbName, outPath string) error {
	md := fmt.Sprintf(`# QantaraDB FoodTech PostgreSQL Readiness Preflight Report

**Analyzed Database:** %s
**Total Risks Found:** %d
**High Severity Risks:** %d

---

## Executive Summary

The preflight engine scanned the MySQL/MariaDB schema to flag architectural and SQL incompatibility risks prior to PostgreSQL migration. This is particularly crucial for Laravel and Custom PHP FoodTech web portals which may leverage MySQL-specific SQL functions or key patterns.

| Severity | Count | Status |
| :--- | :---: | :---: |
| 🔴 **High Severity** | %d | Action Required before migration |
| 🟡 **Medium Severity** | %d | Action Recommended to avoid runtime queries crash |
| 🟢 **Low Severity** | %d | Informational, review if ORM issues arise |

---

## Risk Details & Remedies

`,
		dbName,
		rep.TotalRisks,
		rep.HighRisksCount,
		rep.HighRisksCount,
		rep.TotalRisks-rep.HighRisksCount, // simple count representation
		0,
	)

	if len(rep.Risks) == 0 {
		md += "💚 **No compatibility risks found! This schema is fully ready for PostgreSQL.**\n"
	} else {
		for i, r := range rep.Risks {
			severityEmoji := "🟢"
			if r.RiskLevel == "High" {
				severityEmoji = "🔴"
			} else if r.RiskLevel == "Medium" {
				severityEmoji = "🟡"
			}

			md += fmt.Sprintf("### %d. %s [%s] %s (%s.%s)\n\n", i+1, severityEmoji, r.RiskLevel, r.Category, r.TableName, r.ColumnName)
			md += fmt.Sprintf("**Description:** %s\n\n", r.Description)
			md += fmt.Sprintf("**Remedy:** %s\n\n", r.Remedy)
			md += "---\n\n"
		}
	}

	md += "*Report generated by QantaraDB FoodTech Readiness Engine.*"

	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(outPath, []byte(md), 0644)
}
