package validator

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNormalizeChecksumValueHandlesNaturalDriverDifferences(t *testing.T) {
	cases := []struct {
		name string
		a    interface{}
		b    interface{}
	}{
		{name: "decimal trailing zeros", a: "12.3400", b: "12.34"},
		{name: "json key order", a: `{"b":2,"a":1}`, b: `{"a":1,"b":2}`},
		{name: "boolean rendering", a: true, b: "true"},
		{name: "timestamp rendering", a: "2026-06-27 12:30:00", b: "2026-06-27T12:30:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			left, _ := normalizeChecksumValue(tc.a)
			right, _ := normalizeChecksumValue(tc.b)
			if left != right {
				t.Fatalf("expected normalized values to match, got %q != %q", left, right)
			}
		})
	}
}

func TestNormalizeChecksumValueHandlesPGNumericRendering(t *testing.T) {
	normalized, flags := normalizeChecksumValue(pgtype.Numeric{
		Int:   big.NewInt(2500),
		Exp:   -2,
		Valid: true,
	})
	if flags.Unsupported {
		t.Fatal("expected finite pg numeric to be supported")
	}
	if normalized != "scalar:25" {
		t.Fatalf("unexpected normalized pg numeric: %s", normalized)
	}

	normalized, flags = normalizeChecksumValue(pgtype.Numeric{
		Int:   big.NewInt(1),
		Exp:   1,
		Valid: true,
	})
	if flags.Unsupported {
		t.Fatal("expected finite pg numeric with positive exponent to be supported")
	}
	if normalized != "scalar:10" {
		t.Fatalf("unexpected positive exponent pg numeric: %s", normalized)
	}
}

func TestNormalizeChecksumValueMarksSanitizedText(t *testing.T) {
	normalized, flags := normalizeChecksumValue("abc\x00def")
	if normalized != "string:abcdef" {
		t.Fatalf("unexpected normalized value: %s", normalized)
	}
	if !flags.Sanitized {
		t.Fatal("expected sanitized flag for NUL byte")
	}
}

func TestDetermineValidationStatusClassifiesPartialAndFailure(t *testing.T) {
	partial := &ValidationReport{
		FKIntegrityPassed: true,
		SequenceValidation: SequenceResetReport{
			Status: "reset_passed",
			Items:  []SequenceResetResult{{Status: "no_sequence"}},
		},
		TablesValidation: []TableValidation{{
			CountMatch:         true,
			PrimaryKeyMinMatch: true,
			PrimaryKeyMaxMatch: true,
			ChecksumStatus:     "normalized_equivalent",
		}},
	}
	if got := DetermineValidationStatus(partial); got != "VALIDATION_PARTIAL" {
		t.Fatalf("expected partial status, got %s", got)
	}

	failed := &ValidationReport{
		FKIntegrityPassed: true,
		SequenceValidation: SequenceResetReport{
			Status: "reset_passed",
		},
		TablesValidation: []TableValidation{{
			CountMatch:         true,
			PrimaryKeyMinMatch: true,
			PrimaryKeyMaxMatch: true,
			ChecksumStatus:     "real_mismatch",
		}},
	}
	if got := DetermineValidationStatus(failed); got != "VALIDATION_FAILED" {
		t.Fatalf("expected failed status, got %s", got)
	}
}

func TestStagingReadinessBlocksRealMismatches(t *testing.T) {
	report := &ValidationReport{
		FKIntegrityPassed: true,
		SequenceValidation: SequenceResetReport{
			Status: "reset_passed",
		},
		TablesValidation: []TableValidation{{
			TableName:          "orders",
			CountMatch:         true,
			PrimaryKeyMinMatch: true,
			PrimaryKeyMaxMatch: true,
			ChecksumStatus:     "real_mismatch",
		}},
	}
	gate := DetermineStagingReadiness(report, false)
	if !gate.Blocked {
		t.Fatal("expected staging readiness to be blocked")
	}
}

func TestUnsupportedComparisonDoesNotPass(t *testing.T) {
	_, flags := normalizeChecksumValue([]byte{0xff, 0x00, 0x01})
	if !flags.Unsupported {
		t.Fatal("expected invalid binary bytes to be marked unsupported")
	}

	report := &ValidationReport{
		FKIntegrityPassed: true,
		SequenceValidation: SequenceResetReport{
			Status: "reset_passed",
		},
		TablesValidation: []TableValidation{{
			CountMatch:         true,
			PrimaryKeyMinMatch: true,
			PrimaryKeyMaxMatch: true,
			ChecksumStatus:     "unsupported_comparison",
		}},
	}
	if got := DetermineValidationStatus(report); got != "VALIDATION_PARTIAL" {
		t.Fatalf("expected unsupported comparison to be partial, got %s", got)
	}
}

func TestSequenceResetFailureRemainsFailed(t *testing.T) {
	report := &ValidationReport{
		FKIntegrityPassed: true,
		SequenceValidation: SequenceResetReport{
			Status: "reset_failed",
		},
		TablesValidation: []TableValidation{{
			CountMatch:         true,
			PrimaryKeyMinMatch: true,
			PrimaryKeyMaxMatch: true,
			ChecksumStatus:     "passed",
		}},
	}
	if got := DetermineValidationStatus(report); got != "VALIDATION_FAILED" {
		t.Fatalf("expected sequence reset failure to fail validation, got %s", got)
	}
}
