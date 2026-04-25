//go:build integration

package integration

// scenario_related_drill_through_test.go — Drill-through regression pins.
//
// These tests verify that every registered related-panel pivot and navigable
// field on the graph-root fixture actually resolves to at least one real
// resource in the demo cache. An empty landing means the checker produced a
// resource ID in a format that does not match the target resource type's
// Resource.ID field — the exact bug class that caused the SES EventBridge,
// DDB KMS, opensearch→acm, and efs→backup navigation failures.
//
// The drill-through test (TestScenario_RelatedDrillThrough_All) runs checkers
// DIRECTLY against a prefetched ResourceCache rather than through the TUI
// event loop. The previous TUI-driven design had three loopholes that all
// hid bugs at once:
//
//  1) `Count < 1 → continue` silently skipped pivots that didn't resolve —
//     whether from a genuine bug or a harness race.
//  2) Detail-open fires related checks in parallel; checkers with
//     NeedsTargetCache=true ran against a still-populating cache and
//     committed Count=0 RelatedCheckResultMsgs before the sibling fetchers
//     finished. The scenario harness didn't re-probe, so those zeros
//     propagated to the test.
//  3) When a drill did fire (Count >= 1), `DrillRelated` succeeded on the
//     FetchFilter path even when the checker's emitted ResourceIDs didn't
//     match the target fetcher's Resource.ID format, because that branch
//     fetches and filters server-side — bypassing ID comparison.
//
// Direct invocation eliminates all three: the cache is prewarmed before any
// checker runs (no race), every pivot is evaluated (no skip), and the test
// asserts every returned ResourceID exists in the target fetcher's output
// (ID-format drift fails the test loudly).
//
// TestScenario_NavigableFieldDrillThrough_All still uses the TUI harness
// because it exercises the NavigateMsg dispatch path, which is the thing
// under test there.

import (
	"context"
	"strings"
	"testing"
	"time"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// drillThroughFixtures is the full set of (label, shortName, graphRoot) triples
// that every drill-through subtest runs against. Multiple rows per shortName are
// allowed when a resource has more than one graph-root-equivalent fixture worth
// asserting (e.g. dbi carries both the baseline and Aurora fixtures, redshift
// carries two because logs/s3 are mutually exclusive).
var drillThroughFixtures = []struct {
	label     string
	shortName string
	graphRoot string
}{
	{"ses", "ses", demofixtures.SESGraphRootIdentity},
	{"ddb/orders-prod", "ddb", demofixtures.OrdersProdID},
	{"dbi/prod-dbi-1", "dbi", demofixtures.ProdDbiID},
	{"dbi/prod-dbi-aurora", "dbi", demofixtures.ProdDbiAuroraID},
	{"dbc/acme-docdb-prod", "dbc", demofixtures.ProdDbcID},
	{"dbc/prod-aurora", "dbc", "prod-aurora-cluster"},
	{"redis/prod-redis", "redis", demofixtures.ProdRedisID},
	{"s3/a9s-demo-healthy", "s3", demofixtures.HealthyBucketName},
	{"backup/plan-broken-2failed", "backup", demofixtures.ProdDatabasePlanID},
	{"efs/prod-app-data", "efs", demofixtures.ProdEFSID},
	{"opensearch/acme-logs", "opensearch", demofixtures.GraphRootDomain},
	// redshift: two graph-roots because logs (CloudWatch) and s3 (audit bucket)
	// are mutually exclusive per AWS LogDestinationType. Each root covers 10 of
	// 11 `count shown: yes` pivots; together they cover all 11.
	// See docs/resources/redshift-impl-plan.md §5.1.
	{"redshift/acme-warehouse", "redshift", demofixtures.AcmeWarehouseID},
	{"redshift/acme-reporting", "redshift", demofixtures.AcmeReportingID},
	// rds-snap: Aurora-engine source covers the dbc pivot; non-Aurora source
	// has no DBClusterIdentifier so dbc=0 there. See rds-snap-impl-plan §2.2.
	{"rds-snap/prod-aurora", "rds-snap", demofixtures.ProdRDSSnapAuroraID},
}

// drillThroughGroups collapses the flat fixture list into groups sharing a
// shortName. Resource types with multiple graph-roots (redshift) are
// evaluated with UNION semantics: each pivot must resolve on at least one
// root — per redshift-impl-plan.md §5.1 logs/s3 mutual exclusivity.
type drillThroughGroup struct {
	shortName  string
	graphRoots []struct {
		label, id string
	}
}

func buildDrillThroughGroups() []drillThroughGroup {
	order := []string{}
	byShort := map[string]*drillThroughGroup{}
	for _, f := range drillThroughFixtures {
		g, ok := byShort[f.shortName]
		if !ok {
			g = &drillThroughGroup{shortName: f.shortName}
			byShort[f.shortName] = g
			order = append(order, f.shortName)
		}
		g.graphRoots = append(g.graphRoots, struct{ label, id string }{f.label, f.graphRoot})
	}
	out := make([]drillThroughGroup, 0, len(order))
	for _, sn := range order {
		out = append(out, *byShort[sn])
	}
	return out
}

// rootPivotObservation captures checker output AND the verification of that
// output against the target type's fetcher.
type rootPivotObservation struct {
	count             int      // Count from the checker
	resourceIDs       []string // checker-emitted IDs
	idsMatchedInTgt   int      // how many of resourceIDs exist as Resource.ID in target fetcher output
	targetFetcherSize int      // size of target fetcher output (context when no match)
}

// buildAllTargetCache fetches every registered resource type via its paginated
// fetcher against the demo clients and returns a pre-populated ResourceCache.
// Every NeedsTargetCache=true checker's reads are served from this — eliminating
// the async cache-population race that lets the TUI-driven harness report
// Count=0 for pivots that actually resolve.
func buildAllTargetCache(t *testing.T, clients any) resource.ResourceCache {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cache := resource.ResourceCache{}
	for _, sn := range resource.AllShortNames() {
		fetcher := resource.GetPaginatedFetcher(sn)
		if fetcher == nil {
			continue
		}
		var all []resource.Resource
		token := ""
		for page := 0; page < 20; page++ {
			out, err := fetcher(ctx, clients, token)
			if err != nil {
				// Demo mode may not implement every AWS service — ignore and move on.
				break
			}
			all = append(all, out.Resources...)
			if out.Pagination == nil || !out.Pagination.IsTruncated {
				break
			}
			token = out.Pagination.NextToken
		}
		cache[sn] = resource.ResourceCacheEntry{Resources: all}
	}
	return cache
}

// idExistsInTarget reports whether any resource in the target-type cache
// entry has Resource.ID == id. Exact-match comparison — the whole point of
// this test is to catch ID-format drift between checker and fetcher.
func idExistsInTarget(cache resource.ResourceCache, targetType, id string) bool {
	entry, ok := cache[targetType]
	if !ok {
		return false
	}
	for _, r := range entry.Resources {
		if r.ID == id {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TestScenario_RelatedDrillThrough_All
// ---------------------------------------------------------------------------

// TestScenario_RelatedDrillThrough_All enforces U9 strictly:
//
//   - every non-ct-events pivot on each resource type's graph-root union must
//     have Count >= 1 AND return at least one ResourceID that exists as a
//     Resource.ID in the target type's fetcher output.
//
// Direct checker invocation — no TUI event loop, no async races.
func TestScenario_RelatedDrillThrough_All(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := buildAllTargetCache(t, clients)

	for _, group := range buildDrillThroughGroups() {
		group := group
		t.Run(group.shortName, func(t *testing.T) {
			defs := resource.GetRelated(group.shortName)
			if len(defs) == 0 {
				t.Fatalf("no related defs registered for %q", group.shortName)
			}

			// perPivot: displayName → rootLabel → observation.
			perPivot := make(map[string]map[string]rootPivotObservation, len(defs))
			for _, def := range defs {
				perPivot[def.DisplayName] = make(map[string]rootPivotObservation, len(group.graphRoots))
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			for _, root := range group.graphRoots {
				src := fullIntegrationMustFindResourceByID(t, clients, group.shortName, root.id)

				for _, def := range defs {
					if def.TargetType == "ct-events" {
						continue
					}
					result := def.Checker(ctx, clients, src, cache)
					obs := rootPivotObservation{
						count:             result.Count,
						resourceIDs:       append([]string(nil), result.ResourceIDs...),
						targetFetcherSize: len(cache[def.TargetType].Resources),
					}
					for _, id := range result.ResourceIDs {
						if idExistsInTarget(cache, def.TargetType, id) {
							obs.idsMatchedInTgt++
						}
					}
					perPivot[def.DisplayName][root.label] = obs
				}
			}

			// Strict union-assertion: every non-ct-events pivot must resolve
			// AND its ResourceIDs must match target fetcher output on at least
			// one graph-root.
			for _, def := range defs {
				if def.TargetType == "ct-events" {
					continue
				}
				observations := perPivot[def.DisplayName]

				bestCount := -1
				bestMatched := 0
				bestRoot := ""
				for rootLabel, obs := range observations {
					if obs.idsMatchedInTgt > bestMatched ||
						(obs.idsMatchedInTgt == bestMatched && obs.count > bestCount) {
						bestMatched = obs.idsMatchedInTgt
						bestCount = obs.count
						bestRoot = rootLabel
					}
				}

				switch {
				case bestCount < 1:
					t.Errorf("pivot %q (%s): Count=0 on every graph-root. Per-root: %+v. U9 violation — registered pivot must resolve on at least one graph-root.",
						def.DisplayName, def.TargetType, observations)
				case bestMatched == 0:
					t.Errorf("pivot %q (%s): Count>=1 on root %q but 0/%d returned ResourceIDs match any %s fetcher resource. IDs emitted by checker: %v. Target %s fetcher has %d resources. ID-format drift — drill-through will land empty.",
						def.DisplayName, def.TargetType, bestRoot,
						len(observations[bestRoot].resourceIDs), def.TargetType,
						observations[bestRoot].resourceIDs, def.TargetType,
						observations[bestRoot].targetFetcherSize)
				default:
					t.Logf("pivot %q (%s): OK on root %q — %+v",
						def.DisplayName, def.TargetType, bestRoot, observations[bestRoot])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestScenario_RelatedDrillNavigationLands_All — end-to-end navigation pass.
// ---------------------------------------------------------------------------

// TestScenario_RelatedDrillNavigationLands_All drives the FULL TUI navigation
// path for every registered related pivot on every graph-root fixture. It
// catches failures the direct-checker test (TestScenario_RelatedDrillThrough_All)
// cannot see:
//
//   1) Child-view landings. When a target type registers Children[Key="enter"]
//      (e.g. cfn → cfn_events, s3 → s3_objects, asg → asg_activities, tg →
//      tg_health, cb → cb_builds, ecr → ecr_images, etc.), drilling into a
//      Count=1 pivot must enter that child view and the child-view fetcher
//      must return non-empty content. A fixture gap for the drilled entity
//      (e.g. CFN events missing for a specific stack) makes the drill land
//      on an empty view even though the checker correctly reported Count>=1
//      with a matching ID.
//
//   2) Navigation integration. The direct-checker test verifies ID-format
//      parity with the target fetcher, but does NOT exercise the actual
//      TUI navigation — ResolveRelatedNavigate, handleRelatedNavigate,
//      KindDetail fast path, EnterChildViewMsg dispatch, or the child-view
//      fetcher's ParentContext handling.
//
// To avoid the async race that made earlier TUI-driven tests flaky (related
// checkers are goroutine-based; the scenario harness can't deterministically
// wait for all results), this test:
//
//   a) Runs the checker synchronously against a prefetched cache to compute
//      the expected ResourceIDs / FetchFilter.
//   b) Opens the detail view for the graph-root.
//   c) Dispatches a synthetic RelatedNavigateMsg carrying those IDs, bypassing
//      the async check path entirely.
//   d) Asserts the landing view is non-empty — detail, list, or child view.
//
// A drilled pivot that lands on Count=0 in the target view, OR that lands on
// a child view with an empty resource list, is a bug.
func TestScenario_RelatedDrillNavigationLands_All(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := buildAllTargetCache(t, clients)

	for _, group := range buildDrillThroughGroups() {
		group := group
		t.Run(group.shortName, func(t *testing.T) {
			defs := resource.GetRelated(group.shortName)
			if len(defs) == 0 {
				t.Fatalf("no related defs registered for %q", group.shortName)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			for _, root := range group.graphRoots {
				root := root
				for _, def := range defs {
					def := def
					if def.TargetType == "ct-events" {
						continue
					}
					t.Run(root.label+"/"+def.DisplayName, func(t *testing.T) {
						// 1. Compute checker result synchronously.
						src := fullIntegrationMustFindResourceByID(t, clients, group.shortName, root.id)
						result := def.Checker(ctx, clients, src, cache)
						if result.Count < 1 && len(result.ResourceIDs) == 0 && len(result.FetchFilter) == 0 {
							// Already caught by the direct-checker test as U9 violation — skip
							// drilling here. This subtest is purely about drill-lands-non-empty.
							t.Skipf("checker returned Count=0 — see direct-checker test for the U9 failure")
							return
						}

						// 2. Fresh scenario, open detail on the graph-root.
						t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
						scenario := fullIntegrationNewDemoScenario(t)
						runDemoStartup(t, scenario)
						scenario.OpenList(group.shortName)
						srcView := fullIntegrationMustFindResourceByID(t, scenario.clients, group.shortName, root.id)
						scenario.OpenDetailResource(group.shortName, srcView)
						scenario.ExpectNoAPIError()

						// 3. Dispatch synthetic RelatedNavigateMsg with the checker's
						// output — bypasses the async check-result flow entirely.
						navMsg := messages.RelatedNavigateMsg{
							TargetType:     def.TargetType,
							SourceResource: srcView,
							SourceType:     group.shortName,
							RelatedIDs:     append([]string(nil), result.ResourceIDs...),
							FetchFilter:    cloneStringMap(result.FetchFilter),
							Checker:        def.Checker,
						}
						scenario.applyAndDrain(navMsg)

						// 4. Assert landing is non-empty in one of the three shapes:
						//    (a) a new resource list arrived (top-level or child view).
						//    (b) a detail view pushed.
						//    (c) a filtered list rendered from cache with count > 0.
						rendered := scenario.currentView()

						// Shape A: ResourcesLoadedMsg populated currentListResources.
						if scenario.lastResourcesLoaded != nil {
							if len(scenario.currentListResources) == 0 {
								t.Errorf("drill %q → %s: navigation produced an empty list (type=%q). Checker emitted IDs=%v FetchFilter=%v",
									def.DisplayName, def.TargetType, scenario.currentListType,
									result.ResourceIDs, result.FetchFilter)
							}
							return
						}

						// Shape B: detail view pushed via currentResource change.
						if strings.Contains(rendered, "detail -- ") {
							return
						}

						// Shape C: filtered list from cache — detect via "(N)" title and fail on "(0)".
						targetType := def.TargetType
						titleTokens := []string{targetType}
						if td := resource.FindResourceType(targetType); td != nil && td.ListTitle != "" && td.ListTitle != targetType {
							titleTokens = append(titleTokens, td.ListTitle)
						}
						titleMatched := false
						emptyCount := false
						for _, tok := range titleTokens {
							if strings.Contains(rendered, tok+"(0)") {
								emptyCount = true
								break
							}
							if strings.Contains(rendered, tok+"(") {
								titleMatched = true
							}
						}
						if emptyCount {
							t.Errorf("drill %q → %s: list rendered with (0) — checker emitted IDs=%v that don't match any %s fetcher row. This is the exact drift the test is designed to catch.",
								def.DisplayName, def.TargetType, result.ResourceIDs, def.TargetType)
							return
						}
						if titleMatched {
							return
						}

						// Shape D: child view pushed. The child view may still be fetching
						// when applyAndDrain returns, so distinguish "loading" (in-flight
						// fetch) from "empty" (fetch returned 0 rows).
						if childTitleMatched := strings.Contains(rendered, "| Loading..."); childTitleMatched {
							t.Errorf("drill %q → %s: landed on a child view still in Loading... state after drain — the scenario harness failed to complete the child-view fetch. Pending content:\n%s",
								def.DisplayName, def.TargetType, rendered)
							return
						}
						// Child view with empty list renders "(0)" the same way a top-level
						// list does — the title-token loop above already catches that.

						// Nothing matched — no view was pushed at all.
						if scenario.lastFlash != nil {
							t.Errorf("drill %q → %s: resulted in a flash instead of a view: %q",
								def.DisplayName, def.TargetType, scenario.lastFlash.Text)
							return
						}
						t.Errorf("drill %q → %s: navigation produced neither a list, detail, nor child view. Rendered:\n%s",
							def.DisplayName, def.TargetType, rendered)
					})
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestScenario_NavigableFieldDrillThrough_All
// ---------------------------------------------------------------------------

// TestScenario_NavigableFieldDrillThrough_All verifies that every registered
// navigable field on each graph-root fixture dispatches and lands on a non-empty
// resource. Subtests for resource types with no registered NavigableFields are
// skipped honestly — there is nothing to iterate.
//
// This one still uses the TUI harness because the NavigateMsg dispatch path
// (including fieldpath extraction, NavIDFromValue ARN stripping, and target
// list resolution) IS the thing under test.
func TestScenario_NavigableFieldDrillThrough_All(t *testing.T) {
	for _, tc := range drillThroughFixtures {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			navFields := resource.GetNavigableFields(tc.shortName)
			if len(navFields) == 0 {
				t.Skipf("no navigable fields registered for %q", tc.shortName)
				return
			}

			t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
			scenario := fullIntegrationNewDemoScenario(t)
			runDemoStartup(t, scenario)

			scenario.OpenList(tc.shortName)
			root := fullIntegrationMustFindResourceByID(t, scenario.clients, tc.shortName, tc.graphRoot)

			for _, nf := range navFields {
				// Open detail fresh before each navigable-field follow so the
				// resource's RawStruct is present and the stack is at detail level.
				scenario.OpenDetailResource(tc.shortName, root)
				scenario.ExpectNoAPIError()
				landed := scenario.FollowNavigableField(nf.FieldPath)
				if landed.ID == "" {
					t.Errorf("[%s] FollowNavigableField(%q → %s): empty landing",
						tc.label, nf.FieldPath, nf.TargetType)
					scenario.Press("esc")
					continue
				}
				t.Logf("[%s] FollowNavigableField(%q → %s): landed on %q",
					tc.label, nf.FieldPath, nf.TargetType, landed.ID)
				scenario.Press("esc")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 50% Count>=2 refinement
// ---------------------------------------------------------------------------

// TestScenario_GraphRootAtLeastHalfPivotsCountGE2 enforces the U9 refinement:
// at least 50% of non-ct-events pivots must resolve to Count >= 2 on the
// graph-root union. A graph-root where every pivot resolves to exactly 1 is
// trivially connected — it does not exercise the "which of these related
// resources is the one I care about" path and gives false confidence.
func TestScenario_GraphRootAtLeastHalfPivotsCountGE2(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := buildAllTargetCache(t, clients)

	// Only resource types the user has signed off as "must have dense graph":
	// efs, opensearch, redshift. Adding others is a new row.
	dense := map[string]bool{"efs": true, "opensearch": true, "redshift": true}

	for _, group := range buildDrillThroughGroups() {
		if !dense[group.shortName] {
			continue
		}
		group := group
		t.Run(group.shortName, func(t *testing.T) {
			defs := resource.GetRelated(group.shortName)

			perPivotMaxCount := make(map[string]int, len(defs))
			for _, def := range defs {
				perPivotMaxCount[def.DisplayName] = -1
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			for _, root := range group.graphRoots {
				src := fullIntegrationMustFindResourceByID(t, clients, group.shortName, root.id)
				for _, def := range defs {
					if def.TargetType == "ct-events" {
						continue
					}
					result := def.Checker(ctx, clients, src, cache)
					if result.Count > perPivotMaxCount[def.DisplayName] {
						perPivotMaxCount[def.DisplayName] = result.Count
					}
				}
			}

			total := 0
			ge2 := 0
			for _, def := range defs {
				if def.TargetType == "ct-events" {
					continue
				}
				total++
				if perPivotMaxCount[def.DisplayName] >= 2 {
					ge2++
				}
				t.Logf("pivot %q (%s): max Count across roots = %d",
					def.DisplayName, def.TargetType, perPivotMaxCount[def.DisplayName])
			}

			if total == 0 {
				t.Fatalf("no non-ct-events pivots to evaluate for %q", group.shortName)
			}
			if ge2*2 < total {
				t.Errorf("only %d/%d non-ct-events pivots resolve to Count>=2 on graph-root union (threshold: >= 50%%). Graph-root(s) are trivially connected — U9 violation.",
					ge2, total)
			}
		})
	}
}
