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

	// Horizontal scroll support
	HScrollOffset int

	// Word wrap toggle
	WrapEnabled bool
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

// wrapLine wraps a line to fit within the given width.
func wrapLine(line string, width int) []string {
	if width <= 0 || len(line) <= width {
		return []string{line}
	}
	var wrapped []string
	for len(line) > width {
		wrapped = append(wrapped, line[:width])
		line = line[width:]
	}
	if len(line) > 0 {
		wrapped = append(wrapped, line)
	}
	return wrapped
}

// applyHScroll crops a line by horizontal scroll offset.
func (m DetailModel) applyHScroll(line string) string {
	if m.WrapEnabled || m.HScrollOffset <= 0 {
		return line
	}
	if m.HScrollOffset >= len(line) {
		return ""
	}
	return line[m.HScrollOffset:]
}

// viewConfig renders the config-driven detail view using fieldpath extraction.
func (m DetailModel) viewConfig() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", m.Title))

	if len(m.DetailPaths) == 0 {
		b.WriteString("  No details available.\n")
		return b.String()
	}

	// Build rendered lines first
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

	for _, e := range entries {
		if e.isMulti {
			line := fmt.Sprintf("  %s:", e.path)
			if m.WrapEnabled && m.Width > 0 {
				for _, wl := range wrapLine(line, m.Width) {
					b.WriteString(wl)
					b.WriteString("\n")
				}
			} else {
				b.WriteString(m.applyHScroll(line))
				b.WriteString("\n")
			}
			for _, subline := range strings.Split(e.value, "\n") {
				indented := fmt.Sprintf("    %s", subline)
				if m.WrapEnabled && m.Width > 0 {
					for _, wl := range wrapLine(indented, m.Width) {
						b.WriteString(wl)
						b.WriteString("\n")
					}
				} else {
					b.WriteString(m.applyHScroll(indented))
					b.WriteString("\n")
				}
			}
		} else {
			// Scalar: render as "Key: value" (colon right after key)
			line := fmt.Sprintf("  %s: %s", e.path, e.value)
			if m.WrapEnabled && m.Width > 0 {
				for _, wl := range wrapLine(line, m.Width) {
					b.WriteString(wl)
					b.WriteString("\n")
				}
			} else {
				b.WriteString(m.applyHScroll(line))
				b.WriteString("\n")
			}
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

	// Apply scroll offset
	end := len(m.Keys)
	start := m.Offset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		k := m.Keys[i]
		v := m.Data[k]
		// "Key: value" format (colon right after key)
		line := fmt.Sprintf("  %s: %s", k, v)
		if m.WrapEnabled && m.Width > 0 {
			for _, wl := range wrapLine(line, m.Width) {
				b.WriteString(wl)
				b.WriteString("\n")
			}
		} else {
			b.WriteString(m.applyHScroll(line))
			b.WriteString("\n")
		}
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

// ToggleWrap toggles word wrap mode and resets horizontal scroll when enabled.
func (m *DetailModel) ToggleWrap() {
	m.WrapEnabled = !m.WrapEnabled
	if m.WrapEnabled {
		m.HScrollOffset = 0
	}
}
