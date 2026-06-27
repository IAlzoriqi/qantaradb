package validator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type ChecksumDrilldown struct {
	Status string                 `json:"status"`
	Items  []ChecksumMismatchItem `json:"items"`
}

type ChecksumMismatchItem struct {
	Table                   string `json:"table"`
	PrimaryKey              string `json:"primary_key"`
	Column                  string `json:"column"`
	SourceNormalizedHash    string `json:"source_normalized_value_hash"`
	TargetNormalizedHash    string `json:"target_normalized_value_hash"`
	MismatchReason          string `json:"mismatch_reason"`
	SuggestedClassification string `json:"suggested_classification"`
	SourceSampleLimited     string `json:"source_sample_limited"`
	TargetSampleLimited     string `json:"target_sample_limited"`
}

func buildChecksumDrilldown(
	table string,
	columns []string,
	primaryKeys []string,
	srcRawRows [][]string,
	tgtRawRows [][]string,
	srcNormalizedRows [][]string,
	tgtNormalizedRows [][]string,
	srcFlagRows [][]checksumFlags,
	tgtFlagRows [][]checksumFlags,
) ChecksumDrilldown {
	report := ChecksumDrilldown{Status: "clean", Items: []ChecksumMismatchItem{}}
	rowCount := minInt(len(srcNormalizedRows), len(tgtNormalizedRows))
	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		colCount := minInt(len(srcNormalizedRows[rowIndex]), len(tgtNormalizedRows[rowIndex]))
		for colIndex := 0; colIndex < colCount && colIndex < len(columns); colIndex++ {
			if srcNormalizedRows[rowIndex][colIndex] == tgtNormalizedRows[rowIndex][colIndex] {
				continue
			}
			pk := "unknown"
			if rowIndex < len(primaryKeys) {
				pk = primaryKeys[rowIndex]
			}
			srcFlags := checksumFlags{}
			tgtFlags := checksumFlags{}
			if rowIndex < len(srcFlagRows) && colIndex < len(srcFlagRows[rowIndex]) {
				srcFlags = srcFlagRows[rowIndex][colIndex]
			}
			if rowIndex < len(tgtFlagRows) && colIndex < len(tgtFlagRows[rowIndex]) {
				tgtFlags = tgtFlagRows[rowIndex][colIndex]
			}
			classification := "real_mismatch"
			reason := "normalized values differ"
			if srcFlags.Unsupported || tgtFlags.Unsupported {
				classification = "unsupported_comparison"
				reason = "unsupported binary/blob or non-comparable value"
			} else if srcFlags.Sanitized || tgtFlags.Sanitized {
				classification = "sanitized_equivalent"
				reason = "sanitized value changed representation"
			}

			report.Items = append(report.Items, ChecksumMismatchItem{
				Table:                   table,
				PrimaryKey:              pk,
				Column:                  columns[colIndex],
				SourceNormalizedHash:    hashShort(srcNormalizedRows[rowIndex][colIndex]),
				TargetNormalizedHash:    hashShort(tgtNormalizedRows[rowIndex][colIndex]),
				MismatchReason:          reason,
				SuggestedClassification: classification,
				SourceSampleLimited:     limitedChecksumSample(srcRawRows, rowIndex, colIndex),
				TargetSampleLimited:     limitedChecksumSample(tgtRawRows, rowIndex, colIndex),
			})
		}
	}
	if len(srcNormalizedRows) != len(tgtNormalizedRows) {
		report.Items = append(report.Items, ChecksumMismatchItem{
			Table:                   table,
			PrimaryKey:              "sample_set",
			Column:                  "*",
			SourceNormalizedHash:    hashShort(fmt.Sprintf("%d", len(srcNormalizedRows))),
			TargetNormalizedHash:    hashShort(fmt.Sprintf("%d", len(tgtNormalizedRows))),
			MismatchReason:          "sample row count differs",
			SuggestedClassification: "real_mismatch",
			SourceSampleLimited:     fmt.Sprintf("sample_rows=%d", len(srcNormalizedRows)),
			TargetSampleLimited:     fmt.Sprintf("sample_rows=%d", len(tgtNormalizedRows)),
		})
	}
	if len(report.Items) > 0 {
		report.Status = "mismatches_detected"
	}
	return report
}

func hashShort(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}

func limitedChecksumSample(rows [][]string, rowIndex int, colIndex int) string {
	if rowIndex >= len(rows) || colIndex >= len(rows[rowIndex]) {
		return "missing"
	}
	value := rows[rowIndex][colIndex]
	runes := []rune(value)
	if len(runes) > 24 {
		runes = runes[:24]
	}
	return fmt.Sprintf("len=%d sha256=%s prefix=%q", len(value), hashShort(value), strings.ReplaceAll(string(runes), "\n", "\\n"))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
