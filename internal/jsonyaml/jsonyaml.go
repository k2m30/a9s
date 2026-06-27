// Package jsonyaml holds renderer-free JSON→YAML text helpers shared by the
// app core (internal/semantics/projection) and the TUI views. It lives outside
// internal/tui so shared-core packages can use it without transitively pulling
// in lipgloss (SC-009): the lipgloss-dependent text helpers stay in
// internal/tui/text.
package jsonyaml

import (
	"bytes"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLIndentSpaces is the indent width emitted by TryJSONToYAMLLines. Callers
// that infer nesting depth from leading-space counts MUST divide by this
// constant rather than hard-coding 2 — it is configured via yaml.NewEncoder
// because yaml.Marshal's default is 4.
const YAMLIndentSpaces = 2

// TryJSONToYAMLLines attempts to parse s as JSON. On success, it converts the
// parsed structure to YAML and returns the individual lines. Returns nil if s
// is not valid JSON, or represents an empty object ({}) or empty array ([]).
//
// Lines are indented at YAMLIndentSpaces spaces per nesting level.
func TryJSONToYAMLLines(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return nil
	}
	if s[0] != '{' && s[0] != '[' {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil
	}
	switch v := parsed.(type) {
	case map[string]any:
		if len(v) == 0 {
			return nil
		}
	case []any:
		if len(v) == 0 {
			return nil
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(YAMLIndentSpaces)
	if err := enc.Encode(parsed); err != nil {
		return nil
	}
	if err := enc.Close(); err != nil {
		return nil
	}
	raw := strings.TrimRight(buf.String(), "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
