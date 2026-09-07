// table_render_dead_mirror_ports_test.go — live-seam port, from the
// 022-codebase-cleanup re-audit, for the internal/tui/views/table_render.go dead-mirror
// foursome (phraseFromFindings, resolveIdentityColumn, lifecycleColumnKey,
// widenLifecycleColumn): specs/022-codebase-cleanup/reaudit.md "port+delete:
// table_render.go foursome ... Pinned only by two in-package test files
// (resolve_identity_internal_test.go, widen_lifecycle_column_internal_test.go).
// Port the cascade pins to app.IdentityColumnIndex tests and the widen pins
// to renderListWidenLifecycleColumn, then delete."
//
// Both dead functions are unexported (table_render.go, package views) and so
// are their two pinning test files (internal/tui/views/*_internal_test.go) —
// outside this agent's tests/unit/ write scope. This file re-pins their
// behavior through the two LIVE production seams instead, both reachable
// from tests/unit/ as an external (package unit_test) black-box:
//
//  1. IdentityColumnIndex (core/app/list_columns.go) — mirrors
//     resolveIdentityColumn's 5-step cascade exactly (its own doc comment:
//     "Cascade must match resolveIdentityColumn exactly"). Driven via a
//     fully-controlled per-type config.ViewsConfig (GetViewDef replaces
//     userDef.List wholesale, giving byte-for-byte control over each
//     column's Key/Path/Title) + RegisterFallbackTypeDef (controls
//     IdentityKey/Name), read back via Snapshot().Body.List.MarkerCol — the
//     exported field buildListBody bakes IdentityColumnIndex's return into.
//  2. renderListWidenLifecycleColumn (internal/tui/views/resourcelist.go) —
//     NOT a byte-identical mirror of the dead widenLifecycleColumn (it widens
//     from body.Rows[i].Cells verbatim, post-bake, not by re-deriving from
//     r.Findings/r.Fields), so only the one pin with no live equivalent is
//     ported: the AS-566 regression the dead test's own header names ("prior
//     to this fix, widenLifecycleColumn measured only r.Findings[0].Phrase
//     ... causing the column to truncate the displayed status"). The other
//     three dead-test cases (single-finding, no-findings-fallback, ignores
//     legacy Fields["status"]) are default list-render behavior already
//     covered incidentally by every other RenderList-driven test in this
//     package and are not re-pinned here.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// wave3MarkerColControllerWith builds a Controller navigated to the real "ec2"
// catalog command (so ActionCommand routing succeeds) but with columns and
// identity resolution fully overridden: cfg's Views["ec2"].List replaces the
// built-in column set wholesale (config.GetViewDef: "user-provided fields
// override defaults" when len(userDef.List) > 0), and RegisterFallbackTypeDef
// makes buildListBody prefer td (IdentityKey/Name) over the ec2 catalog entry
// ("the model's explicitly-supplied typeDef rather than the catalog's when
// they differ" — RegisterFallbackTypeDef's own doc). Both must be set before
// the first Snapshot()-triggering call, matching SetViewConfig's contract.
func wave3MarkerColControllerWith(t *testing.T, td resource.ResourceTypeDef, cols []config.ListColumn) *app.Controller {
	t.Helper()
	td.ShortName = "ec2"
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{"ec2": {List: cols}}}
	c := newTestController(t)
	c.SetViewConfig(cfg)
	c.RegisterFallbackTypeDef(td)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	return c
}

// wave3MarkerColOf applies a single resource and returns the resolved
// full-column-space MarkerCol index — the live equivalent of calling
// resolveIdentityColumn(cols, td) directly.
func wave3MarkerColOf(t *testing.T, c *app.Controller) int {
	t.Helper()
	c.ApplyResourcesLoaded("ec2", []resource.Resource{{ID: "r1", Name: "r1"}}, nil, false)
	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("nil list body after ApplyResourcesLoaded")
	}
	return lb.MarkerCol
}

// ===========================================================================
// IdentityColumnIndex cascade — port of resolve_identity_internal_test.go's
// TestResolveIdentityColumn_* cases (cascade order: 1. td.IdentityKey matches
// a column's key; 2. column key == "name"; 3. column path contains "Name" or
// "Identifier"; 4. column title equals "Name" (case-insensitive) or td.Name;
// 5. fall back to index 0). The dead test's EmptyColumns case is dropped —
// config.GetViewDef only replaces defaults when len(userDef.List) > 0, so an
// intentionally-empty column list cannot be driven through this live seam;
// the guard itself is a trivial zero-iteration loop with no branch to lose.
// ===========================================================================

func TestResolveListMarkerCol_MatchesIdentityKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "EC2 Instances", IdentityKey: "foo"}
	cols := []config.ListColumn{
		{Key: "status", Title: "Status", Width: 10},
		{Key: "region", Title: "Region", Width: 10},
		{Key: "foo", Title: "Foo", Width: 10},
		{Key: "name", Title: "Name", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 2 {
		t.Errorf("MarkerCol with IdentityKey=%q: got %d, want 2", td.IdentityKey, got)
	}
}

func TestResolveListMarkerCol_FallsThroughToNameKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "RDS Instances"}
	cols := []config.ListColumn{
		{Key: "id", Title: "ID", Width: 10},
		{Key: "name", Title: "Name", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 1 {
		t.Errorf("MarkerCol via name key: got %d, want 1", got)
	}
}

func TestResolveListMarkerCol_FallsThroughToPath_Identifier(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "RDS Instances"}
	cols := []config.ListColumn{
		{Key: "id", Title: "ID", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
		{Path: "DBInstanceIdentifier", Title: "DB Instance", Width: 10},
		{Path: "Engine", Title: "Engine", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 2 {
		t.Errorf("MarkerCol via path=DBInstanceIdentifier: got %d, want 2", got)
	}
}

func TestResolveListMarkerCol_FallsThroughToPath_NameInPath(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "Lambda Functions"}
	cols := []config.ListColumn{
		{Title: "Arn", Width: 10},
		{Path: "FunctionName", Title: "Function", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 1 {
		t.Errorf("MarkerCol via path containing Name: got %d, want 1", got)
	}
}

func TestResolveListMarkerCol_FallsThroughToTitle(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "ACM Certificates"}
	cols := []config.ListColumn{
		{Title: "ARN", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "Region", Width: 10},
		{Title: "Name", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 3 {
		t.Errorf("MarkerCol via title=Name: got %d, want 3", got)
	}
}

func TestResolveListMarkerCol_CaseInsensitiveTitleMatch(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "ACM Certificates"}
	cols := []config.ListColumn{
		{Title: "ARN", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "NAME", Width: 10},
		{Title: "Region", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 2 {
		t.Errorf("MarkerCol case-insensitive title match: got %d, want 2", got)
	}
}

func TestResolveListMarkerCol_TitleMatchesTypeName(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "S3 Buckets"}
	cols := []config.ListColumn{
		{Title: "ARN", Width: 10},
		{Title: "Bucket", Width: 10},
		{Title: "s3 buckets", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 2 {
		t.Errorf("MarkerCol via td.Name title match: got %d, want 2", got)
	}
}

func TestResolveListMarkerCol_FallsBackToZero(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "ECR Repositories"}
	cols := []config.ListColumn{
		{Key: "arn", Title: "ARN", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
		{Key: "region", Title: "Region", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 0 {
		t.Errorf("MarkerCol fallback: got %d, want 0", got)
	}
}

func TestResolveListMarkerCol_IdentityKeyBeatsNameKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "DB Instances", IdentityKey: "db_id"}
	cols := []config.ListColumn{
		{Key: "name", Title: "Name", Width: 10},
		{Key: "db_id", Title: "DB ID", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 1 {
		t.Errorf("IdentityKey should beat name key: got %d, want 1", got)
	}
}

func TestResolveListMarkerCol_IdentityKeyNotFound_FallsToNameKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "EC2 Instances", IdentityKey: "nonexistent_key"}
	cols := []config.ListColumn{
		{Key: "name", Title: "Name", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
	}
	c := wave3MarkerColControllerWith(t, td, cols)
	if got := wave3MarkerColOf(t, c); got != 0 {
		t.Errorf("IdentityKey not found, should fall to name key at idx 0: got %d, want 0", got)
	}
}

// ===========================================================================
// renderListWidenLifecycleColumn — AS-140/AS-566 regression: a narrow status
// column must widen to fit the merged "<top> (+N)" phrase baked into
// body.Rows[i].Cells by buildListBody, not truncate it. Port of
// widen_lifecycle_column_internal_test.go's
// TestWidenLifecycleColumn_StackedFindingsSizedToMergedPhrase.
// ===========================================================================

func TestRenderListWidenLifecycleColumn_StackedFindingsNotTruncated(t *testing.T) {
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{
		"ec2": {List: []config.ListColumn{
			{Key: "@id", Title: "ID", Width: 20},
			// Narrow on purpose: "stopped" (7 chars) fits, "stopped (+1)"
			// (12 chars) does not — the AS-566 regression this pin guards.
			{Key: "status", Title: "Status", Width: 8},
		}},
	}}
	c := newTestController(t)
	c.SetViewConfig(cfg)
	c.RegisterFallbackTypeDef(td)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	res := resource.Resource{
		ID: "i-stacked", Name: "i-stacked",
		Findings: []domain.Finding{
			{Phrase: "stopped", Severity: domain.SevBroken},
			{Phrase: "maintenance scheduled", Severity: domain.SevWarn},
		},
	}
	c.ApplyResourcesLoaded("ec2", []resource.Resource{res}, nil, false)
	body := *c.Snapshot().Body.List

	if body.StatusCol < 0 {
		t.Fatal("expected a resolved status column, got -1")
	}
	if got := body.Rows[0].Cells[body.StatusCol]; got != "stopped (+1)" {
		t.Fatalf("baked status cell = %q, want %q", got, "stopped (+1)")
	}

	m := views.NewResourceList(td, nil, keys.Default())
	m.SetSize(160, 30)
	rendered := m.RenderList(body)

	if !strings.Contains(rendered, "stopped (+1)") {
		t.Errorf("renderListWidenLifecycleColumn must widen the Status column to fit the merged phrase; rendered output missing untruncated \"stopped (+1)\":\n%s", rendered)
	}
}
