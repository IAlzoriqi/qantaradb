package validator

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ResetAndValidateSequences(ctx context.Context, pgxPool *pgxpool.Pool, tables []string, pkMap map[string]string) SequenceResetReport {
	report := SequenceResetReport{Status: "reset_passed", Items: []SequenceResetResult{}}
	for _, tableName := range tables {
		pkColumn := pkMap[tableName]
		if pkColumn == "" {
			report.Items = append(report.Items, SequenceResetResult{
				TableName: tableName,
				Status:    "unsupported",
				Details:   "table has no primary key column in migration plan",
			})
			continue
		}

		item := SequenceResetResult{
			TableName:  tableName,
			ColumnName: pkColumn,
		}

		var sequenceName sql.NullString
		err := pgxPool.QueryRow(ctx, "SELECT pg_get_serial_sequence($1, $2)", "public."+tableName, pkColumn).Scan(&sequenceName)
		if err != nil {
			item.Status = "unsupported"
			item.Details = err.Error()
			report.Items = append(report.Items, item)
			continue
		}
		if !sequenceName.Valid || strings.TrimSpace(sequenceName.String) == "" {
			item.Status = "no_sequence"
			item.Details = "column is not backed by a PostgreSQL serial/identity sequence"
			report.Items = append(report.Items, item)
			continue
		}

		item.SequenceName = sequenceName.String
		var maxPK sqlNullInt64
		maxQuery := fmt.Sprintf("SELECT MAX(%s) FROM %s", quoteIdentifier(pkColumn), quoteIdentifier(tableName))
		if err := pgxPool.QueryRow(ctx, maxQuery).Scan(&maxPK); err != nil {
			item.Status = "reset_failed"
			item.Details = "failed to read max primary key: " + err.Error()
			report.Items = append(report.Items, item)
			report.Status = "reset_failed"
			continue
		}

		maxValue := int64(0)
		if maxPK.Valid {
			maxValue = maxPK.Int64
		}
		item.MaxPrimaryKey = strconv.FormatInt(maxValue, 10)
		item.ExpectedNextValue = strconv.FormatInt(maxValue+1, 10)

		isCalled := maxValue > 0
		setValue := maxValue
		if setValue == 0 {
			setValue = 1
		}
		if err := pgxPool.QueryRow(ctx, "SELECT setval($1::regclass, $2, $3)", item.SequenceName, setValue, isCalled).Scan(&setValue); err != nil {
			item.Status = "reset_failed"
			item.Details = "failed to reset sequence: " + err.Error()
			report.Items = append(report.Items, item)
			report.Status = "reset_failed"
			continue
		}

		item.Status = "reset_passed"
		item.Details = "sequence reset so nextval is greater than current max primary key"
		report.Items = append(report.Items, item)
	}

	return report
}

type sqlNullInt64 struct {
	Int64 int64
	Valid bool
}

func (n *sqlNullInt64) Scan(value interface{}) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	switch v := value.(type) {
	case int64:
		n.Int64 = v
	case int32:
		n.Int64 = int64(v)
	case int:
		n.Int64 = int64(v)
	default:
		parsed, err := strconv.ParseInt(fmt.Sprintf("%v", value), 10, 64)
		if err != nil {
			return err
		}
		n.Int64 = parsed
	}
	n.Valid = true
	return nil
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
