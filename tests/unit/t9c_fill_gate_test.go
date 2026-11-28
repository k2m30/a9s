package unit_test

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t9cCellFilled(v string) bool {
	return v != "" && v != "-"
}

func t9cColumnFilled(td catalog.ResourceTypeDef, col config.ListColumn, rows []resource.Resource) bool {
	cd := app.ColumnDef{Key: col.Key, Title: col.Title, Path: col.Path, Width: col.Width}
	for _, r := range rows {
		if t9cCellFilled(app.ExtractCellValue(cd, &td, r)) {
			return true
		}
	}
	return false
}

// Every default list column shows a value on at least one demo list row, a
// column the type's Wave 2 enricher fills counting once Wave 2 answered: a
// column the bench never fills is one no reviewer of the demo sees working.
func TestT9c_Row9_EveryDefaultColumnIsFilledOnTheBench(t *testing.T) {
	rows, enrichedIdx, _ := t9bDevTreeRowsEnriched(t)
	byType := map[string][]resource.Resource{}
	for i, r := range rows {
		if !enrichedIdx[i] {
			byType[r.typ] = append(byType[r.typ], r.res)
		}
	}
	var clients *awsclient.ServiceClients
	var cache resource.ResourceCache
	var problems []string
	for _, td := range t9cAllDefs() {
		list := byType[td.ShortName]
		if len(list) == 0 {
			problems = append(problems, fmt.Sprintf("%s: the demo bench has no list row", td.ShortName))
			continue
		}
		var folded []resource.Resource
		for _, col := range config.DefaultViewDef(td.ShortName).List {
			if t9cColumnFilled(td, col, list) {
				continue
			}
			if folded == nil && td.Wave2 != nil {
				if _, ok := awsclient.Wave2EnricherFor(td.ShortName); ok {
					if clients == nil {
						clients = demo.NewServiceClients()
						_, cache = buildVisibilityTypeCache(t)
					}
					folded = t9cAfterWave2(t, td.ShortName, clients, list, cache)
				}
			}
			if t9cColumnFilled(td, col, folded) {
				continue
			}
			problems = append(problems, fmt.Sprintf("%s: column %q (key %q, path %q) is blank on all %d demo rows", td.ShortName, col.Title, col.Key, col.Path, len(list)))
		}
	}
	t9bReport(t, "default columns no demo row fills", problems)
}

// Every default detail path shows a value on at least one demo row as the
// detail screen renders it once the type's detail enricher answered. A type
// with its own projector builds its detail without the default paths.
func TestT9c_Row9_EveryDefaultDetailPathIsFilledOnTheBench(t *testing.T) {
	const perTypeRenders = 300
	rows, enrichedIdx, _ := t9bDevTreeRowsEnriched(t)
	byType := map[string][]resource.Resource{}
	for i, r := range rows {
		td := catalog.FindAny(r.typ)
		if td == nil {
			continue
		}
		if td.DetailEnrich != nil && !enrichedIdx[i] {
			continue
		}
		byType[r.typ] = append(byType[r.typ], r.res)
	}
	var problems []string
	for _, td := range t9cAllDefs() {
		if td.Project != nil {
			continue
		}
		pending := map[string]bool{}
		for _, d := range config.DefaultViewDef(td.ShortName).Detail {
			if d.Path != "" {
				pending[d.Path] = true
			}
		}
		if len(pending) == 0 {
			continue
		}
		list := byType[td.ShortName]
		for i := 0; i < len(list) && i < perTypeRenders && len(pending) > 0; i++ {
			shown := t9bDetailRows(t, list[i], td.ShortName)
			for p := range pending {
				if t9cShown(shown, p) {
					delete(pending, p)
				}
			}
		}
		for p := range pending {
			problems = append(problems, fmt.Sprintf("%s: detail path %s shows no value on any of %d demo rows", td.ShortName, p, min(len(list), perTypeRenders)))
		}
	}
	t9bReport(t, "default detail paths no demo row fills", problems)
}
