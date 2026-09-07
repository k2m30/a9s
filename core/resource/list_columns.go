// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import "github.com/k2m30/a9s/v3/core/config"

// ResolveListColumnCascade resolves the list column set for typeName: prefer
// vc's per-session ViewDef.List when non-empty, else the built-in default
// ViewDef.List when it is a strict superset of td.Columns (guarded by a
// first-column-title match so a custom td with a different layout is not
// silently switched to the built-in defaults), else td.Columns (carrying
// Path/SortKey/SortPath/Humanize from the defaults by title match), else the
// raw built-in defaults.
//
// The only resolver of the column-set cascade. core/app's
// resolveListColumnsForBuild (session-view-config-aware, used for rendering,
// for the sort comparator and for the cache-save projection) and
// core/runtime's resolveSaveColumns (always vc=nil, the cache-save fallback
// lane) both delegate here, so no two callers can disagree about which
// columns a resource type renders.
func ResolveListColumnCascade(vc *config.ViewsConfig, typeName string, td *ResourceTypeDef) []config.ListColumn {
	if vc != nil {
		if vd := config.GetViewDef(vc, typeName); len(vd.List) > 0 {
			return copyListColumns(vd.List)
		}
	}

	defaultVD := config.GetViewDef(nil, typeName)

	if td != nil && len(defaultVD.List) > len(td.Columns) {
		firstMatch := len(td.Columns) == 0 ||
			(len(defaultVD.List) > 0 && defaultVD.List[0].Title == td.Columns[0].Title)
		if firstMatch {
			return copyListColumns(defaultVD.List)
		}
	}

	if td != nil && len(td.Columns) > 0 {
		defaultByTitle := make(map[string]config.ListColumn, len(defaultVD.List))
		for _, lc := range defaultVD.List {
			defaultByTitle[lc.Title] = lc
		}
		cols := make([]config.ListColumn, len(td.Columns))
		for i, c := range td.Columns {
			cd := config.ListColumn{Key: c.Key, Title: c.Title, Width: c.Width}
			if def, ok := defaultByTitle[c.Title]; ok {
				cd.Path = def.Path
				cd.SortKey = def.SortKey
				cd.SortPath = def.SortPath
				cd.Humanize = def.Humanize
			}
			cols[i] = cd
		}
		return cols
	}

	return copyListColumns(defaultVD.List)
}

// copyListColumns copies a view definition's columns whole. Every field a
// column carries decides something downstream — Path and Humanize what the
// cell shows, SortKey and SortPath what the comparator reads — so a branch
// that copies field by field is a branch that silently drops one.
func copyListColumns(src []config.ListColumn) []config.ListColumn {
	if len(src) == 0 {
		return nil
	}
	cols := make([]config.ListColumn, len(src))
	copy(cols, src)
	return cols
}
