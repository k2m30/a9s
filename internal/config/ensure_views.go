package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed views_reference.yaml
var viewsReferenceData []byte

// GenerateViewYAML returns the YAML bytes for a single ViewDef.
func GenerateViewYAML(v ViewDef) []byte {
	var b strings.Builder

	if len(v.List) > 0 {
		b.WriteString("list:\n")
		for _, col := range v.List {
			fmt.Fprintf(&b, "  %s:\n", yamlKey(col.Title))
			if col.Path != "" {
				fmt.Fprintf(&b, "    path: %s\n", col.Path)
			}
			if col.Key != "" {
				fmt.Fprintf(&b, "    key: %s\n", col.Key)
			}
			fmt.Fprintf(&b, "    width: %d\n", col.Width)
		}
	}

	if len(v.Detail) > 0 {
		if len(v.List) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("detail:\n")
		for _, df := range v.Detail {
			switch {
			case df.Key != "" && df.Label != "":
				fmt.Fprintf(&b, "  - { key: %s, label: %q }\n", df.Key, df.Label)
			case df.Key != "":
				fmt.Fprintf(&b, "  - { key: %s }\n", df.Key)
			case df.Path != "" && df.Label != "":
				fmt.Fprintf(&b, "  - { path: %s, label: %q }\n", df.Path, df.Label)
			default:
				// Bare path-form (no label) stays as a simple string for readability.
				fmt.Fprintf(&b, "  - %s\n", df.Path)
			}
		}
	}

	return []byte(b.String())
}

// yamlKey quotes a YAML key that contains special characters.
func yamlKey(s string) string {
	if strings.ContainsAny(s, "#:{}[]&*?|>!%@`") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// EnsureViewsDir writes any missing built-in view YAML files to dir.
// Existing files are never overwritten (user may have edited them).
func EnsureViewsDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	cfg := DefaultConfig()
	keys := make([]string, 0, len(cfg.Views))
	for k := range cfg.Views {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		dest := filepath.Join(dir, name+".yaml")
		if _, statErr := os.Stat(dest); statErr == nil {
			continue // file exists, skip
		}
		data := GenerateViewYAML(cfg.Views[name])
		if writeErr := os.WriteFile(dest, data, 0644); writeErr != nil { //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
			return writeErr
		}
	}
	return nil
}

// EnsureViewsReference writes the embedded views_reference.yaml to configDir.
// Always overwrites — this is generated reference data, not user-editable.
// Upgrades deliver updated field listings automatically.
func EnsureViewsReference(configDir string) error {
	dest := filepath.Join(configDir, "views_reference.yaml")
	return os.WriteFile(dest, viewsReferenceData, 0644) //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
}

