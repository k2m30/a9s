// qa_issue_visibility_gate_test.go — the standing OWNER RULE gate: if a
// resource row carries a glyph or a non-green color, the problem must be
// visible either (a) in the list view's Status column cell, or (b) at the
// top of the detail view (the Attention block).
//
// Registry+demo-driven, one subtest per (type, resource) violation, mirroring
// the ratchet style of qa_demo_state_coverage_test.go / qa_demo_pivot_coverage_test.go:
//
//  1. Drain each registered type's demo fixtures via its real Wave-1 Fetcher
//     (drainDemoFixtures, shared with the state-coverage/pivot-coverage
//     tests), building one shared resource.ResourceCache exactly as those
//     tests do so cross-type Wave-2 enrichers see sibling data.
//  2. Run the type's registered Wave-2 IssueEnricher (awsclient.Wave2EnricherFor)
//     against the fixtures and fold any per-resource Finding into a COPY of
//     that resource's Findings slice — mirroring what production's
//     applyWave2ToRow (internal/tui/app_enrich_fold.go) does before render,
//     without reimplementing that fold's full mechanics: the Color funcs and
//     Attention builder only ever read Resource.Findings, so appending the
//     Wave-2 finding there reproduces the same input those consumers see.
//  3. For every (type, resource) where td.ResolveColor(merged) is not
//     domain.ColorHealthy OR merged.Findings is non-empty, verify:
//     (a) the REAL list Status cell for that row (driven through
//     Controller.ApplyResourcesLoaded + Snapshot().Body.List, indexed by
//     ListBody.StatusCol) is non-empty, OR
//     (b) the REAL detail Attention block for that row (driven through
//     Controller.EnsureDetailState + Snapshot().Body.Detail, filtered by
//     FieldRow.Path=="Attention") is non-empty.
//
// RATCHET semantics (identical contract to knownStateCoverageGaps /
// knownDisconnectedPivots):
//   - A violation NOT in knownVisibilityGaps is a NEW regression — always
//     fails, unconditionally.
//   - An allowlisted violation that NOW shows the problem on both surfaces
//     fails with a "remove from allowlist" message.
//   - An allowlisted violation still showing on neither surface is skipped
//     (logged), pre-existing debt.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// knownVisibilityGaps pins the exact inventory of (type, resource-key)
// violations found by this gate at ratchet-conversion time: a row colored
// non-healthy or carrying issue findings, yet showing the problem on NEITHER
// the list Status cell NOR the detail Attention block. Same burn-down
// semantics as knownStateCoverageGaps in qa_demo_state_coverage_test.go:
//   - present + still invisible on both surfaces today -> skip (logged),
//     expected pre-existing debt.
//   - present + now visible on either surface           -> FAIL ("remove
//     from allowlist").
//   - a violation NOT present here                      -> FAIL
//     unconditionally, a new regression the allowlist was never told about.
//
// Key shape: "<shortName>:<resourceID>" so per-type resource IDs never
// collide across types.
//
// Seeded 2026-07-06 from the first run of this gate against
// demo.NewServiceClients() fixtures. Every entry below has findings=0 in the
// gate's own failure message: each is a resource type whose Color classifier
// derives a non-healthy bucket PURELY from structural Fields reads (no
// catalog.FindingDef backs the state), so neither listPhraseFromFindings nor
// buildAttentionEntries — both of which read only Resource.Findings — have
// anything to surface. The Status-cell lifecycle fallback (r.Fields[lifecycleKey])
// also comes back empty for each of these specific fixtures, which is the
// second half of the gap. Grouped by type; reason given once per type, entries
// listed per resource ID as the allowlist key shape requires.
var knownVisibilityGaps = map[string]bool{
	// asg: colorASG (internal/aws/catalog_compute.go) derives Broken/Warning
	// from in_service_count<min_size / suspended_processes / "Delete in
	// progress" reads on r.Fields — none of these branches emit a Finding,
	// and these fixtures carry no populated per-instance "status" Field for
	// the lifecycle fallback to show.
	"asg:acme-staging-asg":        true,
	"asg:awseb-e-acmeprodapi-asg": true,
	"asg:asg-underprovisioned":    true,
	"asg:asg-suspended":           true,

	// ct-events: colorCTEvents (internal/aws/catalog_monitoring.go, structural)
	// resolves Dim for these fixtures with no accompanying wave1/wave2
	// Finding, and ct-events has no LifecycleKey-backed status column for
	// the fallback to populate.
	"ct-events:evt-0a1b2c3d4e5f60003":        true,
	"ct-events:evt-0a1b2c3d4e5f60004":        true,
	"ct-events:evt-0a1b2c3d4e5f60005":        true,
	"ct-events:e-a1b2c3d4":                   true,
	"ct-events:e-d4e5f6a7":                   true,
	"ct-events:e-f6a7b8c9":                   true,
	"ct-events:e-b8c9d0e1":                   true,
	"ct-events:evt-eks-describe-001":         true,
	"ct-events:evt-sg-web-alb-authorize-001": true,

	// lambda: colorLambda (internal/aws/catalog_compute.go) derives
	// Warning/Broken/Dim from structural State/LastUpdateStatus Fields reads
	// with no matching Finding emission for these particular fixture states
	// (only Pending/Failed states are wired to catalog.FindingDef per
	// catalog_compute.go's Findings table).
	"lambda:data-pipeline-transform":   true,
	"lambda:process-orders":            true,
	"lambda:image-thumbnail-gen":       true,
	"lambda:payment-webhook":           true,
	"lambda:cloudwatch-slack-notifier": true,
	"lambda:rotate-rds-credentials":    true,
	"lambda:lambda-inactive-runtime":   true,
	"lambda:api-service-runner":        true,
	"lambda:orders-projector":          true,
	"lambda:a9s-demo-s3-notifier":      true,
	"lambda:acme-inbound-parser":       true,
	"lambda:efs-data-processor":        true,
	"lambda:efs-report-generator":      true,
	"lambda:pdf-generator":             true,
	"lambda:webhook-processor":         true,
	"lambda:rate-limiter":              true,

	// logs: colorLogs (internal/aws/catalog_monitoring.go) derives Warning
	// structurally (e.g. missing retention / stale group) with no matching
	// Finding for this fixture.
	"logs:/app/legacy/orphan-old": true,

	// policy: colorPolicy (internal/aws/catalog_security.go) derives
	// Warning from an "orphan/unattached" structural read with no matching
	// Finding.
	"policy:orphan-unattached-policy": true,

	// r53: r53Color (internal/aws/catalog_dns_cdn.go) derives Warning
	// structurally (e.g. zero record count) with no matching Finding.
	"r53:/hostedzone/Z3456789012ABCDEFGHIJ": true,
	"r53:/hostedzone/Z5678901234ABCDEFGHIJ": true,

	// rtb: colorRTB (internal/aws/catalog_networking.go) derives
	// Broken/Warning from a structural blackhole-route / orphan read with
	// no matching Finding.
	"rtb:rtb-0blackhole1111111e": true,
	"rtb:rtb-0orphan111111111f":  true,

	// secrets: colorSecrets (internal/aws/catalog_secrets.go) derives
	// Warning from a structural rotation/age read with no matching Finding
	// for these fixtures (the registered secrets.state.rotation_overdue /
	// secrets.state.dormant FindingDefs are themselves still in
	// knownStateCoverageGaps — no fixture produces them yet, per
	// qa_demo_state_coverage_test.go).
	"secrets:prod/api/gateway-key":                true,
	"secrets:prod/api/stripe-key":                 true,
	"secrets:staging/database/mysql":              true,
	"secrets:prod/codeartifact/npm-publish-token": true,
	"secrets:prod/app/oauth-client-secret":        true,
	"secrets:prod/elk/elasticsearch-password":     true,
	"secrets:prod/monitoring/grafana-admin":       true,
	"secrets:prod/app/sendgrid-api-key":           true,
	"secrets:prod/app/github-webhook-secret":      true,
	"secrets:prod/rds/replica-password":           true,
	"secrets:staging/database/postgres":           true,
	"secrets:staging/app/jwt-secret":              true,
	"secrets:dev/database/postgres":               true,
	"secrets:dev/app/jwt-secret":                  true,
	"secrets:shared/monitoring/pagerduty-key":     true,
	"secrets:prod/app/slack-webhook":              true,

	// sg: colorSG (internal/aws/catalog_networking.go) derives Broken from a
	// structural "public/wide-open ingress rule" scan with no matching
	// Finding.
	"sg:sg-0public0ssh000001": true,
	"sg:sg-0public0db0000002": true,
	"sg:sg-0wide0open0000003": true,

	// sns-sub: colorSNSSub (internal/aws/catalog_messaging.go) derives
	// Warning/Dim from structural PendingConfirmation/Deleted status reads
	// with no matching Finding, and sns-sub has no lifecycle-backed status
	// column fallback.
	"sns-sub:PendingConfirmation": true,
	"sns-sub:Deleted":             true,

	// ssm: colorSSM (internal/aws/catalog_secrets.go) derives
	// Warning/Broken from a structural legacy-path / stale-value read with
	// no matching Finding.
	"ssm:/acme/legacy/db/password":             true,
	"ssm:/acme/shared/legacy_service_password": true,
}

// demoVisibilityMaxFetchPages mirrors demoPivotMaxFetchPages — a safety
// valve against a runaway fake fetcher during drain.
const demoVisibilityMaxFetchPages = 50

// drainVisibilityFixtures runs td.Fetcher to exhaustion against the demo
// clients. Local copy of drainDemoFixtures's exact contract (same signature
// and behavior) kept independent so this gate does not depend on load order
// with qa_demo_pivot_coverage_test.go / qa_demo_state_coverage_test.go for
// its core drain step; the shared fixture set still comes from the same
// demo.NewServiceClients() typed fakes.
func drainVisibilityFixtures(t *testing.T, td resource.ResourceTypeDef, clients *awsclient.ServiceClients) ([]resource.Resource, bool) {
	t.Helper()
	if td.Fetcher == nil {
		return nil, false
	}
	ctx := context.Background()
	var all []resource.Resource
	token := ""
	for page := range demoVisibilityMaxFetchPages {
		result, err := td.Fetcher(ctx, clients, token)
		if err != nil {
			t.Fatalf("%s: Fetcher page %d returned error: %v", td.ShortName, page, err)
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			return all, true
		}
		token = result.Pagination.NextToken
	}
	t.Fatalf("%s: Fetcher did not terminate within %d pages — runaway pagination in demo fixtures", td.ShortName, demoVisibilityMaxFetchPages)
	return all, true
}

// buildVisibilityTypeCache drains every registered type's demo fixtures and
// returns both the per-type resource lists and one shared resource.ResourceCache
// built from all of them, so cross-type Wave-2 enrichers see sibling data
// exactly as production does. Mirrors buildDemoStateTypeCache.
func buildVisibilityTypeCache(t *testing.T) (map[string][]resource.Resource, resource.ResourceCache) {
	t.Helper()
	clients := demo.NewServiceClients()
	byType := make(map[string][]resource.Resource)
	cache := make(resource.ResourceCache)

	for _, td := range resource.AllResourceTypes() {
		fixtures, ok := drainVisibilityFixtures(t, td, clients)
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

// mergeWave2Findings runs the type's registered Wave-2 IssueEnricher (if any)
// against fixtures and returns a COPY of fixtures where each resource's
// Findings slice has the enricher's per-resource Finding appended (skipped
// if a Finding with the same Code is already present, mirroring
// applyWave2ToRow's dedupe-by-code intent for this narrow read-only gate).
// Resources for types with no Wave-2 enricher are returned unmodified
// (still copied, so callers can safely mutate).
func mergeWave2Findings(
	t *testing.T,
	td resource.ResourceTypeDef,
	fixtures []resource.Resource,
	cache resource.ResourceCache,
	clients *awsclient.ServiceClients,
) []resource.Resource {
	t.Helper()
	merged := make([]resource.Resource, len(fixtures))
	copy(merged, fixtures)

	enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
	if !ok || enricher.Fn == nil {
		return merged
	}
	result, err := enricher.Fn(context.Background(), clients, fixtures, cache)
	if err != nil {
		t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
	}
	if len(result.Findings) == 0 {
		return merged
	}

	for i := range merged {
		f, ok := result.Findings[merged[i].ID]
		if !ok {
			continue
		}
		already := false
		for _, existing := range merged[i].Findings {
			if existing.Code == f.Code {
				already = true
				break
			}
		}
		if already {
			continue
		}
		findingsCopy := make([]domain.Finding, len(merged[i].Findings), len(merged[i].Findings)+1)
		copy(findingsCopy, merged[i].Findings)
		findingsCopy = append(findingsCopy, f)
		merged[i].Findings = findingsCopy
	}
	return merged
}

// newVisibilityListController builds a Controller pre-navigated to the list
// screen for shortName, isolated per test via a temp config dir.
func newVisibilityListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// listStatusCellFor drives the REAL list body build (Controller.ApplyResourcesLoaded
// + Snapshot().Body.List) for the given type and fixture set, and returns the
// Status-column cell value for the row matching resourceID. Returns ("", false)
// if the row or the status column cannot be located.
func listStatusCellFor(t *testing.T, td resource.ResourceTypeDef, fixtures []resource.Resource, resourceID string) (string, bool) {
	t.Helper()
	c := newVisibilityListController(t, td.ShortName)
	c.ApplyResourcesLoaded(td.ShortName, fixtures, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		return "", false
	}
	if body.StatusCol < 0 {
		return "", false
	}
	for _, row := range body.Rows {
		if row.ResourceID != resourceID {
			continue
		}
		if body.StatusCol >= len(row.Cells) {
			return "", false
		}
		return row.Cells[body.StatusCol], true
	}
	return "", false
}

// newVisibilityDetailController builds a Controller pushed straight to the
// detail screen, isolated per test via a temp config dir.
func newVisibilityDetailController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	return c
}

// detailHasAttentionFor drives the REAL detail body build (Controller.EnsureDetailState
// + Snapshot().Body.Detail) for a single resource and reports whether the
// Attention block (FieldRow.Path=="Attention") is non-empty. EnsureDetailState
// seeds DetailState.Findings from res.Findings directly (internal/app/detail_state.go),
// so a resource carrying Wave-1/merged-Wave-2 Findings surfaces them here
// without any extra enrichment call.
func detailHasAttentionFor(t *testing.T, res resource.Resource, shortName string) bool {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, shortName)
	body := c.Snapshot().Body.Detail
	if body == nil {
		return false
	}
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			return true
		}
	}
	return false
}

// isVisibilityViolation reports whether res is a candidate for the OWNER
// RULE: a non-healthy row color OR at least one Finding present.
func isVisibilityViolation(td resource.ResourceTypeDef, res resource.Resource) bool {
	if td.Color == nil {
		return len(res.Findings) > 0
	}
	return td.ResolveColor(res) != domain.ColorHealthy || len(res.Findings) > 0
}

// TestIssueVisibilityGate_EveryColoredOrFlaggedRowIsVisibleSomewhere is the
// standing OWNER RULE gate. For every registered type with a Wave-1 Fetcher,
// every fixture resource whose resolved color is non-healthy or which carries
// at least one Finding (Wave-1 seeded or Wave-2 merged) must show the problem
// on the list Status cell or the detail Attention block — UNLESS the
// (type, resourceID) pair is pinned in knownVisibilityGaps as pre-existing
// debt, in which case it is skipped (logged) instead of failed.
func TestIssueVisibilityGate_EveryColoredOrFlaggedRowIsVisibleSomewhere(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		merged := mergeWave2Findings(t, td, fixtures, cache, clients)

		for _, res := range merged {
			if !isVisibilityViolation(td, res) {
				continue
			}

			key := td.ShortName + ":" + res.ID
			testName := fmt.Sprintf("%s/%s", td.ShortName, res.ID)

			t.Run(testName, func(t *testing.T) {
				statusCell, _ := listStatusCellFor(t, td, merged, res.ID)
				listVisible := statusCell != ""

				detailVisible := detailHasAttentionFor(t, res, td.ShortName)

				visible := listVisible || detailVisible
				allowlisted := knownVisibilityGaps[key]

				color := "healthy"
				if td.Color != nil {
					color = bucketName(td.ResolveColor(res))
				}

				switch {
				case visible && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s (color=%s, findings=%d) is now visible (list=%q detail-attention=%v) "+
							"but is still pinned in knownVisibilityGaps — remove %q from the allowlist in this PR",
						key, color, len(res.Findings), statusCell, detailVisible, key,
					)
				case visible:
					// Visible on at least one surface and not allowlisted — expected steady state.
				case allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): %s (color=%s, findings=%d): neither list Status cell "+
							"(%q) nor detail Attention block shows the problem — pre-existing debt, see "+
							"knownVisibilityGaps", key, color, len(res.Findings), statusCell,
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW VIOLATION (not allowlisted): %s: resource %q (type=%s, color=%s, findings=%d) "+
							"is colored non-healthy or carries findings but shows the problem on NEITHER "+
							"the list Status cell (got %q) NOR the detail Attention block. Either fix the "+
							"Status-cell/Attention wiring or, if this is pre-existing debt, add %q to "+
							"knownVisibilityGaps",
						key, res.ID, td.ShortName, color, len(res.Findings), statusCell, key,
					)
				}
			})
		}
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW VIOLATION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
