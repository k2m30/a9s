// Package config handles loading and parsing of view configuration from YAML files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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

// ViewDef defines the list and detail view configuration for a single resource type.
type ViewDef struct {
	List   []ListColumn  `yaml:"-"`
	Detail []DetailField `yaml:"-"`
}

// DetailStrings returns the canonical string identifiers of all DetailFields
// (Path if set, else Key). Useful for test assertions and debug logging.
func DetailStrings(fields []DetailField) []string {
	s := make([]string, len(fields))
	for i, df := range fields {
		s[i] = df.String()
	}
	return s
}

// ListColumn is a named column with its configuration, preserving YAML map order.
type ListColumn struct {
	Title    string `yaml:"-"`
	Path     string `yaml:"path"`
	Key      string `yaml:"key"`
	Width    int    `yaml:"width"`
	SortKey  string `yaml:"sort_key"`  // optional: Fields key for sorting (when display value differs from sort value)
	SortPath string `yaml:"sort_path"` // optional: RawStruct path for raw numeric/time sort comparison
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
		return nil, nil
	}

	return &ViewsConfig{Views: merged}, nil
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
