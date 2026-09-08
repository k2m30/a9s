// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r8_columns_agree_test.go — misc4 row 8.
//
// A list column is declared in two places, and ResolveListColumnCascade has
// two arms that read them differently. When the built-in view defaults hold
// more columns than the catalog literal and the first title matches, the
// default column set is used WHOLE — the catalog's Key is dropped and the
// default's Path decides the cell. Otherwise the catalog column keeps its Key
// and only borrows the default's Path, SortKey and Humanize.
//
// So a title the two spell against different facts is a cell whose value
// depends on which arm a type lands in. That is how sns and sns-sub shipped:
// the catalog read Fields["display_name"] and Fields["confirmed"] while the
// defaults read RawStruct.TopicArn and RawStruct.SubscriptionArn, and the
// Topic Name and Confirmed cells said something else entirely for every
// operator on the other arm.
//
// The catalog is the source for a column's identity — which columns a type
// has, what each reads, and the order they are built in. The defaults exist
// beside it because a view file on disk is what an operator edits: they may
// add columns the catalog does not declare, and they carry the rendering
// hints (Path, SortKey, Humanize) the catalog literal has no field for. This
// gate holds the overlap honest by the only measure that reaches the screen —
// the rendered cell — rather than by how the two spell their sources.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestCatalogAndDefaultColumnsAgreeTitleByTitle is the gate: over every demo
// row of every type, a title both declare renders the same cell whichever arm
// of the cascade built it.
func TestCatalogAndDefaultColumnsAgreeTitleByTitle(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	if len(types) < 50 {
		t.Fatalf("only %d types registered; the gate is not seeing the catalog", len(types))
	}
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	for _, td := range types {
		def := config.GetViewDef(nil, td.ShortName)
		if len(def.List) == 0 || len(td.Columns) == 0 {
			continue
		}
		byTitle := make(map[string]config.ListColumn, len(def.List))
		order := make(map[string]int, len(def.List))
		for i, lc := range def.List {
			byTitle[strings.ToLower(lc.Title)] = lc
			order[strings.ToLower(lc.Title)] = i
		}

		prev, prevTitle := -1, ""
		for _, c := range td.Columns {
			key := strings.ToLower(c.Title)
			lc, both := byTitle[key]
			if !both {
				continue
			}
			if at := order[key]; prev >= 0 && at < prev {
				t.Errorf("%s: %q comes after %q in the catalog and before it in the defaults; "+
					"the shared titles must hold one order",
					td.ShortName, c.Title, prevTitle)
			} else {
				prev, prevTitle = at, c.Title
			}

			// The two arms of the cascade, built exactly as it builds them.
			fromCatalog := app.ColumnDef{
				Key: c.Key, Title: c.Title, Width: c.Width,
				Path: lc.Path, SortKey: lc.SortKey, Humanize: lc.Humanize,
			}
			fromDefaults := app.ColumnDef{
				Key: lc.Key, Title: lc.Title, Width: lc.Width,
				Path: lc.Path, SortKey: lc.SortKey, Humanize: lc.Humanize,
			}
			for _, r := range byType[td.ShortName] {
				a := app.ExtractCellValue(fromCatalog, &td, r)
				b := app.ExtractCellValue(fromDefaults, &td, r)
				if a == b {
					continue
				}
				t.Errorf("%s row %q column %q: the catalog arm renders %q and the defaults arm %q — "+
					"one title, two facts; fix the defaults (core/config/defaults_*.go)",
					td.ShortName, r.ID, c.Title, a, b)
				break
			}
		}
	}
}
