package loader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

type SanitizedRowItem struct {
	Table         string `json:"table"`
	Column        string `json:"column"`
	PrimaryKey    string `json:"primary_key"`
	Reason        string `json:"reason"`
	OriginalKind  string `json:"original_kind"`
	SanitizedKind string `json:"sanitized_kind"`
	Count         int64  `json:"count"`
	SampleLimited string `json:"sample_limited"`
}

type SanitizedRowsReport struct {
	Status string             `json:"status"`
	Total  int64              `json:"total"`
	Items  []SanitizedRowItem `json:"items"`
}

type SanitizedRecorder struct {
	mu    sync.Mutex
	items map[string]*SanitizedRowItem
}

func NewSanitizedRecorder() *SanitizedRecorder {
	return &SanitizedRecorder{items: make(map[string]*SanitizedRowItem)}
}

func (r *SanitizedRecorder) Record(table, column, primaryKey, reason, originalKind, sanitizedKind, sample string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	key := strings.Join([]string{table, column, primaryKey, reason, originalKind, sanitizedKind}, "\x1f")
	item, ok := r.items[key]
	if !ok {
		item = &SanitizedRowItem{
			Table:         table,
			Column:        column,
			PrimaryKey:    primaryKey,
			Reason:        reason,
			OriginalKind:  originalKind,
			SanitizedKind: sanitizedKind,
			SampleLimited: sample,
		}
		r.items[key] = item
	}
	item.Count++
}

func (r *SanitizedRecorder) Report() SanitizedRowsReport {
	report := SanitizedRowsReport{Status: "clean", Items: []SanitizedRowItem{}}
	if r == nil {
		return report
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range r.items {
		report.Total += item.Count
		report.Items = append(report.Items, *item)
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Table != report.Items[j].Table {
			return report.Items[i].Table < report.Items[j].Table
		}
		if report.Items[i].Column != report.Items[j].Column {
			return report.Items[i].Column < report.Items[j].Column
		}
		return report.Items[i].Reason < report.Items[j].Reason
	})
	if report.Total > 0 {
		report.Status = "sanitized_rows_detected"
	}
	return report
}

func SanitizePostgresTextForReport(value string) (string, []string) {
	reasons := []string{}
	if strings.Contains(value, "\x00") {
		reasons = append(reasons, "nul_bytes")
	}
	if !utf8.ValidString(value) {
		reasons = append(reasons, "invalid_utf8")
	}

	sanitized := strings.ReplaceAll(strings.ToValidUTF8(value, ""), "\x00", "")
	if sanitized != value && len(reasons) == 0 {
		reasons = append(reasons, "unsafe_text_conversion")
	}
	return sanitized, reasons
}

func LimitedSample(value string) string {
	hash := sha256.Sum256([]byte(value))
	runes := []rune(value)
	if len(runes) > 24 {
		runes = runes[:24]
	}
	return fmt.Sprintf("len=%d sha256=%s prefix=%q", len(value), hex.EncodeToString(hash[:8]), string(runes))
}

func WriteSanitizedRowsReports(report SanitizedRowsReport, jsonPath, mdPath string) error {
	if err := writeJSONFile(jsonPath, report); err != nil {
		return err
	}

	md := "# QantaraDB Sanitized Rows Report\n\n"
	md += fmt.Sprintf("status: %s\n\ntotal: %d\n\n", report.Status, report.Total)
	md += "| table | column | primary_key | reason | original_kind | sanitized_kind | count | sample_limited |\n"
	md += "| --- | --- | --- | --- | --- | --- | ---: | --- |\n"
	if len(report.Items) == 0 {
		md += "| none | none | none | none | none | none | 0 | none |\n"
	} else {
		for _, item := range report.Items {
			md += fmt.Sprintf("| `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | %d | `%s` |\n",
				item.Table, item.Column, item.PrimaryKey, item.Reason, item.OriginalKind, item.SanitizedKind, item.Count, item.SampleLimited)
		}
	}
	return writeTextFile(mdPath, md)
}

func writeJSONFile(path string, payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return writeBytesFile(path, data)
}

func writeTextFile(path string, payload string) error {
	return writeBytesFile(path, []byte(payload))
}

func writeBytesFile(path string, payload []byte) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, payload, 0644)
}
