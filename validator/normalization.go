package validator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
)

type checksumFlags struct {
	Sanitized   bool
	Unsupported bool
}

var decimalPattern = regexp.MustCompile(`^-?\d+\.\d+$`)
var integerPattern = regexp.MustCompile(`^-?\d+$`)

func rawChecksumValue(value interface{}) string {
	if value == nil {
		return "NULL"
	}
	switch v := value.(type) {
	case []byte:
		return string(v)
	case time.Time:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func normalizeChecksumValue(value interface{}) (string, checksumFlags) {
	if value == nil {
		return "NULL", checksumFlags{}
	}

	switch v := value.(type) {
	case pgtype.Numeric:
		normalized, ok := normalizePGNumeric(v)
		if !ok {
			return "UNSUPPORTED_NUMERIC", checksumFlags{Unsupported: true}
		}
		return "scalar:" + normalized, checksumFlags{}
	case []byte:
		if !utf8.Valid(v) {
			return "UNSUPPORTED_BINARY", checksumFlags{Unsupported: true}
		}
		return normalizeStringForChecksum(string(v))
	case string:
		return normalizeStringForChecksum(v)
	case bool:
		if v {
			return "scalar:1", checksumFlags{}
		}
		return "scalar:0", checksumFlags{}
	case time.Time:
		return "time:" + v.UTC().Format(time.RFC3339Nano), checksumFlags{}
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("scalar:%d", reflect.ValueOf(value).Int()), checksumFlags{}
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("scalar:%d", reflect.ValueOf(value).Uint()), checksumFlags{}
	case float32, float64:
		f := reflect.ValueOf(value).Convert(reflect.TypeOf(float64(0))).Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return "UNSUPPORTED_FLOAT", checksumFlags{Unsupported: true}
		}
		return "scalar:" + normalizeDecimal(strconv.FormatFloat(f, 'f', -1, 64)), checksumFlags{}
	case sql.NullString:
		if !v.Valid {
			return "NULL", checksumFlags{}
		}
		return normalizeStringForChecksum(v.String)
	default:
		return normalizeStringForChecksum(fmt.Sprintf("%v", value))
	}
}

func normalizePGNumeric(value pgtype.Numeric) (string, bool) {
	if !value.Valid {
		return "NULL", true
	}
	if value.NaN || value.InfinityModifier != pgtype.Finite {
		return "", false
	}
	if value.Int == nil {
		return "0", true
	}

	digits := value.Int.String()
	negative := strings.HasPrefix(digits, "-")
	if negative {
		digits = strings.TrimPrefix(digits, "-")
	}

	exp := int(value.Exp)
	var out string
	if exp >= 0 {
		out = digits + strings.Repeat("0", exp)
	} else {
		scale := -exp
		if scale >= len(digits) {
			out = "0." + strings.Repeat("0", scale-len(digits)) + digits
		} else {
			cut := len(digits) - scale
			out = digits[:cut] + "." + digits[cut:]
		}
	}
	if negative && out != "0" {
		out = "-" + out
	}
	return normalizeDecimal(out), true
}

func normalizeStringForChecksum(value string) (string, checksumFlags) {
	flags := checksumFlags{}
	if strings.Contains(value, "\x00") || !utf8.ValidString(value) {
		flags.Sanitized = true
		value = strings.ReplaceAll(strings.ToValidUTF8(value, ""), "\x00", "")
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "string:", flags
	}

	if normalizedJSON, ok := normalizeJSON(trimmed); ok {
		return "json:" + normalizedJSON, flags
	}
	if normalizedTime, ok := normalizeTime(trimmed); ok {
		return "time:" + normalizedTime, flags
	}
	if normalizedBool, ok := normalizeBool(trimmed); ok {
		return "scalar:" + normalizedBool, flags
	}
	if integerPattern.MatchString(trimmed) {
		return "scalar:" + normalizeDecimal(trimmed), flags
	}
	if decimalPattern.MatchString(trimmed) {
		return "scalar:" + normalizeDecimal(trimmed), flags
	}

	return "string:" + trimmed, flags
}

func normalizeJSON(value string) (string, bool) {
	if !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "[") {
		return "", false
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return "", false
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func normalizeTime(value string) (string, bool) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano), true
		}
	}
	return "", false
}

func normalizeBool(value string) (string, bool) {
	switch strings.ToLower(value) {
	case "true", "t", "yes", "y", "1":
		return "1", true
	case "false", "f", "no", "n", "0":
		return "0", true
	}
	return "", false
}

func normalizeDecimal(value string) string {
	if !strings.Contains(value, ".") {
		if value == "-0" || value == "" {
			return "0"
		}
		return value
	}
	value = strings.TrimRight(value, "0")
	value = strings.TrimRight(value, ".")
	if value == "-0" || value == "" {
		return "0"
	}
	return value
}
