// qa_demo_related_ids_resolve_test.go — pins the "count shown, drill fails"
// class of demo-mode defect: a related-panel checker reports Count > 0 and a
// list of ResourceIDs, but the demo fakes cannot resolve any of those IDs when
// the user actually drills into the row.
//
// Concrete instance this test pins: ./a9s --demo, EC2 instance
// i-0a1b2c3d4e5f60016 shows a non-zero IAM Roles related count (the ec2
// fixture references instance profile "acme-ec2-instance-profile" at
// internal/demo/fixtures/ec2.go:53), but drilling in fails with
// "role FetchByIDs failed for 1 of 1 IDs: acme-ec2-instance-profile:
// NoSuchEntity" because no such role exists in internal/demo/fixtures/iam.go.
//
// This reuses qa_demo_pivot_coverage_test.go's harness verbatim
// (buildDemoTypeCache / demo.NewServiceClients / the real registered
// checkers) so the fixture graph is walked exactly as that gate walks it, and
// resolves each witnessed ResourceID the same way the app's related-drill
// resolves a single/known target ID: resource.GetFetchByIDs(targetType),
// called as fn(ctx, clients, []string{id}) (see
// internal/runtime/executor.go's KindFetchByIDDetail case and
// internal/runtime/handlers_related.go's ResolveRelatedNavigate /
// relatedFetchTasks, which is the only production consumer of the
// FetchByIDs field on catalog.ResourceTypeDef). Target types with no
// registered FetchByIDs helper cannot be drilled by exact ID in demo mode
// either — those are logged and skipped, not silently ignored.
//
// Unlike qa_demo_pivot_coverage_test.go, this test carries NO allowlist:
// every orphaned ID is a hard failure. The pivot-coverage ratchet tracks
// "the count is a dead zero"; this test tracks "the count lies about what
// you can actually open" — a strictly worse user experience, so it does not
// get a burn-down grace period.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestDemoRelatedIDsResolve_EveryWitnessedIDIsFetchable walks every
// registered (type, pivot) pair exactly as
// TestDemoPivotCoverage_EveryRegisteredPivotHasAWitness does, but instead of
// stopping at "does at least one fixture produce a witness", it takes every
// witness result's ResourceIDs and asserts each one resolves against the
// target type's demo data via the same FetchByIDs helper the real
// related-drill uses. A related-panel count that cannot be drilled into is a
// worse defect than a dead "(0)" row: it actively lies to the user.
func TestDemoRelatedIDsResolve_EveryWitnessedIDIsFetchable(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoTypeCache(t)
	ctx := context.Background()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	skippedTargets := make(map[string]bool)
	var orphans []string

	for _, td := range types {
		defs := resource.GetRelated(td.ShortName)
		if len(defs) == 0 {
			continue
		}
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		for _, def := range defs {
			if def.Checker == nil {
				continue
			}

			fn := resource.GetFetchByIDs(def.TargetType)
			if fn == nil {
				if !skippedTargets[def.TargetType] {
					skippedTargets[def.TargetType] = true
					t.Logf("SKIP: target type %q has no registered FetchByIDs helper — "+
						"exact-ID demo drill cannot be verified for any pivot into it "+
						"(e.g. %s:%s)", def.TargetType, td.ShortName, def.TargetType)
				}
				continue
			}

			for _, res := range fixtures {
				result := def.Checker(ctx, clients, res, cache)
				if result.Count <= 0 || len(result.ResourceIDs) == 0 {
					continue
				}

				for _, id := range result.ResourceIDs {
					resolved, err := fn(ctx, clients, []string{id})
					if err != nil {
						orphans = append(orphans, fmt.Sprintf(
							"%s:%s %s -> %s %s (FetchByIDs error: %v)",
							td.ShortName, def.TargetType, res.ID, def.TargetType, id, err,
						))
						continue
					}
					if !containsResourceID(resolved, id) {
						orphans = append(orphans, fmt.Sprintf(
							"%s:%s %s -> %s %s",
							td.ShortName, def.TargetType, res.ID, def.TargetType, id,
						))
					}
				}
			}
		}
	}

	if len(orphans) > 0 {
		sort.Strings(orphans)
		t.Errorf("%d related-panel ID(s) do not resolve against demo fixtures for their target type "+
			"(count shown but drill fails) — fix worklist, one per line:", len(orphans))
		for _, orphan := range orphans {
			t.Errorf("  %s", orphan)
		}
	}
}

// containsResourceID reports whether resolved (the output of a FetchByIDs
// call) contains a resource whose ID matches id — mirroring how the executor
// treats a FetchByIDs response: any returned resource with a matching ID
// counts as a successful resolution (see internal/runtime/executor.go's
// KindFetchByIDDetail case, which fails the drill only when len(res) == 0 or
// the target ID is simply absent from what was returned).
func containsResourceID(resolved []resource.Resource, id string) bool {
	for _, r := range resolved {
		if r.ID == id {
			return true
		}
	}
	return false
}
