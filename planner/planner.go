package planner

import (
	"fmt"

	"github.com/OWNER/qantaradb/inspector"
	"github.com/OWNER/qantaradb/mapper"
)

type MigrationPlan struct {
	SourceDatabase string            `json:"source_database"`
	Tables         []TablePlan       `json:"tables"`
	TableOrder     []string          `json:"table_order"` // Topologically sorted
	Config         mapper.Config     `json:"config"`
}

type TablePlan struct {
	SourceName       string       `json:"source_name"`
	TargetName       string       `json:"target_name"`
	Columns          []ColumnPlan `json:"columns"`
	PrimaryKeyColumn string       `json:"primary_key_column"`
	Chunkable        bool         `json:"chunkable"`
	EstimatedRows    int64        `json:"estimated_rows"`
}

type ColumnPlan struct {
	SourceName   string   `json:"source_name"`
	TargetName   string   `json:"target_name"`
	SourceType   string   `json:"source_type"`
	TargetType   string   `json:"target_type"`
	IsNullable   bool     `json:"is_nullable"`
	Constraints  []string `json:"constraints"`
}

func CreatePlan(schema *inspector.SchemaInfo, mConfig mapper.Config, excludes []string) (*MigrationPlan, error) {
	plan := &MigrationPlan{
		SourceDatabase: schema.DatabaseName,
		Tables:         []TablePlan{},
		TableOrder:     []string{},
		Config:         mConfig,
	}

	excludeMap := make(map[string]bool)
	for _, ex := range excludes {
		excludeMap[ex] = true
	}

	// Calculate topological order
	order, err := sortTablesTopologically(schema.Tables, excludeMap)
	if err != nil {
		// If circular dependencies exist, fallback to alphabetical or database order, but output warning
		order = []string{}
		for _, t := range schema.Tables {
			if !excludeMap[t.Name] {
				order = append(order, t.Name)
			}
		}
	}
	plan.TableOrder = order

	for _, t := range schema.Tables {
		if excludeMap[t.Name] {
			continue
		}

		tPlan := TablePlan{
			SourceName:       t.Name,
			TargetName:       t.Name,
			Columns:          []ColumnPlan{},
			PrimaryKeyColumn: "",
			Chunkable:        false,
		}

		// Detect primary key
		for _, c := range t.Columns {
			if tPlan.PrimaryKeyColumn == "" && (c.Extra == "auto_increment" || stringsContainsIgnoreCase(c.ColumnType, "pri")) {
				tPlan.PrimaryKeyColumn = c.Name
				tPlan.Chunkable = true
			}
		}

		for _, col := range t.Columns {
			targetType, constraints, err := mapper.MapType(col, mConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to map column %s.%s: %w", t.Name, col.Name, err)
			}

			cPlan := ColumnPlan{
				SourceName:  col.Name,
				TargetName:  col.Name,
				SourceType:  col.ColumnType,
				TargetType:  targetType,
				IsNullable:  col.IsNullable,
				Constraints: constraints,
			}
			tPlan.Columns = append(tPlan.Columns, cPlan)
		}

		plan.Tables = append(plan.Tables, tPlan)
	}

	return plan, nil
}

func stringsContainsIgnoreCase(str, substr string) bool {
	return stringsContains(stringToLower(str), stringToLower(substr))
}

func stringToLower(s string) string {
	return stringsToLower(s)
}

func stringsToLower(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || stringsIndex(s, substr) >= 0)
}

func stringsIndex(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	if len(s) < n {
		return -1
	}
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

// Kahn's algorithm for Topological Sort
func sortTablesTopologically(tables []inspector.Table, excludeMap map[string]bool) ([]string, error) {
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)

	for _, t := range tables {
		if excludeMap[t.Name] {
			continue
		}
		if _, exists := inDegree[t.Name]; !exists {
			inDegree[t.Name] = 0
		}

		for _, fk := range t.ForeignKeys {
			if excludeMap[fk.ReferencedTable] {
				continue
			}
			// t depends on fk.ReferencedTable
			// fk.ReferencedTable -> t
			adjList[fk.ReferencedTable] = append(adjList[fk.ReferencedTable], t.Name)
			inDegree[t.Name]++
		}
	}

	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		result = append(result, u)

		for _, v := range adjList[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if len(result) < len(inDegree) {
		return nil, fmt.Errorf("circular dependency detected in foreign keys")
	}

	return result, nil
}
