// wave3_list_ports_test.go — Wave 3 (022-codebase-cleanup) PORT pins for the
// ResourceList/MainMenu family. Ports three narrow, verified-missing behavior
// pins from the doomed legacy view-model tests (specs/022-codebase-cleanup/
// wave3-map-list-menu.md) onto the LIVE controller seams, so the legacy
// pins can be deleted in a later round without losing coverage:
//
//  1. listFilterResources (internal/app/list_filter.go) — Fields-value and
//     Findings-phrase text-filter match branches. internal/app/list_test.go's
//     TestListFilter_* only exercises Name-based matches.
//  2. reapplyCheckerAgainst (internal/app/list_filter.go), driven through the
//     real c.Handle(messages.ResourcesLoaded{...}) event path — merge-across-
//     LoadMore, non-approx extension, zero-initial filtering, and sort
//     preservation. Existing coverage (reapply_checker_leak_test.go,
//     headless_regression_test.go) only pins leak-prevention and payload
//     shape, never the merge/grow/sort behavior itself.
//  3. resolveListMarkerCol (internal/app/list_columns.go) — byte-parity
//     against the legacy identity-column cascade (resolveIdentityColumn in
//     internal/tui/views/table_render.go), across every real catalog type.
//     tui_viewstate_purity_list_test.go's MarkerCol case only covers the
//     render-CONSUMPTION side (translate body.MarkerCol into a glyph) with
//     one synthetic type; it never checks that resolveListMarkerCol
//     COMPUTES the same index the legacy cascade would for real types. This
//     is deliberately self-contained (no shared helpers with
//     resourcelist_render_parity_test.go) so it keeps running after that
//     file is deleted.
package unit_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// Shared controller helper (distinctly named to avoid colliding with — and
// avoid depending on — the doomed parity/purity test files' own helpers).
// ---------------------------------------------------------------------------

// wave3ListController builds a Controller (via the blessed newTestController
// helper — see qa_controller_construction_discipline_test.go) pre-navigated
// to a ScreenResourceList for the given resource type ShortName.
func wave3ListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// newListController is a compatibility shim: qa_controller_frame_title_issue_badge_test.go
// and qa_title_warning_findings_test.go depend on a helper of this exact name
// that used to live in resourcelist_render_parity_test.go (deleted as part of
// the Wave 3 parity-bridge retirement, its own construction now routed
// through wave3ListController/newTestController instead of a raw app.New
// call). Kept here so those two out-of-scope files keep compiling unchanged.
func newListController(t *testing.T, shortName string) *app.Controller {
	return wave3ListController(t, shortName)
}

// ===========================================================================
// 1. listFilterResources — Fields-value + Findings-phrase match branches
// ===========================================================================

// wave3FilterEC2Resources returns 3 EC2 instances distinguishing the filter
// branches under test: private IP and instance type live in Fields (matched
// by the generic Fields-value loop); the third instance carries NO matching
// Name/Fields signal at all — it can only be found via its Findings[].Phrase.
func wave3FilterEC2Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0aaa111111111111a",
				"name":        "web-server",
				"state":       "running",
				"type":        "m5.large",
				"private_ip":  "10.0.48.11",
				"public_ip":   "203.0.113.20",
			},
		},
		{
			ID: "i-0bbb222222222222b", Name: "db-server", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0bbb222222222222b",
				"name":        "db-server",
				"state":       "stopped",
				"type":        "g4dn.xlarge",
				"private_ip":  "10.0.48.55",
				"public_ip":   "",
			},
		},
		{
			ID: "i-0ccc333333333333c", Name: "cache-node", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0ccc333333333333c",
				"name":        "cache-node",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.99.7",
				"public_ip":   "",
			},
			Findings: []domain.Finding{
				{Code: "ec2.degraded", Phrase: "instance degraded", Severity: domain.SevWarn},
			},
		},
	}
}

func TestWave3ListFilter_MatchesFieldsValue_PrivateIP(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "10.0.48"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 2 {
		t.Fatalf("filter '10.0.48' (private_ip Fields value): Rows count: got %d want 2", len(lb.Rows))
	}
	got := map[string]bool{lb.Rows[0].ResourceID: true, lb.Rows[1].ResourceID: true}
	if !got["i-0aaa111111111111a"] || !got["i-0bbb222222222222b"] {
		t.Errorf("filter '10.0.48': wrong rows matched: %v", got)
	}
}

func TestWave3ListFilter_MatchesFieldsValue_PublicIP(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "203.0.113.20"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter '203.0.113.20' (public_ip Fields value): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0aaa111111111111a" {
		t.Errorf("filter '203.0.113.20': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

func TestWave3ListFilter_MatchesFieldsValue_InstanceType(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "g4dn"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'g4dn' (instance-type Fields value): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0bbb222222222222b" {
		t.Errorf("filter 'g4dn': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

// TestWave3ListFilter_MatchesFindingsPhrase_CaseInsensitive isolates the
// third listFilterResources branch (r.Findings[i].Phrase), which no
// controller-path test exercised before this file: cache-node has no Name or
// Fields value containing "degraded" — it can only be found via its finding
// phrase. Also proves the match is case-insensitive, mirroring the ID/Name/
// Fields branches.
func TestWave3ListFilter_MatchesFindingsPhrase_CaseInsensitive(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "DEGRADED"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'DEGRADED' (Findings[].Phrase, case-insensitive): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0ccc333333333333c" {
		t.Errorf("filter 'DEGRADED': wrong ResourceID: got %q — should match cache-node via its finding phrase only", lb.Rows[0].ResourceID)
	}
}

// ===========================================================================
// 2. reapplyCheckerAgainst — merge-across-LoadMore, non-approx extension,
//    zero-initial filtering, sort preservation, no-checker inert.
//    Driven through the REAL c.Handle(messages.ResourcesLoaded{...}) event
//    path (handle.go calls reapplyCheckerAgainst only from there — the
//    ApplyResourcesLoaded test seam does not).
// ===========================================================================

// wave3VPCSGChecker mirrors the shape of a real reverse-scan checker (e.g.
// checkVPCSecurityGroup): matches "sg" resources whose Fields["vpc_id"]
// equals the source VPC's ID.
func wave3VPCSGChecker(_ context.Context, _ any, src resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	entry, ok := cache["sg"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	var matched []string
	for _, r := range entry.Resources {
		if r.Fields["vpc_id"] == src.ID {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "sg", Count: len(matched), ResourceIDs: matched}
}

func wave3SG(id, vpcID string) resource.Resource {
	return resource.Resource{
		ID: id, Name: id, Type: "sg",
		Fields: map[string]string{"group_name": id, "group_id": id, "vpc_id": vpcID},
	}
}

// TestWave3RelatedCheckerCarry_ZeroInitialGrowsOnLoadMore verifies that a
// (0+) truncated pivot (empty initial RelatedIDSet, non-nil so it hides
// everything) both hides unrelated rows immediately AND grows as further
// LoadMore pages arrive — the merge accumulates across pages, it does not
// reset.
func TestWave3RelatedCheckerCarry_ZeroInitialGrowsOnLoadMore(t *testing.T) {
	c := wave3ListController(t, "sg")
	c.PatchListReapplyChecker(wave3VPCSGChecker, resource.Resource{ID: "vpc-target"})

	// Page 1: two SGs, neither in vpc-target. Zero-initial carry must hide
	// both immediately (nil filter != empty filter).
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Resources: []resource.Resource{
		wave3SG("sg-1", "vpc-other-1"),
		wave3SG("sg-2", "vpc-other-2"),
	}})
	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 0 {
		t.Fatalf("after page1 (no matches): want 0 visible rows, got %d", len(lb.Rows))
	}

	// Page 2 (load-more): one new match.
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-3", "vpc-target"),
		wave3SG("sg-4", "vpc-other-3"),
	}})
	lb = *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("after page2 (1 match): want 1 visible row, got %d", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "sg-3" {
		t.Errorf("after page2: wrong matched row: got %q want sg-3", lb.Rows[0].ResourceID)
	}

	// Page 3 (load-more): two more matches — the set must grow, not reset.
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-5", "vpc-target"),
		wave3SG("sg-6", "vpc-target"),
	}})
	lb = *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("after page3 (2 more matches): want 3 visible rows (grown, not reset), got %d", len(lb.Rows))
	}
}

// TestWave3RelatedCheckerCarry_NonApproxStillExtends verifies that even a
// non-truncated pivot (initial RelatedIDSet seeded from an exact match count,
// not a (0+)/(N+) scan) keeps extending on LoadMore — the carry mechanism is
// identical regardless of how the initial set was seeded.
func TestWave3RelatedCheckerCarry_NonApproxStillExtends(t *testing.T) {
	c := wave3ListController(t, "sg")
	c.PatchListRelatedIDSet([]string{"sg-a", "sg-b"})
	c.PatchListReapplyChecker(wave3VPCSGChecker, resource.Resource{ID: "vpc-target"})

	if got := len(c.GetListRelatedIDSet()); got != 2 {
		t.Fatalf("precondition: RelatedIDSet size = %d, want 2", got)
	}

	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-c", "vpc-target"),
	}})

	if got := len(c.GetListRelatedIDSet()); got != 3 {
		t.Errorf("after load-more match: RelatedIDSet size = %d, want 3 (sg-a, sg-b, sg-c)", got)
	}
}

// TestWave3RelatedCheckerCarry_PreservesSortAfterMerge verifies that when the
// checker-carry merge grows the RelatedIDSet, the active sort is honored for
// the newly visible rows — they land in sorted position, not in arrival
// order.
func TestWave3RelatedCheckerCarry_PreservesSortAfterMerge(t *testing.T) {
	c := wave3ListController(t, "sg")

	alwaysMatch := func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		entry, ok := cache["sg"]
		if !ok {
			return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
		}
		ids := make([]string, len(entry.Resources))
		for i, r := range entry.Resources {
			ids[i] = r.ID
		}
		return resource.RelatedCheckResult{TargetType: "sg", Count: len(ids), ResourceIDs: ids}
	}
	c.PatchListReapplyChecker(alwaysMatch, resource.Resource{ID: "vpc-target"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "group_name"})

	// Rows arrive in reverse-alpha order; if sort is re-applied after the
	// merge, visible order must be alphabetical ascending regardless.
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Resources: []resource.Resource{
		wave3SG("sg-zeta", "vpc-target"),
		wave3SG("sg-mu", "vpc-target"),
		wave3SG("sg-alpha", "vpc-target"),
	}})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("want 3 visible rows, got %d", len(lb.Rows))
	}
	got := []string{lb.Rows[0].Cells[0], lb.Rows[1].Cells[0], lb.Rows[2].Cells[0]}
	want := []string{"sg-alpha", "sg-mu", "sg-zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort not re-applied after checker merge; got order %v, want %v", got, want)
		}
	}
}

// TestWave3RelatedCheckerCarry_NoChecker_Inert verifies that a list without a
// carried checker is unaffected by the merge machinery — the feature is
// opt-in and must not regress ordinary (non-related) list loads.
func TestWave3RelatedCheckerCarry_NoChecker_Inert(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.PatchListRelatedIDSet([]string{"i-1", "i-2"})
	// No PatchListReapplyChecker call.

	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: []resource.Resource{
		{ID: "i-3", Name: "i-3", Type: "ec2", Fields: map[string]string{"instance_id": "i-3"}},
	}})

	if got := len(c.GetListRelatedIDSet()); got != 2 {
		t.Errorf("without a carried checker, RelatedIDSet must remain untouched by Handle; size = %d, want 2", got)
	}
}

// ===========================================================================
// 3. resolveListMarkerCol — S13 parity-bridge precondition. Self-contained
//    byte-parity check (legacy ResourceListModel.View() vs the live
//    RenderList(ListBody) seam) across every real catalog type, so this
//    survives the future deletion of resourcelist_render_parity_test.go.
// ===========================================================================

func wave3MarkerColResources(td resource.ResourceTypeDef, n int) []resource.Resource {
	statuses := []string{"running", "stopped", "pending", "available", "active", "terminated"}
	lk := td.LifecycleKey
	if lk == "" {
		lk = "state"
	}
	out := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("%s-%03d", td.ShortName, i+1)
		fields := make(map[string]string, len(td.Columns)+4)
		for _, col := range td.Columns {
			switch {
			case col.Key == "name" || strings.Contains(strings.ToLower(col.Key), "name"):
				fields[col.Key] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
			case col.Key == lk || col.Key == "state" || col.Key == "status":
				fields[col.Key] = statuses[i%len(statuses)]
			default:
				fields[col.Key] = fmt.Sprintf("v-%s-%d", col.Key, i+1)
			}
		}
		fields["name"] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
		fields[lk] = statuses[i%len(statuses)]
		out[i] = resource.Resource{ID: id, Name: fmt.Sprintf("demo-%s-%d", td.ShortName, i+1), Fields: fields}
	}
	return out
}

func wave3MarkerColFindings(resources []resource.Resource) map[string][]domain.Finding {
	if len(resources) == 0 {
		return nil
	}
	out := make(map[string][]domain.Finding, len(resources)/2+1)
	for i, r := range resources {
		if i%2 == 0 {
			sev := domain.SevBroken
			if i%4 == 0 {
				sev = domain.SevWarn
			}
			out[r.ID] = []domain.Finding{{Code: "WAVE3-MARKERCOL", Phrase: "wave3 test finding", Severity: sev}}
		}
	}
	return out
}

func wave3AssertMarkerColParity(t *testing.T, typeName string, m *views.ResourceListModel, body app.ListBody) {
	t.Helper()
	legacy := m.View()
	got := m.RenderList(body)
	if got == legacy {
		return
	}
	legacyLines := strings.Split(legacy, "\n")
	gotLines := strings.Split(got, "\n")
	maxLines := len(legacyLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf(
		"[%s] S13 MarkerCol parity: RenderList differs from View() (MarkerCol=%d) — View() %d lines, RenderList %d lines\n",
		typeName, body.MarkerCol, len(legacyLines), len(gotLines),
	))
	for i := range maxLines {
		leg, g := "", ""
		if i < len(legacyLines) {
			leg = legacyLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if leg != g {
			diff.WriteString(fmt.Sprintf("  line %d:\n    View():     %q\n    RenderList: %q\n", i+1, leg, g))
		}
	}
	t.Errorf("%s", diff.String())
}

// TestWave3MarkerColParity_EnrichmentFindings_AllResourceTypes ports the
// unique behavior scenario S13 exercised from resourcelist_render_parity_test.go
// (the "PRIMARY MarkerCol gap detector": enrichment glyphs force the render
// path to actually consume MarkerCol, unlike a plain unfiltered list where a
// wrong index can go unnoticed). It runs across every registered resource
// type so a step-3-cascade divergence (column Path contains "Name"/
// "Identifier" but Key isn't "name") in ANY type is caught, not just one
// synthetic type.
func TestWave3MarkerColParity_EnrichmentFindings_AllResourceTypes(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — catalog not registered")
	}

	const stdW, stdH = 160, 30

	for _, td := range allTypes {
		t.Run(td.ShortName, func(t *testing.T) {
			tuitest.NoColor(t)
			k := keys.Default()
			resources := wave3MarkerColResources(td, 6)
			findings := wave3MarkerColFindings(resources)

			m := views.NewResourceList(td, nil, k)
			m.SetSize(stdW, stdH)
			m, _ = m.Update(messages.ResourcesLoaded{Resources: resources, ResourceType: td.ShortName})
			m.SetEnrichmentState(len(findings), false, findings, nil)

			c := wave3ListController(t, td.ShortName)
			c.ApplyResourcesLoaded(td.ShortName, resources, nil, false)
			c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings, nil)
			body := *c.Snapshot().Body.List

			wave3AssertMarkerColParity(t, td.ShortName, &m, body)
		})
	}
}

// TestWave3MarkerColParity_EnrichmentFindingsWithHScroll_AllResourceTypes
// ports S15 from resourcelist_render_parity_test.go: MarkerCol is a
// full-column-space index computed once by resolveListMarkerCol, but the
// GLYPH is placed at a SEPARATE, hscroll-translated visible column index
// (markerColIdx) — a distinct code path from the un-scrolled S13 case above.
// When the identity column scrolls off-screen, both the legacy cascade and
// the live translation must agree the glyph is hidden (no cell wrongly
// prefixed), not just that MarkerCol itself resolves to the right index.
// Neither the S13 pin above (no hscroll) nor tui_viewstate_purity_list_test.go
// (single synthetic type, no hscroll) exercises this interaction — this is a
// second, independent gap from the same parity-bridge precondition check.
func TestWave3MarkerColParity_EnrichmentFindingsWithHScroll_AllResourceTypes(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — catalog not registered")
	}

	const stdW, stdH = 160, 30

	for _, td := range allTypes {
		t.Run(td.ShortName, func(t *testing.T) {
			if len(td.Columns) < 2 {
				t.Skip("type has fewer than 2 columns — hscroll not applicable")
			}
			tuitest.NoColor(t)
			k := keys.Default()
			resources := wave3MarkerColResources(td, 6)
			findings := wave3MarkerColFindings(resources)

			m := views.NewResourceListFromCache(
				td, nil, k,
				resources, nil,
				"",
				views.SortColNone, true,
				0, 1, false, // hScrollOffset=1
			)
			m.SetSize(stdW, stdH)
			m.SetEnrichmentState(len(findings), false, findings, nil)

			c := wave3ListController(t, td.ShortName)
			c.ApplyResourcesLoaded(td.ShortName, resources, nil, false)
			c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings, nil)
			c.Apply(app.Action{Kind: app.ActionScrollRight})
			body := *c.Snapshot().Body.List

			wave3AssertMarkerColParity(t, td.ShortName, &m, body)
		})
	}
}

// ===========================================================================
// 4. applyListFilters attention branch (round 4 unique-pin port) — CodeRabbit
//    PR-273: a resource whose Wave-1 Color always resolves Healthy and whose
//    embedded r.Findings is empty must still be shown under the attention
//    filter (ctrl+z) when it carries a Wave-2 enrichment finding correlated
//    by ID only (c.listEnrichmentFindings, fed by ApplyEnrichmentState).
//    internal/app/list_test.go's TestListAttention_* only exercise resources
//    with a populated r.Findings — this branch (`len(r.Findings) == 0` +
//    `findings[r.ID]` lookup in applyListFilters) was otherwise unpinned.
//    Ported from the deleted qa_attention_filter_test.go's
//    TestAttentionFilter_IncludesResourcesWithFindings.
// ===========================================================================

func TestWave3AttentionFilter_IncludesResourcesWithWave2OnlyFindings(t *testing.T) {
	c := wave3ListController(t, "s3")

	// s3's Color always resolves Healthy regardless of Fields, so all three
	// resources below fail the Wave-1 IsIssue() gate — only the Wave-2
	// enrichment-map lookup can surface res-0 under the attention filter.
	resources := []resource.Resource{
		{ID: "res-0", Name: "bucket-alpha", Fields: map[string]string{"name": "bucket-alpha"}},
		{ID: "res-1", Name: "bucket-beta", Fields: map[string]string{"name": "bucket-beta"}},
		{ID: "res-2", Name: "bucket-gamma", Fields: map[string]string{"name": "bucket-gamma"}},
	}
	c.ApplyResourcesLoaded("s3", resources, nil, false)

	findings := map[string][]domain.Finding{
		"res-0": {{Code: "s3.public.access.enabled", Phrase: "public access enabled", Severity: domain.SevWarn, Source: "wave2:s3"}},
	}
	c.ApplyEnrichmentState("s3", 1, false, findings, nil)
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("attention filter with Wave-2-only finding: Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "res-0" {
		t.Errorf("attention filter must show res-0 (Wave-2 finding only, Wave-1 Color Healthy), got %q", lb.Rows[0].ResourceID)
	}
}

// ===========================================================================
// 5. handleResourcesLoadedEvent mismatched/alias ResourceType routing (round
//    4 unique-pin port) — a ResourcesLoaded event is applied only to a screen
//    whose canonicalized ResourceType (resource.FindResourceType(...).ShortName)
//    matches the message's own canonicalized ResourceType; a late fetch for a
//    different type must never populate the active list, and an alias on
//    either side (e.g. "rds" wire-stamped for canonical "dbi") must still
//    match. Ported from the deleted resourcelist_mismatched_type_test.go's
//    TestResourceListModel_ResourcesLoaded_DropsMismatchedType/
//    AppliesMatchingType — the alias-specific cases were not otherwise pinned
//    at the controller level (internal/app/handle.go:187-230 documents the
//    canonicalization intent but had no positive test).
// ===========================================================================

func TestWave3ResourcesLoaded_DropsMismatchedType(t *testing.T) {
	cases := []struct {
		name          string
		listShortName string // active screen's resource type
		staleType     string // ResourceType on the mismatched message (alias or canonical)
	}{
		{"S3 list rejects EC2 rows", "s3", "ec2"},
		{"EC2 list rejects S3 rows", "ec2", "s3"},
		// "rds" is a registered alias for canonical ShortName "dbi".
		{"S3 list rejects RDS alias rows", "s3", "rds"},
		{"S3 list rejects RDS canonical rows", "s3", "dbi"},
		{"Lambda list rejects EC2 rows", "lambda", "ec2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := wave3ListController(t, tc.listShortName)

			c.Handle(messages.ResourcesLoaded{
				ResourceType: tc.staleType,
				Resources: []resource.Resource{
					{ID: "x-0001", Name: "x-0001", Fields: map[string]string{"id": "x-0001"}},
				},
			})

			lb := c.Snapshot().Body.List
			if lb != nil && len(lb.Rows) != 0 {
				t.Errorf("mismatched ResourcesLoaded (%s -> %s) must not populate rows: got %d rows, want 0", tc.staleType, tc.listShortName, len(lb.Rows))
			}
		})
	}
}

func TestWave3ResourcesLoaded_AppliesMatchingType(t *testing.T) {
	cases := []struct {
		name          string
		listShortName string
		msgType       string // may be an alias
	}{
		{"S3 exact match", "s3", "s3"},
		{"EC2 exact match", "ec2", "ec2"},
		// The fetcher stamps "rds" (alias) on the wire while the list holds
		// canonical ShortName "dbi" — the alias-aware match must not drop it.
		{"RDS alias match (rds to dbi)", "dbi", "rds"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := wave3ListController(t, tc.listShortName)

			c.Handle(messages.ResourcesLoaded{
				ResourceType: tc.msgType,
				Resources: []resource.Resource{
					{ID: "res-a", Name: "res-a", Fields: map[string]string{"id": "res-a"}},
					{ID: "res-b", Name: "res-b", Fields: map[string]string{"id": "res-b"}},
				},
			})

			lb := c.Snapshot().Body.List
			if lb == nil || len(lb.Rows) != 2 {
				got := 0
				if lb != nil {
					got = len(lb.Rows)
				}
				t.Errorf("matching-type ResourcesLoaded (%s -> %s) did not populate rows: got %d, want 2", tc.msgType, tc.listShortName, got)
			}
		})
	}
}
