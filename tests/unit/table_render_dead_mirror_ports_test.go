// table_render_dead_mirror_ports_test.go — pins for the two live production
// seams behind identity-column resolution and lifecycle-column widening,
// reachable from tests/unit/ as an external (package unit_test) black-box:
//
//  1. IdentityColumnIndex (core/app/list_columns.go) — the 5-step identity
//     cascade. Driven via a fully-controlled per-type config.ViewsConfig
//     (GetViewDef replaces userDef.List wholesale, giving byte-for-byte
//     control over each column's Key/Path/Title) + RegisterFallbackTypeDef
//     (controls IdentityKey/Name), read back via
//     Snapshot().Body.List.IdentityCol — the exported field buildListBody
//     bakes IdentityColumnIndex's return into.
//  2. renderListWidenLifecycleColumn (internal/tui/views/resourcelist.go) —
//     widens from body.Rows[i].Cells verbatim, post-bake, not by re-deriving
//     from r.Findings/r.Fields. The one pin here is the stacked-findings
//     width: the column is measured against the merged "<top> (+N)" phrase,
//     not r.Findings[0].Phrase alone. Single-finding, no-findings-fallback
//     and ignores-Fields["status"] are default list-render behavior covered
//     by every other RenderList-driven test in this package.
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

// wave3IdentityColControllerWith builds a Controller navigated to the real "ec2"
// catalog command (so ActionCommand routing succeeds) but with columns and
// identity resolution fully overridden: cfg's Views["ec2"].List replaces the
// built-in column set wholesale (config.GetViewDef: "user-provided fields
// override defaults" when len(userDef.List) > 0), and RegisterFallbackTypeDef
// makes buildListBody prefer td (IdentityKey/Name) over the ec2 catalog entry
// ("the model's explicitly-supplied typeDef rather than the catalog's when
// they differ" — RegisterFallbackTypeDef's own doc). Both must be set before
// the first Snapshot()-triggering call, matching SetViewConfig's contract.
func wave3IdentityColControllerWith(t *testing.T, td resource.ResourceTypeDef, cols []config.ListColumn) *app.Controller {
	t.Helper()
	td.ShortName = "ec2"
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{"ec2": {List: cols}}}
	c := newTestController(t)
	c.SetViewConfig(cfg)
	c.RegisterFallbackTypeDef(td)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	return c
}

// wave3IdentityColOf applies a single resource and returns the resolved
// full-column-space IdentityCol index — the live equivalent of calling
// resolveIdentityColumn(cols, td) directly.
func wave3IdentityColOf(t *testing.T, c *app.Controller) int {
	t.Helper()
	c.ApplyResourcesLoaded("ec2", []resource.Resource{{ID: "r1", Name: "r1"}}, nil, false)
	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("nil list body after ApplyResourcesLoaded")
	}
	return lb.IdentityCol
}

// ===========================================================================
// IdentityColumnIndex cascade — port of resolve_identity_internal_test.go's
// TestResolveIdentityColumn_* cases (cascade order: 1. td.IdentityKey matches
// a column's key; 2. column key == "name"; 3. column title equals "Name"
// (case-insensitive) or td.Name; 4. fall back to index 0). The dead test's
// EmptyColumns case is dropped —
// config.GetViewDef only replaces defaults when len(userDef.List) > 0, so an
// intentionally-empty column list cannot be driven through this live seam;
// the guard itself is a trivial zero-iteration loop with no branch to lose.
// ===========================================================================

func TestResolveListIdentityCol_MatchesIdentityKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "EC2 Instances", IdentityKey: "foo"}
	cols := []config.ListColumn{
		{Key: "status", Title: "Status", Width: 10},
		{Key: "region", Title: "Region", Width: 10},
		{Key: "foo", Title: "Foo", Width: 10},
		{Key: "name", Title: "Name", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 2 {
		t.Errorf("IdentityCol with IdentityKey=%q: got %d, want 2", td.IdentityKey, got)
	}
}

func TestResolveListIdentityCol_FallsThroughToNameKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "RDS Instances"}
	cols := []config.ListColumn{
		{Key: "id", Title: "ID", Width: 10},
		{Key: "name", Title: "Name", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 1 {
		t.Errorf("IdentityCol via name key: got %d, want 1", got)
	}
}

// There is no path-substring election step: "DBInstanceIdentifier" points
// at a name-shaped field; it does not make its column the one that names
// the row, and electing it would put the attention marker on a foreign
// column. The cascade reaches the index-0 default.
func TestResolveListIdentityCol_PathIdentifierSubstringDoesNotElect(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "RDS Instances"}
	cols := []config.ListColumn{
		{Key: "id", Title: "ID", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
		{Path: "DBInstanceIdentifier", Title: "DB Instance", Width: 10},
		{Path: "Engine", Title: "Engine", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 0 {
		t.Errorf("IdentityCol with path=DBInstanceIdentifier: got %d, want 0", got)
	}
}

// As above: "FunctionName" in a path is a substring, not a declaration.
func TestResolveListIdentityCol_PathNameSubstringDoesNotElect(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "Lambda Functions"}
	cols := []config.ListColumn{
		{Title: "Arn", Width: 10},
		{Path: "FunctionName", Title: "Function", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 0 {
		t.Errorf("IdentityCol with path containing Name: got %d, want 0", got)
	}
}

func TestResolveListIdentityCol_FallsThroughToTitle(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "ACM Certificates"}
	cols := []config.ListColumn{
		{Title: "ARN", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "Region", Width: 10},
		{Title: "Name", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 3 {
		t.Errorf("IdentityCol via title=Name: got %d, want 3", got)
	}
}

func TestResolveListIdentityCol_CaseInsensitiveTitleMatch(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "ACM Certificates"}
	cols := []config.ListColumn{
		{Title: "ARN", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "NAME", Width: 10},
		{Title: "Region", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 2 {
		t.Errorf("IdentityCol case-insensitive title match: got %d, want 2", got)
	}
}

func TestResolveListIdentityCol_TitleMatchesTypeName(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "S3 Buckets"}
	cols := []config.ListColumn{
		{Title: "ARN", Width: 10},
		{Title: "Bucket", Width: 10},
		{Title: "s3 buckets", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 2 {
		t.Errorf("IdentityCol via td.Name title match: got %d, want 2", got)
	}
}

func TestResolveListIdentityCol_FallsBackToZero(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "ECR Repositories"}
	cols := []config.ListColumn{
		{Key: "arn", Title: "ARN", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
		{Key: "region", Title: "Region", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 0 {
		t.Errorf("IdentityCol fallback: got %d, want 0", got)
	}
}

func TestResolveListIdentityCol_IdentityKeyBeatsNameKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "DB Instances", IdentityKey: "db_id"}
	cols := []config.ListColumn{
		{Key: "name", Title: "Name", Width: 10},
		{Key: "db_id", Title: "DB ID", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 1 {
		t.Errorf("IdentityKey should beat name key: got %d, want 1", got)
	}
}

func TestResolveListIdentityCol_IdentityKeyNotFound_FallsToNameKey(t *testing.T) {
	td := resource.ResourceTypeDef{Name: "EC2 Instances", IdentityKey: "nonexistent_key"}
	cols := []config.ListColumn{
		{Key: "name", Title: "Name", Width: 10},
		{Key: "status", Title: "Status", Width: 10},
	}
	c := wave3IdentityColControllerWith(t, td, cols)
	if got := wave3IdentityColOf(t, c); got != 0 {
		t.Errorf("IdentityKey not found, should fall to name key at idx 0: got %d, want 0", got)
	}
}

// ===========================================================================
// renderListWidenLifecycleColumn — a narrow status column must widen to fit
// the merged "<top> (+N)" phrase baked into body.Rows[i].Cells by
// buildListBody, not truncate it.
// ===========================================================================

func TestRenderListWidenLifecycleColumn_StackedFindingsNotTruncated(t *testing.T) {
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{
		"ec2": {List: []config.ListColumn{
			{Key: "@id", Title: "ID", Width: 20},
			// Narrow on purpose: "stopped" (7 chars) fits, "stopped (+1)"
			// (12 chars) does not. ec2's status column names ec2's lifecycle key.
			{Key: "state", Title: "Status", Width: 8},
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
