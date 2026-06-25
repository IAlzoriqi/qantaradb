package validator

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ValidationReport struct {
	TotalTables       int                 `json:"total_tables"`
	PassedTables      int                 `json:"passed_tables"`
	TablesValidation  []TableValidation   `json:"tables_validation"`
	FKIntegrityPassed bool                `json:"fk_integrity_passed"`
	FKViolations      []FKViolation       `json:"fk_violations"`
	TypeMappings      []TypeMapItem       `json:"type_mappings"`
	ArabicEmojiAudit  ArabicEmojiReport   `json:"arabic_emoji_audit"`
}

type TableValidation struct {
	TableName          string `json:"table_name"`
	SourceCount        int64  `json:"source_count"`
	TargetCount        int64  `json:"target_count"`
	CountMatch         bool   `json:"count_match"`
	PrimaryKeyMinMatch bool   `json:"pk_min_match"`
	PrimaryKeyMaxMatch bool   `json:"pk_max_match"`
	SourceMinPK        string `json:"source_min_pk"`
	TargetMinPK        string `json:"target_min_pk"`
	SourceMaxPK        string `json:"source_max_pk"`
	TargetMaxPK        string `json:"target_max_pk"`
	ChecksumMatch      bool   `json:"checksum_match"`
	SourceChecksum     string `json:"source_checksum"`
	TargetChecksum     string `json:"target_checksum"`
	Passed             bool   `json:"passed"`
}

type FKViolation struct {
	ConstraintName  string `json:"constraint_name"`
	ChildTable      string `json:"child_table"`
	ChildColumn     string `json:"child_column"`
	ParentTable     string `json:"parent_table"`
	ParentColumn    string `json:"parent_column"`
	ViolationCount  int64  `json:"violation_count"`
}

type TypeMapItem struct {
	TableName      string `json:"table_name"`
	ColumnName     string `json:"column_name"`
	SourceType     string `json:"source_type"`
	TargetType     string `json:"target_type"`
	Constraints    string `json:"constraints"`
}

type ArabicEmojiReport struct {
	ArabicTextMatch  bool   `json:"arabic_text_match"`
	EmojiMatch       bool   `json:"emoji_match"`
	SourceCharset    string `json:"source_charset"`
	TargetEncoding   string `json:"target_encoding"`
	Details          string `json:"details"`
}

func Validate(mysqlDB *sql.DB, pgxPool *pgxpool.Pool, tables []string, pkMap map[string]string) (*ValidationReport, error) {
	ctx := context.Background()
	report := &ValidationReport{
		TotalTables:       len(tables),
		PassedTables:      0,
		TablesValidation:  []TableValidation{},
		FKIntegrityPassed: true,
		FKViolations:      []FKViolation{},
		TypeMappings:      []TypeMapItem{},
		ArabicEmojiAudit: ArabicEmojiReport{
			ArabicTextMatch: true,
			EmojiMatch:      true,
			SourceCharset:   "unknown",
			TargetEncoding:  "unknown",
			Details:         "No utf8mb4 / encoding issues detected.",
		},
	}

	// 1. Audit MySQL and PostgreSQL Encodings/Charsets
	_ = mysqlDB.QueryRow("SELECT COALESCE(character_set_name, 'unknown') FROM information_schema.character_sets WHERE character_set_name = 'utf8mb4' LIMIT 1").Scan(&report.ArabicEmojiAudit.SourceCharset)
	if report.ArabicEmojiAudit.SourceCharset == "unknown" {
		// fallback default database charset
		_ = mysqlDB.QueryRow("SELECT DEFAULT_CHARACTER_SET_NAME FROM information_schema.schemata WHERE schema_name = DATABASE()").Scan(&report.ArabicEmojiAudit.SourceCharset)
	}

	var pgEncoding string
	_ = pgxPool.QueryRow(ctx, "SHOW server_encoding").Scan(&pgEncoding)
	report.ArabicEmojiAudit.TargetEncoding = pgEncoding

	// Check if source or target doesn't support utf8/utf8mb4
	if !strings.Contains(strings.ToLower(report.ArabicEmojiAudit.SourceCharset), "utf8") && 
		!strings.Contains(strings.ToLower(report.ArabicEmojiAudit.SourceCharset), "utf8mb4") {
		report.ArabicEmojiAudit.Details = "Warning: Source character set is not UTF-8/utf8mb4, Arabic and Emojis might suffer transcoding losses!"
	}

	// 2. Loop through Tables for Counts, Checksums, Type Mappings
	for _, t := range tables {
		val := TableValidation{
			TableName:          t,
			CountMatch:         false,
			PrimaryKeyMinMatch: true,
			PrimaryKeyMaxMatch: true,
			ChecksumMatch:      true,
			Passed:             false,
		}

		// Row Counts
		err := mysqlDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", t)).Scan(&val.SourceCount)
		if err != nil {
			return nil, fmt.Errorf("failed to count source table %s: %w", t, err)
		}

		err = pgxPool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", t)).Scan(&val.TargetCount)
		if err != nil {
			return nil, fmt.Errorf("failed to count target table %s: %w", t, err)
		}

		val.CountMatch = val.SourceCount == val.TargetCount

		// Min/Max PK ranges
		pkCol, hasPK := pkMap[t]
		if hasPK && pkCol != "" {
			var srcMin, srcMax sql.NullString
			err = mysqlDB.QueryRow(fmt.Sprintf("SELECT CAST(MIN(`%s`) AS CHAR), CAST(MAX(`%s`) AS CHAR) FROM `%s`", pkCol, pkCol, t)).Scan(&srcMin, &srcMax)
			if err == nil {
				if srcMin.Valid {
					val.SourceMinPK = srcMin.String
				}
				if srcMax.Valid {
					val.SourceMaxPK = srcMax.String
				}
			}

			var tgtMin, tgtMax sql.NullString
			err = pgxPool.QueryRow(ctx, fmt.Sprintf("SELECT CAST(MIN(\"%s\") AS TEXT), CAST(MAX(\"%s\") AS TEXT) FROM \"%s\"", pkCol, pkCol, t)).Scan(&tgtMin, &tgtMax)
			if err == nil {
				if tgtMin.Valid {
					val.TargetMinPK = tgtMin.String
				}
				if tgtMax.Valid {
					val.TargetMaxPK = tgtMax.String
				}
			}

			val.PrimaryKeyMinMatch = val.SourceMinPK == val.TargetMinPK
			val.PrimaryKeyMaxMatch = val.SourceMaxPK == val.TargetMaxPK

			// Chunk / Sample Checksum Validation (SHA-256 over first 100 rows)
			if val.SourceCount > 0 {
				srcChecksum, tgtChecksum, csErr := calculateSampleChecksum(ctx, mysqlDB, pgxPool, t, pkCol)
				if csErr == nil {
					val.SourceChecksum = srcChecksum
					val.TargetChecksum = tgtChecksum
					val.ChecksumMatch = srcChecksum == tgtChecksum
				} else {
					val.ChecksumMatch = false
					val.SourceChecksum = "error"
					val.TargetChecksum = csErr.Error()
				}
			}
		}

		val.Passed = val.CountMatch && val.PrimaryKeyMinMatch && val.PrimaryKeyMaxMatch && val.ChecksumMatch
		if val.Passed {
			report.PassedTables++
		}
		report.TablesValidation = append(report.TablesValidation, val)

		// Dynamic Schema/Type Mapping Physical Audit
		typeItems, _ := auditTypeMappings(ctx, mysqlDB, pgxPool, t)
		report.TypeMappings = append(report.TypeMappings, typeItems...)
	}

	// 3. FK Validation Check
	// (Check if any active foreign key constraints in target are violated by existing data)
	// Query PG system catalog for foreign keys
	fkQuery := `
		SELECT
			tc.constraint_name, 
			tc.table_name AS child_table, 
			kcu.column_name AS child_column, 
			ccu.table_name AS parent_table, 
			ccu.column_name AS parent_column
		FROM 
			information_schema.table_constraints AS tc 
			JOIN information_schema.key_column_usage AS kcu
			  ON tc.constraint_name = kcu.constraint_name
			  AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage AS ccu
			  ON ccu.constraint_name = tc.constraint_name
			  AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public';
	`
	rows, err := pgxPool.Query(ctx, fkQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var violation FKViolation
			err = rows.Scan(&violation.ConstraintName, &violation.ChildTable, &violation.ChildColumn, &violation.ParentTable, &violation.ParentColumn)
			if err == nil {
				// Query if there are any orphaned child records
				checkQuery := fmt.Sprintf(`
					SELECT COUNT(*) 
					FROM "%s" c 
					LEFT JOIN "%s" p ON c."%s" = p."%s" 
					WHERE c."%s" IS NOT NULL AND p."%s" IS NULL`, 
					violation.ChildTable, violation.ParentTable, violation.ChildColumn, violation.ParentColumn, violation.ChildColumn, violation.ParentColumn)
				
				var vCount int64
				err = pgxPool.QueryRow(ctx, checkQuery).Scan(&vCount)
				if err == nil && vCount > 0 {
					violation.ViolationCount = vCount
					report.FKViolations = append(report.FKViolations, violation)
					report.FKIntegrityPassed = false
				}
			}
		}
	}

	// 4. Sequence/Identity validation & adjustment
	for _, t := range tables {
		pkCol, hasPK := pkMap[t]
		if hasPK && pkCol != "" {
			var isIdentity bool
			err := pgxPool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM information_schema.columns 
					WHERE table_name = $1 AND column_name = $2 
					AND is_identity = 'YES'
				)`, t, pkCol).Scan(&isIdentity)
			if err == nil && isIdentity {
				seqQuery := fmt.Sprintf("SELECT setval(pg_get_serial_sequence('\"%s\"', '%s'), COALESCE(MAX(\"%s\"), 1)) FROM \"%s\"", t, pkCol, pkCol, t)
				_, _ = pgxPool.Exec(ctx, seqQuery)
			}
		}
	}

	// 5. Audit for Arabic & Emoji Transcoding Losses (Check if any  symbols were loaded into Target)
	for _, t := range tables {
		// Get text/varchar columns in target
		colsRows, err := pgxPool.Query(ctx, `
			SELECT column_name 
			FROM information_schema.columns 
			WHERE table_name = $1 AND udt_name IN ('varchar', 'text', 'bpchar')`, t)
		if err == nil {
			var textCols []string
			for colsRows.Next() {
				var colName string
				if err := colsRows.Scan(&colName); err == nil {
					textCols = append(textCols, colName)
				}
			}
			colsRows.Close()

			for _, col := range textCols {
				// Query if there's any unicode replacement character \uFFFD () or suspicious '??' loaded
				var hasReplacements bool
				checkQuery := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM "%s" WHERE "%s" LIKE '%%%%')`, t, col)
				_ = pgxPool.QueryRow(ctx, checkQuery).Scan(&hasReplacements)
				if hasReplacements {
					report.ArabicEmojiAudit.ArabicTextMatch = false
					report.ArabicEmojiAudit.EmojiMatch = false
					report.ArabicEmojiAudit.Details = "Encoding Error: Unicode replacement characters () were detected in target text column!"
				}
			}
		}
	}

	return report, nil
}

func calculateSampleChecksum(ctx context.Context, mysqlDB *sql.DB, pgxPool *pgxpool.Pool, table, pkCol string) (string, string, error) {
	// 1. Fetch first 100 rows from MySQL
	srcQuery := fmt.Sprintf("SELECT * FROM `%s` ORDER BY `%s` ASC LIMIT 100", table, pkCol)
	srcRows, err := mysqlDB.Query(srcQuery)
	if err != nil {
		return "", "", err
	}
	defer srcRows.Close()

	srcCols, _ := srcRows.Columns()
	srcValues := make([]interface{}, len(srcCols))
	srcScanArgs := make([]interface{}, len(srcCols))
	for i := range srcValues {
		srcScanArgs[i] = &srcValues[i]
	}

	var srcConcat []string
	for srcRows.Next() {
		_ = srcRows.Scan(srcScanArgs...)
		rowStr := ""
		for _, val := range srcValues {
			if val != nil {
				if b, ok := val.([]byte); ok {
					rowStr += string(b) + "|"
				} else {
					rowStr += fmt.Sprintf("%v", val) + "|"
				}
			} else {
				rowStr += "NULL|"
			}
		}
		srcConcat = append(srcConcat, rowStr)
	}

	// 2. Fetch first 100 rows from PostgreSQL
	tgtQuery := fmt.Sprintf("SELECT * FROM \"%s\" ORDER BY \"%s\" ASC LIMIT 100", table, pkCol)
	tgtRows, err := pgxPool.Query(ctx, tgtQuery)
	if err != nil {
		return "", "", err
	}
	defer tgtRows.Close()

	tgtFields := tgtRows.FieldDescriptions()
	var tgtConcat []string
	for tgtRows.Next() {
		tgtValues, err := tgtRows.Values()
		if err != nil {
			return "", "", err
		}
		rowStr := ""
		for _, val := range tgtValues {
			if val != nil {
				if b, ok := val.([]byte); ok {
					rowStr += string(b) + "|"
				} else {
					rowStr += fmt.Sprintf("%v", val) + "|"
				}
			} else {
				rowStr += "NULL|"
			}
		}
		tgtConcat = append(tgtConcat, rowStr)
	}

	// Compare lengths and generate sha256 checksum strings
	srcBlock := strings.Join(srcConcat, "\n")
	tgtBlock := strings.Join(tgtConcat, "\n")

	srcHash := sha256.Sum256([]byte(srcBlock))
	tgtHash := sha256.Sum256([]byte(tgtBlock))

	return hex.EncodeToString(srcHash[:]), hex.EncodeToString(tgtHash[:]), nil
}

func auditTypeMappings(ctx context.Context, mysqlDB *sql.DB, pgxPool *pgxpool.Pool, table string) ([]TypeMapItem, error) {
	var items []TypeMapItem

	// Query MySQL Column Type details
	mysqlQuery := `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE
		FROM information_schema.columns 
		WHERE table_schema = DATABASE() AND table_name = ?`
	
	mysqlRows, err := mysqlDB.Query(mysqlQuery, table)
	if err != nil {
		return nil, err
	}
	defer mysqlRows.Close()

	type myCol struct {
		Name       string
		DataType   string
		ColumnType string
		IsNullable string
	}
	var mysqlCols []myCol
	for mysqlRows.Next() {
		var col myCol
		if err := mysqlRows.Scan(&col.Name, &col.DataType, &col.ColumnType, &col.IsNullable); err == nil {
			mysqlCols = append(mysqlCols, col)
		}
	}

	// Fetch target Postgres columns
	for _, mCol := range mysqlCols {
		var pgDataType, isNullable string
		pgQuery := `
			SELECT data_type, is_nullable 
			FROM information_schema.columns 
			WHERE table_name = $1 AND column_name = $2`
		
		err = pgxPool.QueryRow(ctx, pgQuery, table, mCol.Name).Scan(&pgDataType, &isNullable)
		if err != nil {
			pgDataType = "not_found"
		}

		constraints := ""
		if mCol.IsNullable == "NO" {
			constraints += "NOT NULL"
		} else {
			constraints += "NULL"
		}

		items = append(items, TypeMapItem{
			TableName:   table,
			ColumnName:  mCol.Name,
			SourceType:  mCol.ColumnType,
			TargetType:  pgDataType,
			Constraints: constraints,
		})
	}

	return items, nil
}
