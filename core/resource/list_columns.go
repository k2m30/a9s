// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/config"
)

// ResolveListColumnCascade resolves the list column set for typeName.
//
// One path, whatever the caller holds. The VIEW — the operator's file when one
// is loaded, the built-in default otherwise, which is what config.GetViewDef
// already falls back to — owns which columns there are, their order and their
// widths, because that file is the thing an operator edits. The CATALOG owns
// what each cell reads: its Key is merged onto every title both declare.
//
// It was two paths. A loaded file came back verbatim, so the catalog's Key was
// dropped for anyone who had ever started a9s (EnsureViewsDir writes those
// files, and internal/tui/app.go falls back to config.SharedDefaultConfig()
// when it finds none), while a nil config got the merged set. The demo bench
// and the account an operator actually looks at therefore resolved cells by
// different rules, and a cell could be right on the bench acceptance reviews
// and wrong on every screen — which is what cb's Source Type was.
//
// The only resolver of the column-set cascade. core/app's
// resolveListColumnsForBuild (session-view-config-aware, used for rendering,
// for the sort comparator and for the cache-save projection) and
// core/runtime's resolveSaveColumns (always vc=nil, the cache-save fallback
// lane) both delegate here, so no two callers can disagree about which
// columns a resource type renders.
func ResolveListColumnCascade(vc *config.ViewsConfig, typeName string, td *ResourceTypeDef) []config.ListColumn {
	// GetViewDef already falls back to the built-in default when vc is nil or
	// declares nothing for this type, so this is the view whatever the caller
	// holds.
	view := config.GetViewDef(vc, typeName)
	if td == nil || len(td.Columns) == 0 {
		return copyListColumns(view.List)
	}

	// Both lookups below are keyed case-insensitively: a catalog column and
	// the view spell the same title differently often enough ("Time" against
	// "TIME") that an exact match silently drops what the other declaration
	// says about that very column.

	// No view declares this type at all — child types are not in the built-in
	// views — so the catalog is the whole declaration and there is nothing to
	// merge onto it.
	if len(view.List) == 0 {
		cols := make([]config.ListColumn, len(td.Columns))
		for i, c := range td.Columns {
			cols[i] = mergeListColumn(c.Title, c.Width, c.Key, config.ListColumn{})
		}
		return cols
	}

	catalogKeyByTitle := make(map[string]string, len(td.Columns))
	for _, c := range td.Columns {
		catalogKeyByTitle[strings.ToLower(c.Title)] = c.Key
	}
	cols := make([]config.ListColumn, len(view.List))
	for i, lc := range view.List {
		cols[i] = mergeListColumn(lc.Title, lc.Width, catalogKeyByTitle[strings.ToLower(lc.Title)], lc)
	}
	return cols
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
