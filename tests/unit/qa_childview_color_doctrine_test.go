// The "color derives from findings"
// gate of qa_color_findings_conformance_test.go, applied to CHILD views:
// resource.AllChildTypes() is a separate registry from
// resource.AllResourceTypes() (core/resource/accessors.go).
//
// A child-view row carrying issue findings renders with the severity-derived
// row color, exactly like top-level lists, and no raw SDK enum reaches a
// rendered child-view cell.
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
// falling through to nil. Isolated per test via a temp config dir.
func newChildColorDoctrineController(t *testing.T, childTD resource.ResourceTypeDef) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
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
	// Four registrations: three instances, one of them registered a second
	// time on another port, and one of the four failing its checks.
	if len(result.Resources) != 4 {
		t.Fatalf("FetchTargetHealth(acme-web-tg): got %d resources, want 4 (3 healthy + 1 unhealthy) — demo fixture in core/demo/fixtures/elb.go's buildTargetHealth changed shape, update this test's assumptions", len(result.Resources))
	}
	return result.Resources
}

// TestChildViewColorDoctrine_TGHealth_UnhealthyTargetCarriesWave1Finding:
// convertTargetHealth (core/aws/tg_health.go, via the exported
// FetchTargetHealth) emits a wave1 Finding for an unhealthy target whose
// Phrase names the cause (lowercase, e.g. contains 'health check').
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

// TestChildViewColorDoctrine_TGHealth_UnhealthyTargetRendersBrokenRow: with
// tg_health wired into a real ScreenChildList (RegisterFallbackTypeDef +
// PushChildListScreen, as views.NewChildResourceList does for the live TUI),
// an unhealthy target row carries the broken/warning row color via the same
// render seam every top-level list uses — core/app/list_columns.go's
// resolveListRowSeverity. A healthy sibling row in the same child list stays
// healthy.
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

// TestChildViewColorDoctrine_TGHealth_ReasonCellNeverShowsRawEnum: the raw
// dotted SDK enum ("Target.FailedHealthChecks", the stringified
// elbv2types.TargetHealthReasonEnumFailedHealthChecks) never reaches a
// rendered child-view cell, while the row still communicates the cause in
// some cell.
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

// knownColorlessChildTypes is the allowlist for the generic census below:
// registered child types whose catalog Findings table holds at least one
// issue-severity entry while their Color func is nil, so ResolveColor uses
// colorFallback on a structural field and never consults Findings.
// Key: child ResourceTypeDef.ShortName.
//
// Census method (static, catalog-driven — see TestChildViewColorDoctrine_
// GenericCensus_FindingsChildTypesHaveColorFunc for why): walk resource.
// AllChildTypes(), keep only types with len(Findings) > 0 and at least one
// issue-severity entry, and require Color != nil. The census is static
// because child fetchers require a real ParentContext (target_group_arn,
// bucket, zone id, listener_arn, ...) per type, most of which have no
// standalone demo-fixture entry point independent of first fetching and
// selecting a live parent row of a DIFFERENT type. The static
// Color-vs-Findings mismatch is a correct signal on its own: a type cannot
// claim "color derives from findings" (the doctrine) while its Color func
// is nil and therefore structurally never reads Findings at all.
var knownColorlessChildTypes = map[string]bool{
	// tg_health derives its colour from its target-health findings.
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
//     known debt — this list IS the worklist.
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
			case allowlisted:
				stillGapped = append(stillGapped, key)
				t.Skipf("KNOWN GAP (allowlisted): child type %q declares %d issue-severity Findings but has no Color func — pre-existing debt, see knownColorlessChildTypes", key, len(ct.Findings))
			default:
				newlyRegressed = append(newlyRegressed, key)
				t.Errorf("NEW REGRESSION (not allowlisted): child type %q declares %d issue-severity Findings but Color is nil — ResolveColor will use colorFallback(Fields[\"status\"]) and never consult Findings at all; either set Color (e.g. colorAnyFindingOrHealthy) or add %q to knownColorlessChildTypes", key, len(ct.Findings), key)
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
// is the complement ratchet: the census of child types with neither a Color
// func nor any declared Findings, so a raw structural signal has no path to
// row color or the Attention block. A child type with no issue-relevant state
// (e.g. a pure audit-log view) belongs here; growth of the list must be
// re-justified by updating it explicitly.
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
