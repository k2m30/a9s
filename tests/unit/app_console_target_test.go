// Controller.ConsoleTarget() is the single resolver behind the TUI's o/O keys
// and the web ConsoleURL snapshot field.
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
// exactly one related row, and turns RelatedFocus on, landing the cursor on
// that row.
func pushDetailWithRelatedRow(c *app.Controller, res resource.Resource, resourceType string, row app.DetailRelatedRow) {
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, resourceType)
	c.InitDetailRelatedRows(resourceType)
	c.ApplyDetailRelated([]app.DetailRelatedRow{row})
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
}

// consoleTargetFromRelatedRow prefers the full row already in the session's
// RowStore over StubCreator and the bare-ID fallback. dbc's ConsoleURL needs
// Fields["engine"], which a bare-ID stub never carries, so a non-empty URL
// here comes only from the full cached row.
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

// A related row with 0 or multiple targets has no single resource to link to.

func TestConsoleTarget_RelatedRow_ZeroTargets_NoTarget(t *testing.T) {
	c := newTestController(t)
	pushDetailWithRelatedRow(c, resource.Resource{ID: "acme-prod-db", Name: "acme-prod-db"}, "dbi", app.DetailRelatedRow{
		TargetType:  "dbc",
		DisplayName: "RDS Clusters",
		State:       domain.RelatedResolved,
		Count:       0,
		ResourceIDs: nil,
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
		ResourceIDs: []string{"cluster-a", "cluster-b"},
	})

	if _, _, ok := c.ConsoleTarget(); ok {
		t.Error("ConsoleTarget() ok = true for a multi-target (2) related row, want false")
	}
}

// "ami" is the catalog type with a StubCreator (core/aws/catalog_compute.go),
// and its ConsoleURL needs only r.ID, so the synthesized stub resolves a real
// console URL with the ami RowStore empty.
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
