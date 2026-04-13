package text

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

// TryJSONToYAMLLines attempts to parse s as JSON. On success, it converts the
// parsed structure to YAML and returns the individual lines. Returns nil if s
// is not valid JSON, or represents an empty object ({}) or empty array ([]).
func TryJSONToYAMLLines(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return nil
	}
	// Quick reject: must start with { or [
	if s[0] != '{' && s[0] != '[' {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil
	}
	// Skip empty objects and arrays
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
	out, err := yaml.Marshal(parsed)
	if err != nil {
		return nil
	}
	raw := strings.TrimRight(string(out), "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
