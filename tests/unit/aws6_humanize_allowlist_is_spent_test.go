package unit_test

// aws6_humanize_allowlist_is_spent_test.go — the "out of scope" half of
// rawEnumDetailAllowlist names nothing that is still raw.
//
// aws6_humanize_owner_test.go's sweep carries two kinds of allowlist entry.
// An "identifier" entry is permanent: the value is a name AWS assigned and a
// person types back, and rewording it would make it wrong. An "out of scope"
// entry was a genuine raw enum on a type the humanize-owner change had not
// reached yet — real debt, recorded so the sweep stayed red for anything NEW
// rather than drowning in what was already there.
//
// Every one of those types now declares the field on itself
// (ResourceTypeDef.HumanizeFields), so the out-of-scope entries excuse
// nothing. This test says so, and goes red the day one of them starts
// excusing something again — which is the only thing that would make deleting
// them from the sweep's map wrong.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// outOfScopeReasonPrefix marks an allowlist entry that recorded debt rather
// than a value that must stay verbatim.
const outOfScopeReasonPrefix = "out of scope"

func TestHumanizeAllowlistOutOfScopeEntriesExcuseNothing(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	// Sanity: the debt half of the map is what this test is about, so an empty
	// one means the assertion below has stopped asserting anything.
	debt := map[string]bool{}
	for key, reason := range rawEnumDetailAllowlist {
		if strings.HasPrefix(reason, outOfScopeReasonPrefix) {
			debt[key] = true
		}
	}
	if len(debt) == 0 {
		t.Skip("rawEnumDetailAllowlist carries no out-of-scope entry left to check")
	}

	var stillRaw []string
	for _, td := range resource.AllResourceTypes() {
		rows := mergeWave2Findings(t, td, byType[td.ShortName], cache, clients)
		for _, r := range rows {
			for _, f := range aws6DetailRows(t, r, td.ShortName) {
				if f.Value == "" {
					continue
				}
				if !rawEnumCellPattern.MatchString(f.Value) && !rawCamelEnumCellPattern.MatchString(f.Value) {
					continue
				}
				key := td.ShortName + "/" + f.Path
				if debt[key] {
					stillRaw = append(stillRaw, key+" (row "+r.ID+") = "+f.Value)
				}
			}
		}
	}
	sort.Strings(stillRaw)
	stillRaw = slicesCompactSorted(stillRaw)
	for _, s := range stillRaw {
		t.Errorf("%s is still a raw SDK constant on the demo detail — declare the field on its type "+
			"(ResourceTypeDef.HumanizeFields); an \"out of scope\" allowlist entry is debt that was paid, "+
			"not a value that may stay verbatim", s)
	}
}

// slicesCompactSorted drops adjacent duplicates from a sorted slice, so one
// field showing on many rows reports once.
func slicesCompactSorted(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
