// app_console_target_test.go — Controller.ConsoleTarget() related-panel
// resolution (the closure wave: a single controller-owned resolver used by
// both the TUI's o/O keys and the web ConsoleURL snapshot field, replacing
// the TUI's own now-deleted duplicate). Follows the newRelatedSkipController
// pattern in app_related_cursor_skip_test.go: ScreenDetail pushed via
// ApplyIntents, related rows injected via the public ApplyDetailRelated
// seam, RelatedFocus turned on via ActionToggleFocus.
//
// Controller construction uses the blessed newTestController helper
// (app_controller_test.go, same package) rather than a local app.New(...)
// call — see TestControllerConstructionDisciplineGate in
// qa_controller_construction_discipline_test.go, which hard-fails any new
// unblessed construction site.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// pushDetailWithRelatedRow pushes ScreenDetail for (res, resourceType), sets
// exactly one related row (row), and turns RelatedFocus on — landing the
// cursor on that single row (mirrors newRelatedSkipController but for a
// single-row case, so no dim-skip is involved).
func pushDetailWithRelatedRow(c *app.Controller, res resource.Resource, resourceType string, row app.DetailRelatedRow) {
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, resourceType)
	c.InitDetailRelatedRows(resourceType)
	c.ApplyDetailRelated([]app.DetailRelatedRow{row})
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
}

// ─── (a) focused single-target row of a type whose rows are loaded resolves
// the FULL cached row, not a bare-ID stub ────────────────────────────────

// TestConsoleTarget_RelatedRow_CachedFullRow_ResolvesFieldsHungryURL pins the
// resolution-order contract in consoleTargetFromRelatedRow: the full row
// already in the session's RowStore (seeded here via a real top-level
// dbc-list open + ApplyResourcesLoaded) wins over both StubCreator and the
// bare-ID fallback. dbc is a deliberately Fields-hungry type — its
// ConsoleURL needs Fields["engine"], which a bare-ID stub never carries (see
// TestConsoleURL_RelatedPanelStub_FieldHungryTypeYieldsNoLink in
// console_url_types_test.go for the OLD stub-only behavior this supersedes)
// — so a resolved, non-empty URL here is only possible via the full cached
// row.
func TestConsoleTarget_RelatedRow_CachedFullRow_ResolvesFieldsHungryURL(t *testing.T) {
	c := newTestController(t)

	// Seed RowStore with the full dbc row via a real top-level list open —
	// ApplyResourcesLoaded only observes into RowStore when the top-of-stack
	// screen is the canonical top-level list for the type being seeded.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "dbc"})
	fullRow := resource.Resource{
		ID:     "prod-docdb-cluster",
		Name:   "prod-docdb-cluster",
		Type:   "dbc",
		Fields: map[string]string{"engine": "docdb"},
	}
	c.ApplyResourcesLoaded("dbc", []resource.Resource{fullRow}, nil, false)

	// Push a detail screen for a DIFFERENT resource (a dbi instance) whose
	// related panel points at the dbc cluster above.
	pushDetailWithRelatedRow(c, resource.Resource{ID: "acme-prod-db", Name: "acme-prod-db"}, "dbi", app.DetailRelatedRow{
		TargetType:  "dbc",
		DisplayName: "RDS Clusters",
		State:       domain.RelatedResolved,
		Count:       1,
		ResourceIDs: []string{"prod-docdb-cluster"},
	})

	td, res, ok := c.ConsoleTarget()
	if !ok {
		t.Fatal("ConsoleTarget() ok = false, want true for a single-target related row of a loaded type")
	}
	if td.ShortName != "dbc" {
		t.Errorf("ConsoleTarget() td.ShortName = %q, want %q", td.ShortName, "dbc")
	}
	if res.Fields["engine"] != "docdb" {
		t.Errorf("ConsoleTarget() resolved resource Fields[engine] = %q, want %q — the STUB path never carries this field, so a mismatch means the full cached row was not used", res.Fields["engine"], "docdb")
	}

	wantURL := "https://us-east-1.console.aws.amazon.com/docdb/home?region=us-east-1#cluster-details/prod-docdb-cluster"
	gotURL, urlOK := consolelink.Resolve(*td, res, "us-east-1", "")
	if !urlOK || gotURL != wantURL {
		t.Errorf("consolelink.Resolve(ConsoleTarget()) = (%q, %v), want (%q, true)", gotURL, urlOK, wantURL)
	}

	// End-to-end: Snapshot().ConsoleURL (the web/headless render path) must
	// carry the exact same URL — it is built from the same consoleTarget().
	if got := c.Snapshot().ConsoleURL; got != wantURL {
		t.Errorf("Snapshot().ConsoleURL = %q, want %q", got, wantURL)
	}
}

// ─── (b) a related row with 0 or multiple targets has no single resource to
// link to ───────────────────────────────────────────────────────────────

func TestConsoleTarget_RelatedRow_ZeroTargets_NoTarget(t *testing.T) {
	c := newTestController(t)
	pushDetailWithRelatedRow(c, resource.Resource{ID: "acme-prod-db", Name: "acme-prod-db"}, "dbi", app.DetailRelatedRow{
		TargetType:  "dbc",
		DisplayName: "RDS Clusters",
		State:       domain.RelatedResolved,
		Count:       0,
		ResourceIDs: nil, // aggregate/empty row — nothing to link to
	})

	if _, _, ok := c.ConsoleTarget(); ok {
		t.Error("ConsoleTarget() ok = true for a 0-target related row, want false")
	}
}

func TestConsoleTarget_RelatedRow_MultipleTargets_NoTarget(t *testing.T) {
	c := newTestController(t)
	pushDetailWithRelatedRow(c, resource.Resource{ID: "acme-prod-db", Name: "acme-prod-db"}, "dbi", app.DetailRelatedRow{
		TargetType:  "dbc",
		DisplayName: "RDS Clusters",
		State:       domain.RelatedResolved,
		Count:       2,
		ResourceIDs: []string{"cluster-a", "cluster-b"}, // aggregate row — no single target
	})

	if _, _, ok := c.ConsoleTarget(); ok {
		t.Error("ConsoleTarget() ok = true for a multi-target (2) related row, want false")
	}
}

// ─── (c) a type with StubCreator and no cached row still resolves via the
// stub ──────────────────────────────────────────────────────────────────

// TestConsoleTarget_RelatedRow_NoCachedRow_FallsBackToStubCreator pins the
// second resolution-order step: "ami" is the only catalog type carrying a
// StubCreator (core/aws/catalog_compute.go), and its ConsoleURL only needs
// r.ID (no Fields), so the synthesized stub resolves a real console URL even
// though the ami RowStore was never populated (no top-level ami list open in
// this test).
func TestConsoleTarget_RelatedRow_NoCachedRow_FallsBackToStubCreator(t *testing.T) {
	c := newTestController(t)
	pushDetailWithRelatedRow(c, resource.Resource{ID: "i-0a1b2c3d4e5f60001", Name: "web-prod-01"}, "ec2", app.DetailRelatedRow{
		TargetType:  "ami",
		DisplayName: "AMI",
		State:       domain.RelatedResolved,
		Count:       1,
		ResourceIDs: []string{"ami-0a1b2c3d4e5f60001"},
	})

	td, res, ok := c.ConsoleTarget()
	if !ok {
		t.Fatal("ConsoleTarget() ok = false, want true via the StubCreator fallback")
	}
	if td.ShortName != "ami" {
		t.Errorf("ConsoleTarget() td.ShortName = %q, want %q", td.ShortName, "ami")
	}
	if res.ID != "ami-0a1b2c3d4e5f60001" {
		t.Errorf("ConsoleTarget() resolved resource ID = %q, want %q", res.ID, "ami-0a1b2c3d4e5f60001")
	}

	wantURL := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#ImageDetails:imageId=ami-0a1b2c3d4e5f60001"
	if got := c.Snapshot().ConsoleURL; got != wantURL {
		t.Errorf("Snapshot().ConsoleURL = %q, want %q", got, wantURL)
	}
}
