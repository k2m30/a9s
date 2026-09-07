// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

// GenerateViewYAML returns the YAML bytes for a single ViewDef, stamped with
// the build that generated it.
func GenerateViewYAML(v ViewDef) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "generated: %d\n", GeneratedViewsVersion)

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
			if col.SortKey != "" {
				fmt.Fprintf(&b, "    sort_key: %s\n", col.SortKey)
			}
			if col.Humanize {
				b.WriteString("    humanize: true\n")
			}
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

// EnsureViewsDir writes any missing built-in view YAML files to dir and brings
// files written by an older build up to this one's column set.
//
// A file already stamped with this build is left byte for byte alone: whatever
// the operator did to it is theirs. A file with an older stamp (or none, which
// is every file written before the stamp existed) keeps every column it has,
// with the widths, paths and keys it has, and gains only the built-in columns
// whose titles it does not carry — each at the position it holds in the
// built-in set, so a column added in the middle does not land at the far right
// of the operator's table.
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
		data, readErr := os.ReadFile(dest) //nolint:gosec // dest is built from the caller's own config dir
		switch {
		case readErr != nil && !os.IsNotExist(readErr):
			return readErr
		case readErr != nil:
			data = GenerateViewYAML(cfg.Views[name])
		default:
			merged, changed := mergeGeneratedColumns(data, cfg.Views[name])
			if !changed {
				continue
			}
			data = merged
		}
		if writeErr := os.WriteFile(dest, data, 0644); writeErr != nil { //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
			return writeErr
		}
	}
	return nil
}

// mergeGeneratedColumns returns the YAML for onDisk with def's missing columns
// added, and changed=false when there is nothing to do: onDisk already carries
// this build's stamp, or it does not parse, which is the operator's problem to
// see and fix rather than this function's to overwrite.
func mergeGeneratedColumns(onDisk []byte, def ViewDef) ([]byte, bool) {
	vd, err := ParseSingle(onDisk)
	if err != nil || vd.Generated >= GeneratedViewsVersion {
		return nil, false
	}
	have := make(map[string]bool, len(vd.List))
	for _, c := range vd.List {
		have[c.Title] = true
	}
	for i, c := range def.List {
		if have[c.Title] {
			continue
		}
		at := min(i, len(vd.List))
		vd.List = append(vd.List[:at], append([]ListColumn{c}, vd.List[at:]...)...)
	}
	return GenerateViewYAML(*vd), true
}

// EnsureViewsReference writes the embedded views_reference.yaml to configDir.
// Always overwrites — this is generated reference data, not user-editable.
// Upgrades deliver updated field listings automatically.
func EnsureViewsReference(configDir string) error {
	dest := filepath.Join(configDir, "views_reference.yaml")
	return os.WriteFile(dest, viewsReferenceData, 0644) //nolint:gosec // view YAML files are non-sensitive, world-readable is acceptable
}
