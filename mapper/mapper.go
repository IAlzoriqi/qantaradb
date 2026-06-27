package mapper

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/OWNER/qantaradb/inspector"
)

type Config struct {
	Tinyint1AsBool         bool   `yaml:"tinyint1_as_bool" json:"tinyint1_as_bool"`
	DateTimeTimezonePolicy string `yaml:"datetime_timezone_policy" json:"datetime_timezone_policy"` // "utc", "local", "none"
	GeometryPostGISMode    bool   `yaml:"geometry_postgis_mode" json:"geometry_postgis_mode"`
	EnumAsDomain           bool   `yaml:"enum_as_domain" json:"enum_as_domain"`
	SetAsArray             bool   `yaml:"set_as_array" json:"set_as_array"`
}

var (
	enumRegex = regexp.MustCompile(`(?i)^enum\((.*)\)$`)
	setRegex  = regexp.MustCompile(`(?i)^set\((.*)\)$`)
)

func MapType(col inspector.Column, config Config) (string, []string, error) {
	dataType := strings.ToLower(col.DataType)
	columnType := strings.ToLower(col.ColumnType)

	switch dataType {
	case "tinyint":
		if config.Tinyint1AsBool && (strings.Contains(columnType, "(1)") || col.ColumnType == "tinyint(1)") {
			return "boolean", nil, nil
		}
		if col.IsUnsigned {
			return "smallint", []string{fmt.Sprintf("CHECK (%s >= 0)", quoteIdent(col.Name))}, nil
		}
		return "smallint", nil, nil

	case "smallint", "mediumint":
		if col.IsUnsigned {
			return "integer", []string{fmt.Sprintf("CHECK (%s >= 0)", quoteIdent(col.Name))}, nil
		}
		return "smallint", nil, nil

	case "int", "integer":
		if col.IsUnsigned {
			return "bigint", []string{fmt.Sprintf("CHECK (%s >= 0)", quoteIdent(col.Name))}, nil
		}
		return "integer", nil, nil

	case "bigint":
		if col.IsUnsigned {
			// PostgreSQL signed bigint can't hold unsigned bigint max (18446744073709551615)
			// Map to numeric(20,0) to hold all values precisely
			return "numeric(20,0)", []string{fmt.Sprintf("CHECK (%s >= 0)", quoteIdent(col.Name))}, nil
		}
		return "bigint", nil, nil

	case "float":
		return "real", nil, nil

	case "double":
		return "double precision", nil, nil

	case "decimal", "numeric":
		prec := 10
		scale := 0
		if col.NumericPrecision != nil {
			prec = *col.NumericPrecision
		}
		if col.NumericScale != nil {
			scale = *col.NumericScale
		}
		return fmt.Sprintf("numeric(%d,%d)", prec, scale), nil, nil

	case "char":
		lenVal := 1
		if col.CharLength != nil {
			lenVal = *col.CharLength
		}
		return fmt.Sprintf("char(%d)", lenVal), nil, nil

	case "varchar":
		lenVal := 255
		if col.CharLength != nil {
			lenVal = *col.CharLength
		}
		return fmt.Sprintf("varchar(%d)", lenVal), nil, nil

	case "text", "tinytext", "mediumtext", "longtext":
		return "text", nil, nil

	case "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		return "bytea", nil, nil

	case "json":
		return "jsonb", nil, nil

	case "date":
		return "date", nil, nil

	case "datetime", "timestamp":
		if config.DateTimeTimezonePolicy == "utc" {
			return "timestamp with time zone", nil, nil
		} else if config.DateTimeTimezonePolicy == "local" {
			return "timestamp with time zone", nil, nil
		}
		return "timestamp without time zone", nil, nil

	case "time":
		return "time without time zone", nil, nil

	case "year":
		return "smallint", []string{fmt.Sprintf("CHECK (%s >= 1901 AND %s <= 2155)", quoteIdent(col.Name), quoteIdent(col.Name))}, nil

	case "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection":
		if config.GeometryPostGISMode {
			// Map to dynamic PostGIS types
			return "geometry", nil, nil
		}
		// Fallback to text representation or bytea (WKB)
		return "bytea", nil, nil

	default:
		// Check enum
		if enumRegex.MatchString(columnType) {
			matches := enumRegex.FindStringSubmatch(columnType)
			if len(matches) > 1 {
				values := matches[1]
				if config.EnumAsDomain {
					// User may create custom enum type; but inline text CHECK constraint is highly robust and requires no schema DDL setup
					constraint := fmt.Sprintf("CHECK (%s IN (%s))", quoteIdent(col.Name), values)
					return "text", []string{constraint}, nil
				}
				return "text", nil, nil
			}
		}

		// Check set
		if setRegex.MatchString(columnType) {
			if config.SetAsArray {
				return "text[]", nil, nil
			}
			return "text", nil, nil
		}

		return "text", nil, nil
	}
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
