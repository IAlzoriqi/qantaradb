package planner

import (
	"testing"

	"github.com/OWNER/qantaradb/inspector"
)

func TestSortTablesTopologically(t *testing.T) {
	// Setup tables with foreign key dependencies:
	// 'users' has no dependencies
	// 'restaurants' has no dependencies
	// 'orders' depends on 'users' and 'restaurants'
	// 'order_items' depends on 'orders'
	tables := []inspector.Table{
		{
			Name:        "order_items",
			ForeignKeys: []inspector.ForeignKey{{ReferencedTable: "orders"}},
		},
		{
			Name:        "orders",
			ForeignKeys: []inspector.ForeignKey{{ReferencedTable: "users"}, {ReferencedTable: "restaurants"}},
		},
		{
			Name:        "users",
			ForeignKeys: []inspector.ForeignKey{},
		},
		{
			Name:        "restaurants",
			ForeignKeys: []inspector.ForeignKey{},
		},
	}

	exclude := make(map[string]bool)
	order, err := sortTablesTopologically(tables, exclude)
	if err != nil {
		t.Fatalf("unexpected topological sorting error: %v", err)
	}

	// Verify dependencies are respected:
	// 'users' and 'restaurants' must come before 'orders'
	// 'orders' must come before 'order_items'
	positions := make(map[string]int)
	for idx, name := range order {
		positions[name] = idx
	}

	if len(order) != 4 {
		t.Errorf("expected 4 tables in sorted order, got %d", len(order))
	}

	if positions["users"] >= positions["orders"] {
		t.Errorf("expected 'users' to come before 'orders'")
	}
	if positions["restaurants"] >= positions["orders"] {
		t.Errorf("expected 'restaurants' to come before 'orders'")
	}
	if positions["orders"] >= positions["order_items"] {
		t.Errorf("expected 'orders' to come before 'order_items'")
	}
}

func TestCircularDependencyFallback(t *testing.T) {
	// Create circular dependency: tableA -> tableB -> tableA
	tables := []inspector.Table{
		{
			Name:        "tableA",
			ForeignKeys: []inspector.ForeignKey{{ReferencedTable: "tableB"}},
		},
		{
			Name:        "tableB",
			ForeignKeys: []inspector.ForeignKey{{ReferencedTable: "tableA"}},
		},
	}

	exclude := make(map[string]bool)
	_, err := sortTablesTopologically(tables, exclude)
	if err == nil {
		t.Errorf("expected circular dependency error, got nil")
	}
}
