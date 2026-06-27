package validator

func DetermineValidationStatus(report *ValidationReport) string {
	if report == nil {
		return "VALIDATION_FAILED"
	}

	partial := false
	if !report.FKIntegrityPassed || report.SequenceValidation.Status == "reset_failed" {
		return "VALIDATION_FAILED"
	}

	for _, table := range report.TablesValidation {
		if !table.CountMatch || !table.PrimaryKeyMinMatch || !table.PrimaryKeyMaxMatch || table.ChecksumStatus == "real_mismatch" {
			return "VALIDATION_FAILED"
		}
		if table.ChecksumStatus == "unsupported_comparison" {
			partial = true
		}
		if table.ChecksumStatus == "normalized_equivalent" || table.ChecksumStatus == "sanitized_equivalent" {
			partial = true
		}
	}

	for _, item := range report.SequenceValidation.Items {
		if item.Status == "unsupported" || item.Status == "no_sequence" {
			partial = true
		}
	}

	if partial {
		return "VALIDATION_PARTIAL"
	}
	return "VALIDATION_PASSED"
}

func DetermineStagingReadiness(report *ValidationReport, sanitizedRowsReportMissing bool) StagingReadinessGate {
	gate := StagingReadinessGate{Status: "ready", Reasons: []string{}, Warnings: []string{}}
	if report == nil {
		gate.Status = "blocked"
		gate.Blocked = true
		gate.Reasons = append(gate.Reasons, "validation report is missing")
		return gate
	}
	if !report.FKIntegrityPassed {
		gate.Reasons = append(gate.Reasons, "FK validation failed")
	}
	if report.SequenceValidation.Status == "reset_failed" {
		gate.Reasons = append(gate.Reasons, "sequence reset failed")
	}
	for _, table := range report.TablesValidation {
		if !table.CountMatch {
			gate.Reasons = append(gate.Reasons, "row count failed: "+table.TableName)
		}
		if table.ChecksumStatus == "real_mismatch" {
			gate.Reasons = append(gate.Reasons, "real checksum mismatch: "+table.TableName)
		}
		if table.ChecksumStatus == "unsupported_comparison" {
			gate.Warnings = append(gate.Warnings, "unsupported checksum comparison: "+table.TableName)
		}
		if table.ChecksumStatus == "normalized_equivalent" || table.ChecksumStatus == "sanitized_equivalent" {
			gate.Warnings = append(gate.Warnings, table.ChecksumStatus+": "+table.TableName)
		}
	}
	if sanitizedRowsReportMissing {
		gate.Reasons = append(gate.Reasons, "sanitized rows exist without a report")
	}

	if len(gate.Reasons) > 0 {
		gate.Status = "blocked"
		gate.Blocked = true
	}
	return gate
}
