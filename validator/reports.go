package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteSequenceResetReports(report SequenceResetReport, jsonPath, mdPath string) error {
	if err := writeJSON(jsonPath, report); err != nil {
		return err
	}
	md := "# QantaraDB Sequence Reset Report\n\n"
	md += fmt.Sprintf("status: %s\n\n", report.Status)
	md += "| table | column | sequence | max_primary_key | expected_next_value | status | details |\n"
	md += "| --- | --- | --- | ---: | ---: | --- | --- |\n"
	if len(report.Items) == 0 {
		md += "| none | none | none | 0 | 0 | unsupported | no sequence-bearing tables were inspected |\n"
	} else {
		for _, item := range report.Items {
			md += fmt.Sprintf("| `%s` | `%s` | `%s` | %s | %s | `%s` | %s |\n",
				item.TableName, item.ColumnName, item.SequenceName, item.MaxPrimaryKey, item.ExpectedNextValue, item.Status, item.Details)
		}
	}
	return writeText(mdPath, md)
}

func WriteChecksumDrilldownReports(report ChecksumDrilldown, jsonPath, mdPath string) error {
	if err := writeJSON(jsonPath, report); err != nil {
		return err
	}
	md := "# QantaraDB Checksum Mismatch Drilldown\n\n"
	md += fmt.Sprintf("status: %s\n\n", report.Status)
	md += "| table | primary_key | column | reason | suggested_classification | source_hash | target_hash | source_sample | target_sample |\n"
	md += "| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n"
	if len(report.Items) == 0 {
		md += "| none | none | none | none | none | none | none | none | none |\n"
	} else {
		for _, item := range report.Items {
			md += fmt.Sprintf("| `%s` | `%s` | `%s` | %s | `%s` | `%s` | `%s` | `%s` | `%s` |\n",
				item.Table,
				item.PrimaryKey,
				item.Column,
				item.MismatchReason,
				item.SuggestedClassification,
				item.SourceNormalizedHash,
				item.TargetNormalizedHash,
				item.SourceSampleLimited,
				item.TargetSampleLimited,
			)
		}
	}
	return writeText(mdPath, md)
}

func writeJSON(path string, payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(path, data)
}

func writeText(path string, payload string) error {
	return writeBytes(path, []byte(payload))
}

func writeBytes(path string, payload []byte) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, payload, 0644)
}
