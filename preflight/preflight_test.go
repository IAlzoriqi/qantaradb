package preflight

import (
	"testing"

	"github.com/OWNER/qantaradb/inspector"
)

func TestRunPreflight(t *testing.T) {
	// Setup a schema info containing a reserved word table 'user' and column 'order'
	// plus an unsigned auto-increment key (Laravel pattern)
	schema := &inspector.SchemaInfo{
		DatabaseName: "foodtech_test",
		Tables: []inspector.Table{
			{
				Name: "user", // Reserved word (High risk)
				Columns: []inspector.Column{
					{
						Name:       "id",
						DataType:   "bigint",
						ColumnType: "bigint unsigned",
						IsNullable: false,
						IsUnsigned: true,
						Extra:      "auto_increment",
					},
					{
						Name:       "order", // Reserved word (Medium risk)
						DataType:   "int",
						ColumnType: "int",
						IsNullable: true,
					},
				},
			},
		},
	}

	report, err := RunPreflight(schema)
	if err != nil {
		t.Fatalf("preflight execution failed: %v", err)
	}

	if report.TotalRisks != 3 { // Table 'user', Unsigned key 'id', Column 'order'
		t.Errorf("expected 3 risks, got %d", report.TotalRisks)
	}

	if report.HighRisksCount != 1 {
		t.Errorf("expected 1 high risk, got %d", report.HighRisksCount)
	}

	// Verify categories of flagged risks
	hasReservedTable := false
	hasReservedColumn := false
	hasUnsignedKey := false

	for _, risk := range report.Risks {
		if risk.Category == "Reserved Identifier" && risk.TableName == "user" && risk.ColumnName == "*" {
			hasReservedTable = true
		}
		if risk.Category == "Reserved Identifier" && risk.TableName == "user" && risk.ColumnName == "order" {
			hasReservedColumn = true
		}
		if risk.Category == "Unsigned Keys" && risk.TableName == "user" && risk.ColumnName == "id" {
			hasUnsignedKey = true
		}
	}

	if !hasReservedTable {
		t.Error("failed to flag reserved table name 'user'")
	}
	if !hasReservedColumn {
		t.Error("failed to flag reserved column name 'order'")
	}
	if !hasUnsignedKey {
		t.Error("failed to flag unsigned key 'id'")
	}
}
