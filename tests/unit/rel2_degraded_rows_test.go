package unit_test

// rel2_degraded_rows_test.go — a details-denied/details-unavailable row
// never answers a confident zero for a struct-only pivot: a fabricated,
// assertable struct would let such pivots find their field empty and answer
// zero for a row nobody could describe. This gate covers every type that
// ships such a row, not just ng.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRel2DegradedRowsNeverClaimAConfidentZeroFromAnAbsentStruct walks every
// demo row across every registered type, cold — exactly as its own Fetcher
// built it, no manual RawStruct override. For any row carrying a
// details-denied or details-unavailable finding, every registered pivot is
// run against it.
//
// A resolved zero on a row whose RawStruct is nil can only have come from
// Fields or the target cache: a type assertion against a nil interface
// always fails, so there is no struct left to have answered from. That is
// the row-10 fix's own guarantee, and it needs no further proof here.
//
// A resolved zero on a row whose RawStruct is NOT nil (lt and transfer pass
// the real, non-fabricated API item) must still not depend on that struct:
// stripping it must reproduce the same zero, proving Fields alone carries
// the filter and the real struct is incidental to this answer.
func TestRel2DegradedRowsNeverClaimAConfidentZeroFromAnAbsentStruct(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{}
	rowsByType := map[string][]resource.Resource{}
	for _, td := range resource.AllResourceTypes() {
		var rows []resource.Resource
		if td.Fetcher != nil {
			// A partial page is still a usable list; several demo fetchers
			// deliberately fail one id to produce the degraded rows this
			// test walks for.
			out, _ := td.Fetcher(context.Background(), clients, "") //nolint:errcheck // partial results are the point
			rows = out.Resources
		}
		rowsByType[td.ShortName] = rows
		cache[td.ShortName] = resource.ResourceCacheEntry{Resources: rows}
	}

	tested := 0
	for _, td := range resource.AllResourceTypes() {
		defs := resource.GetRelated(td.ShortName)
		if len(defs) == 0 {
			continue
		}
		for _, row := range rowsByType[td.ShortName] {
			degraded := false
			for _, f := range row.Findings {
				if f.Code == awsclient.DetailsDeniedCode(td.ShortName) || f.Code == awsclient.DetailsUnavailableCode(td.ShortName) {
					degraded = true
					break
				}
			}
			if !degraded {
				continue
			}
			tested++

			for _, def := range defs {
				if def.Checker == nil {
					continue
				}
				cold := def.Checker(context.Background(), clients, row, cache)
				if cold.State() != domain.RelatedResolved || cold.Count() != 0 {
					continue
				}
				if row.RawStruct == nil {
					continue
				}
				stripped := row
				stripped.RawStruct = nil
				strippedResult := def.Checker(context.Background(), clients, stripped, cache)
				if strippedResult.State() != domain.RelatedResolved || strippedResult.Count() != 0 {
					t.Errorf("%s/%s -> %s: resolved 0 with its real struct present but %v/%d with it stripped — "+
						"the zero depends on a struct this row's describe call never returned",
						td.ShortName, row.ID, def.TargetType, strippedResult.State(), strippedResult.Count())
				}
			}
		}
	}
	if tested == 0 {
		t.Fatal("no degraded demo row (details-denied or details-unavailable) found across any type; the walk found nothing to check")
	}
}
