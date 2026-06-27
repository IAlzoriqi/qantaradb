package loader

import "testing"

func TestNormalizeBoolForCopyHandlesMySQLTinyintValues(t *testing.T) {
	cases := []struct {
		name     string
		value    interface{}
		expected bool
		ok       bool
	}{
		{name: "int64 zero", value: int64(0), expected: false, ok: true},
		{name: "int64 one", value: int64(1), expected: true, ok: true},
		{name: "byte zero", value: []byte("0"), expected: false, ok: true},
		{name: "byte one", value: []byte("1"), expected: true, ok: true},
		{name: "string false", value: "false", expected: false, ok: true},
		{name: "string true", value: "true", expected: true, ok: true},
		{name: "unsupported literal", value: "maybe", expected: false, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeBoolForCopy(tc.value)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.expected {
				t.Fatalf("normalized value = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestNormalizeValueForCopyOnlyCoercesBooleanTargets(t *testing.T) {
	l := &Loader{sanitizedRecorder: NewSanitizedRecorder()}

	boolColumn := ColumnPlan{SourceName: "is_active", TargetName: "is_active", TargetType: "boolean"}
	if got := l.normalizeValueForCopy("sample", boolColumn, "1", int64(0)); got != false {
		t.Fatalf("boolean target normalized to %#v, want false", got)
	}

	smallintColumn := ColumnPlan{SourceName: "status", TargetName: "status", TargetType: "smallint"}
	if got := l.normalizeValueForCopy("sample", smallintColumn, "1", int64(0)); got != int64(0) {
		t.Fatalf("smallint target normalized to %#v, want int64(0)", got)
	}
}
