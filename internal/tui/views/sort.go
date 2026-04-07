package views

import (
	"sort"
	"strings"
)

// SortField identifies the active sort column.
type SortField int

const (
	SortNone SortField = iota
	SortName
	SortID
	SortAge
)

// isAgeKey reports whether a field key represents a time-related field.
func isAgeKey(key string) bool {
	kl := strings.ToLower(key)
	return strings.Contains(kl, "time") || strings.Contains(kl, "date") ||
		strings.Contains(kl, "launch") || strings.Contains(kl, "creation") ||
		strings.Contains(kl, "event") || strings.Contains(kl, "start") ||
		strings.Contains(kl, "timestamp")
}

// ageFieldKey returns the deterministic field key for age sorting.
// It scans the resolved columns in order and returns the first time-related key.
// Config-driven columns may have empty keys, so it falls back to typeDef columns
// which always carry canonical field keys.
//
// Special-case: ct-events stores a display-formatted "time" field for rendering
// and a raw RFC3339 "event_time" field for sorting — return "event_time" so
// the comparator gets a sortable value. The display column (TIME) still shows
// the sort glyph because sortColKey="time" (set in NewResourceList) is matched
// by the TIME column key independently of the comparator field.
func (m ResourceListModel) ageFieldKey() string {
	if m.typeDef.ShortName == "ct-events" {
		return "event_time"
	}
	for _, c := range m.resolveColumns() {
		if isAgeKey(c.key) {
			return c.key
		}
	}
	// Fallback: scan typeDef columns which always have canonical keys.
	for _, c := range m.typeDef.Columns {
		if isAgeKey(c.Key) {
			return c.Key
		}
	}
	return ""
}

// sortFiltered sorts filteredResources in place based on current sort settings.
func (m *ResourceListModel) sortFiltered() {
	if m.sort == SortNone {
		return
	}

	var ageKey string
	if m.sort == SortAge {
		ageKey = m.ageFieldKey()
	}

	sort.SliceStable(m.filteredResources, func(i, j int) bool {
		a := m.filteredResources[i]
		b := m.filteredResources[j]
		var va, vb string
		switch m.sort {
		case SortName:
			va, vb = a.Name, b.Name
		case SortID:
			va, vb = a.ID, b.ID
		case SortAge:
			va, vb = a.Fields[ageKey], b.Fields[ageKey]
		}
		if m.sortAsc {
			return va < vb
		}
		return va > vb
	})
}
