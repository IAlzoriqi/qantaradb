package loader

import "testing"

func TestSanitizePostgresTextForReportDetectsNULAndInvalidUTF8(t *testing.T) {
	value := string([]byte{'o', 'k', 0x00, 0xff})
	sanitized, reasons := SanitizePostgresTextForReport(value)
	if sanitized != "ok" {
		t.Fatalf("unexpected sanitized value: %q", sanitized)
	}
	if len(reasons) != 2 {
		t.Fatalf("expected two reasons, got %v", reasons)
	}
}

func TestSanitizedRecorderAggregatesWithoutFullSensitiveValues(t *testing.T) {
	recorder := NewSanitizedRecorder()
	recorder.Record("users", "name", "42", "nul_bytes", "mysql_string", "postgres_text", LimitedSample("secret\x00name"))
	recorder.Record("users", "name", "42", "nul_bytes", "mysql_string", "postgres_text", LimitedSample("secret\x00name"))

	report := recorder.Report()
	if report.Status != "sanitized_rows_detected" {
		t.Fatalf("unexpected status: %s", report.Status)
	}
	if report.Total != 2 || len(report.Items) != 1 {
		t.Fatalf("unexpected aggregation: total=%d items=%d", report.Total, len(report.Items))
	}
	if report.Items[0].SampleLimited == "secret\x00name" {
		t.Fatal("sample should be limited, not the raw value")
	}
}
