// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

import (
	"maps"
	"sync"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// defaultViews holds the built-in view definitions for all supported resource
// types. A view is two declarations from two owners: the DETAIL fields, which
// this package's per-category files below declare, and the LIST columns, which
// belong to the resource type — one column list, declared on the catalog entry
// beside the fetcher that fills it, and read from there.
//
// It is built on first use rather than at package-init time because the
// catalog is installed by aws.Install() at program start, which runs after
// every package-level initialiser. A caller that reaches a default view before
// the install gets catalog's own panic, which names the missing call.
var defaultViews = sync.OnceValue(buildDefaultViews)

// defaultDetails holds the per-type detail field lists. Paths use Go field
// names from AWS SDK v2 structs. ExtractValue matches case-insensitively.
func defaultDetails() map[string]ViewDef {
	m := make(map[string]ViewDef, 100)
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

func buildDefaultViews() map[string]ViewDef {
	m := defaultDetails()
	install := func(td catalog.ResourceTypeDef) {
		v := m[td.ShortName]
		v.List = catalogListColumns(td)
		m[td.ShortName] = v
	}
	for _, td := range catalog.All() {
		install(td)
	}
	for _, td := range catalog.AllChildren() {
		install(td)
	}
	return m
}

// catalogListColumns is the one translation of a catalog column into the view
// column the config hands out. Every field of the catalog column that decides
// what a cell shows is carried; Sortable is not, because it is not a view
// file's to say — it stays the type's own fact and is read off the catalog.
func catalogListColumns(td catalog.ResourceTypeDef) []ListColumn {
	if len(td.Columns) == 0 {
		return nil
	}
	cols := make([]ListColumn, len(td.Columns))
	for i, c := range td.Columns {
		cols[i].Title = c.Title
		cols[i].Path = c.Path
		cols[i].Key = c.Key
		cols[i].Width = c.Width
		cols[i].SortKey = c.SortKey
	}
	return cols
}

// sharedReadOnlyDefault is the lazily-built, never-mutated default config
// instance shared across all callers that need read-only access.
var sharedReadOnlyDefault = sync.OnceValue(buildDefaultConfig)

func buildDefaultConfig() *ViewsConfig {
	views := defaultViews()
	cp := ViewsConfig{
		Views: make(map[string]ViewDef, len(views)),
	}
	for k, v := range views {
		cols := make([]ListColumn, len(v.List))
		copy(cols, v.List)
		detail := make([]DetailField, len(v.Detail))
		copy(detail, v.Detail)
		cp.Views[k] = ViewDef{List: cols, Detail: detail}
	}
	return &cp
}

// DefaultConfig returns a copy of the built-in default configuration.
// Each call returns a new, independently-mutable copy.
func DefaultConfig() *ViewsConfig {
	return buildDefaultConfig()
}

// SharedDefaultConfig returns the shared read-only default configuration.
// The caller MUST NOT mutate the returned value or any of its nested slices/maps.
// Use DefaultConfig when a mutable copy is required.
func SharedDefaultConfig() *ViewsConfig {
	return sharedReadOnlyDefault()
}

// DefaultViewDef returns the built-in default ViewDef for the given resource
// short name. Returns an empty ViewDef if no default exists for the name.
func DefaultViewDef(shortName string) ViewDef {
	v, ok := defaultViews()[shortName]
	if !ok {
		return ViewDef{}
	}
	// Return a copy so callers cannot mutate the package-level defaults.
	cols := make([]ListColumn, len(v.List))
	copy(cols, v.List)
	detail := make([]DetailField, len(v.Detail))
	copy(detail, v.Detail)
	return ViewDef{List: cols, Detail: detail}
}
