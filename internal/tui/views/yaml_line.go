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
// into its structural components. Uses the package-level yamlKeyRe regex.
func parseYAMLLine(raw string) yamlLine {
	trimmed := strings.TrimSpace(raw)
	matches := yamlKeyRe.FindStringSubmatch(trimmed)
	if matches != nil {
		return yamlLine{
			Dash:  matches[1],
			Key:   matches[2],
			Value: strings.TrimSpace(matches[3]),
		}
	}
	if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
		return yamlLine{Dash: "- ", Raw: rest}
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
