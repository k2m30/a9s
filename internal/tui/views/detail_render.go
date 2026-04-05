// detail_render.go contains YAML/plain content generation and config-driven rendering for DetailModel.
// Specifically: RawYAML, PlainContent, renderContent, computeKeyWidth, renderFromConfig.
package views

import (
	"reflect"
	"sort"
	"strings"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"

	"gopkg.in/yaml.v3"
)

// RawYAML returns the resource as YAML for clipboard copy (same format as YAML view).
func (m DetailModel) RawYAML() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(m.res.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(m.res.Fields) > 0 {
		data, err = yaml.Marshal(m.res.Fields)
	}

	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

// PlainContent returns the detail content as plain text (no ANSI) for clipboard copy.
func (m DetailModel) PlainContent() string {
	content := m.renderContent()
	// Strip ANSI escape codes
	result := make([]byte, 0, len(content))
	i := 0
	for i < len(content) {
		if content[i] == '\x1b' && i+1 < len(content) && content[i+1] == '[' {
			// Skip until we hit a letter
			j := i + 2
			for j < len(content) && (content[j] < 'a' || content[j] > 'z') && (content[j] < 'A' || content[j] > 'Z') {
				j++
			}
			if j < len(content) {
				j++ // skip the letter
			}
			i = j
		} else {
			result = append(result, content[i])
			i++
		}
	}
	return string(result)
}

// renderContent builds the styled key-value lines from the resource.
func (m DetailModel) renderContent() string {
	// Use structured field list when available.
	if m.fieldList != nil {
		return m.renderFromFieldList()
	}

	// Try config-driven rendering.
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.resourceType)
		if len(vd.Detail) > 0 {
			keyW := computeKeyWidth(vd.Detail)
			kv := func(key, val string) string {
				return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
			}
			if lines := m.renderFromConfig(kv); len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
		}
	}

	// Fallback: render from Fields map (no config or no matching ViewDef).
	if len(m.res.Fields) == 0 {
		return styles.DimText.Render("  No detail data available")
	}

	// Sort keys for stable output.
	fieldKeys := make([]string, 0, len(m.res.Fields))
	for k := range m.res.Fields {
		fieldKeys = append(fieldKeys, k)
	}
	sort.Strings(fieldKeys)

	keyW := computeKeyWidth(fieldKeys)
	kv := func(key, val string) string {
		return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
	}

	var lines []string
	for _, k := range fieldKeys {
		lines = append(lines, kv(k, m.res.Fields[k]))
	}
	return strings.Join(lines, "\n")
}

// computeKeyWidth returns the width needed for the key column: longest key + 1 (for colon), minimum 22.
func computeKeyWidth(keys []string) int {
	w := 22
	for _, k := range keys {
		if len(k)+1 > w {
			w = len(k) + 1
		}
	}
	return w
}

// renderFromConfig looks up the correct ViewDef by resource type and renders detail lines.
// Tries RawStruct extraction first, then falls back to Fields map for each path.
// Empty/nil fields are shown as "-" (not skipped).
func (m DetailModel) renderFromConfig(kv func(string, string) string) []string {
	vd := config.GetViewDef(m.viewConfig, m.resourceType)
	if len(vd.Detail) == 0 {
		return nil
	}
	var lines []string
	for _, path := range vd.Detail {
		val := ""
		// Try Fields map first — fetchers populate Fields with pre-formatted
		// values (e.g., formatted timestamps instead of raw epoch ms).
		if len(m.res.Fields) > 0 {
			// Try exact case-insensitive match
			for k, v := range m.res.Fields {
				if strings.EqualFold(k, path) {
					val = v
					break
				}
			}
			// Try underscore-separated version: "InstanceId" → "instance_id"
			if val == "" {
				snakeKey := fieldpath.ToSnakeCase(path)
				if v, ok := m.res.Fields[snakeKey]; ok {
					val = v
				}
			}
		}
		// Fall back to RawStruct extraction for fields not in Fields map
		if val == "" && m.res.RawStruct != nil {
			val = fieldpath.ExtractSubtree(m.res.RawStruct, path)
		}
		if val == "" {
			val = "-"
		}
		if strings.Contains(val, "\n") {
			lines = append(lines, " "+styles.DetailSection.Render(path+":"))
			for subLine := range strings.SplitSeq(val, "\n") {
				lines = append(lines, "     "+styles.DetailVal.Render(subLine))
			}
		} else {
			lines = append(lines, kv(path, val))
		}
	}
	return lines
}

