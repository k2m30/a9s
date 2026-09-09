// qa_demo_related_ids_resolve_test.go — pins the "count shown, drill fails"
// class of demo-mode defect: a related-panel checker reports Count > 0 and a
// list of ResourceIDs, but the demo fakes cannot resolve any of those IDs when
// the user actually drills into the row.
//
// Concrete instance this test pins: ./a9s --demo, EC2 instance
// i-0a1b2c3d4e5f60016 shows a non-zero IAM Roles related count (the ec2
// fixture references instance profile "acme-ec2-instance-profile" at
// core/demo/fixtures/ec2.go:53), but drilling in fails with
// "role FetchByIDs failed for 1 of 1 IDs: acme-ec2-instance-profile:
// NoSuchEntity" because no such role exists in core/demo/fixtures/iam.go.
//
// This reuses qa_demo_pivot_coverage_test.go's harness verbatim
// (buildDemoTypeCache / demo.NewServiceClients / the real registered
// checkers) so the fixture graph is walked exactly as that gate walks it, and
// resolves each witnessed ResourceID the same way the app's related-drill
// resolves a single/known target ID: resource.GetFetchByIDs(targetType),
// called as fn(ctx, clients, []string{id}) (see
// core/runtime/executor.go's KindFetchByIDDetail case and
// core/runtime/handlers_related.go's ResolveRelatedNavigate /
// relatedFetchTasks, which is the only production consumer of the
// FetchByIDs field on catalog.ResourceTypeDef). Target types with no
// registered FetchByIDs helper cannot be drilled by exact ID in demo mode
// either — those are logged and skipped, not silently ignored.
//
// Unlike qa_demo_pivot_coverage_test.go, this test carries NO burn-down
// allowlist: every orphaned ID is a hard failure. The pivot-coverage ratchet
// tracks "the count is a dead zero"; this test tracks "the count lies about
// what you can actually open" — a strictly worse user experience, so it does
// not get a grace period.
//
// Exactly one witnessed ID is expected NOT to resolve, and it is REQUIRED to
// be present: a demo role attaches fixtures.RetiredManagedPolicyName, a
// policy AWS has retired, so the attachment survives on the role while
// GetPolicy answers NoSuchEntity. That is not the defect this gate hunts:
// the lie is a FIXTURE gap, and here the name came from the role's OWN
// attachment list, which is what AWS itself reports, and the aggregate says
// which id it could not read. Dropping the name instead would render the
// role's attachments one row short with no sign anything was missing. The
// exception is not an allowlist: it names one id, and it goes red if that
// id ever starts resolving or stops being witnessed, so it cannot quietly
// widen to cover a real orphan.
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
	sawRetiredPolicy := false
	for _, orphan := range orphans {
		// The exemption matches the TARGET ID exactly, not the formatted line:
		// a substring test would also swallow a genuinely broken pivot whose
		// source id happened to contain the retired policy's name.
		if orphan.targetType == "policy" && orphan.id == fixtures.RetiredManagedPolicyName {
			sawRetiredPolicy = true
			continue
		}
		real = append(real, orphan.line)
	}

	if !sawRetiredPolicy {
		t.Errorf("no witnessed related ID names %s — the demo bench is supposed to carry exactly one "+
			"policy a role attaches and the account cannot read, so the aggregate's navigation-defect "+
			"failure is something an operator can actually see (core/demo/fixtures/iam.go)",
			fixtures.RetiredManagedPolicyName)
	}

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
