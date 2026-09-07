// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// qa_finding_code_registered_gate_test.go — a code a fetcher puts in a Finding
// has to be a code the catalog declares.
//
// An emitter reads its operator sentence back out of the catalog by code, and
// an unknown code reads as the empty string rather than as an error, so an
// undeclared code renders a phrase with no reason under it and the generated
// signals page cannot list the signal at all. That is how three certificate
// status signals shipped: emitted on six demo rows, declared nowhere, and
// invisible to every gate. check-catalogen compares the page against the
// declarations, so a signal with no declaration is exactly what it cannot see.
//
// The declared side is the whole installed catalog, parents and children. The
// emitted side is every demo row of every type with a fetcher, wave 2 folded
// the way the app folds it — so this reads what actually renders, not what the
// source appears to say. A code no fixture ever produces is outside it; the
// fixtures are the same ones every other rendered-surface gate stands on, and
// a signal worth declaring is worth a witness.
func TestFindingCodesAreDeclared(t *testing.T) {
	declared := map[domain.FindingCode]bool{}
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			declared[f.Code] = true
		}
	}
	if len(declared) < 300 {
		t.Fatalf("only %d declared finding codes; the gate is not seeing the catalog", len(declared))
	}

	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	seen := map[string]bool{}
	var undeclared []string
	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, fixtures, cache, clients) {
			for _, f := range res.Findings {
				if declared[f.Code] {
					continue
				}
				key := td.ShortName + " " + string(f.Code)
				if seen[key] {
					continue
				}
				seen[key] = true
				undeclared = append(undeclared,
					key+" (phrase "+f.Phrase+", first on "+res.ID+")")
			}
		}
	}

	sort.Strings(undeclared)
	for _, u := range undeclared {
		t.Errorf("emitted but not declared: %s — add a catalog.FindingDef with its phrase, "+
			"severity and operator sentence, or the row renders a phrase with no reason "+
			"and the signals page cannot list it", u)
	}
}
