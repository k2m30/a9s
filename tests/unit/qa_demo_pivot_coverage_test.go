// qa_demo_pivot_coverage_test.go — the standing demo-verification gate.
//
// Demo mode (./a9s --demo) is the bench humans use to verify every
// related-resource pivot and issue glyph without live AWS credentials. This
// test is registry-driven and demo-clients-driven: for every registered type
// with RelatedDefs (resource.GetRelated), it drains that type's real Wave-1
// fetcher against demo.NewServiceClients() (the typed fakes) to get the
// actual demo fixture resources, builds one shared resource.ResourceCache
// from every fetchable type (so reverse-scan / cross-type checkers see
// sibling data exactly as production does), then re-runs each registered
// RelatedDef.Checker — the real production checker, not a demo override —
// against every fixture resource of the owning type.
//
// Contract asserted per (type, pivot):
//
//	At least one fixture resource of that type must produce a witness result
//	for that pivot (isWitnessResult — Count > 0 for ordinary pivots; for the
//	FetchFilter-bearing "search CloudTrail" pivot family, Count != 0 is
//	sufficient, mirroring resource.IsRelatedActionable's real actionability
//	rule where a server-side-filtered re-fetch is drillable even at Count=-1).
//	Not every resource x every pivot — one witness suffices. A pivot with
//	zero witnesses across every fixture resource means the demo fixture graph
//	is disconnected for that pivot: a user who opens that type in demo mode
//	and looks at every row will see a dead "(0)" row forever, so the panel is
//	dead weight in the one environment meant to demonstrate it works.
//
// Each type+pivot combination is its own t.Run subtest so the full backlog
// of disconnected pivots is enumerable from `go test -v` output in one pass,
// per type independently — a fix to one type's fixtures does not hide
// failures in another's.
//
// Plus: for every issue-capable type (a Wave-2 IssueEnricher is registered
// via aws.Wave2EnricherFor, and the type is not excluded from the issue
// badge via ExcludeFromIssueBadge), at least one fixture resource of that
// type must carry an issue in demo mode — either a Wave-1 Finding already
// present on the fixture Resource, or the type's registered Wave-2 enricher
// returning IssueCount > 0 / a non-empty Findings map when run against the
// type's own fixture resources. A type with an issue-capable enricher but
// zero fixtures ever flagged means ctrl+z / the issue badge has nothing to
// demonstrate for that type in demo mode.
//
// RATCHET: at the time this test was written, s3's fixture graph was
// disconnected for 9 of its registered pivots (a coder was rebuilding
// core/demo/fixtures/s3.go in parallel). That fix landed and s3 is now
// fully connected. Every OTHER disconnected pivot and issue-coverage gap
// found at that time is pinned below in knownDisconnectedPivots /
// knownIssueCoverageGaps — the documented burn-down backlog. The gate is a
// ratchet, not a static allowlist:
//
//   - An entry NOT in the allowlist with no witness is a NEW regression —
//     always fails, unconditionally.
//   - An allowlisted entry that NOW HAS a witness fails with a "remove from
//     allowlist" message — this forces the fix to be reflected here in the
//     same PR that lands it, so the backlog only ever shrinks.
//   - An allowlisted entry that is still disconnected is skipped (logged,
//     not failed) — expected, pre-existing debt.
//
// s3 must NOT appear in either allowlist below: it is already connected, and
// re-adding it would silently mask a regression in the very type this gate
// was built to protect.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// knownDisconnectedPivots pins the exact (type, pivot) inventory captured at
// ratchet-conversion time, in "type:pivot" form (matching def.TargetType, not
// DisplayName). This is the burn-down backlog, not a permanent exemption:
//
//   - present here + still disconnected today  -> skip (logged), expected debt.
//   - present here + now has a witness         -> FAIL ("remove from allowlist"),
//     forcing the fixture fix and this list to land in the same PR.
//   - a disconnected pivot NOT in this list     -> FAIL unconditionally, a new
//     regression the allowlist was never told about.
//
// s3 is deliberately absent: its 9 originally-disconnected pivots were fixed
// by a parallel fixture rebuild before this ratchet was written, and must
// never be re-added here.
//
// TERMINAL STATE: the map is empty. Every pivot ever pinned here
// (apigw:r53/vpce/waf/sfn/sns, athena:glue, eip:logs, elb:r53, kms:s3,
// tg:backup/dbc/dbi/dbi-snap/logs/sg/subnet, vpce:acm/cf/s3/tg/waf) was
// structurally unwitnessable per its own checker's documented comment — no
// fixture graph could ever produce a witness, unlike s3's gap, which was a
// fixable fixture problem. Rather than carry 20 permanent burn-down entries
// that could never burn down, each checker and its RegisterRelated entry was
// deleted outright (see core/aws/catalog_dns_cdn.go, catalog_data.go,
// catalog_networking.go, catalog_secrets.go). resource.GetRelated no longer
// returns these TargetTypes for their owning types, so this loop never visits
// their keys again — do not re-add them; a genuinely new structurally-
// unwitnessable pivot should not be registered at all, following this
// precedent, rather than added here.
var knownDisconnectedPivots = map[string]bool{}

// knownIssueCoverageGaps pins the exact issue-capable types that had zero
// flagged demo fixtures at ratchet-conversion time. Same burn-down semantics
// as knownDisconnectedPivots: still-gapped -> skip (logged); now-flagged ->
// FAIL ("remove from allowlist"); a gap NOT in this list -> FAIL unconditionally.
var knownIssueCoverageGaps = map[string]bool{}

// demoPivotMaxFetchPages bounds the pagination drain per type as a safety
// valve against a runaway fake that never sets IsTruncated=false. Every real
// demo fixture set fits comfortably within a handful of pages.
const demoPivotMaxFetchPages = 50

// drainDemoFixtures runs td.Fetcher to exhaustion against the demo clients,
// exactly as the production fetch loop does (see aws.FetchS3Buckets), and
// returns every resource.Resource the type's demo fixtures produce. Returns
// (nil, false) when the type has no Wave-1 Fetcher registered — such types
// cannot be driven generically and are skipped by the caller.
func drainDemoFixtures(t *testing.T, td resource.ResourceTypeDef, clients *awsclient.ServiceClients) ([]resource.Resource, bool) {
	t.Helper()
	if td.Fetcher == nil {
		return nil, false
	}
	ctx := context.Background()
	var all []resource.Resource
	token := ""
	for page := range demoPivotMaxFetchPages {
		result, err := td.Fetcher(ctx, clients, token)
		if err != nil && len(result.Resources) == 0 {
			// Rows + composite error together are the designed E5
			// partial-success outcome (e.g. mwaa's details-denied demo
			// witness); only a row-less error is a harness failure.
			t.Fatalf("%s: Fetcher page %d returned error: %v", td.ShortName, page, err)
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			return all, true
		}
		token = result.Pagination.NextToken
	}
	t.Fatalf("%s: Fetcher did not terminate within %d pages — runaway pagination in demo fixtures", td.ShortName, demoPivotMaxFetchPages)
	return all, true
}

// buildDemoTypeCache drains every registered type's demo fixtures via its
// real Wave-1 Fetcher and returns both the per-type resource lists and one
// shared resource.ResourceCache built from all of them, so that reverse-scan
// / cross-type RelatedCheckers see sibling data exactly as production does
// (the same shape TestCtEventsDemoRightColumnCheckers's buildFakeResourceCache
// establishes for ct-events, generalized to every registered type).
func buildDemoTypeCache(t *testing.T) (map[string][]resource.Resource, resource.ResourceCache) {
	t.Helper()
	clients := demo.NewServiceClients()
	byType := make(map[string][]resource.Resource)
	cache := make(resource.ResourceCache)

	for _, td := range resource.AllResourceTypes() {
		fixtures, ok := drainDemoFixtures(t, td, clients)
		if !ok {
			continue
		}
		byType[td.ShortName] = fixtures
		if len(fixtures) > 0 {
			cache[td.ShortName] = resource.ResourceCacheEntry{Resources: fixtures, IsTruncated: false}
		}
	}
	return byType, cache
}

// isWitnessResult reports whether a RelatedCheckResult demonstrates a
// graph-connected pivot in the sense the UI actually cares about: the row
// would be drillable. Delegates directly to resource.IsRelatedActionable for
// the FetchFilter-bearing branch (the ct-events "search CloudTrail" affordance
// family, per BuildCTEventsPivotChecker) since that family can legitimately
// reach every State — RelatedDeferred (absent/truncated cache — always
// actionable regardless of Count, "re-fetch server-side"), RelatedError
// (never actionable, even though FetchFilter is still populated for
// navigation bookkeeping), or RelatedResolved with a real Count (actionable
// only when Count > 0) — so no single Count-only proxy can distinguish them
// post-task-#58 (RelatedDeferred and a resolved zero both carry Count==0). A
// witness for a non-FetchFilter pivot still requires Count > 0 (matches
// IsRelatedActionable's RelatedResolved count>0 branch; a resolved zero is
// genuinely a dead row).
func isWitnessResult(result resource.RelatedCheckResult) bool {
	if len(result.FetchFilter()) > 0 {
		return resource.IsRelatedActionable(result.State(), result.Count(), result.Truncated())
	}
	return result.Count() > 0
}

// TestDemoPivotCoverage_EveryRegisteredPivotHasAWitness is the standing
// ratchet: for every registered type with RelatedDefs, every pivot must have
// at least one fixture resource for which the real production checker
// produces a witness (isWitnessResult) — UNLESS the (type, pivot) pair is
// pinned in knownDisconnectedPivots as pre-existing debt, in which case it is
// skipped (logged) instead of failed. An allowlisted pair that now has a
// witness fails with a "remove from allowlist" message so the burn-down
// bookkeeping cannot silently drift from reality. One subtest per (type,
// pivot) so the full backlog is enumerable in -v output.
func TestDemoPivotCoverage_EveryRegisteredPivotHasAWitness(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoTypeCache(t)
	ctx := context.Background()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillDisconnected []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		defs := resource.GetRelated(td.ShortName)
		if len(defs) == 0 {
			continue
		}
		fixtures := byType[td.ShortName]

		for _, def := range defs {
			key := td.ShortName + ":" + def.TargetType
			testName := fmt.Sprintf("%s/%s", td.ShortName, def.TargetType)
			t.Run(testName, func(t *testing.T) {
				if def.Checker == nil {
					t.Fatalf("%s -> %s: RelatedDef has a nil Checker (structural bug)", td.ShortName, def.TargetType)
				}
				if len(fixtures) == 0 {
					t.Fatalf("%s -> %s: type %q has zero demo fixtures — cannot have a pivot witness", td.ShortName, def.TargetType, td.ShortName)
				}

				maxCount := -2
				hasWitness := false
				for _, res := range fixtures {
					result := def.Checker(ctx, clients, res, cache)
					if result.Count() > maxCount {
						maxCount = result.Count()
					}
					if isWitnessResult(result) {
						hasWitness = true
						break
					}
				}

				allowlisted := knownDisconnectedPivots[key]

				switch {
				case hasWitness && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s -> %s now has a witness (best Count seen: %d) but is still pinned in "+
							"knownDisconnectedPivots — remove %q from the allowlist in this PR",
						td.ShortName, def.TargetType, maxCount, key,
					)
				case hasWitness:
					// Connected and not allowlisted — the expected steady state.
				case allowlisted:
					stillDisconnected = append(stillDisconnected, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): %s -> %s has no witness among %d %q fixtures (best Count seen: %d) — "+
							"pre-existing debt, see knownDisconnectedPivots",
						td.ShortName, def.TargetType, len(fixtures), td.ShortName, maxCount,
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW REGRESSION (not allowlisted): %s -> %s: no fixture resource among %d %q fixtures "+
							"produced an actionable result for this pivot (best Count seen: %d) — demo mode shows a "+
							"dead \"(0)\" row for this pivot forever. Either fix the fixture graph or, if this is "+
							"pre-existing debt, add %q to knownDisconnectedPivots",
						td.ShortName, def.TargetType, len(fixtures), td.ShortName, maxCount, key,
					)
				}
			})
		}
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW REGRESSION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillDisconnected) > 0 {
		t.Logf("STILL-DISCONNECTED (allowlisted, skipped) INVENTORY (%d): %v", len(stillDisconnected), stillDisconnected)
	}
}

// isIssueCapable reports whether a resource type participates in the issue
// badge / ctrl+z surface: it has a registered Wave-2 IssueEnricher AND is not
// explicitly excluded from the issue badge. Mirrors the production check at
// (*runtime.Core).HasIssueEnricher (awsclient.Wave2EnricherFor) plus the
// ExcludeFromIssueBadge gate that controls menu/list issue-glyph visibility.
func isIssueCapable(td resource.ResourceTypeDef) bool {
	if td.ExcludeFromIssueBadge {
		return false
	}
	_, ok := awsclient.Wave2EnricherFor(td.ShortName)
	return ok
}

// TestDemoIssueCoverage_EveryIssueCapableTypeHasAFlaggedFixture is the
// issue-glyph half of the standing ratchet: for every issue-capable type
// (Wave-2 enricher registered, not excluded from the issue badge), at least
// one demo fixture resource of that type must carry an issue — either a
// Wave-1 Finding already on the fixture, or the type's own Wave-2 enricher
// returning IssueCount > 0 / a non-empty Findings map when run against the
// type's fixtures — UNLESS the type is pinned in knownIssueCoverageGaps as
// pre-existing debt, in which case it is skipped (logged) instead of failed.
// An allowlisted type that now has a flagged fixture fails with a "remove
// from allowlist" message. One subtest per type so gaps are independently
// enumerable.
func TestDemoIssueCoverage_EveryIssueCapableTypeHasAFlaggedFixture(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoTypeCache(t)
	ctx := context.Background()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		if !isIssueCapable(td) {
			continue
		}
		t.Run(td.ShortName, func(t *testing.T) {
			fixtures := byType[td.ShortName]
			if len(fixtures) == 0 {
				t.Fatalf("%s: issue-capable type has zero demo fixtures — cannot demonstrate an issue", td.ShortName)
			}

			flagged := false
			for _, res := range fixtures {
				if len(res.Findings) > 0 {
					flagged = true
					break
				}
			}

			var findingsLen int
			if !flagged {
				enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
				if !ok || enricher.Fn == nil {
					t.Fatalf("%s: isIssueCapable reported true but Wave2EnricherFor now returns ok=%v", td.ShortName, ok)
				}

				result, err := enricher.Fn(ctx, clients, fixtures, cache)
				if err != nil {
					t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
				}
				findingsLen = len(result.Findings)
				flagged = findingsLen > 0
			}

			allowlisted := knownIssueCoverageGaps[td.ShortName]

			switch {
			case flagged && allowlisted:
				readyForBurnDown = append(readyForBurnDown, td.ShortName)
				t.Errorf(
					"BURN-DOWN: %s now has a flagged demo fixture but is still pinned in knownIssueCoverageGaps — "+
						"remove %q from the allowlist in this PR",
					td.ShortName, td.ShortName,
				)
			case flagged:
				// Issue demonstrated and not allowlisted — the expected steady state.
			case allowlisted:
				stillGapped = append(stillGapped, td.ShortName)
				t.Skipf(
					"KNOWN GAP (allowlisted): %s: none of %d demo fixtures carry a Wave-1 Finding, and the Wave-2 "+
						"enricher flagged zero issues (len(Findings)=%d) — pre-existing debt, see "+
						"knownIssueCoverageGaps",
					td.ShortName, len(fixtures), findingsLen,
				)
			default:
				newlyRegressed = append(newlyRegressed, td.ShortName)
				t.Errorf(
					"NEW REGRESSION (not allowlisted): %s: none of %d demo fixtures carry a Wave-1 Finding, and the "+
						"Wave-2 enricher flagged zero issues (len(Findings)=%d) — ctrl+z / the issue "+
						"badge has nothing to demonstrate for this type in demo mode. Either fix the fixtures or, if "+
						"this is pre-existing debt, add %q to knownIssueCoverageGaps",
					td.ShortName, len(fixtures), findingsLen, td.ShortName,
				)
			}
		})
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW REGRESSION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
