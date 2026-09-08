// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/config"
)

// ResolveListColumnCascade resolves the list column set for typeName: prefer
// vc's per-session ViewDef.List when non-empty, else the built-in default
// ViewDef.List when it is a strict superset of td.Columns, else td.Columns
// (carrying Path/SortKey/Humanize from the defaults by title match), else the
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
	if td == nil {
		return copyListColumns(defaultVD.List)
	}

	// Both lookups below are keyed case-insensitively: a catalog column and
	// the built-in view spell the same title differently often enough ("Time"
	// against "TIME") that an exact match silently drops what the other
	// declaration says about that very column.

	// The defaults hold columns the catalog does not, so they drive the order
	// and the widths. They do not drive what a cell reads: mergeListColumn
	// puts the catalog's Key back on every title both declare.
	if len(defaultVD.List) > len(td.Columns) {
		catalogKeyByTitle := make(map[string]string, len(td.Columns))
		for _, c := range td.Columns {
			catalogKeyByTitle[strings.ToLower(c.Title)] = c.Key
		}
		cols := make([]config.ListColumn, len(defaultVD.List))
		for i, lc := range defaultVD.List {
			cols[i] = mergeListColumn(lc.Title, lc.Width, catalogKeyByTitle[strings.ToLower(lc.Title)], lc)
		}
		return cols
	}

	if len(td.Columns) > 0 {
		defaultByTitle := make(map[string]config.ListColumn, len(defaultVD.List))
		for _, lc := range defaultVD.List {
			defaultByTitle[strings.ToLower(lc.Title)] = lc
		}
		cols := make([]config.ListColumn, len(td.Columns))
		for i, c := range td.Columns {
			cols[i] = mergeListColumn(c.Title, c.Width, c.Key, defaultByTitle[strings.ToLower(c.Title)])
		}
		return cols
	}

	return copyListColumns(defaultVD.List)
}

// mergeListColumn is the one merge of the two declarations of a column, called
// by both arms of the cascade so neither can read the pair its own way.
//
// The catalog owns what the cell reads: its Key wins wherever it declares one.
// The defaults own the rendering hints the catalog literal has no field for —
// Path, SortKey — and their own Key on a column the catalog does not
// declare at all. Title and width come from whichever declaration the calling
// arm is built around, which is the only thing the two arms still differ on.
func mergeListColumn(title string, width int, catalogKey string, def config.ListColumn) config.ListColumn {
	key := def.Key
	if catalogKey != "" {
		key = catalogKey
	}
	return config.ListColumn{
		Title:   title,
		Width:   width,
		Key:     key,
		Path:    def.Path,
		SortKey: def.SortKey,
	}
}

// copyListColumns copies a view definition's columns whole. Every field a
// column carries decides something downstream — Path what the cell shows and
// SortKey what the comparator reads — so a branch
// that copies field by field is a branch that silently drops one.
func copyListColumns(src []config.ListColumn) []config.ListColumn {
	if len(src) == 0 {
		return nil
	}
	cols := make([]config.ListColumn, len(src))
	copy(cols, src)
	return cols
}
