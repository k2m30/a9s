// list_test.go — contract tests for the controller-side list machinery.
//
// Behaviors covered:
//
//  1. applyResourcesLoaded stores rows: N resources → N rows in Snapshot body;
//     cells extracted per type columns; ResourceID set; append mode appends;
//     replace mode replaces.
//  2. buildListBody filter: ListState.Filter → only matching rows; Selected
//     reset to 0.
//  3. buildListBody sort: SortCol/SortDir → rows in correct order; Body.Sort
//     reflects the spec; resets SelectedRow.
//  4. buildListBody attention: AttentionOnly=true → only rows with issue findings;
//     toggle restores; resets SelectedRow.
//  5. relatedIDSet prefilter: PatchListRelatedIDSet → only those IDs visible;
//     nil clears the filter.
//  6. List actions mutate ListState correctly: MoveDown/Up clamped,
//     MoveTop/Bottom jump, PageDown/Up page, ScrollLeft/Right clamped ≥0,
//     SetFilter resets Selected, Sort toggles col/dir, ToggleAttention flips flag.
//  7. ListSelected: returns resource at cursor; safe on empty / non-list screen.
//  8. Pagination: truncated PaginationMeta → Body.Truncated / Pagination.HasMore;
//     append accumulates; nil meta clears.
//  9. Enrichment: ApplyEnrichmentState → EnrichmentFindings populated; rows
//     get correct Decorator for SevBroken/SevWarn; attention filter picks up
//     enrichment-only rows.
// 10. ListFrameTitle: returns type name while loading; includes count after load;
//     returns "" on non-list screen.
// 11. Edge cases: empty list, single item, 1000-item list, actions on non-list
//     screen don't panic.
//
// All test data uses synthetic fake values — no real AWS account IDs, ARNs,
// or profile names.
package app_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// newListController builds a Controller and navigates to a ScreenResourceList
// for typeName via ActionCommand. After this call topListState() is non-nil.
func newListController(typeName string) *app.Controller {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: typeName})
	return c
}

// newBaseController builds a Controller on the menu root (no list screen).
func newBaseController() *app.Controller {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	return app.New(core)
}

// fakeEC2Resources returns three synthetic EC2 resources in name order
// cache-node < db-server < web-server, so sort tests have a deterministic answer.
func fakeEC2Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID:   "i-0aaa111111111111a",
			Name: "web-server",
			Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0aaa111111111111a",
				"name":        "web-server",
				"state":       "running",
				"type":        "t3.large",
				"private_ip":  "10.0.1.10",
			},
		},
		{
			ID:   "i-0bbb222222222222b",
			Name: "db-server",
			Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0bbb222222222222b",
				"name":        "db-server",
				"state":       "stopped",
				"type":        "m5.xlarge",
				"private_ip":  "10.0.1.20",
			},
		},
		{
			ID:   "i-0ccc333333333333c",
			Name: "cache-node",
			Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0ccc333333333333c",
				"name":        "cache-node",
				"state":       "running",
				"type":        "r5.large",
				"private_ip":  "10.0.1.30",
			},
		},
	}
}

// fakeS3Resources returns three synthetic S3 bucket resources.
func fakeS3Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID:   "acme-app-state",
			Name: "acme-app-state",
			Type: "s3",
			Fields: map[string]string{
				"name":          "acme-app-state",
				"bucket_name":   "acme-app-state",
				"creation_date": "2025-06-20 11:35",
			},
		},
		{
			ID:   "acme-cdn-logs",
			Name: "acme-cdn-logs",
			Type: "s3",
			Fields: map[string]string{
				"name":          "acme-cdn-logs",
				"bucket_name":   "acme-cdn-logs",
				"creation_date": "2025-05-12 19:24",
			},
		},
		{
			ID:   "acme-backups",
			Name: "acme-backups",
			Type: "s3",
			Fields: map[string]string{
				"name":          "acme-backups",
				"bucket_name":   "acme-backups",
				"creation_date": "2025-04-01 08:00",
			},
		},
	}
}

// listBodyOrFail returns Body.List from the Snapshot or fails the test.
func listBodyOrFail(t *testing.T, c *app.Controller) *app.ListBody {
	t.Helper()
	vs := c.Snapshot()
	if vs.Body.List == nil {
		t.Fatal("Body.List is nil — controller not on a list screen")
	}
	return vs.Body.List
}

// intStr converts a non-negative int to its decimal string (test helper for
// large-list ID generation — avoids importing strconv).
func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// pad4 zero-pads intStr(n) to 4 digits.
func pad4(n int) string {
	s := "0000" + intStr(n)
	return s[len(s)-4:]
}

// ─────────────────────────────────────────────────────────────────────────────
// 1. applyResourcesLoaded stores rows
// ─────────────────────────────────────────────────────────────────────────────

func TestApplyResourcesLoaded_EC2_StoresNRows(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != len(resources) {
		t.Fatalf("Rows count: got %d want %d", len(lb.Rows), len(resources))
	}
	for i, r := range resources {
		if lb.Rows[i].ResourceID != r.ID {
			t.Errorf("Rows[%d].ResourceID: got %q want %q", i, lb.Rows[i].ResourceID, r.ID)
		}
	}
}

func TestApplyResourcesLoaded_S3_StoresNRows(t *testing.T) {
	c := newListController("s3")
	resources := fakeS3Resources()
	c.ApplyResourcesLoaded("s3", resources, nil, false)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != len(resources) {
		t.Fatalf("Rows count: got %d want %d", len(lb.Rows), len(resources))
	}
	for i, r := range resources {
		if lb.Rows[i].ResourceID != r.ID {
			t.Errorf("Rows[%d].ResourceID: got %q want %q", i, lb.Rows[i].ResourceID, r.ID)
		}
	}
}

func TestApplyResourcesLoaded_ReplacesCache(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	single := fakeEC2Resources()[:1]
	c.ApplyResourcesLoaded("ec2", single, nil, false)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Errorf("after replace: Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != single[0].ID {
		t.Errorf("Rows[0].ResourceID: got %q want %q", lb.Rows[0].ResourceID, single[0].ID)
	}
}

func TestApplyResourcesLoaded_AppendMode(t *testing.T) {
	c := newListController("ec2")
	all := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", all[:2], nil, false)
	c.ApplyResourcesLoaded("ec2", all[2:], nil, true)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != len(all) {
		t.Fatalf("after append: Rows count: got %d want %d", len(lb.Rows), len(all))
	}
}

func TestApplyResourcesLoaded_CellCountMatchesColumns(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) == 0 {
		t.Fatal("no rows")
	}
	nCols := len(lb.Columns)
	if nCols == 0 {
		t.Fatal("no columns defined for ec2")
	}
	for i, row := range lb.Rows {
		if len(row.Cells) != nCols {
			t.Errorf("Rows[%d]: cell count %d != column count %d", i, len(row.Cells), nCols)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. buildListBody filter
// ─────────────────────────────────────────────────────────────────────────────

func TestListFilter_MatchingRowsOnly(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "web-server"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'web-server': Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0aaa111111111111a" {
		t.Errorf("filter 'web-server': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
	if lb.Filter != "web-server" {
		t.Errorf("Body.Filter: got %q want %q", lb.Filter, "web-server")
	}
}

func TestListFilter_NoMatchProducesZeroRows(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "xyzzy-no-match-at-all"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 0 {
		t.Errorf("filter no-match: got %d rows want 0", len(lb.Rows))
	}
}

func TestListFilter_EmptyFilterShowsAll(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "web-server"})
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: ""})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != len(resources) {
		t.Errorf("empty filter: Rows count: got %d want %d", len(lb.Rows), len(resources))
	}
}

func TestListFilter_ResetsSelectedRow(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if c.GetListSelectedRow() != 2 {
		t.Fatalf("precondition: SelectedRow should be 2, got %d", c.GetListSelectedRow())
	}
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "web-server"})

	lb := listBodyOrFail(t, c)
	if lb.Selected != 0 {
		t.Errorf("after SetFilter: Selected: got %d want 0", lb.Selected)
	}
}

func TestListFilter_S3_MatchesBucketName(t *testing.T) {
	c := newListController("s3")
	c.ApplyResourcesLoaded("s3", fakeS3Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "acme-cdn"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'acme-cdn': Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "acme-cdn-logs" {
		t.Errorf("filter 'acme-cdn': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 3. buildListBody sort
// ─────────────────────────────────────────────────────────────────────────────

func TestListSort_FirstSortSetsAsc(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})

	lb := listBodyOrFail(t, c)
	if lb.Sort.Col != "name" {
		t.Errorf("Sort.Col: got %q want %q", lb.Sort.Col, "name")
	}
	if lb.Sort.Dir != "asc" {
		t.Errorf("Sort.Dir: got %q want %q", lb.Sort.Dir, "asc")
	}
}

func TestListSort_SecondSortSameColTogglesToDesc(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})

	lb := listBodyOrFail(t, c)
	if lb.Sort.Dir != "desc" {
		t.Errorf("Sort.Dir after toggle: got %q want %q", lb.Sort.Dir, "desc")
	}
}

func TestListSort_ThirdSortTogglesBackToAsc(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})

	lb := listBodyOrFail(t, c)
	if lb.Sort.Dir != "asc" {
		t.Errorf("Sort.Dir after 3 sorts: got %q want %q", lb.Sort.Dir, "asc")
	}
}

func TestListSort_DifferentColResetsToAsc(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"}) // desc
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "type"}) // new col → asc

	lb := listBodyOrFail(t, c)
	if lb.Sort.Col != "type" {
		t.Errorf("Sort.Col: got %q want %q", lb.Sort.Col, "type")
	}
	if lb.Sort.Dir != "asc" {
		t.Errorf("Sort.Dir after col change: got %q want %q", lb.Sort.Dir, "asc")
	}
}

func TestListSort_RowsOrderedAscByName(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("sort asc: Rows count: got %d want 3", len(lb.Rows))
	}
	// cache-node < db-server < web-server
	wantOrder := []string{"i-0ccc333333333333c", "i-0bbb222222222222b", "i-0aaa111111111111a"}
	for i, want := range wantOrder {
		if lb.Rows[i].ResourceID != want {
			t.Errorf("sort asc Rows[%d].ResourceID: got %q want %q", i, lb.Rows[i].ResourceID, want)
		}
	}
}

func TestListSort_RowsOrderedDescByName(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"}) // desc

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("sort desc: Rows count: got %d want 3", len(lb.Rows))
	}
	// web-server > db-server > cache-node
	wantOrder := []string{"i-0aaa111111111111a", "i-0bbb222222222222b", "i-0ccc333333333333c"}
	for i, want := range wantOrder {
		if lb.Rows[i].ResourceID != want {
			t.Errorf("sort desc Rows[%d].ResourceID: got %q want %q", i, lb.Rows[i].ResourceID, want)
		}
	}
}

func TestListSort_ResetsSelectedRow(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if c.GetListSelectedRow() != 2 {
		t.Fatalf("precondition: SelectedRow should be 2")
	}
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})

	lb := listBodyOrFail(t, c)
	if lb.Selected != 0 {
		t.Errorf("after Sort: Selected: got %d want 0", lb.Selected)
	}
}

func TestListSort_EmptyArgIsNoop(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: ""}) // no-op

	col, dir := c.GetListSort()
	if col != "name" {
		t.Errorf("Sort empty arg: Col: got %q want %q", col, "name")
	}
	if dir != "asc" {
		t.Errorf("Sort empty arg: Dir: got %q want %q", dir, "asc")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4. buildListBody attention filter
// ─────────────────────────────────────────────────────────────────────────────

func TestListAttention_OnlyRowsWithIssueFindingsVisible(t *testing.T) {
	c := newListController("ec2")
	resources := []resource.Resource{
		{ID: "i-healthy", Fields: map[string]string{"state": "running", "instance_id": "i-healthy"}},
		{
			ID:     "i-broken",
			Fields: map[string]string{"state": "running", "instance_id": "i-broken"},
			Findings: []domain.Finding{
				{Code: "ec2.impaired", Phrase: "instance impaired", Severity: domain.SevBroken},
			},
		},
		{
			ID:     "i-warn",
			Fields: map[string]string{"state": "running", "instance_id": "i-warn"},
			Findings: []domain.Finding{
				{Code: "ec2.check", Phrase: "status degraded", Severity: domain.SevWarn},
			},
		},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := listBodyOrFail(t, c)
	if !lb.AttentionOnly {
		t.Fatal("AttentionOnly should be true")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("attention filter: Rows count: got %d want 2", len(lb.Rows))
	}
	for _, row := range lb.Rows {
		if row.ResourceID == "i-healthy" {
			t.Error("healthy resource should not appear under attention filter")
		}
	}
}

func TestListAttention_EmptyWhenNoIssues(t *testing.T) {
	c := newListController("ec2")
	// All running — no issue color, no findings.
	resources := []resource.Resource{
		{ID: "i-x1", Fields: map[string]string{"instance_id": "i-x1", "state": "running"}},
		{ID: "i-x2", Fields: map[string]string{"instance_id": "i-x2", "state": "running"}},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 0 {
		t.Errorf("attention no issues: got %d rows want 0", len(lb.Rows))
	}
}

func TestListAttention_ToggleOffRestoresAll(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := listBodyOrFail(t, c)
	if lb.AttentionOnly {
		t.Error("AttentionOnly should be false after double toggle")
	}
	if len(lb.Rows) != len(resources) {
		t.Errorf("after toggle off: Rows count: got %d want %d", len(lb.Rows), len(resources))
	}
}

func TestListAttention_ResetsSelectedRow(t *testing.T) {
	c := newListController("ec2")
	resources := []resource.Resource{
		{ID: "i-a", Fields: map[string]string{"instance_id": "i-a", "state": "running"}},
		{
			ID:     "i-b",
			Fields: map[string]string{"instance_id": "i-b", "state": "running"},
			Findings: []domain.Finding{
				{Code: "test.broken", Phrase: "broken", Severity: domain.SevBroken},
			},
		},
		{ID: "i-c", Fields: map[string]string{"instance_id": "i-c", "state": "running"}},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if c.GetListSelectedRow() != 2 {
		t.Fatalf("precondition: SelectedRow should be 2")
	}
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := listBodyOrFail(t, c)
	if lb.Selected != 0 {
		t.Errorf("after ToggleAttention: Selected: got %d want 0", lb.Selected)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 5. relatedIDSet prefilter
// ─────────────────────────────────────────────────────────────────────────────

func TestRelatedIDSet_FiltersToMatchingIDs(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.PatchListRelatedIDSet([]string{"i-0aaa111111111111a", "i-0bbb222222222222b"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 2 {
		t.Fatalf("relatedIDSet: Rows count: got %d want 2", len(lb.Rows))
	}
	for _, row := range lb.Rows {
		if row.ResourceID != "i-0aaa111111111111a" && row.ResourceID != "i-0bbb222222222222b" {
			t.Errorf("relatedIDSet: unexpected ResourceID %q in rows", row.ResourceID)
		}
	}
}

func TestRelatedIDSet_UnknownIDHidesAll(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.PatchListRelatedIDSet([]string{"i-does-not-exist"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 0 {
		t.Errorf("relatedIDSet unknown ID: got %d rows want 0", len(lb.Rows))
	}
}

func TestRelatedIDSet_NilClearsFilter(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.PatchListRelatedIDSet([]string{"i-0aaa111111111111a"})
	c.PatchListRelatedIDSet(nil)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != len(resources) {
		t.Errorf("after nil relatedIDSet: Rows count: got %d want %d", len(lb.Rows), len(resources))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 6. List actions
// ─────────────────────────────────────────────────────────────────────────────

func TestListAction_MoveDown_IncrementsCursor(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if got := c.GetListSelectedRow(); got != 1 {
		t.Errorf("after MoveDown: SelectedRow: got %d want 1", got)
	}
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if got := c.GetListSelectedRow(); got != 2 {
		t.Errorf("after 2 MoveDown: SelectedRow: got %d want 2", got)
	}
}

func TestListAction_MoveDown_ClampsAtLastRow(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	n := len(resources)
	for i := 0; i < n+5; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	if got := c.GetListSelectedRow(); got != n-1 {
		t.Errorf("MoveDown clamp: SelectedRow: got %d want %d", got, n-1)
	}
}

func TestListAction_MoveUp_DecrementsCursor(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveUp})
	if got := c.GetListSelectedRow(); got != 1 {
		t.Errorf("after MoveUp: SelectedRow: got %d want 1", got)
	}
}

func TestListAction_MoveUp_ClampsAtZero(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveUp})
	c.Apply(app.Action{Kind: app.ActionMoveUp})
	if got := c.GetListSelectedRow(); got != 0 {
		t.Errorf("MoveUp clamp: SelectedRow: got %d want 0", got)
	}
}

func TestListAction_MoveTop_JumpsToFirst(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveTop})
	if got := c.GetListSelectedRow(); got != 0 {
		t.Errorf("MoveTop: SelectedRow: got %d want 0", got)
	}
}

func TestListAction_MoveBottom_JumpsToLast(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveBottom})
	want := len(resources) - 1
	if got := c.GetListSelectedRow(); got != want {
		t.Errorf("MoveBottom: SelectedRow: got %d want %d", got, want)
	}
}

func TestListAction_MoveBottom_EmptyList_NoPanic(t *testing.T) {
	c := newListController("ec2")
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MoveBottom on empty list panicked: %v", r)
			}
		}()
		c.Apply(app.Action{Kind: app.ActionMoveBottom})
	}()
	if got := c.GetListSelectedRow(); got != 0 {
		t.Errorf("MoveBottom empty: SelectedRow: got %d want 0", got)
	}
}

func TestListAction_PageDown_MovesRows(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionPageDown, N: 2})
	if got := c.GetListSelectedRow(); got != 2 {
		t.Errorf("PageDown N=2: SelectedRow: got %d want 2", got)
	}
}

func TestListAction_PageDown_ClampsAtLastRow(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionPageDown, N: 100})
	if got := c.GetListSelectedRow(); got != len(resources)-1 {
		t.Errorf("PageDown clamp: SelectedRow: got %d want %d", got, len(resources)-1)
	}
}

func TestListAction_PageUp_DecreasesRows(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionMoveBottom})
	c.Apply(app.Action{Kind: app.ActionPageUp, N: 1})
	if got := c.GetListSelectedRow(); got != 1 {
		t.Errorf("PageUp N=1: SelectedRow: got %d want 1", got)
	}
}

func TestListAction_PageUp_ClampsAtZero(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionPageUp, N: 100})
	if got := c.GetListSelectedRow(); got != 0 {
		t.Errorf("PageUp clamp: SelectedRow: got %d want 0", got)
	}
}

func TestListAction_ScrollRight_IncreasesScrollX(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionScrollRight})
	if got := c.GetListScrollX(); got != 1 {
		t.Errorf("ScrollRight: ScrollX: got %d want 1", got)
	}
	c.Apply(app.Action{Kind: app.ActionScrollRight})
	if got := c.GetListScrollX(); got != 2 {
		t.Errorf("ScrollRight x2: ScrollX: got %d want 2", got)
	}
}

func TestListAction_ScrollLeft_DecreasesScrollX(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionScrollRight})
	c.Apply(app.Action{Kind: app.ActionScrollRight})
	c.Apply(app.Action{Kind: app.ActionScrollLeft})
	if got := c.GetListScrollX(); got != 1 {
		t.Errorf("ScrollLeft: ScrollX: got %d want 1", got)
	}
}

func TestListAction_ScrollLeft_ClampsAtZero(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionScrollLeft})
	c.Apply(app.Action{Kind: app.ActionScrollLeft})
	if got := c.GetListScrollX(); got != 0 {
		t.Errorf("ScrollLeft clamp: ScrollX: got %d want 0", got)
	}
}

func TestListAction_SetFilter_SetsFilter(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "cache"})
	if got := c.GetListFilter(); got != "cache" {
		t.Errorf("SetFilter: Filter: got %q want %q", got, "cache")
	}
	if got := c.GetListSelectedRow(); got != 0 {
		t.Errorf("SetFilter: SelectedRow: got %d want 0", got)
	}
}

func TestListAction_ToggleAttention_FlipsFlag(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	if c.GetListAttentionOnly() {
		t.Fatal("precondition: AttentionOnly should start false")
	}
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	if !c.GetListAttentionOnly() {
		t.Error("after first ToggleAttention: AttentionOnly should be true")
	}
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	if c.GetListAttentionOnly() {
		t.Error("after second ToggleAttention: AttentionOnly should be false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 7. ListSelected
// ─────────────────────────────────────────────────────────────────────────────

func TestListSelected_ReturnsResourceAtCursor(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	r0, ok0 := c.ListSelected()
	if !ok0 {
		t.Fatal("ListSelected at row 0: ok should be true")
	}
	if r0.ID != resources[0].ID {
		t.Errorf("ListSelected row 0: ID: got %q want %q", r0.ID, resources[0].ID)
	}

	c.Apply(app.Action{Kind: app.ActionMoveDown})
	r1, ok1 := c.ListSelected()
	if !ok1 {
		t.Fatal("ListSelected at row 1: ok should be true")
	}
	if r1.ID != resources[1].ID {
		t.Errorf("ListSelected row 1: ID: got %q want %q", r1.ID, resources[1].ID)
	}
}

func TestListSelected_EmptyList_ReturnsFalse(t *testing.T) {
	c := newListController("ec2")
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ListSelected on empty list panicked: %v", r)
			}
		}()
		_, ok := c.ListSelected()
		if ok {
			t.Error("ListSelected on empty list: ok should be false")
		}
	}()
}

func TestListSelected_AfterFilter_ReturnsFilteredResource(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "db-server"})

	r, ok := c.ListSelected()
	if !ok {
		t.Fatal("ListSelected after filter: ok should be true")
	}
	if r.ID != "i-0bbb222222222222b" {
		t.Errorf("ListSelected after filter: ID: got %q want %q", r.ID, "i-0bbb222222222222b")
	}
}

func TestListSelected_NoListScreen_ReturnsFalse(t *testing.T) {
	c := newBaseController()
	r, ok := c.ListSelected()
	if ok {
		t.Errorf("ListSelected on menu screen: ok should be false, got %q", r.ID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 8. Pagination
// ─────────────────────────────────────────────────────────────────────────────

func TestPagination_TruncatedFlagPropagates(t *testing.T) {
	c := newListController("ec2")
	pagination := &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-next-page",
	}
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), pagination, false)

	lb := listBodyOrFail(t, c)
	if !lb.Truncated {
		t.Error("Body.List.Truncated should be true")
	}
	if !lb.Pagination.HasMore {
		t.Error("Body.List.Pagination.HasMore should be true")
	}
	if lb.Pagination.Cursor != "tok-next-page" {
		t.Errorf("Pagination.Cursor: got %q want %q", lb.Pagination.Cursor, "tok-next-page")
	}
}

func TestPagination_NotTruncated(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), &resource.PaginationMeta{IsTruncated: false}, false)

	lb := listBodyOrFail(t, c)
	if lb.Truncated {
		t.Error("Body.List.Truncated should be false when IsTruncated=false")
	}
	if lb.Pagination.HasMore {
		t.Error("Body.List.Pagination.HasMore should be false")
	}
}

func TestPagination_NilMetaClearsPagination(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok"}, false)
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)

	lb := listBodyOrFail(t, c)
	if lb.Truncated {
		t.Error("after nil pagination: Body.List.Truncated should be false")
	}
	if lb.Pagination.HasMore {
		t.Error("after nil pagination: Pagination.HasMore should be false")
	}
}

func TestPagination_AppendAccumulatesCount(t *testing.T) {
	c := newListController("ec2")
	all := fakeEC2Resources()
	c.ApplyResourcesLoaded("ec2", all[:2], &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-2"}, false)
	c.ApplyResourcesLoaded("ec2", all[2:], &resource.PaginationMeta{IsTruncated: false}, true)

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != len(all) {
		t.Errorf("after append: Rows count: got %d want %d", len(lb.Rows), len(all))
	}
	if lb.Truncated {
		t.Error("after final page: Truncated should be false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 9. Enrichment
// ─────────────────────────────────────────────────────────────────────────────

func TestEnrichment_FindingsInBody(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.ApplyEnrichmentState("ec2", 1, false, map[string]domain.Finding{
		"i-0bbb222222222222b": {Code: "ec2.stopped", Phrase: "instance stopped", Severity: domain.SevWarn},
	})

	lb := listBodyOrFail(t, c)
	if lb.EnrichmentFindings == nil {
		t.Fatal("EnrichmentFindings should not be nil")
	}
	f, ok := lb.EnrichmentFindings["i-0bbb222222222222b"]
	if !ok {
		t.Fatal("EnrichmentFindings missing key i-0bbb222222222222b")
	}
	if f.Phrase != "instance stopped" {
		t.Errorf("finding Phrase: got %q want %q", f.Phrase, "instance stopped")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("finding Severity: got %d want %d", f.Severity, domain.SevWarn)
	}
}

func TestEnrichment_BrokenRowHasDecoratorError(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaa111111111111a", "state": "running"}},
	}, nil, false)
	c.ApplyEnrichmentState("ec2", 1, false, map[string]domain.Finding{
		"i-0aaa111111111111a": {Code: "ec2.impaired", Phrase: "system check failed", Severity: domain.SevBroken},
	})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(lb.Rows))
	}
	if lb.Rows[0].Decorator != app.DecoratorError {
		t.Errorf("Decorator: got %q want %q", lb.Rows[0].Decorator, app.DecoratorError)
	}
}

func TestEnrichment_WarnRowHasDecoratorWarning(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-0bbb222222222222b", Name: "db-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0bbb222222222222b", "state": "running"}},
	}, nil, false)
	c.ApplyEnrichmentState("ec2", 1, false, map[string]domain.Finding{
		"i-0bbb222222222222b": {Code: "ec2.degraded", Phrase: "instance degraded", Severity: domain.SevWarn},
	})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(lb.Rows))
	}
	if lb.Rows[0].Decorator != app.DecoratorWarning {
		t.Errorf("Decorator: got %q want %q", lb.Rows[0].Decorator, app.DecoratorWarning)
	}
}

func TestEnrichment_AttentionFilterIncludesEnrichmentRows(t *testing.T) {
	c := newListController("ec2")
	resources := []resource.Resource{
		{ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaa111111111111a", "state": "running"}},
		{ID: "i-0bbb222222222222b", Name: "db-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0bbb222222222222b", "state": "running"}},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)
	c.ApplyEnrichmentState("ec2", 1, false, map[string]domain.Finding{
		"i-0bbb222222222222b": {Code: "ec2.degraded", Phrase: "degraded", Severity: domain.SevWarn},
	})
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Fatalf("attention+enrichment: Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0bbb222222222222b" {
		t.Errorf("attention+enrichment: got %q want i-0bbb222222222222b", lb.Rows[0].ResourceID)
	}
}

func TestEnrichment_TypeIsolation(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	c.ApplyEnrichmentState("ec2", 1, false, map[string]domain.Finding{
		"i-0aaa111111111111a": {Code: "test.x", Phrase: "broken", Severity: domain.SevBroken},
	})
	// Applying s3 enrichment must not erase ec2 findings.
	c.ApplyEnrichmentState("s3", 0, false, map[string]domain.Finding{})

	lb := listBodyOrFail(t, c)
	if lb.EnrichmentFindings == nil {
		t.Fatal("EnrichmentFindings nil after applying ec2 enrichment")
	}
	if _, ok := lb.EnrichmentFindings["i-0aaa111111111111a"]; !ok {
		t.Error("EC2 finding for i-0aaa111111111111a should still be present")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 10. ListFrameTitle
// ─────────────────────────────────────────────────────────────────────────────

func TestListFrameTitle_LoadingState_NonEmpty(t *testing.T) {
	c := newListController("ec2")
	// No resources loaded — Loading=true.
	title := c.ListFrameTitle()
	if title == "" {
		t.Error("ListFrameTitle should not be empty while loading")
	}
}

func TestListFrameTitle_ShowsCountAfterLoad(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), nil, false)
	title := c.ListFrameTitle()
	if title == "" {
		t.Error("ListFrameTitle should not be empty after loading")
	}
	// The count "3" must appear somewhere in the title.
	found := false
	needle := "3"
	for i := 0; i <= len(title)-len(needle); i++ {
		if title[i:i+len(needle)] == needle {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListFrameTitle after loading 3 resources: got %q, want '3' in title", title)
	}
}

func TestListFrameTitle_NoListScreen_ReturnsEmpty(t *testing.T) {
	c := newBaseController()
	if title := c.ListFrameTitle(); title != "" {
		t.Errorf("ListFrameTitle on menu screen: got %q want empty", title)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 11. Edge cases
// ─────────────────────────────────────────────────────────────────────────────

func TestListBody_EmptyCache_NoPanic(t *testing.T) {
	c := newListController("ec2")
	var lb *app.ListBody
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("buildListBody on empty cache panicked: %v", r)
			}
		}()
		lb = listBodyOrFail(t, c)
	}()
	if lb != nil && len(lb.Rows) != 0 {
		t.Errorf("empty cache: Rows count: got %d want 0", len(lb.Rows))
	}
}

func TestListBody_SingleResource(t *testing.T) {
	c := newListController("ec2")
	c.ApplyResourcesLoaded("ec2", fakeEC2Resources()[:1], nil, false)
	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1 {
		t.Errorf("single resource: Rows count: got %d want 1", len(lb.Rows))
	}
}

func TestListBody_LargeList_NoPanic(t *testing.T) {
	c := newListController("ec2")
	large := make([]resource.Resource, 1000)
	for i := range large {
		large[i] = resource.Resource{
			ID:   "i-fake" + pad4(i),
			Name: "server-" + pad4(i),
			Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-fake" + pad4(i),
				"name":        "server-" + pad4(i),
				"state":       "running",
			},
		}
	}
	c.ApplyResourcesLoaded("ec2", large, nil, false)
	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 1000 {
		t.Errorf("large list: Rows count: got %d want 1000", len(lb.Rows))
	}
}

func TestListActions_NoPanicOnMenuScreen(t *testing.T) {
	actions := []app.Action{
		{Kind: app.ActionMoveDown},
		{Kind: app.ActionMoveUp},
		{Kind: app.ActionMoveTop},
		{Kind: app.ActionMoveBottom},
		{Kind: app.ActionPageDown, N: 5},
		{Kind: app.ActionPageUp, N: 5},
		{Kind: app.ActionScrollLeft},
		{Kind: app.ActionScrollRight},
		{Kind: app.ActionSetFilter, Arg: "test"},
		{Kind: app.ActionSort, Arg: "name"},
		{Kind: app.ActionToggleAttention},
	}
	for _, a := range actions {
		a := a
		t.Run(string(a.Kind), func(t *testing.T) {
			c := newBaseController()
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Apply(%q) on menu screen panicked: %v", a.Kind, r)
					}
				}()
				_, _ = c.Apply(a) //nolint:ineffassign,staticcheck // crash-verification only
			}()
		})
	}
}

func TestGetters_NoListScreen_ReturnZeroValues(t *testing.T) {
	c := newBaseController()
	if col, dir := c.GetListSort(); col != "" || dir != "" {
		t.Errorf("GetListSort on menu: got col=%q dir=%q want both empty", col, dir)
	}
	if x := c.GetListScrollX(); x != 0 {
		t.Errorf("GetListScrollX on menu: got %d want 0", x)
	}
	if f := c.GetListFilter(); f != "" {
		t.Errorf("GetListFilter on menu: got %q want empty", f)
	}
	if a := c.GetListAttentionOnly(); a {
		t.Error("GetListAttentionOnly on menu: should be false")
	}
	if row := c.GetListSelectedRow(); row != 0 {
		t.Errorf("GetListSelectedRow on menu: got %d want 0", row)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Regression tests for the three list-path bugs
// ─────────────────────────────────────────────────────────────────────────────

// TestBug1_PerScreenRowStorage_SameTypeStackedScreensAreIndependent tests that
// two stacked list screens of the same resource type each hold their own row
// set, so navigating back to the first screen shows the original rows (not the
// second screen's filtered/related rows).
//
// Scenario:
//   - Push list screen 1 for "ec2" → load rows A (3 instances).
//   - Push list screen 2 for "ec2" (simulates a filtered/related pivot) → load
//     rows B (1 instance).
//   - Assert: top screen (screen 2) shows rows B only.
//   - Pop back to screen 1.
//   - Assert: top screen (screen 1) shows rows A (not B), with correct count.
func TestBug1_PerScreenRowStorage_SameTypeStackedScreensAreIndependent(t *testing.T) {
	c := newListController("ec2")

	// Screen 1: load full row set A.
	rowsA := fakeEC2Resources() // 3 items
	c.ApplyResourcesLoaded("ec2", rowsA, nil, false)

	lb1 := listBodyOrFail(t, c)
	if len(lb1.Rows) != 3 {
		t.Fatalf("screen 1 after load: want 3 rows, got %d", len(lb1.Rows))
	}

	// Push screen 2 for same type "ec2" (simulates a related/filtered child list).
	c.PushChildListScreen("ec2")

	// Screen 2: load a smaller row set B (1 item).
	rowsB := fakeEC2Resources()[:1] // 1 item
	c.ApplyResourcesLoaded("ec2", rowsB, nil, false)

	lb2 := listBodyOrFail(t, c)
	if len(lb2.Rows) != 1 {
		t.Fatalf("screen 2 after load: want 1 row, got %d", len(lb2.Rows))
	}
	if lb2.Rows[0].ResourceID != rowsB[0].ID {
		t.Errorf("screen 2 row ID: got %q want %q", lb2.Rows[0].ResourceID, rowsB[0].ID)
	}

	// Pop back to screen 1.
	c.Apply(app.Action{Kind: app.ActionBack})

	// Screen 1 must still show rows A — not rows B.
	lb1After := listBodyOrFail(t, c)
	if len(lb1After.Rows) != 3 {
		t.Fatalf("screen 1 after pop: want 3 rows (original A), got %d — screen 2's rows corrupted screen 1", len(lb1After.Rows))
	}
	for i, want := range rowsA {
		if lb1After.Rows[i].ResourceID != want.ID {
			t.Errorf("screen 1 after pop row[%d]: got %q want %q", i, lb1After.Rows[i].ResourceID, want.ID)
		}
	}

	// Pagination on screen 1 must also be clean (no bleed from screen 2).
	if lb1After.Truncated {
		t.Error("screen 1 after pop: Truncated should be false (not bled from screen 2)")
	}
}

// TestBug2_ListSelected_ClampsWhenVisibleShrinks tests that ListSelected
// returns the clamped visible row (not false/zero-value) when a refresh shrinks
// the list below the stored SelectedRow index.  Before the fix, the cursor
// pointed past the end and ListSelected returned (Resource{}, false) even
// though buildListBody rendered a highlighted last row.
func TestBug2_ListSelected_ClampsWhenVisibleShrinks(t *testing.T) {
	c := newListController("ec2")
	resources := fakeEC2Resources() // 3 rows
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	// Move cursor to row 2 (last row in the 3-item list).
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if got := c.GetListSelectedRow(); got != 2 {
		t.Fatalf("precondition: SelectedRow should be 2, got %d", got)
	}

	// Replace the cache with only 1 item — cursor is now out-of-range.
	c.ApplyResourcesLoaded("ec2", resources[:1], nil, false)

	// ListSelected must return the clamped last visible row, not false.
	r, ok := c.ListSelected()
	if !ok {
		t.Fatal("ListSelected after shrink: ok should be true (clamped to last row), got false — cursor stuck past end")
	}
	if r.ID != resources[0].ID {
		t.Errorf("ListSelected after shrink: got ID %q want %q (clamped to row 0)", r.ID, resources[0].ID)
	}

	// buildListBody must also reflect the clamp.
	lb := listBodyOrFail(t, c)
	if lb.Selected != 0 {
		t.Errorf("buildListBody after shrink: Selected=%d want 0", lb.Selected)
	}
}

// TestBug3_SortUsesViewConfig_CustomSortKeyApplied tests that listSortResources
// resolves column definitions from the controller's viewConfig (not just the
// built-in defaults), so a user-configured sort_key column sorts by the correct
// field — mirroring the exact same priority as buildListBody and resourcelist.go.
func TestBug3_SortUsesViewConfig_CustomSortKeyApplied(t *testing.T) {
	// Build a ViewsConfig that overrides the ec2 column set with a "Score"
	// column whose sort_key maps to a numeric "score" field.  The built-in
	// ec2 defaults do not have this column, so without the Bug 3 fix the sort
	// falls back to lexicographic string comparison on the wrong key.
	vc := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				List: []config.ListColumn{
					{Title: "Name", Key: "name", Width: 20},
					{Title: "Score", Key: "score", Width: 10, SortKey: "score"},
				},
			},
		},
	}

	c := newListController("ec2")
	c.SetViewConfig(vc)

	// Resources with numeric scores stored in Fields["score"].  Lexicographic
	// order would give "10" < "2" < "9", but numeric order is 2 < 9 < 10.
	resources := []resource.Resource{
		{ID: "i-score-10", Name: "high", Type: "ec2", Fields: map[string]string{"name": "high", "score": "10"}},
		{ID: "i-score-02", Name: "low", Type: "ec2", Fields: map[string]string{"name": "low", "score": "2"}},
		{ID: "i-score-09", Name: "mid", Type: "ec2", Fields: map[string]string{"name": "mid", "score": "9"}},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	// Sort ascending by the custom "score" column.
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "score"})

	lb := listBodyOrFail(t, c)
	if len(lb.Rows) != 3 {
		t.Fatalf("sort by viewConfig score: want 3 rows, got %d", len(lb.Rows))
	}

	// Numeric ascending: 2, 9, 10.
	wantOrder := []string{"i-score-02", "i-score-09", "i-score-10"}
	for i, want := range wantOrder {
		if lb.Rows[i].ResourceID != want {
			t.Errorf("sort by viewConfig score asc: Rows[%d].ResourceID got %q want %q", i, lb.Rows[i].ResourceID, want)
		}
	}

	// Now descending: 10, 9, 2.
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "score"})
	lb = listBodyOrFail(t, c)
	wantDesc := []string{"i-score-10", "i-score-09", "i-score-02"}
	for i, want := range wantDesc {
		if lb.Rows[i].ResourceID != want {
			t.Errorf("sort by viewConfig score desc: Rows[%d].ResourceID got %q want %q", i, lb.Rows[i].ResourceID, want)
		}
	}
}
