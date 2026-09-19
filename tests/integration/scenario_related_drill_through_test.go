//go:build integration

package integration

// Drill-through checks.
//
// These tests verify that every registered related-panel pivot and navigable
// field on the graph-root fixture resolves to at least one real resource in
// the demo cache. An empty landing means the checker produced a resource ID
// in a format that does not match the target resource type's Resource.ID
// field.
//
// TestScenario_RelatedDrillThrough_All runs checkers directly against a
// prefetched ResourceCache rather than through the TUI event loop:
//
//   - Detail-open fires related checks in parallel, so a NeedsTargetCache
//     checker driven through the TUI can run against a still-populating
//     cache and commit Count=0 before the sibling fetchers finish.
//   - `DrillRelated` on the FetchFilter path fetches and filters
//     server-side, so it succeeds even when the checker's ResourceIDs do not
//     match the target fetcher's Resource.ID format.
//
// With the cache prewarmed before any checker runs, every pivot is
// evaluated and every returned ResourceID is checked against the target
// fetcher's output.
//
// TestScenario_NavigableFieldDrillThrough_All uses the TUI harness because
// the NavigateMsg dispatch path is the thing under test there.

import (
	"context"
	"strings"
	"testing"
	"time"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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
	// are mutually exclusive per AWS LogDestinationType.
	{"redshift/acme-warehouse", "redshift", demofixtures.AcmeWarehouseID},
	{"redshift/acme-reporting", "redshift", demofixtures.AcmeReportingID},
	// dbi-snap: graph-root is the non-Aurora prod fixture. The dbc pivot has
	// no realistic non-zero case for dbi-snap (Aurora cluster snapshots live
	// in dbc-snap), so dbc=0 is the AWS-API truth here.
	{"dbi-snap/prod", "dbi-snap", demofixtures.ProdDBISnapID},
	// dbc-snap: covers BOTH Aurora and DocumentDB cluster snapshots
	// (DescribeDBClusterSnapshots is engine-agnostic). Aurora root covers
	// the dbc back-pivot to prod-aurora-cluster; DocDB root covers the
	// non-Aurora case.
	{"dbc-snap/aurora", "dbc-snap", demofixtures.ProdDBCSnapAuroraID},
	{"dbc-snap/docdb", "dbc-snap", demofixtures.ProdDBCSnapDocDBID},
	{"mwaa/prod-airflow-etl", "mwaa", demofixtures.ProdAirflowEtlID},
	{"transfer/prod-as2-gateway", "transfer", demofixtures.ProdAS2GatewayID},
	{"transfer/sftp-lambda-auth", "transfer", demofixtures.SftpLambdaAuthID},
	// lt: two graph-roots — field pivots (ami/kms/sg) + cache cross-refs
	// (asg/ec2) live on prod-web-lt; the NI-path pivots (ng/subnet) on
	// eks-node-lt.
	{"lt/prod-web-lt", "lt", demofixtures.ProdWebLTID},
	{"lt/eks-node-lt", "lt", demofixtures.EKSNodeLTID},
	{"vpc-peer/prod-peer-shared", "vpc-peer", demofixtures.ProdPeerSharedID},
}

// drillThroughGroups collapses the flat fixture list into groups sharing a
// shortName. Resource types with multiple graph-roots (redshift) are
// evaluated with UNION semantics: each pivot must resolve on at least one
// root, because redshift's logs/s3 destinations are mutually exclusive.
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
// Every NeedsTargetCache=true checker's reads are served from this, so no
// checker races the async cache population.
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
			all = append(all, out.Resources...)
			if err != nil {
				// Demo mode may not implement every AWS service — ignore and
				// move on; a partial-failure page still carried its rows.
				break
			}
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
// entry has Resource.ID == id. The comparison is exact so ID-format drift
// between checker and fetcher fails.
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

// TestScenario_RelatedDrillThrough_All enforces:
//
//   - every non-ct-events pivot on each resource type's graph-root union must
//     have Count >= 1 AND return at least one ResourceID that exists as a
//     Resource.ID in the target type's fetcher output.
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
						count:             result.Count(),
						resourceIDs:       append([]string(nil), result.ResourceIDs()...),
						targetFetcherSize: len(cache[def.TargetType].Resources),
					}
					for _, id := range result.ResourceIDs() {
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

// TestScenario_RelatedDrillNavigationLands_All drives the FULL TUI navigation
// path for every registered related pivot on every graph-root fixture. It
// catches failures the direct-checker test (TestScenario_RelatedDrillThrough_All)
// cannot see:
//
//  1. Child-view landings. When a target type registers Children[Key="enter"]
//     (e.g. cfn → cfn_events, s3 → s3_objects, asg → asg_activities, tg →
//     tg_health, cb → cb_builds, ecr → ecr_images, etc.), drilling into a
//     Count=1 pivot must enter that child view and the child-view fetcher
//     must return non-empty content. A fixture gap for the drilled entity
//     (e.g. CFN events missing for a specific stack) makes the drill land
//     on an empty view even though the checker correctly reported Count>=1
//     with a matching ID.
//
//  2. Navigation integration. The direct-checker test verifies ID-format
//     parity with the target fetcher, but does NOT exercise the actual
//     TUI navigation — ResolveRelatedNavigate, handleRelatedNavigate,
//     NavigationKindDetail fast path, EnterChildViewMsg dispatch, or the child-view
//     fetcher's ParentContext handling.
//
// Related checkers are goroutine-based and the scenario harness cannot
// deterministically wait for all results, so this test:
//
//	a) Runs the checker synchronously against a prefetched cache to compute
//	   the expected ResourceIDs / FetchFilter.
//	b) Opens the detail view for the graph-root.
//	c) Dispatches a synthetic RelatedNavigateMsg carrying those IDs, bypassing
//	   the async check path entirely.
//	d) Asserts the landing view is non-empty — detail, list, or child view.
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
						src := fullIntegrationMustFindResourceByID(t, clients, group.shortName, root.id)
						result := def.Checker(ctx, clients, src, cache)
						if result.Count() < 1 && len(result.ResourceIDs()) == 0 && len(result.FetchFilter()) == 0 {
							// TestScenario_RelatedDrillThrough_All reports an unresolved pivot.
							t.Skipf("checker returned Count=0 — see direct-checker test for the U9 failure")
							return
						}

						t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
						scenario := fullIntegrationNewDemoScenario(t)
						runDemoStartup(t, scenario)
						scenario.OpenList(group.shortName)
						srcView := fullIntegrationMustFindResourceByID(t, scenario.clients, group.shortName, root.id)
						scenario.OpenDetailResource(group.shortName, srcView)
						scenario.ExpectNoAPIError()

						navMsg := messages.RelatedNavigate{
							TargetType:     def.TargetType,
							SourceResource: srcView,
							SourceType:     group.shortName,
							RelatedIDs:     append([]string(nil), result.ResourceIDs()...),
							FetchFilter:    cloneStringMap(result.FetchFilter()),
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
									result.ResourceIDs(), result.FetchFilter())
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
								def.DisplayName, def.TargetType, result.ResourceIDs(), def.TargetType)
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

// TestScenario_NavigableFieldDrillThrough_All verifies that every registered
// navigable field on each graph-root fixture dispatches and lands on a non-empty
// resource. Subtests for resource types with no registered NavigableFields are
// skipped honestly — there is nothing to iterate.
//
// This one uses the TUI harness because the NavigateMsg dispatch path
// (including fieldpath extraction, NavIDFromValue ARN stripping, and target
// list resolution) IS the thing under test.
func TestScenario_NavigableFieldDrillThrough_All(t *testing.T) {
	// Union semantics across graph roots of the same type, mirroring the
	// related-pivot walk above: a conditional field (e.g. transfer's
	// FTPS-only Certificate) absent on one root skips there, but every
	// registered navigable field must land on at least one root of its type.
	required := map[string]bool{}
	witnessed := map[string]bool{}

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
				key := tc.shortName + "/" + nf.FieldPath
				required[key] = true
				// Presence probe: scalar paths first, then scalar-on-list
				// paths (e.g. "VpcSecurityGroups.VpcSecurityGroupId") which
				// ExtractScalar reports as "" by design.
				if fieldpath.ExtractScalar(root.RawStruct, nf.FieldPath) == "" &&
					fieldpath.ExtractFirstListScalar(root.RawStruct, nf.FieldPath) == "" {
					t.Logf("[%s] navigable field %q absent on this root (conditional field) — union check applies",
						tc.label, nf.FieldPath)
					continue
				}
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
				witnessed[key] = true
				t.Logf("[%s] FollowNavigableField(%q → %s): landed on %q",
					tc.label, nf.FieldPath, nf.TargetType, landed.ID)
				scenario.Press("esc")
			}
		})
	}

	for key := range required {
		if !witnessed[key] {
			t.Errorf("navigable field %s landed on no graph root — add a root fixture that witnesses it", key)
		}
	}
}

// TestScenario_GraphRootAtLeastHalfPivotsCountGE2 enforces that at least
// 50% of non-ct-events pivots resolve to Count >= 2 on the
// graph-root union. A graph-root where every pivot resolves to exactly 1 is
// trivially connected — it does not exercise the "which of these related
// resources is the one I care about" path and gives false confidence.
func TestScenario_GraphRootAtLeastHalfPivotsCountGE2(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := buildAllTargetCache(t, clients)

	// Only these resource types are held to a dense graph.
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
					if result.Count() > perPivotMaxCount[def.DisplayName] {
						perPivotMaxCount[def.DisplayName] = result.Count()
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
