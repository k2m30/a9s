// Package config handles loading and parsing of view configuration from YAML files.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ViewsConfig is the top-level YAML structure parsed from views.yaml.
type ViewsConfig struct {
	Views map[string]ViewDef `yaml:"views"`
}

// ViewDef defines the list and detail view configuration for a single resource type.
type ViewDef struct {
	List   []ListColumn `yaml:"-"`
	Detail []string     `yaml:"detail"`
}

// ListColumn is a named column with its configuration, preserving YAML map order.
type ListColumn struct {
	Title string `yaml:"-"`
	Path  string `yaml:"path"`
	Key   string `yaml:"key"`
	Width int    `yaml:"width"`
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
			var details []string
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

// Load discovers and loads the first views.yaml found in the standard
// lookup chain:
//  1. ./views.yaml (current working directory)
//  2. $A9S_CONFIG_FOLDER/views.yaml
//  3. ~/.a9s/views.yaml
//
// Returns (nil, nil) when no config file is found.
func Load() (*ViewsConfig, error) {
	paths := lookupPaths()
	return LoadFrom(paths)
}

// LoadFrom tries each path in order and loads the first file that exists.
// Returns (nil, nil) when none of the paths exist.
// Returns an error if a file is found but cannot be parsed.
func LoadFrom(paths []string) (*ViewsConfig, error) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		cfg, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}
		return cfg, nil
	}
	return nil, nil
}

// Parse decodes raw YAML bytes into a ViewsConfig.
func Parse(data []byte) (*ViewsConfig, error) {
	var cfg ViewsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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

// lookupPaths returns the ordered list of candidate config file paths.
func lookupPaths() []string {
	var paths []string

	// 1. Current working directory
	paths = append(paths, "views.yaml")

	// 2. $A9S_CONFIG_FOLDER
	if folder := os.Getenv("A9S_CONFIG_FOLDER"); folder != "" {
		paths = append(paths, filepath.Join(folder, "views.yaml"))
	}

	// 3. ~/.a9s/
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".a9s", "views.yaml"))
	}

	return paths
}
