// qa_childview_color_doctrine_test.go — extends the OWNER RULE gate in
// qa_color_findings_conformance_test.go ("color derives from findings") to
// CHILD views, which that gate never covers (it only walks
// resource.AllResourceTypes(), the top-level catalog — resource.AllChildTypes()
// is a separate registry entirely, see core/resource/accessors.go's
// GetChildType/AllChildTypes).
//
// OWNER BUG (acme-dev screenshot, tg_health child view): 7 of 8 targets are
// unhealthy (Reason Target.FailedHealthChecks) and ALL rows render GREEN; the
// Reason cell shows the raw dotted enum verbatim. Root cause, traced end to
// end:
//
//  1. core/aws/tg_health.go's convertTargetHealth (the tg_health
//     ChildFetcher's row converter) never populates Resource.Findings and
//     never sets Fields["status"] (only Fields["health"]).
//  2. core/aws/catalog_networking.go's tg_health ResourceTypeDef entry
//     (networkingChildTypes) has no Color func at all.
//  3. catalog.ResourceTypeDef.ResolveColor (core/catalog/types.go) falls
//     back to colorFallback(r.Fields["status"]) whenever Color is nil.
//  4. colorFallback (core/catalog/color_helpers.go) matches "" (the
//     never-set status field) against none of its known-bad buckets and
//     falls through every case to `return domain.ColorHealthy` — every
//     tg_health row renders green regardless of TargetHealth.State.
//  5. The raw SDK enum "Target.FailedHealthChecks" (elbv2types.
//     TargetHealthReasonEnumFailedHealthChecks stringified) is copied
//     verbatim into Fields["reason"], which the tg_health list/detail column
//     config (core/config/defaults_networking.go) renders directly —
//     no humanization layer exists for child-view enum fields.
//
// This file pins the ARCHITECTURAL CONTRACT (owner doctrine extended to
// child views): a child-view row carrying issue findings renders with the
// severity-derived row color, exactly like top-level lists; no raw enum
// reaches a rendered child-view cell.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newChildColorDoctrineController builds a Controller pushed directly onto a
// ScreenChildList for shortName, with childTD registered as the fallback
// typeDef exactly as views.NewChildResourceList does in production
// (internal/tui/views/resourcelist.go: c.PushChildListScreen +
// c.RegisterFallbackTypeDef) — the two calls that make buildListBody resolve
// td to the REAL child ResourceTypeDef (Color func included) instead of
// falling through to nil. Isolated per test via a temp config dir, mirroring
// newVisibilityListController in qa_issue_visibility_gate_test.go.
func newChildColorDoctrineController(t *testing.T, childTD resource.ResourceTypeDef) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.RegisterFallbackTypeDef(childTD)
	c.PushChildListScreen(childTD.ShortName)
	return c
}

// childListRowFor drives the REAL child list body build (Controller.
// ApplyResourcesLoaded + Snapshot().Body.List) and returns the ListRow
// matching resourceID plus the Status-column cell value.
func childListRowFor(t *testing.T, c *app.Controller, shortName string, fixtures []resource.Resource, resourceID string) (app.ListRow, string, bool) {
	t.Helper()
	c.ApplyResourcesLoaded(shortName, fixtures, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		return app.ListRow{}, "", false
	}
	for _, row := range body.Rows {
		if row.ResourceID != resourceID {
			continue
		}
		statusCell := ""
		if body.StatusCol >= 0 && body.StatusCol < len(row.Cells) {
			statusCell = row.Cells[body.StatusCol]
		}
		return row, statusCell, true
	}
	return app.ListRow{}, "", false
}

// fetchTargetHealthDemoResources drives the REAL production fetch path for
// tg_health: demo.NewServiceClients() -> awsclient.FetchTargetHealth (the
// tg_health ChildFetcher's underlying call) -> the unexported converter
// convertTargetHealth. This is deliberately NOT a hand-built resource.Resource
// — it is today's actual fetcher output for the ELB fixture bench's
// acme-web-tg target group (2 healthy + 1 unhealthy/Target.FailedHealthChecks,
// core/demo/fixtures/elb.go's buildTargetHealth), so a RED failure here
// indicts production code, not a test fixture.
func fetchTargetHealthDemoResources(t *testing.T) []resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	const prodWebTGARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"
	result, err := awsclient.FetchTargetHealth(t.Context(), clients.ELBv2, prodWebTGARN, "")
	if err != nil {
		t.Fatalf("FetchTargetHealth(acme-web-tg): unexpected error: %v", err)
	}
	if len(result.Resources) != 3 {
		t.Fatalf("FetchTargetHealth(acme-web-tg): got %d resources, want 3 (2 healthy + 1 unhealthy) — demo fixture in core/demo/fixtures/elb.go's buildTargetHealth changed shape, update this test's assumptions", len(result.Resources))
	}
	return result.Resources
}

// TestChildViewColorDoctrine_TGHealth_UnhealthyTargetCarriesWave1Finding pins
// item 1 of the owner doctrine: convertTargetHealth (via the exported
// FetchTargetHealth) must emit a wave1 Finding for an unhealthy target whose
// Phrase names the cause (owner: "lowercase, names the cause, e.g. contains
// 'health check'") — not silently drop the signal into Fields only.
//
// RED today: convertTargetHealth (core/aws/tg_health.go) never appends to
// Resource.Findings at all; every tg_health resource has Findings == nil
// regardless of TargetHealth.State.
func TestChildViewColorDoctrine_TGHealth_UnhealthyTargetCarriesWave1Finding(t *testing.T) {
	resources := fetchTargetHealthDemoResources(t)

	var unhealthy *resource.Resource
	for i := range resources {
		if resources[i].Fields["health"] == "unhealthy" {
			unhealthy = &resources[i]
			break
		}
	}
	if unhealthy == nil {
		t.Fatal("expected at least one unhealthy target in the acme-web-tg demo fixture, found none")
	}
	if unhealthy.Fields["reason"] != "Target.FailedHealthChecks" {
		t.Fatalf("precondition: unhealthy target Fields[reason] = %q, want %q", unhealthy.Fields["reason"], "Target.FailedHealthChecks")
	}

	if len(unhealthy.Findings) == 0 {
		t.Fatalf("unhealthy target %q (Reason=%q) carries ZERO Findings — the row has no way to communicate its cause via the findings-derived color/Attention machinery every other resource type uses; convertTargetHealth must emit a wave1 Finding for an unhealthy TargetHealth.State", unhealthy.ID, unhealthy.Fields["reason"])
	}

	var worst domain.Finding
	worstSeen := false
	for _, f := range unhealthy.Findings {
		if !worstSeen || f.Severity > worst.Severity {
			worst = f
			worstSeen = true
		}
	}
	if !worst.Severity.IsIssue() {
		t.Errorf("unhealthy target's worst Finding severity = %v, want an issue severity (SevWarn or SevBroken)", worst.Severity)
	}
	phraseLower := strings.ToLower(worst.Phrase)
	if !strings.Contains(phraseLower, "health check") {
		t.Errorf("unhealthy target's Finding.Phrase = %q, want it to name the cause (contain \"health check\")", worst.Phrase)
	}
	if worst.Source != "wave1" {
		t.Errorf("unhealthy target's worst Finding.Source = %q, want %q — tg_health has no Wave-2 enricher, this must be a wave1 (fetcher-emitted) finding", worst.Source, "wave1")
	}
}

// TestChildViewColorDoctrine_TGHealth_UnhealthyTargetRendersBrokenRow pins
// item 1's render-side observable: once tg_health is wired into a real
// ScreenChildList (RegisterFallbackTypeDef + PushChildListScreen, exactly as
// views.NewChildResourceList does for the live TUI), an unhealthy target row
// must carry the broken/warning row color via the SAME render seam every
// top-level list uses — core/app/list_columns.go's
// resolveListDecoratorFull, which is td.ResolveColor(r) fed through
// colorToTag into ListRow.Color, and IsIssue() fed into ListRow.Severity.
// A healthy sibling row in the SAME child list must stay default/healthy.
//
// RED today: tg_health's ResourceTypeDef.Color is nil (core/aws/
// catalog_networking.go's networkingChildTypes has no Color: entry for
// tg_health), so ResolveColor falls back to colorFallback(r.Fields["status"])
// — and Fields["status"] is never set by convertTargetHealth (only
// Fields["health"] is), so colorFallback("") falls through every branch to
// ColorHealthy. Every row, healthy or not, renders "healthy"/"".
func TestChildViewColorDoctrine_TGHealth_UnhealthyTargetRendersBrokenRow(t *testing.T) {
	childTD := resource.GetChildType("tg_health")
	if childTD == nil {
		t.Fatal("tg_health child resource type not registered")
	}

	resources := fetchTargetHealthDemoResources(t)

	var unhealthyID, healthyID string
	for _, r := range resources {
		switch r.Fields["health"] {
		case "unhealthy":
			unhealthyID = r.ID
		case "healthy":
			if healthyID == "" {
				healthyID = r.ID
			}
		}
	}
	if unhealthyID == "" || healthyID == "" {
		t.Fatalf("demo fixture must contain both a healthy and an unhealthy target, got unhealthyID=%q healthyID=%q", unhealthyID, healthyID)
	}

	c := newChildColorDoctrineController(t, *childTD)

	unhealthyRow, unhealthyStatusCell, ok := childListRowFor(t, c, "tg_health", resources, unhealthyID)
	if !ok {
		t.Fatalf("unhealthy target row %q not found in tg_health child list body", unhealthyID)
	}
	if unhealthyRow.Color == "healthy" || unhealthyRow.Color == "" {
		t.Errorf("unhealthy target row %q: ListRow.Color = %q, want \"broken\" or \"warning\" — the 7-of-8-unhealthy-all-green owner bug: an unhealthy tg_health row must not render with the healthy/default row color", unhealthyID, unhealthyRow.Color)
	}
	if !domain.Color(colorFromTag(unhealthyRow.Color)).IsIssue() {
		t.Errorf("unhealthy target row %q: resolved color tag %q is not an issue color (IsIssue()==false)", unhealthyID, unhealthyRow.Color)
	}
	if unhealthyStatusCell == "" {
		t.Errorf("unhealthy target row %q: Status column cell is empty — the problem is invisible on the list surface", unhealthyID)
	}

	healthyRow, _, ok := childListRowFor(t, c, "tg_health", resources, healthyID)
	if !ok {
		t.Fatalf("healthy target row %q not found in tg_health child list body", healthyID)
	}
	if healthyRow.Color != "healthy" && healthyRow.Color != "" {
		t.Errorf("healthy target row %q: ListRow.Color = %q, want \"healthy\" or \"\" (default) — a healthy target must not be miscolored either", healthyID, healthyRow.Color)
	}
}

// colorFromTag inverts core/app/list_columns.go's colorToTag so this test
// can reuse domain.Color.IsIssue() on the string tag observed on ListRow.Color
// without duplicating IsIssue's severity table. "" (colorToTag's fallthrough)
// maps to ColorHealthy, matching colorToTag's own default branch.
func colorFromTag(tag string) domain.Color {
	switch tag {
	case "warning":
		return domain.ColorWarning
	case "broken":
		return domain.ColorBroken
	case "dim":
		return domain.ColorDim
	default:
		return domain.ColorHealthy
	}
}

// TestChildViewColorDoctrine_TGHealth_ReasonCellNeverShowsRawEnum pins item 2:
// the raw dotted SDK enum ("Target.FailedHealthChecks", the stringified
// elbv2types.TargetHealthReasonEnumFailedHealthChecks) must never reach a
// rendered child-view cell verbatim — either the Reason cell is humanized, or
// the Description field carries the human sentence and callers read that
// instead. This test pins the weaker, owner-specified invariant: no raw
// dotted token appears in ANY column cell of the row, while the row must
// still communicate the cause SOMEWHERE in its cells (owner: "the row still
// communicates the cause somewhere").
//
// RED today: convertTargetHealth copies string(thd.TargetHealth.Reason)
// (the raw enum "Target.FailedHealthChecks") directly into
// Fields["reason"], and defaults_networking.go's tg_health List column
// config renders Fields["reason"] as the "Reason" column verbatim — the raw
// token reaches the rendered cell unchanged.
func TestChildViewColorDoctrine_TGHealth_ReasonCellNeverShowsRawEnum(t *testing.T) {
	childTD := resource.GetChildType("tg_health")
	if childTD == nil {
		t.Fatal("tg_health child resource type not registered")
	}

	resources := fetchTargetHealthDemoResources(t)

	var unhealthy resource.Resource
	found := false
	for _, r := range resources {
		if r.Fields["health"] == "unhealthy" {
			unhealthy = r
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected an unhealthy target in the acme-web-tg demo fixture")
	}

	const rawEnumToken = "Target.FailedHealthChecks"
	if unhealthy.Fields["reason"] != rawEnumToken {
		t.Fatalf("precondition: unhealthy target Fields[reason] = %q, want raw enum %q (fixture drifted)", unhealthy.Fields["reason"], rawEnumToken)
	}

	c := newChildColorDoctrineController(t, *childTD)
	row, _, ok := childListRowFor(t, c, "tg_health", resources, unhealthy.ID)
	if !ok {
		t.Fatalf("unhealthy target row %q not found in tg_health child list body", unhealthy.ID)
	}

	for i, cell := range row.Cells {
		if strings.Contains(cell, rawEnumToken) {
			t.Errorf("row %q cell[%d] = %q contains the raw dotted SDK enum %q verbatim — humanize it (e.g. \"failed health checks\") or route the cause through Description instead", unhealthy.ID, i, cell, rawEnumToken)
		}
	}

	causeVisible := false
	for _, cell := range row.Cells {
		lower := strings.ToLower(cell)
		if strings.Contains(lower, "health check") {
			causeVisible = true
			break
		}
	}
	if !causeVisible {
		t.Errorf("row %q: no cell communicates the cause (expected some cell to mention \"health check\" in human-readable form) — Cells=%v", unhealthy.ID, row.Cells)
	}
}

// knownColorlessChildTypes is the burn-down allowlist for the GENERIC census
// below: every registered child ResourceTypeDef whose catalog Findings table
// (catalog.FindingDef entries, the declarative source ResolveColor's
// colorWave1OrHealthy-style classifiers are meant to read) contains at least
// one issue-severity (SevWarn/SevBroken) entry, yet the type's own Color func
// does not derive from Findings at all (Color == nil, so ResolveColor uses
// colorFallback on a structural field instead of ever consulting Findings).
//
// This mirrors knownColorDivergence's ratchet contract (qa_color_findings_
// conformance_test.go) but is seeded, not empty: unlike the top-level gate
// (which the owner's prior wave already burned down to zero), this is the
// FIRST census of the child-view analogue, taken as part of the SAME PR that
// discovered the tg_health instance of the bug. Every entry here is
// today's real, verified inventory (Findings declared, Color nil) — the
// coder's worklist, not aspirational debt.
//
// Key: child ResourceTypeDef.ShortName.
//
// Census method (static, catalog-driven — see TestChildViewColorDoctrine_
// GenericCensus_FindingsChildTypesHaveColorFunc for why): walk resource.
// AllChildTypes(), keep only types with len(Findings) > 0 and at least one
// issue-severity entry, and require Color != nil. A live-fetch-driven census
// (drain each ChildFetcher through demo fixtures, mirroring qa_issue_
// visibility_gate_test.go's per-type drain) was evaluated and rejected for
// this pass: child fetchers require a real ParentContext (target_group_arn,
// bucket, zone id, listener_arn, ...) per type, most of which have no
// standalone demo-fixture entry point independent of first fetching and
// selecting a live parent row of a DIFFERENT type — that plumbing is a
// separate, larger effort than this bug-fix PR's scope. The static
// Color-vs-Findings mismatch this census checks is still a real, correct
// signal: a type cannot claim "color derives from findings" (the doctrine)
// while its Color func is nil and therefore structurally never reads
// Findings at all.
var knownColorlessChildTypes = map[string]bool{
	// tg_health: this PR's own bug — Findings will be added by the coder in
	// the SAME change that must also set Color (colorWave1OrHealthy or
	// equivalent). Present here as a starting inventory entry so the
	// generic census below does not immediately fail on the very type this
	// PR exists to fix; the concrete tests above are what pin the fix itself.
	"tg_health": true,
}

// TestChildViewColorDoctrine_GenericCensus_FindingsChildTypesHaveColorFunc is
// the RATCHET gate: for every registered child type declaring at least one
// issue-severity FindingDef in its catalog Findings table, the type's Color
// func must be non-nil (i.e. capable of deriving color from Findings at all
// — ResolveColor's fallback path, colorFallback(Fields["status"]), never
// consults Findings by construction, see core/catalog/types.go and
// core/catalog/color_helpers.go).
//
// RATCHET semantics (identical contract to knownColorDivergence /
// knownVisibilityGaps):
//   - A violation NOT in knownColorlessChildTypes is a NEW regression —
//     always fails, unconditionally.
//   - An allowlisted violation that NOW has a Color func fails with a
//     "remove from allowlist" message.
//   - An allowlisted violation still Color==nil is skipped (logged),
//     pre-existing debt — this list IS the coder's worklist.
func TestChildViewColorDoctrine_GenericCensus_FindingsChildTypesHaveColorFunc(t *testing.T) {
	childTypes := resource.AllChildTypes()

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, ct := range childTypes {
		hasIssueFinding := false
		for _, fd := range ct.Findings {
			if fd.Severity.IsIssue() {
				hasIssueFinding = true
				break
			}
		}
		if !hasIssueFinding {
			continue
		}

		key := ct.ShortName
		t.Run(key, func(t *testing.T) {
			hasColor := ct.Color != nil
			allowlisted := knownColorlessChildTypes[key]

			switch {
			case hasColor && allowlisted:
				readyForBurnDown = append(readyForBurnDown, key)
				t.Errorf("BURN-DOWN: child type %q now has a Color func but is still pinned in knownColorlessChildTypes — remove %q from the allowlist", key, key)
			case hasColor:
				// Color func present — this type CAN derive color from Findings.
			case allowlisted:
				stillGapped = append(stillGapped, key)
				t.Skipf("KNOWN GAP (allowlisted): child type %q declares %d issue-severity Findings but has no Color func — pre-existing debt, see knownColorlessChildTypes", key, len(ct.Findings))
			default:
				newlyRegressed = append(newlyRegressed, key)
				t.Errorf("NEW REGRESSION (not allowlisted): child type %q declares %d issue-severity Findings but Color is nil — ResolveColor will use colorFallback(Fields[\"status\"]) and never consult Findings at all; either set Color (e.g. colorWave1OrHealthy) or add %q to knownColorlessChildTypes", key, len(ct.Findings), key)
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

// TestChildViewColorDoctrine_GenericCensus_ColorlessChildTypesNeverEmitFindings
// is the COMPLEMENT ratchet: today's full census of child types that have
// NEITHER a Color func NOR any declared Findings at all — the exact
// structural shape of the tg_health bug before this PR (no Color, no
// Findings, so a raw structural signal like TargetHealth.State has no path
// to ever influence row color or the Attention block). This is the coder's
// broader worklist beyond tg_health: any type in this list that starts
// emitting issue-relevant status information without also wiring Color+
// Findings will silently repeat this exact bug class.
//
// Not a pass/fail correctness gate on its own (a child type with genuinely
// no issue-relevant state, e.g. a pure audit-log child view, has no bug here)
// — this is an inventory-only ratchet: shrinking is fine (burn-down),
// growing is a signal to re-audit whether the new colorless/findingsless
// child type actually carries a status signal, and MUST be re-justified by
// updating this list explicitly rather than silently drifting.
func TestChildViewColorDoctrine_GenericCensus_ColorlessChildTypesNeverEmitFindings(t *testing.T) {
	want := map[string]bool{
		"cb_build_logs":         true,
		"pipeline_stages":       true,
		"lambda_invocations":    true,
		"asg_activities":        true,
		"ecr_images":            true,
		"ecs_svc_events":        true,
		"ecs_svc_logs":          true,
		"s3_objects":            true,
		"dbi_events":            true,
		"r53_records":           true,
		"eb_rule_targets":       true,
		"sfn_executions":        true,
		"sfn_execution_history": true,
		"sns_subscriptions":     true,
		"log_streams":           true,
		"alarm_history":         true,
		"elb_listeners":         true,
		"elb_listener_rules":    true,
		"tg_health":             true,
		"iam_group_members":     true,
	}

	got := map[string]bool{}
	for _, ct := range resource.AllChildTypes() {
		if ct.Color == nil && len(ct.Findings) == 0 {
			got[ct.ShortName] = true
		}
	}

	var missingFromWant []string
	for name := range got {
		if !want[name] {
			missingFromWant = append(missingFromWant, name)
		}
	}
	if len(missingFromWant) > 0 {
		t.Errorf("NEW colorless+findingsless child type(s) not in this test's inventory: %v — verify whether the new type carries an issue-relevant status signal (repeat of the tg_health bug class) and either wire Color+Findings or add it to this test's `want` map with justification", missingFromWant)
	}

	var goneFromGot []string
	for name := range want {
		if !got[name] {
			goneFromGot = append(goneFromGot, name)
		}
	}
	if len(goneFromGot) > 0 {
		t.Logf("BURN-DOWN: child type(s) no longer colorless+findingsless (now have Color or Findings, or were removed): %v — consider trimming this test's `want` map", goneFromGot)
	}
}

// Compile-time reference to the catalog package's FindingDef so this file's
// intent (reading ct.Findings, whose element type is catalog.FindingDef via
// the resource.ResourceTypeDef = catalog.ResourceTypeDef alias) is explicit
// even though no test here constructs a catalog.FindingDef literal directly.
var _ catalog.FindingDef
