// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package config handles loading and parsing of view configuration from YAML files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// DetailField references a single line in the detail view. Exactly one of
// Path or Key is set:
//   - Path = SDK struct field path (resolved via reflection on RawStruct)
//   - Key  = Resource.Fields[] map key (e.g. fetcher-populated "is_logging")
//
// Label is optional — when empty, the renderer derives a label from
// Path (last segment) or Key (humanized).
type DetailField struct {
	Path  string `yaml:"path"`
	Key   string `yaml:"key"`
	Label string `yaml:"label"`
}

// String returns the canonical identifier (Path if set, else Key).
// Used for tests and debug logging that don't care which form it is.
func (d DetailField) String() string {
	if d.Path != "" {
		return d.Path
	}
	return d.Key
}

// DisplayLabel returns the user-facing label, falling back to Path's last
// segment or Key.
func (d DetailField) DisplayLabel() string {
	if d.Label != "" {
		return d.Label
	}
	if d.Path != "" {
		if idx := strings.LastIndex(d.Path, "."); idx >= 0 {
			return d.Path[idx+1:]
		}
		return d.Path
	}
	return d.Key
}

// UnmarshalYAML accepts both string-form ("- TrailARN") and map-form
// ("- {key: is_logging, label: Logging}").
func (d *DetailField) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		d.Path = value.Value
		return nil
	case yaml.MappingNode:
		// Use a temporary type to avoid infinite recursion on UnmarshalYAML.
		var tmp struct {
			Path  string `yaml:"path"`
			Key   string `yaml:"key"`
			Label string `yaml:"label"`
		}
		if err := value.Decode(&tmp); err != nil {
			return fmt.Errorf("decoding detail field: %w", err)
		}
		d.Path = tmp.Path
		d.Key = tmp.Key
		d.Label = tmp.Label
		if d.Path == "" && d.Key == "" {
			return fmt.Errorf("detail field requires either path or key")
		}
		if d.Path != "" && d.Key != "" {
			return fmt.Errorf("detail field cannot have both path and key")
		}
		return nil
	default:
		return fmt.Errorf("expected scalar or mapping for detail field, got kind %d", value.Kind)
	}
}

// ViewsConfig is the top-level YAML structure parsed from views.yaml.
type ViewsConfig struct {
	Views map[string]ViewDef `yaml:"views"`
}

// GeneratedViewsVersion stamps the view files this build generates. Bump it in
// the same change that ADDS a built-in column, or that CORRECTS any field of an
// existing one — where it reads its value from, how wide it is, whether it
// humanizes. EnsureViewsDir reads the stamp off a file already on disk and,
// when it is older, adds the columns this build ships that the file has never
// heard of and carries every correction listed in viewColumnChanges onto the
// fields the operator has not touched. Without the stamp an operator who ran
// a9s once keeps the columns of that day forever — and without the correction
// half, they keep a column reading the field, or the raw constant, that the
// correction moved away from, which is worse than a missing column because it
// looks like it works.
//
// Those two are its whole reach. REMOVING a built-in column and RENAMING one
// are NOT handled, and bumping the stamp does not deliver them: the merge only
// ever adds and re-sources, so a removed column stays on the operator's disk,
// and a rename arrives as a title the file has never heard of — the operator
// keeps the old column beside the new one. Shipping either needs a migration
// this file does not have yet.
const GeneratedViewsVersion = 7

// ViewDef defines the list and detail view configuration for a single resource type.
type ViewDef struct {
	List   []ListColumn  `yaml:"-"`
	Detail []DetailField `yaml:"-"`
	// Generated is the GeneratedViewsVersion the file on disk was written with,
	// zero for a file written before the stamp existed.
	Generated int `yaml:"-"`
}

// DetailStringsForTest returns the canonical string identifiers of all
// DetailFields (Path if set, else Key). Test-only: no production caller —
// used by test assertions that need the field-identifier list as strings.
func DetailStringsForTest(fields []DetailField) []string {
	s := make([]string, len(fields))
	for i, df := range fields {
		s[i] = df.String()
	}
	return s
}

// ListColumn is a named column with its configuration, preserving YAML map order.
type ListColumn struct {
	Title string `yaml:"-"`
	Path  string `yaml:"path"`
	Key   string `yaml:"key"`
	Width int    `yaml:"width"`
	// SortKey is the Fields key the comparator reads when the displayed value
	// does not sort the way the value does (a size rendered "900 B", a status
	// rendered as a finding phrase). It is a stored key and not a RawStruct
	// path on purpose: the list opens on cache rows that have no RawStruct, so
	// a path-based comparison would order the warm frame differently from the
	// frame the fetch lands, and the rows would move under the operator.
	SortKey string `yaml:"sort_key"`
}

// TitleFieldKey is the single Fields-map spelling a column title answers to:
// lowercased, spaces as underscores. Fetchers already write snake_case keys,
// so a multi-word title resolves to the spelling the fetcher used instead of
// putting a second one beside it — a row that answers to one title two ways
// renders whichever key Go's map iteration hands back first, which is a
// different list on two consecutive starts.
func TitleFieldKey(title string) string {
	return strings.ReplaceAll(strings.ToLower(title), " ", "_")
}

// TitleFieldKeys are the spellings a column title answers to when it has to
// read a value back, most preferred first.
//
// The spaced spelling leads because of what is already on disk: a type file
// an older build wrote carries BOTH, the spaced one holding the value that
// build's screen showed and the underscored one holding the raw scalar its
// fetcher stored. A file this build writes carries the underscored key only,
// so the spaced lookup misses and costs nothing.
func TitleFieldKeys(title string) [2]string {
	return [2]string{strings.ToLower(title), TitleFieldKey(title)}
}

// IsStatusColumn reports whether a list column is the status/lifecycle
// column, whose cell is derived from Findings first and only then from a
// stored value. The type DECLARES it: its status column is the one naming the
// type's lifecycle key, and nothing else is one.
//
// It used to answer on the title as well — a column titled "Status" or
// "State" was the status column whatever it declared — and on the literal key
// "status". Both were inference, and the cascade that went with them read
// three spellings of the fact in a fixed order, so a type whose status column
// declared its own key (tg's health_summary, cb's last_build) showed whatever
// else happened to be in Fields and its declaration was consulted third.
//
// The render cascade, its decorator lookup and the status-column resolver all
// ask here, so none of the three can disagree about which column this is.
// Each resolves the type's lifecycle key first — "state" when the type
// declares none — so this takes the resolved key and does not default again.
func IsStatusColumn(key, lifecycleKey string) bool {
	return key != "" && key == lifecycleKey
}

// UnmarshalYAML implements custom unmarshaling for ViewDef to preserve
// the ordering of list columns from the YAML map.
func (v *ViewDef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node for ViewDef, got %d", value.Kind)
	}

	for i := 0; i < len(value.Content)-1; i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		switch key.Value {
		case "list":
			if val.Kind != yaml.MappingNode {
				return fmt.Errorf("expected mapping node for 'list', got %d", val.Kind)
			}
			cols, err := parseListColumns(val)
			if err != nil {
				return err
			}
			v.List = cols

		case "detail":
			var details []DetailField
			if err := val.Decode(&details); err != nil {
				return fmt.Errorf("decoding detail: %w", err)
			}
			v.Detail = details

		case "generated":
			if err := val.Decode(&v.Generated); err != nil {
				return fmt.Errorf("decoding generated: %w", err)
			}
		}
	}
	return nil
}

// parseListColumns extracts ordered columns from a YAML mapping node.
func parseListColumns(node *yaml.Node) ([]ListColumn, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node, got %d", node.Kind)
	}

	var cols []ListColumn
	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		var col ListColumn
		if err := valNode.Decode(&col); err != nil {
			return nil, fmt.Errorf("decoding column %q: %w", keyNode.Value, err)
		}
		col.Title = keyNode.Value
		cols = append(cols, col)
	}
	return cols, nil
}

// ParseSingle decodes raw YAML bytes for a single resource view definition.
// The YAML has no "views:" wrapper — it starts directly with "list:" and/or "detail:".
// Returns a non-nil ViewDef even for empty data.
func ParseSingle(data []byte) (*ViewDef, error) {
	var vd ViewDef
	if len(data) == 0 {
		return &vd, nil
	}
	if err := yaml.Unmarshal(data, &vd); err != nil {
		return nil, err
	}
	return &vd, nil
}

// LoadFromDirs scans directories for per-resource YAML files ({ShortName}.yaml).
// Directories are processed in order; later directories overlay earlier ones
// on a per-resource basis (project dir overlays global dir).
// Returns (nil, nil) when no directories exist or contain no .yaml files.
func LoadFromDirs(dirs []string) (*ViewsConfig, error) {
	merged := make(map[string]ViewDef)
	var unresolved []string

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".yaml") {
				continue
			}

			resourceName := strings.TrimSuffix(name, ".yaml")
			filePath := filepath.Join(dir, name)

			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", filePath, err)
			}

			vd, err := ParseSingle(data)
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", filePath, err)
			}

			// The file's name is the type it configures, resolved the way
			// every other lookup resolves one — parents and children, and the
			// canonical spelling whatever case or alias the file used. Storing
			// the file's own spelling meant "EC2.yaml" validated at load and
			// was then never read, because the runtime asks for "ec2".
			// Resolved after the parse so a malformed file is still a parse
			// error, whatever its name says.
			td := catalog.FindAny(resourceName)
			if td == nil {
				unresolved = append(unresolved, name)
				continue
			}
			resourceName = td.ShortName

			if existing, ok := merged[resourceName]; ok {
				if len(vd.List) > 0 {
					existing.List = vd.List
				}
				if len(vd.Detail) > 0 {
					existing.Detail = vd.Detail
				}
				merged[resourceName] = existing
			} else {
				merged[resourceName] = *vd
			}
		}
	}

	if len(merged) == 0 {
		// No file this build can use — but a file whose name names no type is
		// still a file the operator wrote, and the report is the only thing
		// that tells them the screen they are looking at is not theirs.
		return nil, loadReport(nil, unresolved)
	}

	cfg := &ViewsConfig{Views: merged}
	return cfg, loadReport(cfg, unresolved)
}

// loadReport is everything the load has to say about the files it read: a file
// whose name is no type this build has, and a column nothing can fill. Both
// are the same mistake — a line the operator wrote that reaches no screen —
// and both are reported rather than rejected, so the rest of the file is used.
func loadReport(cfg *ViewsConfig, unresolved []string) error {
	reports := make([]string, 0, len(unresolved))
	sort.Strings(unresolved)
	for _, name := range unresolved {
		reports = append(reports, fmt.Sprintf("%s: names no resource type this build has", name))
	}
	if cfg != nil {
		reports = append(reports, unfillableColumnKeys(cfg)...)
	}
	if len(reports) == 0 {
		return nil
	}
	return errors.New(strings.Join(reports, "; "))
}

// ColumnFilled reports whether anything puts a value in this column's cell on
// td: a RawStruct path it reads the value from, or a key some producer writes
// — the type's Wave 1 fields, its Wave 2 enricher's, a key the type's own
// columns name, or its status key, which the findings fill and the save lane
// stores.
//
// A Path is a producer, which is the half the report used to miss: the cascade
// reads the struct live, and MaterializeListFields writes that same value
// under the column's key for the row a restart replays. So a column naming
// both was reported as filled by nothing while rendering correctly on both
// lanes.
//
// The load's report and the producers gate ask here, so a column the gate
// accepts is never one the operator is told is broken.
func ColumnFilled(td catalog.ResourceTypeDef, col ListColumn) bool {
	if col.Path != "" || col.Key == "" {
		return true
	}
	if col.Key == "@id" || col.Key == td.StatusKey() {
		return true
	}
	if slices.Contains(td.FieldKeys, col.Key) || slices.Contains(td.IssueEnricherFieldKeys, col.Key) {
		return true
	}
	for _, c := range td.Columns {
		if c.Key == col.Key {
			return true
		}
	}
	return false
}

// unfillableColumnKeys reports every list column nothing can fill. Such a cell
// is empty on every row, forever, and a view file is the one thing in a9s a
// person is invited to edit, so the load says which file and which key rather
// than leaving them to wonder.
func unfillableColumnKeys(cfg *ViewsConfig) []string {
	names := make([]string, 0, len(cfg.Views))
	for name := range cfg.Views {
		names = append(names, name)
	}
	sort.Strings(names)

	var reports []string
	for _, name := range names {
		td := catalog.FindAny(name)
		if td == nil {
			continue
		}
		for _, col := range cfg.Views[name].List {
			if ColumnFilled(*td, col) {
				continue
			}
			reports = append(reports, fmt.Sprintf("%s.yaml: column %q reads key %q, which nothing on %s writes",
				name, col.Title, col.Key, name))
		}
	}
	return reports
}

// ReportText is the one sentence both lanes show for a config that did not
// load cleanly: the terminal flashes it, the web page carries it in the same
// flash, and the web server logs a copy.
//
// Whether the operator's file is in use is what changes the sentence, and the
// caller holds that fact at load time: a nil config means nothing loaded and
// the built-in defaults are in use, while a config with a report beside it is
// the operator's file, minus the one column that named nothing. "(using
// defaults)" used to be said in both cases, which was false in the second and
// the more alarming of the two to read.
//
// Returns "" when err is nil, so a caller can hand the result on unconditionally.
func ReportText(cfg *ViewsConfig, err error) string {
	switch {
	case err == nil:
		return ""
	case cfg == nil:
		return fmt.Sprintf("Config error: %v (using defaults)", err)
	default:
		return fmt.Sprintf("Config error: %v (the rest of your config is in use)", err)
	}
}

// Load discovers and loads per-resource YAML files from the standard
// lookup chain:
//  1. ConfigDir()/views/ (global defaults — env var or ~/.a9s/)
//  2. .a9s/views/ (per-project overrides in CWD)
//
// Later directories overlay earlier ones on a per-resource basis.
// Returns (nil, nil) when no config files are found.
func Load() (*ViewsConfig, error) {
	dirs := lookupDirs()
	return LoadFromDirs(dirs)
}

// GetViewDef returns the view definition for the given resource short name.
// If cfg is nil or the resource is not configured, built-in defaults are used.
// Partial configs are merged: missing List falls back to defaults, missing
// Detail falls back to defaults.
func GetViewDef(cfg *ViewsConfig, shortName string) ViewDef {
	def := DefaultViewDef(shortName)

	if cfg == nil {
		return def
	}
	userDef, ok := cfg.Views[shortName]
	if !ok {
		return def
	}

	// Merge: user-provided fields override defaults; empty fields fall back.
	if len(userDef.List) > 0 {
		def.List = userDef.List
	}
	if len(userDef.Detail) > 0 {
		def.Detail = userDef.Detail
	}
	return def
}

// ConfigDir returns the resolved a9s config directory path.
// Resolution order:
//  1. $A9S_CONFIG_FOLDER (if set and non-empty)
//  2. ~/.a9s/
//
// Returns empty string only if $HOME cannot be determined and $A9S_CONFIG_FOLDER is not set.
func ConfigDir() string {
	if folder := os.Getenv("A9S_CONFIG_FOLDER"); folder != "" {
		return folder
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".a9s")
	}
	return ""
}

// EnsureConfigDir ensures the config directory exists, creating it if needed.
// Returns the resolved config directory path.
// Returns an error if the directory cannot be created.
func EnsureConfigDir() (string, error) {
	dir := ConfigDir()
	if dir == "" {
		return "", fmt.Errorf("cannot determine config directory: $HOME not set and $A9S_CONFIG_FOLDER not set")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("creating config directory %s: %w", dir, err)
	}
	return dir, nil
}

// ConfigFilePath returns the full path to a file within the config directory.
// Returns empty string if the config directory cannot be determined.
func ConfigFilePath(filename string) string {
	if dir := ConfigDir(); dir != "" {
		return filepath.Join(dir, filename)
	}
	return ""
}

// lookupDirs returns the ordered list of view config directories.
// Global dir is listed first, project dir second (so project overlays global).
func lookupDirs() []string {
	var dirs []string

	// 1. Global config directory (env var or ~/.a9s/)
	if dir := ConfigDir(); dir != "" {
		dirs = append(dirs, filepath.Join(dir, "views"))
	}

	// 2. CWD .a9s/views/ directory (per-project overrides)
	dirs = append(dirs, filepath.Join(".a9s", "views"))

	return dirs
}
