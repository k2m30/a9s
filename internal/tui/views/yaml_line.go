// yaml_line.go — shared YAML line tokenization and plain-text formatting.
// Used by both the YAML document view and the detail field-list view
// so that markers and spacing stay identical across cursor states and views.
package views

import "strings"

// yamlLine holds the parsed tokens of a single YAML line.
// Leading whitespace is NOT stored — callers own indentation.
type yamlLine struct {
	Dash  string // "- " or ""
	Key   string // key name (empty for bare list items)
	Value string // value after colon (empty for parent keys or bare items)
	Raw   string // bare content for lines that don't match key: value
}

// parseYAMLLine tokenizes a raw YAML line (may include leading whitespace)
// into its structural components. Uses the package-level splitYAMLKeyValue
// helper so key/value detection matches colorizeYAML exactly (bare colons
// inside an unquoted key are never treated as the separator).
func parseYAMLLine(raw string) yamlLine {
	trimmed := strings.TrimSpace(raw)
	dash := ""
	rest := trimmed
	if r, ok := strings.CutPrefix(trimmed, "- "); ok {
		dash = "- "
		rest = r
	}
	if key, val, ok := splitYAMLKeyValue(rest); ok && key != "" {
		return yamlLine{
			Dash:  dash,
			Key:   key,
			Value: strings.TrimSpace(val),
		}
	}
	if dash != "" {
		return yamlLine{Dash: dash, Raw: rest}
	}
	return yamlLine{Raw: trimmed}
}

// plain returns the canonical plain-text form of the line.
// Spacing is always: [dash] key ": " value (one space after the colon).
func (l yamlLine) plain() string {
	if l.Key != "" {
		s := l.Dash + l.Key + ":"
		if l.Value != "" {
			s += " " + l.Value
		}
		return s
	}
	return l.Dash + l.Raw
}
