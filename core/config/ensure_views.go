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
// is every file written before the stamp existed) keeps every column it has and
// gains the built-in columns whose titles it does not carry — each at the
// position it holds in the built-in set, so a column added in the middle does
// not land at the far right of the operator's table.
//
// A column it does carry keeps the width, path and key it has, with one
// exception: a built-in column whose source this build corrected, and which the
// file still reads from the source the older build generated, takes the new
// source — and the new width too, while the width on disk is still the one that
// older build wrote. Such a column has never been edited, and leaving it behind
// would leave the operator reading the field the correction moved away from.
// Any other source under that title is the operator's own and is left alone, as
// is a width they changed. viewColumnSourceChanges is the list of corrections.
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
			merged, changed := mergeGeneratedColumns(name, data, cfg.Views[name])
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

// viewColumnSourceChange records one built-in column whose SOURCE changed, and
// the source the build before Version generated for it. The corrected source is
// deliberately not recorded here: it is whatever the defaults now say, which is
// the one place a built-in column is defined.
//
// A column that changes source keeps its title, so the missing-title rule below
// never sees it and an operator who had run a9s once kept reading the field the
// column was corrected away from — a cell that looks like it works and is
// wrong. Matching on the previous generated source is what separates a column
// nobody has touched from one the operator set themselves: theirs does not
// match, and is left exactly as it is.
type viewColumnSourceChange struct {
	Version  int    // the GeneratedViewsVersion that introduced the correction
	View     string // the view file's name, e.g. "sns"
	Title    string // the column title, unchanged by the correction
	WasPath  string // what the build before Version generated
	WasKey   string
	WasWidth int
}

// viewColumnSourceChanges is the migration table, scanned in order. Add a row
// in the same change that alters a built-in column's Path or Key, and bump
// GeneratedViewsVersion with it.
var viewColumnSourceChanges = []viewColumnSourceChange{
	{Version: 2, View: "sns", Title: "Topic Name", WasPath: "TopicArn", WasWidth: 40},
	{Version: 2, View: "sns-sub", Title: "Confirmed", WasPath: "SubscriptionArn", WasWidth: 22},
	{Version: 3, View: "alarm", Title: "Threshold", WasPath: "Threshold", WasWidth: 12},
	{Version: 3, View: "ecs-task", Title: "Task ID", WasPath: "TaskArn", WasWidth: 38},
	{Version: 3, View: "logs", Title: "Retention", WasPath: "RetentionInDays", WasWidth: 10},
	{Version: 3, View: "ng", Title: "Node Group", WasPath: "NodegroupName", WasWidth: 28},
	{Version: 3, View: "secrets", Title: "Last Accessed", WasPath: "LastAccessedDate", WasWidth: 18},
	{Version: 3, View: "secrets", Title: "Last Changed", WasPath: "LastChangedDate", WasWidth: 18},
	{Version: 3, View: "sns-sub", Title: "Subscription ARN", WasPath: "SubscriptionArn", WasWidth: 60},
}

// mergeGeneratedColumns returns the YAML for onDisk brought up to this build:
// def's columns whose titles are missing are added, and a column still carrying
// the source an older build generated for it takes this build's source and
// width. changed=false when there is nothing to do: onDisk already carries this
// build's stamp, or it does not parse, which is the operator's problem to see
// and fix rather than this function's to overwrite.
func mergeGeneratedColumns(name string, onDisk []byte, def ViewDef) ([]byte, bool) {
	vd, err := ParseSingle(onDisk)
	if err != nil || vd.Generated >= GeneratedViewsVersion {
		return nil, false
	}
	have := make(map[string]bool, len(vd.List))
	for _, c := range vd.List {
		have[c.Title] = true
	}
	want := make(map[string]ListColumn, len(def.List))
	for _, c := range def.List {
		want[c.Title] = c
	}
	for i, c := range def.List {
		if have[c.Title] {
			continue
		}
		at := min(i, len(vd.List))
		vd.List = append(vd.List[:at], append([]ListColumn{c}, vd.List[at:]...)...)
	}
	for _, ch := range viewColumnSourceChanges {
		if ch.View != name || ch.Version <= vd.Generated {
			continue
		}
		now, ok := want[ch.Title]
		if !ok {
			continue
		}
		for i, on := range vd.List {
			if on.Title != ch.Title || on.Path != ch.WasPath || on.Key != ch.WasKey {
				continue
			}
			vd.List[i].Path, vd.List[i].Key = now.Path, now.Key
			// The width follows only while it is still the one the older build
			// generated. A width the operator set is theirs even on a column
			// they never re-sourced, and the corrected source is what makes the
			// cell right — the width only makes it comfortable.
			if on.Width == ch.WasWidth {
				vd.List[i].Width = now.Width
			}
		}
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
