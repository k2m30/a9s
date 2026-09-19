// A related-panel row that
// resolved unknown on a cold cache (State: RelatedUnknown, no FetchFilter, no
// RelatedIDs) renders with no count badge. Enter on it resolves in place: the
// detail stays on screen, the checks are re-dispatched, and once the target
// cache is warm the row shows its numeric badge.
//
// The pair is "ng" -> "ebs": checkNGEBS (core/aws/ng_related.go) joins against
// the "ec2" RowStore entry by tag rather than calling AWS, so it returns
// resource.UnknownRelated("ebs") while "ec2" is cold (docs/resources/ng.md).
//
// "ng" registers several related defs, and the keyboard cursor
// (detailSkipUnselectableRelated / stepToSelectable, core/app/detail_cursor.go)
// stops only on an actionable row. scopeNGToEBSOnly re-registers "ng" with its
// real ebs def alone so that row sits at cursor 0.
//
// Assertions read the ANSI-stripped rendered view: tui.Model exposes no
// controller accessor from tests/unit. Package unit (not unit_test) reaches
// the root-model harness helpers.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// transientUnknownNGResource returns a node-group resource with no registered
// fetcher of its own, the same shape as ngResourceForCacheMissBadge in
// related_unknown_badge_test.go.
func transientUnknownNGResource() resource.Resource {
	return resource.Resource{
		ID:   "prod-workers",
		Name: "prod-workers",
		Fields: map[string]string{
			"nodegroup_name": "prod-workers",
			"cluster_name":   "prod-cluster",
			"status":         "ACTIVE",
		},
	}
}

// scopeNGToEBSOnly captures the live ng->ebs RelatedDef ("EBS Volumes",
// checkNGEBS) and re-registers "ng" with only that def for the calling test
// (reset via t.Cleanup(resource.CleanupRelatedForTest)), so the lone row is
// the cursor-0 landing spot while TargetType, DisplayName and Checker stay the
// production values.
func scopeNGToEBSOnly(t *testing.T) resource.RelatedDef {
	t.Helper()
	var ebsDef resource.RelatedDef
	found := false
	for _, def := range resource.GetRelated("ng") {
		if def.TargetType == "ebs" {
			ebsDef = def
			found = true
			break
		}
	}
	if !found {
		t.Fatal("test setup: no ng->ebs RelatedDef registered in production (expected \"EBS Volumes\", checkNGEBS)")
	}
	resource.SetRelatedForTest("ng", []resource.RelatedDef{ebsDef})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ng") })
	return ebsDef
}

// transientUnknownSetup builds a demo root model, opens the ng detail via a
// real Navigate message, and feeds one messages.RelatedCheckResult carrying
// checkNGEBS's cold-cache output (State: RelatedUnknown, no FetchFilter, no
// RelatedIDs). "ec2" stays unloaded so the unknown state is genuine.
func transientUnknownSetup(t *testing.T) (tui.Model, resource.Resource, resource.RelatedDef) {
	t.Helper()

	def := scopeNGToEBSOnly(t)
	ngRes := transientUnknownNGResource()

	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	// tui.WithClients only seeds the construction options; the runtime Core's
	// ServiceClients, which runtime/fetchers.go checks before any live fetch, are
	// set only by a messages.ClientsReady dispatch (handleClientsReady ->
	// core.HandleClientsReady).
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: demo.NewServiceClients()})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ng",
		Resource:     &ngRes,
	})

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ng",
		SourceResourceID: ngRes.ID,
		DefDisplayName:   def.DisplayName,
		Result:           resource.UnknownRelated("ebs"),
	})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+ngRes.ID) {
		t.Fatalf("setup did not land on ng detail (\"detail -- %s\"); got:\n%s", ngRes.ID, view)
	}
	// Four-state contract: a transient resolved-unknown row shows NO count badge
	// (blank), never "(?)". It must be present and badge-less before the drill.
	if !strings.Contains(view, def.DisplayName) || strings.Contains(view, def.DisplayName+" (") {
		t.Fatalf("test setup: ng detail's related panel must show %q as a blank (no-count) transient row before driving Enter; got view:\n%s",
			def.DisplayName, view)
	}

	return m, ngRes, def
}

// focusRelatedRow toggles right-column focus with Tab, which sets the
// controller's RelatedCursor to 0; with "ng" scoped to one def, the cursor
// lands on the ebs row.
func focusRelatedRow(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	return m
}

// Enter on a scoreless row (no ResourceIDs, no FetchFilter) resolves in place;
// a pushed list would replace the "detail -- <id>" frame.
func TestTransientUnknownDrill_EnterResolvesInPlaceStaysOnDetail(t *testing.T) {
	m, ngRes, def := transientUnknownSetup(t)
	m = focusRelatedRow(m)

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = drainCmds(t, m, cmd, 6)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+ngRes.ID) {
		t.Fatalf("BUG: Enter on the scoreless row (%s) must RESOLVE IN PLACE (stay on the ng detail), not navigate to a list; view:\n%s",
			def.DisplayName, view)
	}
}

// TestTransientUnknownDrill_EnterRecomputesInPlace verifies that once the
// "ec2" cache the ng->ebs pivot joins against is warm, Enter on the scoreless
// row re-dispatches the checks and the row firms up to a numeric badge — in
// place, still on the ng detail, with no target list ever pushed.
func TestTransientUnknownDrill_EnterRecomputesInPlace(t *testing.T) {
	m, ngRes, def := transientUnknownSetup(t)
	m = focusRelatedRow(m)

	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(t.Context(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: ec2Res})

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = drainCmds(t, m, cmd, 8)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+ngRes.ID) {
		t.Fatalf("scoreless-row Enter must stay on the ng detail (resolve in place); view:\n%s", view)
	}
	if !strings.Contains(view, def.DisplayName+" (") {
		t.Fatalf("BUG: scoreless-row Enter must RECOMPUTE %q to a numeric badge in place; row still blank. View:\n%s",
			def.DisplayName, view)
	}
}
