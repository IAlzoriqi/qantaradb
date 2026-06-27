package inspector

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type Table struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"` // "BASE TABLE" or "VIEW"
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
	Engine      string       `json:"engine"`
	Collation   string       `json:"collation"`
}

type Column struct {
	Name             string  `json:"name"`
	DataType         string  `json:"data_type"`
	ColumnType       string  `json:"column_type"` // e.g. varchar(255), bigint(20) unsigned
	IsNullable       bool    `json:"is_nullable"`
	DefaultValue     *string `json:"default_value"`
	Extra            string  `json:"extra"` // e.g. auto_increment
	CharLength       *int    `json:"char_length"`
	NumericPrecision *int    `json:"numeric_precision"`
	NumericScale     *int    `json:"numeric_scale"`
	Collation        *string `json:"collation"`
	IsUnsigned       bool    `json:"is_unsigned"`
}

type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
	Type    string   `json:"type"` // BTREE, FULLTEXT, etc.
}

type ForeignKey struct {
	Name             string `json:"name"`
	ColumnName       string `json:"column_name"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
	UpdateRule       string `json:"update_rule"`
	DeleteRule       string `json:"delete_rule"`
}

type SchemaInfo struct {
	DatabaseName  string  `json:"database_name"`
	ServerVersion string  `json:"server_version"`
	Tables        []Table `json:"tables"`
}

func Inspect(dsn string) (*SchemaInfo, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql database: %w", err)
	}

	var dbName string
	err = db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get current database: %w", err)
	}

	var version string
	_ = db.QueryRow("SELECT VERSION()").Scan(&version)

	schema := &SchemaInfo{
		DatabaseName:  dbName,
		ServerVersion: version,
		Tables:        []Table{},
	}

	// Fetch tables and views
	tableRows, err := db.Query(`
		SELECT TABLE_NAME, TABLE_TYPE, ENGINE, TABLE_COLLATION 
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = ?`, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer tableRows.Close()

	for tableRows.Next() {
		var t Table
		var collation sql.NullString
		var engine sql.NullString
		err := tableRows.Scan(&t.Name, &t.Type, &engine, &collation)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		if collation.Valid {
			t.Collation = collation.String
		}
		if engine.Valid {
			t.Engine = engine.String
		}
		schema.Tables = append(schema.Tables, t)
	}
	_ = tableRows.Close()

	tableIndexes := make(map[string]int, len(schema.Tables))
	for i, t := range schema.Tables {
		tableIndexes[t.Name] = i
	}

	columnsByTable, err := inspectAllColumns(db, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect columns: %w", err)
	}
	indexesByTable, err := inspectAllIndexes(db, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect indexes: %w", err)
	}
	foreignKeysByTable, err := inspectAllForeignKeys(db, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect foreign keys: %w", err)
	}

	for tableName, idx := range tableIndexes {
		schema.Tables[idx].Columns = columnsByTable[tableName]
		schema.Tables[idx].Indexes = indexesByTable[tableName]
		schema.Tables[idx].ForeignKeys = foreignKeysByTable[tableName]
	}

	return schema, nil
}

func inspectAllColumns(db *sql.DB, dbName string) (map[string][]Column, error) {
	query := `
		SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA,
		       CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, COLLATION_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnsByTable := make(map[string][]Column)
	for rows.Next() {
		var tableName string
		var c Column
		var isNullableStr string
		var defVal sql.NullString
		var charLen sql.NullInt64
		var numPrec sql.NullInt64
		var numScale sql.NullInt64
		var collation sql.NullString

		err := rows.Scan(
			&tableName, &c.Name, &c.DataType, &c.ColumnType, &isNullableStr, &defVal, &c.Extra,
			&charLen, &numPrec, &numScale, &collation,
		)
		if err != nil {
			return nil, err
		}

		c.IsNullable = (isNullableStr == "YES")
		if defVal.Valid {
			c.DefaultValue = &defVal.String
		}
		if charLen.Valid {
			val := int(charLen.Int64)
			c.CharLength = &val
		}
		if numPrec.Valid {
			val := int(numPrec.Int64)
			c.NumericPrecision = &val
		}
		if numScale.Valid {
			val := int(numScale.Int64)
			c.NumericScale = &val
		}
		if collation.Valid {
			c.Collation = &collation.String
		}

		c.IsUnsigned = strings.Contains(strings.ToLower(c.ColumnType), "unsigned")
		columnsByTable[tableName] = append(columnsByTable[tableName], c)
	}

	return columnsByTable, rows.Err()
}

func inspectAllIndexes(db *sql.DB, dbName string) (map[string][]Index, error) {
	query := `
		SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, COLUMN_NAME, INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMaps := make(map[string]map[string]*Index)
	for rows.Next() {
		var tableName string
		var idxName string
		var nonUnique int
		var colName string
		var idxType string

		if err := rows.Scan(&tableName, &idxName, &nonUnique, &colName, &idxType); err != nil {
			return nil, err
		}

		if idxName == "PRIMARY" {
			continue
		}

		if _, ok := indexMaps[tableName]; !ok {
			indexMaps[tableName] = make(map[string]*Index)
		}

		if idx, ok := indexMaps[tableName][idxName]; ok {
			idx.Columns = append(idx.Columns, colName)
		} else {
			indexMaps[tableName][idxName] = &Index{
				Name:    idxName,
				Unique:  nonUnique == 0,
				Columns: []string{colName},
				Type:    idxType,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexesByTable := make(map[string][]Index)
	for tableName, idxMap := range indexMaps {
		for _, idx := range idxMap {
			indexesByTable[tableName] = append(indexesByTable[tableName], *idx)
		}
	}

	return indexesByTable, nil
}

func inspectAllForeignKeys(db *sql.DB, dbName string) (map[string][]ForeignKey, error) {
	query := `
		SELECT
			k.TABLE_NAME,
			k.CONSTRAINT_NAME,
			k.COLUMN_NAME,
			k.REFERENCED_TABLE_NAME,
			k.REFERENCED_COLUMN_NAME,
			r.UPDATE_RULE,
			r.DELETE_RULE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS r
			ON k.CONSTRAINT_NAME = r.CONSTRAINT_NAME AND k.CONSTRAINT_SCHEMA = r.CONSTRAINT_SCHEMA
		WHERE k.TABLE_SCHEMA = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	foreignKeysByTable := make(map[string][]ForeignKey)
	for rows.Next() {
		var tableName string
		var fk ForeignKey
		if err := rows.Scan(&tableName, &fk.Name, &fk.ColumnName, &fk.ReferencedTable, &fk.ReferencedColumn, &fk.UpdateRule, &fk.DeleteRule); err != nil {
			return nil, err
		}
		foreignKeysByTable[tableName] = append(foreignKeysByTable[tableName], fk)
	}

	return foreignKeysByTable, rows.Err()
}

func inspectColumns(db *sql.DB, dbName, tableName string) ([]Column, error) {
	query := `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA,
		       CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, COLLATION_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var c Column
		var isNullableStr string
		var defVal sql.NullString
		var charLen sql.NullInt64
		var numPrec sql.NullInt64
		var numScale sql.NullInt64
		var collation sql.NullString

		err := rows.Scan(
			&c.Name, &c.DataType, &c.ColumnType, &isNullableStr, &defVal, &c.Extra,
			&charLen, &numPrec, &numScale, &collation,
		)
		if err != nil {
			return nil, err
		}

		c.IsNullable = (isNullableStr == "YES")
		if defVal.Valid {
			c.DefaultValue = &defVal.String
		}
		if charLen.Valid {
			val := int(charLen.Int64)
			c.CharLength = &val
		}
		if numPrec.Valid {
			val := int(numPrec.Int64)
			c.NumericPrecision = &val
		}
		if numScale.Valid {
			val := int(numScale.Int64)
			c.NumericScale = &val
		}
		if collation.Valid {
			c.Collation = &collation.String
		}

		c.IsUnsigned = strings.Contains(strings.ToLower(c.ColumnType), "unsigned")
		cols = append(cols, c)
	}

	return cols, nil
}

func inspectIndexes(db *sql.DB, dbName, tableName string) ([]Index, error) {
	query := `
		SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME, INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idxMap := make(map[string]*Index)
	for rows.Next() {
		var idxName string
		var nonUnique int
		var colName string
		var idxType string

		err := rows.Scan(&idxName, &nonUnique, &colName, &idxType)
		if err != nil {
			return nil, err
		}

		// Skip PRIMARY; PostgreSQL handles PK constraints as part of tables
		if idxName == "PRIMARY" {
			continue
		}

		if idx, ok := idxMap[idxName]; ok {
			idx.Columns = append(idx.Columns, colName)
		} else {
			idxMap[idxName] = &Index{
				Name:    idxName,
				Unique:  nonUnique == 0,
				Columns: []string{colName},
				Type:    idxType,
			}
		}
	}

	var idxs []Index
	for _, idx := range idxMap {
		idxs = append(idxs, *idx)
	}
	return idxs, nil
}

func inspectForeignKeys(db *sql.DB, dbName, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT 
			k.CONSTRAINT_NAME, 
			k.COLUMN_NAME, 
			k.REFERENCED_TABLE_NAME, 
			k.REFERENCED_COLUMN_NAME,
			r.UPDATE_RULE,
			r.DELETE_RULE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS r 
			ON k.CONSTRAINT_NAME = r.CONSTRAINT_NAME AND k.CONSTRAINT_SCHEMA = r.CONSTRAINT_SCHEMA
		WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		err := rows.Scan(
			&fk.Name, &fk.ColumnName, &fk.ReferencedTable, &fk.ReferencedColumn,
			&fk.UpdateRule, &fk.DeleteRule,
		)
		if err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, nil
}
