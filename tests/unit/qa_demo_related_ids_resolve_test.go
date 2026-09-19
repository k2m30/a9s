// Every related ID a demo checker
// reports resolves when the user drills into the row: a related-panel count
// whose IDs the demo fakes cannot fetch lies about what the user can open.
//
// It walks the fixture graph with qa_demo_pivot_coverage_test.go's harness
// (buildDemoTypeCache / demo.NewServiceClients / the real registered
// checkers) and resolves each ID the way the app's related drill does:
// resource.GetFetchByIDs(targetType) called as fn(ctx, clients,
// []string{id}) (core/runtime/executor.go's KindFetchByIDDetail case,
// core/runtime/handlers_related.go's ResolveRelatedNavigate /
// relatedFetchTasks). Target types with no registered FetchByIDs helper
// cannot be drilled by exact ID in demo mode either; those are logged and
// skipped. There is no allowlist: every orphaned ID is a hard failure.
//
// Exactly one reported ID must NOT resolve, and it must be present: a demo
// role attaches fixtures.RetiredManagedPolicyName, a policy AWS has retired,
// so the attachment survives on the role while GetPolicy answers
// NoSuchEntity. That name is what AWS itself reports in the role's
// attachment list; dropping it would render the role's attachments one row
// short with no sign anything was missing. The exception names one id and
// goes red if that id ever starts resolving or stops being reported.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
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
	var orphans []relatedOrphan

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
				if result.Count() <= 0 || len(result.ResourceIDs()) == 0 {
					continue
				}

				for _, id := range result.ResourceIDs() {
					resolved, err := fn(ctx, clients, []string{id})
					if err != nil {
						orphans = append(orphans, relatedOrphan{
							targetType: def.TargetType,
							id:         id,
							line: fmt.Sprintf("%s:%s %s -> %s %s (FetchByIDs error: %v)",
								td.ShortName, def.TargetType, res.ID, def.TargetType, id, err),
						})
						continue
					}
					if !containsResourceID(resolved, id) {
						orphans = append(orphans, relatedOrphan{
							targetType: def.TargetType,
							id:         id,
							line: fmt.Sprintf("%s:%s %s -> %s %s",
								td.ShortName, def.TargetType, res.ID, def.TargetType, id),
						})
					}
				}
			}
		}
	}

	var real []string
	var retired retiredPolicyWitness
	for _, orphan := range orphans {
		if retired.exempt(orphan.targetType, orphan.id) {
			continue
		}
		real = append(real, orphan.line)
	}
	retired.require(t)

	if len(real) > 0 {
		sort.Strings(real)
		t.Errorf("%d related-panel ID(s) do not resolve against demo fixtures for their target type "+
			"(count shown but drill fails) — fix worklist, one per line:", len(real))
		for _, orphan := range real {
			t.Errorf("  %s", orphan)
		}
	}
}

// relatedOrphan is one witnessed related ID that does not resolve: the target
// it points at, kept apart from the human-readable line so the one named
// exception below can match an id rather than a substring of a sentence.
type relatedOrphan struct {
	targetType string
	id         string
	line       string
}

// containsResourceID reports whether resolved (the output of a FetchByIDs
// call) contains a resource whose ID matches id — mirroring how the executor
// treats a FetchByIDs response: any returned resource with a matching ID
// counts as a successful resolution (see core/runtime/executor.go's
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

// retiredPolicyWitness is the one related ID the demo bench must show and
// must not resolve: a role's attachment of fixtures.RetiredManagedPolicyName,
// a policy AWS has retired, so the role still lists it while GetPolicy answers
// NoSuchEntity. The name is the correct ID and the drill says it cannot read
// it, which is what an operator should see. Every gate that requires related
// IDs to open a row reads this exception from here, never from a copy.
//
// Callers pass only IDs that opened no row. exempt matches the target and the
// ID exactly, so a broken pivot whose source merely mentions the name is not
// swallowed; require fails when the ID was never seen unresolved, whether the
// demo does not show it or it resolves.
type retiredPolicyWitness struct{ seen bool }

func (w *retiredPolicyWitness) exempt(target, id string) bool {
	if target == "policy" && id == fixtures.RetiredManagedPolicyName {
		w.seen = true
		return true
	}
	return false
}

func (w *retiredPolicyWitness) require(t *testing.T) {
	t.Helper()
	if !w.seen {
		t.Errorf("no related ID names %s without resolving — the demo bench must carry exactly one "+
			"policy a role attaches and the account cannot read, so the drill's cannot-read error "+
			"is something an operator can see (core/demo/fixtures/iam.go)",
			fixtures.RetiredManagedPolicyName)
	}
}
