package config

import "maps"

// defaultViews holds the built-in view definitions for all supported resource types.
// Paths use Go field names from AWS SDK v2 structs. ExtractValue matches case-insensitively.
var defaultViews = ViewsConfig{Views: mergeDefaultViews()}

func mergeDefaultViews() map[string]ViewDef {
	m := make(map[string]ViewDef, 80)
	maps.Copy(m, computeDefaultViews())
	maps.Copy(m, containersDefaultViews())
	maps.Copy(m, networkingDefaultViews())
	maps.Copy(m, databasesDefaultViews())
	maps.Copy(m, monitoringDefaultViews())
	maps.Copy(m, messagingDefaultViews())
	maps.Copy(m, secretsDefaultViews())
	maps.Copy(m, dnsCdnDefaultViews())
	maps.Copy(m, securityDefaultViews())
	maps.Copy(m, cicdDefaultViews())
	maps.Copy(m, dataDefaultViews())
	maps.Copy(m, backupDefaultViews())
	return m
}

// DefaultConfig returns a copy of the built-in default configuration.
func DefaultConfig() *ViewsConfig {
	cp := ViewsConfig{
		Views: make(map[string]ViewDef, len(defaultViews.Views)),
	}
	for k, v := range defaultViews.Views {
		cols := make([]ListColumn, len(v.List))
		copy(cols, v.List)
		detail := make([]string, len(v.Detail))
		copy(detail, v.Detail)
		cp.Views[k] = ViewDef{List: cols, Detail: detail}
	}
	return &cp
}

// DefaultViewDef returns the built-in default ViewDef for the given resource
// short name. Returns an empty ViewDef if no default exists for the name.
func DefaultViewDef(shortName string) ViewDef {
	v, ok := defaultViews.Views[shortName]
	if !ok {
		return ViewDef{}
	}
	// Return a copy so callers cannot mutate the package-level defaults.
	cols := make([]ListColumn, len(v.List))
	copy(cols, v.List)
	detail := make([]string, len(v.Detail))
	copy(detail, v.Detail)
	return ViewDef{List: cols, Detail: detail}
}
