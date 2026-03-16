package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/k2m30/a9s/internal/fieldpath"
)

// DetailModel represents a key-value detail view for a single resource.
type DetailModel struct {
	Title  string
	Data   map[string]string
	Keys   []string // ordered keys for display
	Offset int      // scroll offset
	Width  int
	Height int

	// Config-driven detail fields (nil = use legacy Data/Keys rendering)
	RawStruct   interface{} // raw AWS SDK struct for reflection
	DetailPaths []string    // configured dot-notation paths
}

// NewDetailModel creates a new DetailModel with sorted keys (legacy mode).
func NewDetailModel(title string, data map[string]string) DetailModel {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return DetailModel{
		Title:  title,
		Data:   data,
		Keys:   keys,
		Offset: 0,
	}
}

// NewConfigDetailModel creates a DetailModel that renders fields by extracting
// them from rawStruct using the given dot-notation detailPaths.
func NewConfigDetailModel(title string, rawStruct interface{}, detailPaths []string) DetailModel {
	return DetailModel{
		Title:       title,
		RawStruct:   rawStruct,
		DetailPaths: detailPaths,
		Offset:      0,
	}
}

// View renders the detail view as key-value pairs.
func (m DetailModel) View() string {
	if len(m.DetailPaths) > 0 && m.RawStruct != nil {
		return m.viewConfig()
	}
	return m.viewLegacy()
}

// viewConfig renders the config-driven detail view using fieldpath extraction.
func (m DetailModel) viewConfig() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", m.Title))

	if len(m.DetailPaths) == 0 {
		b.WriteString("  No details available.\n")
		return b.String()
	}

	// Build rendered lines first so we can compute alignment for scalar keys
	type entry struct {
		path    string
		value   string
		isMulti bool
	}

	entries := make([]entry, 0, len(m.DetailPaths))
	for _, path := range m.DetailPaths {
		val := fieldpath.ExtractSubtree(m.RawStruct, path)
		isMulti := val != "" && strings.Contains(val, "\n")
		entries = append(entries, entry{path: path, value: val, isMulti: isMulti})
	}

	// Find the longest scalar key for alignment
	maxKeyLen := 0
	for _, e := range entries {
		if !e.isMulti && len(e.path) > maxKeyLen {
			maxKeyLen = len(e.path)
		}
	}

	for _, e := range entries {
		if e.isMulti {
			// Multi-line (YAML subtree): render header + indented lines
			b.WriteString(fmt.Sprintf("  %s:\n", e.path))
			for _, line := range strings.Split(e.value, "\n") {
				b.WriteString(fmt.Sprintf("    %s\n", line))
			}
		} else {
			// Scalar: render as key : value
			b.WriteString(fmt.Sprintf("  %-*s : %s\n", maxKeyLen, e.path, e.value))
		}
	}

	return b.String()
}

// viewLegacy renders the legacy map-based detail view with sorted keys.
func (m DetailModel) viewLegacy() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", m.Title))

	if len(m.Keys) == 0 {
		b.WriteString("  No details available.\n")
		return b.String()
	}

	// Find the longest key for alignment
	maxKeyLen := 0
	for _, k := range m.Keys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	// Apply scroll offset
	end := len(m.Keys)
	start := m.Offset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		k := m.Keys[i]
		v := m.Data[k]
		b.WriteString(fmt.Sprintf("  %-*s : %s\n", maxKeyLen, k, v))
	}

	return b.String()
}

// ScrollUp scrolls the view up by one line.
func (m *DetailModel) ScrollUp() {
	if m.Offset > 0 {
		m.Offset--
	}
}

// ScrollDown scrolls the view down by one line.
func (m *DetailModel) ScrollDown() {
	if m.Offset < len(m.Keys)-1 {
		m.Offset++
	}
}

// GoTop scrolls to the top of the detail view.
func (m *DetailModel) GoTop() {
	m.Offset = 0
}

// GoBottom scrolls to the bottom of the detail view.
func (m *DetailModel) GoBottom() {
	max := len(m.Keys) - m.Height
	if max < 0 {
		max = 0
	}
	m.Offset = max
}
