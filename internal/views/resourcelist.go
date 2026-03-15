package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"

	"github.com/k2m30/a9s/internal/resource"
)

// ResourceListModel is a generic resource list view using evertras/bubble-table.
type ResourceListModel struct {
	Table     table.Model
	Resources []resource.Resource
	TypeDef   resource.ResourceTypeDef
	Width     int
	Height    int
}

// NewResourceList creates a new ResourceListModel from a resource type definition
// and a slice of resources. It builds columns and rows for the bubble-table.
func NewResourceList(typeDef resource.ResourceTypeDef, resources []resource.Resource, width, height int) ResourceListModel {
	// Build columns from the type definition
	columns := make([]table.Column, len(typeDef.Columns))
	for i, col := range typeDef.Columns {
		columns[i] = table.NewFlexColumn(col.Key, col.Title, col.Width)
	}

	// Build rows from resources
	rows := make([]table.Row, len(resources))
	for i, r := range resources {
		data := make(table.RowData)
		for k, v := range r.Fields {
			data[k] = v
		}
		rows[i] = table.NewRow(data)
	}

	// Set page size: reserve space for header, footer, borders
	pageSize := height - 4
	if pageSize < 1 {
		pageSize = 1
	}

	// Use lipgloss v1 styles for bubble-table compatibility
	baseStyle := lipgloss.NewStyle()
	headerStyle := lipgloss.NewStyle().Bold(true)
	highlightStyle := lipgloss.NewStyle().Reverse(true)

	t := table.New(columns).
		WithRows(rows).
		Focused(true).
		WithPageSize(pageSize).
		WithTargetWidth(width).
		WithBaseStyle(baseStyle).
		HeaderStyle(headerStyle).
		HighlightStyle(highlightStyle)

	return ResourceListModel{
		Table:     t,
		Resources: resources,
		TypeDef:   typeDef,
		Width:     width,
		Height:    height,
	}
}

// View renders the resource list table as a string.
func (m ResourceListModel) View() string {
	return m.Table.View()
}

// SelectedResource returns a pointer to the currently highlighted resource,
// or nil if there are no resources.
func (m ResourceListModel) SelectedResource() *resource.Resource {
	if len(m.Resources) == 0 {
		return nil
	}

	row := m.Table.HighlightedRow()
	// Match the highlighted row back to a resource by the first column key
	if len(m.TypeDef.Columns) > 0 {
		firstKey := m.TypeDef.Columns[0].Key
		if val, ok := row.Data[firstKey]; ok {
			valStr, _ := val.(string)
			for i := range m.Resources {
				if m.Resources[i].Fields[firstKey] == valStr {
					return &m.Resources[i]
				}
			}
		}
	}

	return nil
}

// SetSize updates the table dimensions.
func (m *ResourceListModel) SetSize(w, h int) {
	m.Width = w
	m.Height = h

	pageSize := h - 4
	if pageSize < 1 {
		pageSize = 1
	}

	m.Table = m.Table.WithTargetWidth(w).WithPageSize(pageSize)
}

// SortByColumn sorts the table by the given column key. If ascending is true
// the column is sorted in ascending order; otherwise descending. The column
// key must match one of the ResourceTypeDef column keys.
func (m *ResourceListModel) SortByColumn(key string, ascending bool) {
	if ascending {
		m.Table = m.Table.SortByAsc(key)
	} else {
		m.Table = m.Table.SortByDesc(key)
	}
}

// SortByName sorts the table by the first column marked as sortable whose key
// contains "name". Falls back to the first column if none match.
func (m *ResourceListModel) SortByName(ascending bool) {
	key := m.findColumnKey("name")
	if key != "" {
		m.SortByColumn(key, ascending)
	}
}

// SortByStatus sorts the table by the first column whose key contains "status"
// or "state". Falls back to nothing if no matching column exists.
func (m *ResourceListModel) SortByStatus(ascending bool) {
	key := m.findColumnKey("status")
	if key == "" {
		key = m.findColumnKey("state")
	}
	if key != "" {
		m.SortByColumn(key, ascending)
	}
}

// SortByAge sorts the table by the first column whose key contains "time",
// "date", "created", "launch", or "age".
func (m *ResourceListModel) SortByAge(ascending bool) {
	for _, suffix := range []string{"time", "date", "created", "launch", "age", "accessed", "changed"} {
		key := m.findColumnKey(suffix)
		if key != "" {
			m.SortByColumn(key, ascending)
			return
		}
	}
}

// findColumnKey returns the key of the first column whose key contains the
// given substring (case-insensitive). Returns "" if no match is found.
func (m *ResourceListModel) findColumnKey(substr string) string {
	lower := strings.ToLower(substr)
	for _, col := range m.TypeDef.Columns {
		if strings.Contains(strings.ToLower(col.Key), lower) {
			return col.Key
		}
	}
	return ""
}
