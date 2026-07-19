// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// list_body_memo_seq_test.go — mutate-after-render sequence pins for
// core/app's list body build (buildListBody, applyListFilters,
// listSortResources, extractListCells: core/app/list_body.go:325-450,
// core/app/list_filter.go). An upcoming refactor memoizes the filtered+
// sorted rows and extracted cells, keyed on (RowStore generation, filter
// text, attention-only flag, sort column, sort direction), invalidating on
// any of those changing.
//
// Every test below renders through the real Controller.Snapshot() surface at
// least twice, with exactly one input mutated between renders, and asserts
// the SECOND render reflects the change. A memo key that omits or
// mis-tracks that one input shows up as a stale second render here — the
// class of bug a mutate-then-render-once test cannot catch. list_test.go's
// TestListFilter_*/TestListSort_*/TestListAttention_* and this file's own
// TestWave3AttentionFilter_* siblings all either render only once after
// every mutation has already landed, or (the two TestWave3AttentionFilter_*
// cases) render twice but only ever vary the attention-filter row-inclusion
// dimension — never sort, never a second distinct filter value, never a
// row's own Decorator/Severity on an already-included row.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// ===========================================================================
// (a) filter text set / changed / cleared
// ===========================================================================

// TestListBodyMemoSeq_FilterSetChangedCleared_RowsAndCountUpdate renders
// through three DIFFERENT filter states in sequence (none → "web-server" →
// "db-server" → "") and asserts both Rows and the frame-title count line
// after each transition. The middle-to-middle step (one non-empty filter to
// a DIFFERENT non-empty filter) is the case no existing test exercises: a
// memo key that tracks only "filter active" as a bool rather than the
// filter's own text would incorrectly reuse the first filter's cached rows.
func TestListBodyMemoSeq_FilterSetChangedCleared_RowsAndCountUpdate(t *testing.T) {
	c := wave3ListController(t, "ec2")
	resources := wave3FilterEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	// cache-node carries a Wave-1 SevWarn Finding, so the frame title always
	// shows the unconditional " !1" issue-count suffix (docs/attention-
	// signals.md §Visualization Surfaces) whenever cache-node is among the
	// visible rows.
	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 3 {
		t.Fatalf("render1 (no filter): Rows count: got %d want 3", len(render1.Rows))
	}
	if title := c.ListFrameTitle(); title != "ec2(3) !1" {
		t.Errorf("render1 title: got %q want %q", title, "ec2(3) !1")
	}

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "web-server"})
	render2 := *c.Snapshot().Body.List
	if len(render2.Rows) != 1 || render2.Rows[0].ResourceID != "i-0aaa111111111111a" {
		t.Fatalf("render2 (filter=web-server): got rows %+v", render2.Rows)
	}
	if title := c.ListFrameTitle(); title != "ec2(1/3) !1" {
		t.Errorf("render2 title: got %q want %q", title, "ec2(1/3) !1")
	}

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "db-server"})
	render3 := *c.Snapshot().Body.List
	if len(render3.Rows) != 1 || render3.Rows[0].ResourceID != "i-0bbb222222222222b" {
		t.Fatalf("render3 (filter changed to db-server): got rows %+v — stale render2 rows would still show web-server", render3.Rows)
	}
	if title := c.ListFrameTitle(); title != "ec2(1/3) !1" {
		t.Errorf("render3 title: got %q want %q", title, "ec2(1/3) !1")
	}

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: ""})
	render4 := *c.Snapshot().Body.List
	if len(render4.Rows) != 3 {
		t.Fatalf("render4 (filter cleared): Rows count: got %d want 3", len(render4.Rows))
	}
	if title := c.ListFrameTitle(); title != "ec2(3) !1" {
		t.Errorf("render4 title: got %q want %q", title, "ec2(3) !1")
	}
}

// ===========================================================================
// (b) sort column / direction toggled
// ===========================================================================

// TestListBodyMemoSeq_SortToggle_RowOrderUpdatesAcrossRenders renders after
// every sort mutation (asc → desc on the same column → asc on a DIFFERENT
// column), unlike TestListSort_RowsOrderedDescByName and
// TestListSort_DifferentColResetsToAsc (core/app/list_test.go), which apply
// every Action first and render exactly once at the end. A memo keyed on
// Col but not Dir, or vice-versa, would return the previous render's row
// order here.
func TestListBodyMemoSeq_SortToggle_RowOrderUpdatesAcrossRenders(t *testing.T) {
	c := wave3ListController(t, "ec2")
	resources := wave3FilterEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 3 {
		t.Fatalf("render1 (unsorted): Rows count: got %d want 3", len(render1.Rows))
	}

	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	render2 := *c.Snapshot().Body.List
	wantAsc := []string{"i-0ccc333333333333c", "i-0bbb222222222222b", "i-0aaa111111111111a"} // cache-node, db-server, web-server
	if got := rowIDs(render2.Rows); !idsEqual(got, wantAsc) {
		t.Fatalf("render2 (sort name asc): got %v want %v", got, wantAsc)
	}

	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})
	render3 := *c.Snapshot().Body.List
	wantDesc := []string{"i-0aaa111111111111a", "i-0bbb222222222222b", "i-0ccc333333333333c"} // web-server, db-server, cache-node
	if got := rowIDs(render3.Rows); !idsEqual(got, wantDesc) {
		t.Fatalf("render3 (sort name desc, toggled from asc): got %v want %v — a Dir-blind memo key would still return render2's asc order", got, wantDesc)
	}
	if render3.Sort.Dir != "desc" {
		t.Errorf("render3 Sort.Dir: got %q want %q", render3.Sort.Dir, "desc")
	}

	c.Apply(app.Action{Kind: app.ActionSort, Arg: "type"})
	render4 := *c.Snapshot().Body.List
	wantType := []string{"i-0bbb222222222222b", "i-0aaa111111111111a", "i-0ccc333333333333c"} // g4dn.xlarge, m5.large, t3.micro
	if got := rowIDs(render4.Rows); !idsEqual(got, wantType) {
		t.Fatalf("render4 (sort type, new column resets to asc): got %v want %v — a Col-blind memo key would still return render3's name-desc order", got, wantType)
	}
	if render4.Sort.Col != "type" || render4.Sort.Dir != "asc" {
		t.Errorf("render4 Sort: got %+v want {Col:type Dir:asc}", render4.Sort)
	}
}

func rowIDs(rows []app.ListRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ResourceID
	}
	return ids
}

func idsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ===========================================================================
// (c) attention-only toggle, both directions
// ===========================================================================

// TestListBodyMemoSeq_AttentionToggle_BothDirectionsAcrossRenders renders
// after enabling AND after disabling the attention filter.
// TestListAttention_ToggleOffRestoresAll (core/app/list_test.go) applies
// both toggle actions back-to-back and renders only once at the end, so it
// never observes the ON render — a memo that fails to invalidate on the
// OFF transition specifically (e.g. treats "AttentionOnly flipped" as a
// one-shot cache-bust already consumed by the ON render) would leak the
// filtered row set into this render.
func TestListBodyMemoSeq_AttentionToggle_BothDirectionsAcrossRenders(t *testing.T) {
	c := wave3ListController(t, "ec2")
	resources := wave3FilterEC2Resources() // cache-node carries a SevWarn Finding
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 3 {
		t.Fatalf("render1 (attention off): Rows count: got %d want 3", len(render1.Rows))
	}

	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	render2 := *c.Snapshot().Body.List
	if !render2.AttentionOnly {
		t.Fatal("render2: AttentionOnly should be true")
	}
	if len(render2.Rows) != 1 || render2.Rows[0].ResourceID != "i-0ccc333333333333c" {
		t.Fatalf("render2 (attention on): got rows %+v want only cache-node", render2.Rows)
	}

	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	render3 := *c.Snapshot().Body.List
	if render3.AttentionOnly {
		t.Error("render3: AttentionOnly should be false")
	}
	if len(render3.Rows) != 3 {
		t.Fatalf("render3 (attention off again): Rows count: got %d want 3 — a stale memo would still show render2's 1-row filtered set", len(render3.Rows))
	}
}

// ===========================================================================
// (d) RowStore generation advance via ResourcesLoaded, including load-more
// ===========================================================================

// TestListBodyMemoSeq_LoadMoreAppend_NewRowsAppearOnNextRender renders once
// after the initial page lands, appends a second page through the same
// ApplyResourcesLoaded/ObserveRows seam a real load-more (m key) drives
// (core/app/list_body.go:145-150 routes both the initial and the appended
// call through Core.ObserveRows, bumping ls.RowsGen), then renders again and
// asserts the appended rows are visible. No existing test renders BEFORE an
// append lands to prove the append is what changed the second render's
// content, rather than the append call itself doing so out of band.
func TestListBodyMemoSeq_LoadMoreAppend_NewRowsAppearOnNextRender(t *testing.T) {
	c := wave3ListController(t, "ec2")
	page1 := []resource.Resource{
		{ID: "i-page1-a", Name: "alpha", Type: "ec2", Fields: map[string]string{"instance_id": "i-page1-a", "name": "alpha", "state": "running"}},
		{ID: "i-page1-b", Name: "beta", Type: "ec2", Fields: map[string]string{"instance_id": "i-page1-b", "name": "beta", "state": "running"}},
	}
	c.ApplyResourcesLoaded("ec2", page1, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"}, false)

	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 2 {
		t.Fatalf("render1 (page 1): Rows count: got %d want 2", len(render1.Rows))
	}

	page2 := []resource.Resource{
		{ID: "i-page2-c", Name: "gamma", Type: "ec2", Fields: map[string]string{"instance_id": "i-page2-c", "name": "gamma", "state": "running"}},
		{ID: "i-page2-d", Name: "delta", Type: "ec2", Fields: map[string]string{"instance_id": "i-page2-d", "name": "delta", "state": "running"}},
	}
	c.Apply(app.Action{Kind: app.ActionLoadMore})
	c.ApplyResourcesLoaded("ec2", page2, nil, true)

	render2 := *c.Snapshot().Body.List
	if len(render2.Rows) != 4 {
		t.Fatalf("render2 (after load-more append): Rows count: got %d want 4, rows=%v — appended rows must appear on the next render", len(render2.Rows), rowIDs(render2.Rows))
	}
	seen := map[string]bool{}
	for _, r := range render2.Rows {
		seen[r.ResourceID] = true
	}
	for _, id := range []string{"i-page1-a", "i-page1-b", "i-page2-c", "i-page2-d"} {
		if !seen[id] {
			t.Errorf("render2 missing row %q", id)
		}
	}
}

// ===========================================================================
// (e) wave-2 finding lands on an already-rendered list
// ===========================================================================

// TestListBodyMemoSeq_EnrichmentLandsAfterFirstRender_DecoratorFlipsOnNextRender
// renders a healthy row, applies a Wave-2 finding via ApplyEnrichmentState
// (the same seam production code's Wave-2 sweep uses), then renders again.
// TestEnrichment_BrokenRowHasDecoratorError (core/app/list_test.go) and
// TestWave3ListStatusColumn_EnrichmentMapOnlyFinding_OverridesRawState
// (this file) both apply the finding BEFORE ever rendering, so neither
// proves a SECOND render — as opposed to the first-ever render of that
// screen — picks up the change.
//
// This sequence is the sharpest edge for an incoming memo key:
// applyEnrichmentState (core/app/list_filter.go:296-348) writes only
// c.enrichmentStore/enrichmentDetails/enrichmentTruncated — it never
// advances ls.RowsGen or any other generation counter. A memo key built
// only from (RowsGen, filter, attention, sort) would therefore hit on
// render2 and return render1's pre-enrichment Decorator/Severity/status
// cell; the key must also incorporate the enrichment store's own state (or
// enrichment must be given its own generation bump) to invalidate here.
func TestListBodyMemoSeq_EnrichmentLandsAfterFirstRender_DecoratorFlipsOnNextRender(t *testing.T) {
	c := wave3ListController(t, "ec2")
	resources := []resource.Resource{
		{ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaa111111111111a", "name": "web-server", "state": "running"}},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 1 {
		t.Fatalf("render1: Rows count: got %d want 1", len(render1.Rows))
	}
	if render1.Rows[0].Decorator != app.DecoratorNormal {
		t.Fatalf("render1 (before enrichment): Decorator: got %q want %q", render1.Rows[0].Decorator, app.DecoratorNormal)
	}
	if render1.Rows[0].Severity != "" {
		t.Fatalf("render1 (before enrichment): Severity: got %q want empty", render1.Rows[0].Severity)
	}

	c.ApplyEnrichmentState("ec2", 1, false, map[string][]domain.Finding{
		"i-0aaa111111111111a": {{Code: "ec2.impaired", Phrase: "system check failed", Severity: domain.SevBroken, Source: "wave2:ec2"}},
	}, nil)

	render2 := *c.Snapshot().Body.List
	if len(render2.Rows) != 1 {
		t.Fatalf("render2: Rows count: got %d want 1", len(render2.Rows))
	}
	if render2.Rows[0].Decorator != app.DecoratorError {
		t.Errorf("render2 (after enrichment): Decorator: got %q want %q — stale cache would still show %q", render2.Rows[0].Decorator, app.DecoratorError, app.DecoratorNormal)
	}
	if render2.Rows[0].Severity != "broken" {
		t.Errorf("render2 (after enrichment): Severity: got %q want %q", render2.Rows[0].Severity, "broken")
	}
}

// ===========================================================================
// Benchmark: baseline evidence for the memoization refactor
// ===========================================================================

// newBenchListController mirrors newTestController (app_controller_test.go)
// but takes a *testing.B, since the buildListBody memoization refactor's
// baseline must be measured, not test-asserted.
func newBenchListController(b *testing.B) *app.Controller {
	b.Helper()
	b.Setenv("A9S_CONFIG_FOLDER", b.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	b.Cleanup(c.Close)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	return c
}

// memoBenchEC2Rows builds n realistic, multi-column EC2 resources: distinct
// IDs/names/IPs, a rotating instance-type/state set, and a Wave-1 Finding on
// roughly one row in eight — the same shape wave3FilterEC2Resources uses at
// small scale, extended to a large-list size.
func memoBenchEC2Rows(n int) []resource.Resource {
	states := []string{"running", "stopped", "pending", "terminated"}
	types := []string{"m5.large", "g4dn.xlarge", "t3.micro", "c6g.xlarge", "r5.2xlarge"}
	rows := make([]resource.Resource, n)
	for i := range n {
		id := "i-" + hexPad(i)
		r := resource.Resource{
			ID: id, Name: "host-" + hexPad(i), Type: "ec2",
			Fields: map[string]string{
				"instance_id": id,
				"name":        "host-" + hexPad(i),
				"state":       states[i%len(states)],
				"type":        types[i%len(types)],
				"private_ip":  "10.0.0." + hexPad(i%254+1),
				"public_ip":   "203.0.113." + hexPad(i%254+1),
			},
		}
		if i%8 == 0 {
			r.Findings = []domain.Finding{
				{Code: "ec2.degraded", Phrase: "instance degraded", Severity: domain.SevWarn},
			}
		}
		rows[i] = r
	}
	return rows
}

func hexPad(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(digits[n%10]) + s
		n /= 10
	}
	return s
}

// BenchmarkListSnapshot_LargeList is the before/after baseline for the
// buildListBody memoization refactor: a realistic 3000-row EC2 list, with
// Snapshot() called every frame and the cursor moved on roughly one frame in
// ten — the common steady-state render (repeated filter/sort/cell-extract
// with no other input changing), which is exactly what the refactor's cache
// is meant to skip.
func BenchmarkListSnapshot_LargeList(b *testing.B) {
	c := newBenchListController(b)
	c.ApplyResourcesLoaded("ec2", memoBenchEC2Rows(3000), nil, false)

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		if i%10 == 0 {
			c.Apply(app.Action{Kind: app.ActionMoveDown})
		}
		_ = c.Snapshot()
		i++
	}
}
