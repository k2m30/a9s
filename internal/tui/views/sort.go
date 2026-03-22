package views

import (
	"sort"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// SortField identifies the active sort column.
type SortField int

const (
	SortNone SortField = iota
	SortName
	SortID
	SortAge
)

// sortFiltered sorts filteredResources in place based on current sort settings.
func (m *ResourceListModel) sortFiltered() {
	if m.sort == SortNone {
		return
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
			// Use ID as fallback for age sorting (launch_time/creation_date fields)
			va = m.getAgeField(a)
			vb = m.getAgeField(b)
		}
		if m.sortAsc {
			return va < vb
		}
		return va > vb
	})
}

// getAgeField extracts the time-related field for age sorting.
func (m ResourceListModel) getAgeField(r resource.Resource) string {
	for k, v := range r.Fields {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "time") || strings.Contains(kl, "date") ||
			strings.Contains(kl, "launch") || strings.Contains(kl, "creation") {
			return v
		}
	}
	return ""
}
