// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Each test renders through Controller.Snapshot() at least twice with exactly
// one list-body input changed between renders, and asserts the second render
// reflects the change. A memo key that mis-tracks that input shows up as a
// stale second render, which a mutate-then-render-once test cannot catch.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// Moving from one non-empty filter to a different one must not reuse the
// first filter's rows: the memo keys on the filter text, not on whether a
// filter is active.
func TestListBodyMemoSeq_FilterSetChangedCleared_RowsAndCountUpdate(t *testing.T) {
	c := openListController(t, "ec2")
	resources := wave3FilterEC2Resources()
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	// cache-node carries a Wave-1 SevWarn Finding, so the frame title shows the
	// unconditional " !1" issue-count suffix (docs/attention-signals.md) whenever
	// cache-node is among the visible rows.
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

// A memo keyed on the sort column but not its direction, or vice-versa,
// would return the previous render's row order.
func TestListBodyMemoSeq_SortToggle_RowOrderUpdatesAcrossRenders(t *testing.T) {
	c := openListController(t, "ec2")
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

// A memo that fails to invalidate on the attention filter's OFF transition
// would leak the filtered row set into the next render.
func TestListBodyMemoSeq_AttentionToggle_BothDirectionsAcrossRenders(t *testing.T) {
	c := openListController(t, "ec2")
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

// The initial page and a load-more append both go through Core.ObserveRows,
// which bumps ls.RowsGen; the appended rows appear on the next render.
func TestListBodyMemoSeq_LoadMoreAppend_NewRowsAppearOnNextRender(t *testing.T) {
	c := openListController(t, "ec2")
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

// applyEnrichmentState writes only the enrichment store and advances no
// generation counter, so the memo key must also track enrichment state for a
// second render to pick up a Wave-2 finding.
func TestListBodyMemoSeq_EnrichmentLandsAfterFirstRender_DecoratorFlipsOnNextRender(t *testing.T) {
	c := openListController(t, "ec2")
	resources := []resource.Resource{
		{ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaa111111111111a", "name": "web-server", "state": "running"}},
	}
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 1 {
		t.Fatalf("render1: Rows count: got %d want 1", len(render1.Rows))
	}
	// The probe is the rendered Status cell, not the row decorator: a list
	// row's colour is the worst finding over both waves, so buildListBody
	// produces no glyph. The status-cell override reads the enrichment store
	// directly.
	if got := render1.Rows[0].Cells[render1.StatusCol]; got == "system check failed" {
		t.Fatalf("render1 (before enrichment): Status cell already carries the Wave-2 phrase")
	}

	c.ApplyEnrichmentState("ec2", 1, false, map[string][]domain.Finding{
		"i-0aaa111111111111a": {{Code: "ec2.impaired", Phrase: "system check failed", Severity: domain.SevBroken, Source: "wave2:ec2"}},
	}, nil)

	render2 := *c.Snapshot().Body.List
	if len(render2.Rows) != 1 {
		t.Fatalf("render2: Rows count: got %d want 1", len(render2.Rows))
	}
	if got := render2.Rows[0].Cells[render2.StatusCol]; got != "system check failed" {
		t.Errorf("render2 (after enrichment): Status cell: got %q want %q — a stale memo would still show the pre-enrichment cell", got, "system check failed")
	}
}

// openListController never calls ApplyResourcesLoaded, so ls.Rows stays nil
// and listScreenResources falls back to the session RowStore. An
// AvailabilityChecked event writes the RowStore through Core.ObserveRows
// without touching the screen's ListState, so only the memo key's
// AnyOriginResourceCacheGen term can invalidate the memo.
//
// Gen: domain.Gen(1) is session.New()'s AvailabilityGen seed and
// openListController never bumps it, so the same literal passes
// messages.IsStale on both deliveries in a test.
func TestListBodyMemoSeq_BackgroundAvailabilityReplace_FallbackRowsStaleAcrossRenders(t *testing.T) {
	c := openListController(t, "ec2")

	r1 := []resource.Resource{
		{ID: "i-r1-aaaa", Name: "batch-1-alpha", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-r1-aaaa", "name": "batch-1-alpha", "state": "running"}},
		{ID: "i-r1-bbbb", Name: "batch-1-beta", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-r1-bbbb", "name": "batch-1-beta", "state": "running"}},
	}
	c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        len(r1),
		Resources:    r1,
		Gen:          domain.Gen(1),
	})

	render1 := *c.Snapshot().Body.List
	wantR1 := []string{"i-r1-aaaa", "i-r1-bbbb"}
	if got := rowIDs(render1.Rows); !idsEqual(got, wantR1) {
		t.Fatalf("render1 (ls.Rows nil, first background AvailabilityChecked write): got %v want %v", got, wantR1)
	}

	r2 := []resource.Resource{
		{ID: "i-r2-cccc", Name: "batch-2-gamma", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-r2-cccc", "name": "batch-2-gamma", "state": "running"}},
		{ID: "i-r2-dddd", Name: "batch-2-delta", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-r2-dddd", "name": "batch-2-delta", "state": "running"}},
	}
	c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        len(r2),
		Resources:    r2,
		Gen:          domain.Gen(1),
	})

	render2 := *c.Snapshot().Body.List
	wantR2 := []string{"i-r2-cccc", "i-r2-dddd"}
	if got := rowIDs(render2.Rows); !idsEqual(got, wantR2) {
		t.Fatalf("render2 (second background AvailabilityChecked replaced the RowStore's ec2 rows while ls.Rows stayed nil the whole time): got %v want %v — a stale listBodyMemo (its key never saw the RowStore write since none of rowsVersion/Filter/AttentionOnly/SortCol/SortDir/enrichmentGen changed) would still return render1's rows", got, wantR2)
	}
}

func TestListBodyMemoSeq_BackgroundAvailabilityFirstWrite_EmptyFallbackStaysStaleAfterPopulate(t *testing.T) {
	c := openListController(t, "ec2")

	render1 := *c.Snapshot().Body.List
	if len(render1.Rows) != 0 {
		t.Fatalf("render1 (ls.Rows nil, RowStore never written for ec2): Rows count: got %d want 0, rows=%v", len(render1.Rows), rowIDs(render1.Rows))
	}

	r1 := []resource.Resource{
		{ID: "i-eb-aaaa", Name: "empty-batch-alpha", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-eb-aaaa", "name": "empty-batch-alpha", "state": "running"}},
		{ID: "i-eb-bbbb", Name: "empty-batch-beta", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-eb-bbbb", "name": "empty-batch-beta", "state": "running"}},
	}
	c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        len(r1),
		Resources:    r1,
		Gen:          domain.Gen(1),
	})

	render2 := *c.Snapshot().Body.List
	want := []string{"i-eb-aaaa", "i-eb-bbbb"}
	if got := rowIDs(render2.Rows); !idsEqual(got, want) {
		t.Fatalf("render2 (RowStore's first-ever ec2 write landed via a background AvailabilityChecked, ls.Rows still nil): got %v want %v — render1 cached the empty fallback and the memo key never saw the write", got, want)
	}
}

// newBenchListController mirrors newTestController but takes a *testing.B.
func newBenchListController(b *testing.B) *app.Controller {
	b.Helper()
	b.Setenv("A9S_CONFIG_FOLDER", b.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(b, core)
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

// BenchmarkListSnapshot_LargeList measures a 3000-row EC2 list with
// Snapshot() called every frame and the cursor moved on roughly one frame in
// ten: the steady-state render where no memo input changes.
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
