package loader

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	BatchSize     int    `yaml:"batch_size" json:"batch_size"`
	Workers       int    `yaml:"workers" json:"workers"`
	StateFilePath string `yaml:"state_file_path" json:"state_file_path"`
}

type TableProgress struct {
	TableName     string      `json:"table_name"`
	TotalRows     int64       `json:"total_rows"`
	CompletedRows int64       `json:"completed_rows"`
	Status        string      `json:"status"` // "pending", "running", "completed", "failed"
	LastPKValue   interface{} `json:"last_pk_value"`
	ElapsedSec    float64     `json:"elapsed_sec"`
	RowsPerSec    float64     `json:"rows_per_sec"`
	ETASeconds    float64     `json:"eta_seconds"`
}

type JobState struct {
	SourceDatabase string                    `json:"source_database"`
	StartTime      time.Time                 `json:"start_time"`
	Progresses     map[string]*TableProgress `json:"progress_map"`
}

type Loader struct {
	mysqlDB  *sql.DB
	pgxPool  *pgxpool.Pool
	config   Config
	state    *JobState
	stateMux sync.Mutex
}

func NewLoader(mysqlDSN, pgDSN string, config Config) (*Loader, error) {
	mysqlDB, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql: %w", err)
	}

	ctx := context.Background()
	pgConfig, err := pgxpool.ParseConfig(pgDSN)
	if err != nil {
		mysqlDB.Close()
		return nil, fmt.Errorf("failed to parse pgx config: %w", err)
	}

	pgxPool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		mysqlDB.Close()
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	loader := &Loader{
		mysqlDB: mysqlDB,
		pgxPool: pgxPool,
		config:  config,
		state: &JobState{
			Progresses: make(map[string]*TableProgress),
		},
	}

	_ = loader.loadState()
	return loader, nil
}

func (l *Loader) Close() {
	if l.mysqlDB != nil {
		l.mysqlDB.Close()
	}
	if l.pgxPool != nil {
		l.pgxPool.Close()
	}
}

func (l *Loader) ExecPostgres(ctx context.Context, statement string) error {
	_, err := l.pgxPool.Exec(ctx, statement)
	return err
}

func (l *Loader) MySQLDB() *sql.DB {
	return l.mysqlDB
}

func (l *Loader) PostgresPool() *pgxpool.Pool {
	return l.pgxPool
}

func (l *Loader) saveState() error {
	l.stateMux.Lock()
	defer l.stateMux.Unlock()

	if l.config.StateFilePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(l.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(l.config.StateFilePath, data, 0644)
}

func (l *Loader) loadState() error {
	if l.config.StateFilePath == "" {
		return nil
	}

	data, err := os.ReadFile(l.config.StateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var loadedState JobState
	if err := json.Unmarshal(data, &loadedState); err != nil {
		return err
	}

	l.state = &loadedState
	return nil
}

func (l *Loader) StreamTable(ctx context.Context, tableName, pkCol string, columns []string) error {
	l.stateMux.Lock()
	prog, exists := l.state.Progresses[tableName]
	if !exists {
		prog = &TableProgress{
			TableName: tableName,
			Status:    "pending",
		}
		l.state.Progresses[tableName] = prog
	}
	l.stateMux.Unlock()

	if prog.Status == "completed" {
		fmt.Printf("Table %s already fully migrated. Skipping.\n", tableName)
		return nil
	}

	prog.Status = "running"
	_ = l.saveState()

	// Get total rows count
	var totalRows int64
	err := l.mysqlDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&totalRows)
	if err != nil {
		prog.Status = "failed"
		_ = l.saveState()
		return fmt.Errorf("failed to count rows: %w", err)
	}
	prog.TotalRows = totalRows

	start := time.Now()

	// Decide streaming mode
	if pkCol != "" {
		// Use Primary Key Chunking Stream
		err = l.streamWithPKChunking(ctx, tableName, pkCol, columns, prog, start)
	} else {
		// Fallback to offset-free streaming cursor using MySQL cursor query
		err = l.streamWithCursor(ctx, tableName, columns, prog, start)
	}

	if err != nil {
		prog.Status = "failed"
		_ = l.saveState()
		return err
	}

	prog.Status = "completed"
	prog.CompletedRows = totalRows
	prog.RowsPerSec = float64(totalRows) / time.Since(start).Seconds()
	prog.ETASeconds = 0
	_ = l.saveState()

	return nil
}

func (l *Loader) streamWithPKChunking(ctx context.Context, tableName, pkCol string, columns []string, prog *TableProgress, start time.Time) error {
	var minVal, maxVal int64
	err := l.mysqlDB.QueryRow(fmt.Sprintf("SELECT COALESCE(MIN(`%s`), 0), COALESCE(MAX(`%s`), 0) FROM `%s`", pkCol, pkCol, tableName)).Scan(&minVal, &maxVal)
	if err != nil {
		return fmt.Errorf("failed to fetch min/max keys: %w", err)
	}

	currentPK := minVal
	if prog.LastPKValue != nil {
		// Resume state
		switch v := prog.LastPKValue.(type) {
		case float64:
			currentPK = int64(v)
		case int64:
			currentPK = v
		}
		fmt.Printf("Resuming migration for %s at PK > %d\n", tableName, currentPK)
	}

	colNames := ""
	for i, col := range columns {
		if i > 0 {
			colNames += ", "
		}
		colNames += fmt.Sprintf("`%s`", col)
	}

	pgCols := make([]string, len(columns))
	for i, col := range columns {
		pgCols[i] = col
	}

	batchSize := int64(l.config.BatchSize)
	if batchSize <= 0 {
		batchSize = 5000
	}

	for currentPK <= maxVal {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		nextPK := currentPK + batchSize
		query := fmt.Sprintf("SELECT %s FROM `%s` WHERE `%s` >= %d AND `%s` < %d", colNames, tableName, pkCol, currentPK, pkCol, nextPK)
		rows, err := l.mysqlDB.Query(query)
		if err != nil {
			return fmt.Errorf("source query error: %w", err)
		}

		// Read all columns dynamically
		scanArgs := make([]interface{}, len(columns))
		values := make([]interface{}, len(columns))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		var chunkRows [][]interface{}
		for rows.Next() {
			err = rows.Scan(scanArgs...)
			if err != nil {
				rows.Close()
				return fmt.Errorf("scan error: %w", err)
			}

			rowVals := make([]interface{}, len(columns))
			for i, val := range values {
				if val != nil {
					// Extract bytes or string correctly depending on types if necessary
					if b, ok := val.([]byte); ok {
						rowVals[i] = sanitizePostgresText(string(b))
					} else if s, ok := val.(string); ok {
						rowVals[i] = sanitizePostgresText(s)
					} else {
						rowVals[i] = val
					}
				} else {
					rowVals[i] = nil
				}
			}
			chunkRows = append(chunkRows, rowVals)
		}
		rows.Close()

		if len(chunkRows) > 0 {
			// Bulk Copy into postgres using CopyFrom
			_, err = l.pgxPool.CopyFrom(
				ctx,
				pgx.Identifier{tableName},
				pgCols,
				pgx.CopyFromRows(chunkRows),
			)
			if err != nil {
				return fmt.Errorf("postgres CopyFrom failed: %w", err)
			}

			prog.CompletedRows += int64(len(chunkRows))
		}

		currentPK = nextPK
		prog.LastPKValue = currentPK
		elapsed := time.Since(start).Seconds()
		prog.ElapsedSec = elapsed
		if prog.CompletedRows > 0 {
			prog.RowsPerSec = float64(prog.CompletedRows) / elapsed
			remainingRows := prog.TotalRows - prog.CompletedRows
			if remainingRows > 0 && prog.RowsPerSec > 0 {
				prog.ETASeconds = float64(remainingRows) / prog.RowsPerSec
			} else {
				prog.ETASeconds = 0
			}
		}
		_ = l.saveState()
	}

	return nil
}

func (l *Loader) streamWithCursor(ctx context.Context, tableName string, columns []string, prog *TableProgress, start time.Time) error {
	colNames := ""
	for i, col := range columns {
		if i > 0 {
			colNames += ", "
		}
		colNames += fmt.Sprintf("`%s`", col)
	}

	pgCols := make([]string, len(columns))
	for i, col := range columns {
		pgCols[i] = col
	}

	// For standard queries, scan streaming in batches
	// Offset-free cursor is harder without PK, but we stream the connection safely using single driver cursor
	query := fmt.Sprintf("SELECT %s FROM `%s`", colNames, tableName)
	rows, err := l.mysqlDB.Query(query)
	if err != nil {
		return fmt.Errorf("source query error: %w", err)
	}
	defer rows.Close()

	scanArgs := make([]interface{}, len(columns))
	values := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	batchSize := l.config.BatchSize
	if batchSize <= 0 {
		batchSize = 5000
	}

	var batch [][]interface{}

	for rows.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			return fmt.Errorf("scan error: %w", err)
		}

		rowVals := make([]interface{}, len(columns))
		for i, val := range values {
			if val != nil {
				if b, ok := val.([]byte); ok {
					rowVals[i] = sanitizePostgresText(string(b))
				} else if s, ok := val.(string); ok {
					rowVals[i] = sanitizePostgresText(s)
				} else {
					rowVals[i] = val
				}
			} else {
				rowVals[i] = nil
			}
		}
		batch = append(batch, rowVals)

		if len(batch) >= batchSize {
			_, err = l.pgxPool.CopyFrom(
				ctx,
				pgx.Identifier{tableName},
				pgCols,
				pgx.CopyFromRows(batch),
			)
			if err != nil {
				return fmt.Errorf("postgres CopyFrom failed: %w", err)
			}

			prog.CompletedRows += int64(len(batch))
			elapsed := time.Since(start).Seconds()
			prog.ElapsedSec = elapsed
			if prog.CompletedRows > 0 {
				prog.RowsPerSec = float64(prog.CompletedRows) / elapsed
				remainingRows := prog.TotalRows - prog.CompletedRows
				if remainingRows > 0 && prog.RowsPerSec > 0 {
					prog.ETASeconds = float64(remainingRows) / prog.RowsPerSec
				}
			}
			_ = l.saveState()
			batch = nil
		}
	}

	// Copy final batch
	if len(batch) > 0 {
		_, err = l.pgxPool.CopyFrom(
			ctx,
			pgx.Identifier{tableName},
			pgCols,
			pgx.CopyFromRows(batch),
		)
		if err != nil {
			return fmt.Errorf("postgres copy of final batch failed: %w", err)
		}
		prog.CompletedRows += int64(len(batch))
	}

	return nil
}

func sanitizePostgresText(value string) string {
	return strings.ReplaceAll(strings.ToValidUTF8(value, ""), "\x00", "")
}
