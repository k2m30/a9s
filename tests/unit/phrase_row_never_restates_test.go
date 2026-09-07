// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// phrase_row_never_restates_test.go — the U11 rule, bench-wide.
//
// A supporting row and its finding's phrase render one line apart, so a row
// that says what the phrase already said prints one fact twice and costs the
// reader the line that could have said why. TestW6ADetailAttentionNeverRepeats
// Itself holds this for the seven types of one batch; moving a phrase out of
// the item and into the code makes it a whole-bench rule, because the danger
// is exactly the shape the move creates — lifting the row's own text into the
// declaration and leaving the row behind.
//
// Scope is every registered type, over the app's own fold, so a type gaining
// its first supporting row is covered without anyone adding it to a list.

import (
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestNoSupportingRowRestatesItsPhrase(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var offenders []string
	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, fixtures, cache, clients) {
			for _, f := range res.Findings {
				ad, ok := res.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := normalizeRowText(f.Phrase)
				for _, row := range ad.Rows {
					if normalizeRowText(row.Label+" "+row.Value) != phrase &&
						normalizeRowText(row.Value) != phrase {
						continue
					}
					offenders = append(offenders, td.ShortName+" "+string(f.Code)+
						": row "+row.Label+" = "+row.Value+", phrase "+f.Phrase+
						" (first on "+res.ID+")")
				}
			}
		}
	}

	sort.Strings(offenders)
	seen := map[string]bool{}
	for _, o := range offenders {
		if seen[o] {
			continue
		}
		seen[o] = true
		t.Errorf("supporting row restates its phrase: %s — the row has to add what the phrase "+
			"cannot say; give it the measurement or the item and leave the condition to the "+
			"phrase", o)
	}
}
