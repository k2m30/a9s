// list_ports_test.go — live-seam port pins for the ResourceList/MainMenu
// family. Ports three narrow, verified-missing behavior pins from the doomed
// legacy view-model tests (specs/022-codebase-cleanup/wave3-map-list-menu.md)
// onto the LIVE controller seams, so the legacy pins can be deleted in a
// later round without losing coverage:
//
//  1. listFilterResources (core/app/list_filter.go) — Fields-value and
//     Findings-phrase text-filter match branches. core/app/list_test.go's
//     TestListFilter_* only exercises Name-based matches.
//  2. reapplyCheckerAgainst (core/app/list_filter.go), driven through the
//     real c.Handle(messages.ResourcesLoaded{...}) event path — merge-across-
//     LoadMore, non-truncated extension, zero-initial filtering, and sort
//     preservation. Existing coverage (reapply_checker_leak_test.go,
//     headless_regression_test.go) only pins leak-prevention and payload
//     shape, never the merge/grow/sort behavior itself.
//  3. resolveListMarkerCol (core/app/list_columns.go) — byte-parity
//     against the legacy identity-column cascade (resolveIdentityColumn in
//     internal/tui/views/table_render.go), across every real catalog type.
//     tui_viewstate_purity_list_test.go's MarkerCol case only covers the
//     render-CONSUMPTION side (translate body.MarkerCol into a glyph) with
//     one synthetic type; it never checks that resolveListMarkerCol
//     COMPUTES the same index the legacy cascade would for real types. This
//     is deliberately self-contained (no shared helpers with
//     resourcelist_render_parity_test.go) so it keeps running after that
//     file is deleted.
package unit_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// Shared controller helper (distinctly named to avoid colliding with — and
// avoid depending on — the doomed parity/purity test files' own helpers).
// ---------------------------------------------------------------------------

// openListController builds a Controller (via the blessed newTestController
// helper — see qa_controller_construction_discipline_test.go) pre-navigated
// to a ScreenResourceList for the given resource type ShortName.
func openListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// newListController is a compatibility shim: qa_controller_frame_title_issue_badge_test.go
// and qa_title_warning_findings_test.go depend on a helper of this exact name
// that used to live in resourcelist_render_parity_test.go (deleted after its
// pins were ported here, construction now routed through
// openListController/newTestController instead of a raw app.New call). Kept
// here so those two out-of-scope files keep compiling unchanged.
func newListController(t *testing.T, shortName string) *app.Controller {
	return openListController(t, shortName)
}

// ===========================================================================
// 1. listFilterResources — Fields-value + Findings-phrase match branches
// ===========================================================================

// wave3FilterEC2Resources returns 3 EC2 instances distinguishing the filter
// branches under test: private IP and instance type live in Fields (matched
// by the generic Fields-value loop); the third instance carries NO matching
// Name/Fields signal at all — it can only be found via its Findings[].Phrase.
func wave3FilterEC2Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0aaa111111111111a",
				"name":        "web-server",
				"state":       "running",
				"type":        "m5.large",
				"private_ip":  "10.0.48.11",
				"public_ip":   "203.0.113.20",
			},
		},
		{
			ID: "i-0bbb222222222222b", Name: "db-server", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0bbb222222222222b",
				"name":        "db-server",
				"state":       "stopped",
				"type":        "g4dn.xlarge",
				"private_ip":  "10.0.48.55",
				"public_ip":   "",
			},
		},
		{
			ID: "i-0ccc333333333333c", Name: "cache-node", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0ccc333333333333c",
				"name":        "cache-node",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.99.7",
				"public_ip":   "",
			},
			Findings: []domain.Finding{
				{Code: "ec2.degraded", Phrase: "instance degraded", Severity: domain.SevWarn},
			},
		},
	}
}

func TestListFilter_MatchesFieldsValue_PrivateIP(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "10.0.48"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 2 {
		t.Fatalf("filter '10.0.48' (private_ip Fields value): Rows count: got %d want 2", len(lb.Rows))
	}
	got := map[string]bool{lb.Rows[0].ResourceID: true, lb.Rows[1].ResourceID: true}
	if !got["i-0aaa111111111111a"] || !got["i-0bbb222222222222b"] {
		t.Errorf("filter '10.0.48': wrong rows matched: %v", got)
	}
}

func TestListFilter_MatchesFieldsValue_PublicIP(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "203.0.113.20"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter '203.0.113.20' (public_ip Fields value): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0aaa111111111111a" {
		t.Errorf("filter '203.0.113.20': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

func TestListFilter_MatchesFieldsValue_InstanceType(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "g4dn"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'g4dn' (instance-type Fields value): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0bbb222222222222b" {
		t.Errorf("filter 'g4dn': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

// TestListFilter_MatchesFindingsPhrase_CaseInsensitive isolates the
// third listFilterResources branch (r.Findings[i].Phrase), which no
// controller-path test exercised before this file: cache-node has no Name or
// Fields value containing "degraded" — it can only be found via its finding
// phrase. Also proves the match is case-insensitive, mirroring the ID/Name/
// Fields branches.
func TestListFilter_MatchesFindingsPhrase_CaseInsensitive(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "DEGRADED"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'DEGRADED' (Findings[].Phrase, case-insensitive): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0ccc333333333333c" {
		t.Errorf("filter 'DEGRADED': wrong ResourceID: got %q — should match cache-node via its finding phrase only", lb.Rows[0].ResourceID)
	}
}

// ===========================================================================
// 2. reapplyCheckerAgainst — merge-across-LoadMore, non-truncated extension,
//    zero-initial filtering, sort preservation, no-checker inert.
//    Driven through the REAL c.Handle(messages.ResourcesLoaded{...}) event
//    path (handle.go calls reapplyCheckerAgainst only from there — the
//    ApplyResourcesLoaded test seam does not).
// ===========================================================================

// wave3VPCSGChecker mirrors the shape of a real reverse-scan checker (e.g.
// checkVPCSecurityGroup): matches "sg" resources whose Fields["vpc_id"]
// equals the source VPC's ID.
func wave3VPCSGChecker(_ context.Context, _ any, src resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	entry, ok := cache["sg"]
	if !ok {
		return resource.KnownRelated("sg", nil, false)
	}
	var matched []string
	for _, r := range entry.Resources {
		if r.Fields["vpc_id"] == src.ID {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 {
		return resource.KnownRelated("sg", nil, false)
	}
	return resource.KnownRelated("sg", matched, false)
}

func wave3SG(id, vpcID string) resource.Resource {
	return resource.Resource{
		ID: id, Name: id, Type: "sg",
		Fields: map[string]string{"group_name": id, "group_id": id, "vpc_id": vpcID},
	}
}

// TestRelatedCheckerCarry_ZeroInitialGrowsOnLoadMore verifies that a
// (0+) truncated pivot (empty initial RelatedIDSet, non-nil so it hides
// everything) both hides unrelated rows immediately AND grows as further
// LoadMore pages arrive — the merge accumulates across pages, it does not
// reset.
func TestRelatedCheckerCarry_ZeroInitialGrowsOnLoadMore(t *testing.T) {
	c := openListController(t, "sg")
	c.PatchListReapplyChecker(wave3VPCSGChecker, resource.Resource{ID: "vpc-target"})

	// Page 1: two SGs, neither in vpc-target. Zero-initial carry must hide
	// both immediately (nil filter != empty filter).
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Resources: []resource.Resource{
		wave3SG("sg-1", "vpc-other-1"),
		wave3SG("sg-2", "vpc-other-2"),
	}})
	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 0 {
		t.Fatalf("after page1 (no matches): want 0 visible rows, got %d", len(lb.Rows))
	}

	// Page 2 (load-more): one new match.
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-3", "vpc-target"),
		wave3SG("sg-4", "vpc-other-3"),
	}})
	lb = *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("after page2 (1 match): want 1 visible row, got %d", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "sg-3" {
		t.Errorf("after page2: wrong matched row: got %q want sg-3", lb.Rows[0].ResourceID)
	}

	// Page 3 (load-more): two more matches — the set must grow, not reset.
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-5", "vpc-target"),
		wave3SG("sg-6", "vpc-target"),
	}})
	lb = *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("after page3 (2 more matches): want 3 visible rows (grown, not reset), got %d", len(lb.Rows))
	}
}

// TestRelatedCheckerCarry_NonTruncatedStillExtends verifies that even a
// non-truncated pivot (initial RelatedIDSet seeded from an exact match count,
// not a (0+)/(N+) scan) keeps extending on LoadMore — the carry mechanism is
// identical regardless of how the initial set was seeded.
func TestRelatedCheckerCarry_NonTruncatedStillExtends(t *testing.T) {
	c := openListController(t, "sg")
	c.PatchListRelatedIDSet([]string{"sg-a", "sg-b"})
	c.PatchListReapplyChecker(wave3VPCSGChecker, resource.Resource{ID: "vpc-target"})

	if got := len(c.GetListRelatedIDSet()); got != 2 {
		t.Fatalf("precondition: RelatedIDSet size = %d, want 2", got)
	}

	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-c", "vpc-target"),
	}})

	if got := len(c.GetListRelatedIDSet()); got != 3 {
		t.Errorf("after load-more match: RelatedIDSet size = %d, want 3 (sg-a, sg-b, sg-c)", got)
	}
}

// TestRelatedCheckerCarry_PreservesSortAfterMerge verifies that when the
// checker-carry merge grows the RelatedIDSet, the active sort is honored for
// the newly visible rows — they land in sorted position, not in arrival
// order.
func TestRelatedCheckerCarry_PreservesSortAfterMerge(t *testing.T) {
	c := openListController(t, "sg")

	alwaysMatch := func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		entry, ok := cache["sg"]
		if !ok {
			return resource.KnownRelated("sg", nil, false)
		}
		ids := make([]string, len(entry.Resources))
		for i, r := range entry.Resources {
			ids[i] = r.ID
		}
		return resource.KnownRelated("sg", ids, false)
	}
	c.PatchListReapplyChecker(alwaysMatch, resource.Resource{ID: "vpc-target"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "group_name"})

	// Rows arrive in reverse-alpha order; if sort is re-applied after the
	// merge, visible order must be alphabetical ascending regardless.
	c.Handle(messages.ResourcesLoaded{ResourceType: "sg", Resources: []resource.Resource{
		wave3SG("sg-zeta", "vpc-target"),
		wave3SG("sg-mu", "vpc-target"),
		wave3SG("sg-alpha", "vpc-target"),
	}})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("want 3 visible rows, got %d", len(lb.Rows))
	}
	got := []string{lb.Rows[0].Cells[0], lb.Rows[1].Cells[0], lb.Rows[2].Cells[0]}
	want := []string{"sg-alpha", "sg-mu", "sg-zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort not re-applied after checker merge; got order %v, want %v", got, want)
		}
	}
}

// TestRelatedCheckerCarry_NoChecker_Inert verifies that a list without a
// carried checker is unaffected by the merge machinery — the feature is
// opt-in and must not regress ordinary (non-related) list loads.
func TestRelatedCheckerCarry_NoChecker_Inert(t *testing.T) {
	c := openListController(t, "ec2")
	c.PatchListRelatedIDSet([]string{"i-1", "i-2"})
	// No PatchListReapplyChecker call.

	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: []resource.Resource{
		{ID: "i-3", Name: "i-3", Type: "ec2", Fields: map[string]string{"instance_id": "i-3"}},
	}})

	if got := len(c.GetListRelatedIDSet()); got != 2 {
		t.Errorf("without a carried checker, RelatedIDSet must remain untouched by Handle; size = %d, want 2", got)
	}
}

// ===========================================================================
// 3. resolveListMarkerCol — S13 marker-glyph placement check. Verifies
//    RenderList(ListBody) places the enrichment glyph (row.Decorator) at the
//    exact cell body.MarkerCol/body.ScrollX identify, across every real
//    catalog type. The legacy ResourceListModel.View() byte-parity oracle
//    this once ran against is gone (View() is dead code); this now asserts
//    directly against the already-resolved body fields instead.
// ===========================================================================

func wave3MarkerColResources(td resource.ResourceTypeDef, n int) []resource.Resource {
	statuses := []string{"running", "stopped", "pending", "available", "active", "terminated"}
	lk := td.LifecycleKey
	if lk == "" {
		lk = "state"
	}
	out := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("%s-%03d", td.ShortName, i+1)
		fields := make(map[string]string, len(td.Columns)+4)
		for _, col := range td.Columns {
			switch {
			case col.Key == "name" || strings.Contains(strings.ToLower(col.Key), "name"):
				fields[col.Key] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
			case col.Key == lk || col.Key == "state" || col.Key == "status":
				fields[col.Key] = statuses[i%len(statuses)]
			default:
				fields[col.Key] = fmt.Sprintf("v-%s-%d", col.Key, i+1)
			}
		}
		fields["name"] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
		fields[lk] = statuses[i%len(statuses)]
		out[i] = resource.Resource{ID: id, Name: fmt.Sprintf("demo-%s-%d", td.ShortName, i+1), Fields: fields}
	}
	return out
}

func wave3MarkerColFindings(resources []resource.Resource) map[string][]domain.Finding {
	if len(resources) == 0 {
		return nil
	}
	out := make(map[string][]domain.Finding, len(resources)/2+1)
	for i, r := range resources {
		if i%2 == 0 {
			sev := domain.SevBroken
			if i%4 == 0 {
				sev = domain.SevWarn
			}
			out[r.ID] = []domain.Finding{{Code: "WAVE3-MARKERCOL", Phrase: "wave3 test finding", Severity: sev}}
		}
	}
	return out
}

// wave3ColSep matches the gap between two adjacent rendered column slots:
// renderListDataRow (resourcelist.go) pads every cell to its fixed column
// width via text.PadOrTrunc and joins columns with a literal "  " (2-space)
// separator, so the run of whitespace between two slots is always >= 2 chars.
var wave3ColSep = regexp.MustCompile(`\s{2,}`)

// wave3ColStarts returns the character offset, within a rendered list line,
// at which each column's slot begins. It derives these offsets from the
// HEADER line rather than a data line: text.PadOrTrunc left-aligns (content
// first, spaces padded on the right) and every column's Title is non-empty,
// so the header's per-column content always starts exactly at its slot's
// start with no ambiguity. A DATA row's own cell can be empty (a keyless
// Path column with no RawStruct renders "" — see resource.ExtractScalar),
// which collapses that slot's padding into the surrounding 2-space
// separators when naively split on whitespace runs, shifting the inferred
// index of every later column. Header and data rows share the exact same
// resolved column list (post sort-prefix-widen, post status-widen, post
// fitColumns) and join scheme, so a slot's start offset measured on the
// header line applies unchanged to every data line rendered alongside it.
func wave3ColStarts(headerLine string) []int {
	trimmed := strings.TrimRight(headerLine, " ")
	pos := 0
	for pos < len(trimmed) && trimmed[pos] == ' ' {
		pos++
	}
	starts := []int{pos}
	for _, gap := range wave3ColSep.FindAllStringIndex(trimmed, -1) {
		starts = append(starts, gap[1])
	}
	return starts
}

// wave3AssertMarkerGlyphPlacement checks that RenderList's output places each
// row's already-resolved glyph (row.Decorator) at the cell body.MarkerCol
// names, when that column is on-screen (body.MarkerCol - body.ScrollX >= 0),
// and that the glyph does NOT leak into the output when the marker column
// has scrolled off-screen. It reads body.MarkerCol/ScrollX/Decorator
// verbatim (already computed by resolveListMarkerCol/resolveListDecoratorFull)
// rather than re-deriving them, so it only pins RenderList's OWN placement
// logic, not the column/decorator resolution cascade (covered elsewhere).
// wave3AssertT is the subset of *testing.T that wave3AssertMarkerGlyphPlacement
// needs, so a meta-test can verify it actually fails on broken input (via
// wave3FailSpy) without the failure bubbling up through t.Run and marking an
// otherwise-passing parent test as failed.
type wave3AssertT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// wave3FailSpy records whether Errorf/Fatalf was ever called, without
// touching *testing.T's internals or aborting the calling goroutine.
type wave3FailSpy struct{ failed bool }

func (s *wave3FailSpy) Helper()                           {}
func (s *wave3FailSpy) Errorf(format string, args ...any) { s.failed = true }
func (s *wave3FailSpy) Fatalf(format string, args ...any) { s.failed = true }

func wave3AssertMarkerGlyphPlacement(t wave3AssertT, typeName string, out string, body app.ListBody) {
	t.Helper()
	// Cursor-row reverse-video (and any other non-color SGR attribute) wraps
	// a cell's rendered text even under tuitest.NoColor (NO_COLOR only
	// suppresses color, not attributes like reverse-video/bold/underline —
	// confirmed empirically: the selected row's cells carry \x1b[7m...\x1b[m
	// regardless). Substring-matching the raw output risks a false negative
	// (or worse, a false positive on a coincidental byte sequence) if any
	// styling ever wraps the glyph and its cell text separately instead of
	// as one contiguous span; stripping ANSI first makes the check depend
	// only on the actual rendered characters.
	out = stripAnsi(out)
	onScreen := body.MarkerCol-body.ScrollX >= 0

	// Per-row, per-COLUMN check instead of a whole-output occurrence count:
	// a plain strings.Count(out, decorator+" "+cellText) still passes if the
	// decorator renders beside the right text in the WRONG column (e.g. a
	// wrong markerColIdx translation that happens to land on a column
	// holding an identical value). Column slot offsets are measured once
	// from the header line (wave3ColStarts) — not re-derived per data row —
	// so an empty cell in an earlier column of THIS row can never shift the
	// slot boundary used to check the marker column.
	lines := strings.Split(out, "\n")
	var colStarts []int
	if onScreen && len(lines) > 0 {
		colStarts = wave3ColStarts(lines[0])
	}
	for i, row := range body.Rows {
		if row.Decorator == "" {
			continue
		}
		lineIdx := i + 1 // header occupies line 0
		if lineIdx >= len(lines) {
			t.Fatalf("[%s]: row %d (id=%s) has a decorator but no corresponding rendered line (MarkerCol=%d, ScrollX=%d):\n%s",
				typeName, i, row.ResourceID, body.MarkerCol, body.ScrollX, out)
		}
		cellText := ""
		if body.MarkerCol >= 0 && body.MarkerCol < len(row.Cells) {
			cellText = row.Cells[body.MarkerCol]
		}
		want := string(row.Decorator)
		if cellText != "" {
			want += " " + cellText
		}

		if !onScreen {
			// Check for the decorator glyph ANYWHERE in the rendered line, not
			// just as part of the full "decorator+cellText" combo in a given
			// column — a decorator that leaks into a DIFFERENT column (one
			// that doesn't happen to hold the marker column's own cell text)
			// would otherwise pass undetected.
			if strings.Contains(lines[lineIdx], string(row.Decorator)) {
				t.Errorf("[%s]: row %d (id=%s) decorator glyph %q leaked into its rendered line even though the marker column scrolled off-screen (MarkerCol=%d, ScrollX=%d):\n%s",
					typeName, i, row.ResourceID, row.Decorator, body.MarkerCol, body.ScrollX, lines[lineIdx])
			}
			continue
		}

		wantIdx := body.MarkerCol - body.ScrollX
		line := lines[lineIdx]
		if wantIdx < 0 || wantIdx >= len(colStarts) {
			t.Errorf("[%s]: row %d (id=%s) marker column index %d out of range (have %d rendered columns; MarkerCol=%d, ScrollX=%d):\n%s",
				typeName, i, row.ResourceID, wantIdx, len(colStarts), body.MarkerCol, body.ScrollX, line)
			continue
		}
		start := colStarts[wantIdx]
		got := ""
		switch {
		case start+len(want) <= len(line):
			got = line[start : start+len(want)]
		case start < len(line):
			got = line[start:]
		}
		if got != want {
			t.Errorf("[%s]: row %d (id=%s) expected glyph-prefixed cell %q at column index %d (char offset %d; MarkerCol=%d, ScrollX=%d), got %q:\n%s",
				typeName, i, row.ResourceID, want, wantIdx, start, body.MarkerCol, body.ScrollX, got, line)
		}
	}
}

// TestMarkerColParity_EnrichmentFindings_AllResourceTypes ports the
// unique behavior scenario S13 exercised from resourcelist_render_parity_test.go
// (the "PRIMARY MarkerCol gap detector": enrichment glyphs force the render
// path to actually consume MarkerCol, unlike a plain unfiltered list where a
// wrong index can go unnoticed). It runs across every registered resource
// type so a step-3-cascade divergence (column Path contains "Name"/
// "Identifier" but Key isn't "name") in ANY type is caught, not just one
// synthetic type.
func TestMarkerColParity_EnrichmentFindings_AllResourceTypes(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — catalog not registered")
	}

	const stdW, stdH = 160, 30

	for _, td := range allTypes {
		t.Run(td.ShortName, func(t *testing.T) {
			tuitest.NoColor(t)
			k := keys.Default()
			resources := wave3MarkerColResources(td, 6)
			findings := wave3MarkerColFindings(resources)

			m := views.NewResourceList(td, nil, k)
			m.SetSize(stdW, stdH)

			c := openListController(t, td.ShortName)
			c.ApplyResourcesLoaded(td.ShortName, resources, nil, false)
			c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings, nil)
			body := *c.Snapshot().Body.List

			wave3AssertMarkerGlyphPlacement(t, td.ShortName, m.RenderList(body), body)
		})
	}
}

// TestMarkerColParity_EnrichmentFindingsWithHScroll_AllResourceTypes
// ports S15 from resourcelist_render_parity_test.go: MarkerCol is a
// full-column-space index computed once by resolveListMarkerCol, but the
// GLYPH is placed at a SEPARATE, hscroll-translated visible column index
// (markerColIdx) — a distinct code path from the un-scrolled S13 case above.
// When the identity column scrolls off-screen, both the legacy cascade and
// the live translation must agree the glyph is hidden (no cell wrongly
// prefixed), not just that MarkerCol itself resolves to the right index.
// Neither the S13 pin above (no hscroll) nor tui_viewstate_purity_list_test.go
// (single synthetic type, no hscroll) exercises this interaction — this is a
// second, independent gap from the same parity-bridge precondition check.
func TestMarkerColParity_EnrichmentFindingsWithHScroll_AllResourceTypes(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — catalog not registered")
	}

	const stdW, stdH = 160, 30

	for _, td := range allTypes {
		t.Run(td.ShortName, func(t *testing.T) {
			if len(td.Columns) < 2 {
				t.Skip("type has fewer than 2 columns — hscroll not applicable")
			}
			tuitest.NoColor(t)
			k := keys.Default()
			resources := wave3MarkerColResources(td, 6)
			findings := wave3MarkerColFindings(resources)

			m := views.NewResourceList(td, nil, k)
			m.SetSize(stdW, stdH)

			c := openListController(t, td.ShortName)
			c.ApplyResourcesLoaded(td.ShortName, resources, nil, false)
			c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings, nil)
			c.Apply(app.Action{Kind: app.ActionScrollRight})
			body := *c.Snapshot().Body.List

			wave3AssertMarkerGlyphPlacement(t, td.ShortName, m.RenderList(body), body)
		})
	}
}

// TestMarkerColParity_EmptyPrecedingCell_KeylessPathColumn pins
// wave3AssertMarkerGlyphPlacement's own robustness against a column BEFORE
// the marker column rendering empty — the "keyless Path column with no
// RawStruct" case CodeRabbit flagged: naively splitting a data row on
// whitespace runs collapses an empty cell's padding into its neighboring
// separators, shifting the inferred index of every later column (including
// the marker column) and false-failing correct output. Uses a synthetic
// ResourceTypeDef (not a real catalog entry — RegisterFallbackTypeDef /
// PushChildListScreen mirrors the unifiedIssueCount helper's ad hoc
// registration pattern in qa_issue_count_invariant_test.go) with the marker
// column (Key: "name") at index 1, so an empty index-0 column sits strictly
// before it.
func TestMarkerColParity_EmptyPrecedingCell_KeylessPathColumn(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "wave3synthetic",
		Name:      "Wave3 Synthetic",
		Columns: []resource.Column{
			{Key: "empty_col", Title: "Empty", Width: 10},
			{Key: "name", Title: "Name", Width: 20},
			{Key: "val", Title: "Val", Width: 10},
		},
	}
	resources := make([]resource.Resource, 4)
	for i := range resources {
		resources[i] = resource.Resource{
			ID:   fmt.Sprintf("synthetic-%03d", i+1),
			Name: fmt.Sprintf("demo-synthetic-%d", i+1),
			// Deliberately no "empty_col" entry: mirrors a keyless Path
			// column with no RawStruct backing it, which renders "".
			Fields: map[string]string{
				"name": fmt.Sprintf("demo-synthetic-%d", i+1),
				"val":  fmt.Sprintf("v-val-%d", i+1),
			},
		}
	}
	findings := wave3MarkerColFindings(resources)

	buildBody := func(t *testing.T) (app.ListBody, string) {
		t.Helper()
		c := newTestController(t)
		c.RegisterFallbackTypeDef(td)
		c.PushChildListScreen(td.ShortName)
		c.ApplyResourcesLoaded(td.ShortName, resources, nil, false)
		c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings, nil)
		body := *c.Snapshot().Body.List
		if body.MarkerCol != 1 {
			t.Fatalf("precondition: MarkerCol = %d, want 1 (the \"name\" column) — synthetic td.Columns changed?", body.MarkerCol)
		}
		m := views.NewResourceList(td, nil, keys.Default())
		m.SetSize(160, 30)
		return body, m.RenderList(body)
	}

	t.Run("correct render with an empty preceding cell must NOT false-fail", func(t *testing.T) {
		body, out := buildBody(t)
		wave3AssertMarkerGlyphPlacement(t, td.ShortName, out, body)
	})

	t.Run("a marker column genuinely shifted by one must still fail", func(t *testing.T) {
		body, out := buildBody(t)
		shifted := body
		shifted.MarkerCol = body.MarkerCol + 1
		spy := &wave3FailSpy{}
		wave3AssertMarkerGlyphPlacement(spy, td.ShortName, out, shifted)
		if !spy.failed {
			t.Fatal("expected wave3AssertMarkerGlyphPlacement to fail against a shifted MarkerCol, but it passed")
		}
	})
}

// ===========================================================================
// 4. applyListFilters attention branch (round 4 unique-pin port) — CodeRabbit
//    PR-273: a resource whose Wave-1 Color always resolves Healthy and whose
//    embedded r.Findings is empty must still be shown under the attention
//    filter (ctrl+z) when it carries a Wave-2 enrichment finding correlated
//    by ID only (c.listEnrichmentFindings, fed by ApplyEnrichmentState).
//    core/app/list_test.go's TestListAttention_* only exercise resources
//    with a populated r.Findings — this branch (`len(r.Findings) == 0` +
//    `findings[r.ID]` lookup in applyListFilters) was otherwise unpinned.
//    Ported from the deleted qa_attention_filter_test.go's
//    TestAttentionFilter_IncludesResourcesWithFindings.
// ===========================================================================

func TestAttentionFilter_IncludesResourcesWithWave2OnlyFindings(t *testing.T) {
	c := openListController(t, "s3")

	// s3's Color always resolves Healthy regardless of Fields, so all three
	// resources below fail the Wave-1 IsIssue() gate — only the Wave-2
	// enrichment-map lookup can surface res-0 under the attention filter.
	resources := []resource.Resource{
		{ID: "res-0", Name: "bucket-alpha", Fields: map[string]string{"name": "bucket-alpha"}},
		{ID: "res-1", Name: "bucket-beta", Fields: map[string]string{"name": "bucket-beta"}},
		{ID: "res-2", Name: "bucket-gamma", Fields: map[string]string{"name": "bucket-gamma"}},
	}
	c.ApplyResourcesLoaded("s3", resources, nil, false)

	findings := map[string][]domain.Finding{
		"res-0": {{Code: "s3.public.access.enabled", Phrase: "public access enabled", Severity: domain.SevWarn, Source: "wave2:s3"}},
	}
	c.ApplyEnrichmentState("s3", 1, false, findings, nil)
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("attention filter with Wave-2-only finding: Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "res-0" {
		t.Errorf("attention filter must show res-0 (Wave-2 finding only, Wave-1 Color Healthy), got %q", lb.Rows[0].ResourceID)
	}
}

// TestAttentionFilter_ReappliesOnLateEnrichmentArrival ports
// qa_attention_filter_enrichment_test.go's TestAttentionFilter_
// SetEnrichmentState_ReappliesFilter: unlike the sibling test above (which
// applies enrichment BEFORE toggling attention), this activates the
// attention filter FIRST — mimicking a user pressing ctrl+z while Wave 2 is
// still in flight — then applies enrichment. Every render derives Body.List
// fresh from current filter+enrichment state (no cached filtered-rows
// snapshot to go stale), so this ordering is structurally safe by
// architecture; kept as a regression pin against exactly the historical bug
// (ResourceListModel.SetEnrichmentState not re-running applySortAndFilter).
func TestAttentionFilter_ReappliesOnLateEnrichmentArrival(t *testing.T) {
	c := openListController(t, "s3")

	resources := []resource.Resource{
		{ID: "b-0", Name: "bucket-alpha", Fields: map[string]string{"name": "bucket-alpha"}},
		{ID: "b-1", Name: "bucket-beta", Fields: map[string]string{"name": "bucket-beta"}},
		{ID: "b-2", Name: "bucket-gamma", Fields: map[string]string{"name": "bucket-gamma"}},
	}
	c.ApplyResourcesLoaded("s3", resources, nil, false)

	// Enable the attention filter BEFORE Wave 2 lands.
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	if got := len(c.Snapshot().Body.List.Rows); got != 0 {
		t.Fatalf("precondition: attention filter with no findings yet should hide all healthy rows, got %d visible", got)
	}

	findings := map[string][]domain.Finding{
		"b-0": {{Code: "s3.public.access.enabled", Phrase: "public access enabled", Severity: domain.SevBroken, Source: "wave2:s3"}},
	}
	c.ApplyEnrichmentState("s3", 1, false, findings, nil)

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("an already-active attention filter must immediately reflect a Wave-2 finding that lands afterward: got %d rows, want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "b-0" {
		t.Errorf("attention filter must show b-0 (newly Wave-2-flagged), got %q", lb.Rows[0].ResourceID)
	}
}

// ===========================================================================
// 5. handleResourcesLoadedEvent mismatched/alias ResourceType routing (round
//    4 unique-pin port) — a ResourcesLoaded event is applied only to a screen
//    whose canonicalized ResourceType (resource.FindResourceType(...).ShortName)
//    matches the message's own canonicalized ResourceType; a late fetch for a
//    different type must never populate the active list, and an alias on
//    either side (e.g. "rds" wire-stamped for canonical "dbi") must still
//    match. Ported from the deleted resourcelist_mismatched_type_test.go's
//    TestResourceListModel_ResourcesLoaded_DropsMismatchedType/
//    AppliesMatchingType — the alias-specific cases were not otherwise pinned
//    at the controller level (core/app/handle.go:187-230 documents the
//    canonicalization intent but had no positive test).
// ===========================================================================

func TestResourcesLoaded_DropsMismatchedType(t *testing.T) {
	cases := []struct {
		name          string
		listShortName string // active screen's resource type
		staleType     string // ResourceType on the mismatched message (alias or canonical)
	}{
		{"S3 list rejects EC2 rows", "s3", "ec2"},
		{"EC2 list rejects S3 rows", "ec2", "s3"},
		// "rds" is a registered alias for canonical ShortName "dbi".
		{"S3 list rejects RDS alias rows", "s3", "rds"},
		{"S3 list rejects RDS canonical rows", "s3", "dbi"},
		{"Lambda list rejects EC2 rows", "lambda", "ec2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := openListController(t, tc.listShortName)

			c.Handle(messages.ResourcesLoaded{
				ResourceType: tc.staleType,
				Resources: []resource.Resource{
					{ID: "x-0001", Name: "x-0001", Fields: map[string]string{"id": "x-0001"}},
				},
			})

			lb := c.Snapshot().Body.List
			if lb != nil && len(lb.Rows) != 0 {
				t.Errorf("mismatched ResourcesLoaded (%s -> %s) must not populate rows: got %d rows, want 0", tc.staleType, tc.listShortName, len(lb.Rows))
			}
		})
	}
}

func TestResourcesLoaded_AppliesMatchingType(t *testing.T) {
	cases := []struct {
		name          string
		listShortName string
		msgType       string // may be an alias
	}{
		{"S3 exact match", "s3", "s3"},
		{"EC2 exact match", "ec2", "ec2"},
		// The fetcher stamps "rds" (alias) on the wire while the list holds
		// canonical ShortName "dbi" — the alias-aware match must not drop it.
		{"RDS alias match (rds to dbi)", "dbi", "rds"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := openListController(t, tc.listShortName)

			c.Handle(messages.ResourcesLoaded{
				ResourceType: tc.msgType,
				Resources: []resource.Resource{
					{ID: "res-a", Name: "res-a", Fields: map[string]string{"id": "res-a"}},
					{ID: "res-b", Name: "res-b", Fields: map[string]string{"id": "res-b"}},
				},
			})

			lb := c.Snapshot().Body.List
			if lb == nil || len(lb.Rows) != 2 {
				got := 0
				if lb != nil {
					got = len(lb.Rows)
				}
				t.Errorf("matching-type ResourcesLoaded (%s -> %s) did not populate rows: got %d, want 2", tc.msgType, tc.listShortName, got)
			}
		})
	}
}

// ===========================================================================
// 6. RenderList narrow-screen column fit — port of tui_resourcelist_test.go's
// TestResourceList_NarrowScreen_ShowsAllColumns (a fixed bug: a narrow
// terminal used to DROP a wide column entirely instead of shrinking it).
// m.fitColumns (resourcelist.go), called from the live RenderList seam, is
// otherwise untested — the legacy pin only exercised it via the dead View().
// log_events's real catalog columns (Timestamp:22, Message:120) already
// exceed an 80-col terminal, so no synthetic type is needed.
// ===========================================================================

func TestRenderList_NarrowScreen_ShrinksWideColumnInsteadOfDropping(t *testing.T) {
	td := resource.GetChildType("log_events")
	if td == nil {
		t.Fatal("log_events child type not registered")
	}

	c := wave3ChildListController(t, "log_events")
	c.ApplyResourcesLoaded("log_events", []resource.Resource{
		{
			ID: "evt-1", Name: "test event",
			Fields: map[string]string{
				"timestamp": "2025-07-25 16:05",
				"message":   "Downloading snowflake_connector_python-3.2.1",
			},
		},
	}, nil, false)
	body := *c.Snapshot().Body.List

	m := views.NewResourceList(*td, nil, keys.Default())
	m.SetSize(80, 20) // narrow — 80 cols can't fit 22+120

	// This test does not call tuitest.NoColor(t), so it runs with the
	// package's TestMain "colors on" baseline — headers and cells are real
	// candidates for SGR-wrapped styling. Strip ANSI before substring
	// checks so a styling change can't split "Timestamp"/"Message"/
	// "Downloading"/"snowflake" mid-string and produce a false negative.
	out := stripAnsi(m.RenderList(body))
	if !strings.Contains(out, "Timestamp") {
		t.Errorf("Timestamp header should be visible on narrow screen:\n%s", out)
	}
	if !strings.Contains(out, "Message") {
		t.Errorf("Message header should be visible on narrow screen (shrunk to fit):\n%s", out)
	}
	if !strings.Contains(out, "Downloading") || !strings.Contains(out, "snowflake") {
		t.Errorf("Message content should be visible (truncated) on narrow screen:\n%s", out)
	}
}

// ===========================================================================
// 7. Controller.PatchListDisplayName / PatchListParentContext — port of
// child_view_resourcelist_test.go's NewChildResourceList constructor pins
// (ResourceType/FrameTitle/ParentContext on the dead ResourceListModel).
// NewResourceList's child-view constructor (resourcelist.go:120) calls these
// two Controller setters directly; GetListDisplayName/GetListParentContext
// had ZERO test callers anywhere before this port — the display-name (child
// breadcrumb, e.g. an S3 bucket name) and parent-context (feeds the
// related-panel ContextKeys lookup, docs/related-resources.md) wiring was
// otherwise completely unpinned at the live seam.
// ===========================================================================

// TestChildList_DisplayNameAndParentContext_SetByConstructorPath drives
// the actual production constructor, views.NewChildResourceList (which itself
// calls PatchListDisplayName/PatchListParentContext internally,
// resourcelist.go:120) — a direct c.PatchListDisplayName/PatchListParentContext
// call from the test bypasses that wiring entirely and would stay green even
// if the constructor stopped calling either setter.
func TestChildList_DisplayNameAndParentContext_SetByConstructorPath(t *testing.T) {
	c := wave3ChildListController(t, "s3_objects")
	td := resource.GetChildType("s3_objects")
	if td == nil {
		t.Fatal("s3_objects child resource type not registered")
	}

	views.NewChildResourceList(*td, map[string]string{"bucket": "test-bucket"}, "test-bucket", nil, keys.Default(), c)

	if got := c.GetListDisplayName(); got != "test-bucket" {
		t.Errorf("GetListDisplayName(): got %q, want %q", got, "test-bucket")
	}
	if got := c.GetListParentContext()["bucket"]; got != "test-bucket" {
		t.Errorf("GetListParentContext()[bucket]: got %q, want %q", got, "test-bucket")
	}
	if title := c.ListFrameTitle(); title != "test-bucket" {
		t.Errorf("ListFrameTitle() with a DisplayName set and no rows loaded: got %q, want %q", title, "test-bucket")
	}
}

// TestChildList_DisplayName_CombinesWithRowCount verifies the
// DisplayName+count FrameTitle format ("b1(2)") once rows are loaded — the
// display-name substitutes only the leading name, the count suffix behaves
// identically to a top-level list.
func TestChildList_DisplayName_CombinesWithRowCount(t *testing.T) {
	c := wave3ChildListController(t, "s3_objects")
	c.PatchListDisplayName("b1")
	c.ApplyResourcesLoaded("s3_objects", []resource.Resource{
		{ID: "file1.txt", Name: "file1.txt", Fields: map[string]string{"status": "file", "key": "file1.txt"}},
		{ID: "file2.txt", Name: "file2.txt", Fields: map[string]string{"status": "file", "key": "file2.txt"}},
	}, nil, false)

	if title := c.ListFrameTitle(); title != "b1(2)" {
		t.Errorf("ListFrameTitle() with DisplayName + 2 rows: got %q, want %q", title, "b1(2)")
	}
}

func TestChildList_ParentContext_EmptyForTopLevelList(t *testing.T) {
	c := openListController(t, "ec2")
	if got := c.GetListParentContext(); len(got) != 0 {
		t.Errorf("GetListParentContext() on a top-level (non-child) list: got %v, want empty", got)
	}
}

// ===========================================================================
// 8. ct-events default sort (event_time RFC3339, not the display "time"
// string) across a real month boundary — port of aws_ct_events_review_fixes_
// test.go's TestCTSort_RFC3339_AcrossMonthBoundary. ct-events's TIME column
// config sets SortKey:"event_time" (core/config/defaults_monitoring.go), and
// ensureListState seeds SortCol="event_time"/SortDir="desc" automatically
// (core/app/list_defaults_test.go's TestEnsureListState_SeedsCTEventsDefaultSort
// pins the SEEDED column/direction only, not that a real cross-month
// comparison actually resolves correctly) — this closes that gap on the live
// ApplyResourcesLoaded seam.
// ===========================================================================

func TestCTEventsSort_RFC3339_AcrossMonthBoundary(t *testing.T) {
	c := openListController(t, "ct-events")
	resources := []resource.Resource{
		{
			ID: "event-a", Name: "GetObject",
			Fields: map[string]string{"time": "Apr 02 10:00:00", "event_time": "2026-04-02T10:00:00Z", "status": "ct-info"},
		},
		{
			ID: "event-b", Name: "DescribeInstances",
			Fields: map[string]string{"time": "Mar 28 10:00:00", "event_time": "2026-03-28T10:00:00Z", "status": "ct-info"},
		},
		{
			ID: "event-c", Name: "PutObject",
			Fields: map[string]string{"time": "Apr 07 17:00:59", "event_time": "2026-04-07T17:00:59Z", "status": "ct-info"},
		},
	}
	c.ApplyResourcesLoaded("ct-events", resources, nil, false)

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(lb.Rows))
	}
	// Lexicographic display-string order would be B (Mar 28), C (Apr 07), A
	// (Apr 02) — 'M' > 'A' in ASCII, so "Mar 28" wrongly sorts newest. The
	// correct RFC3339 event_time DESC order is C, A, B (newest first).
	got := []string{lb.Rows[0].ResourceID, lb.Rows[1].ResourceID, lb.Rows[2].ResourceID}
	want := []string{"event-c", "event-a", "event-b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ct-events default sort must use event_time (RFC3339), not the display time string, across a month boundary; got order %v, want %v", got, want)
		}
	}
}

// ===========================================================================
// 9. buildListBody's enrichment-map-only status-cell override — port of
// wave2_risk_text_s4_s5_test.go's TestWave2_ListStatusColumn_
// ShowsConcretePhrase_ForIssueFinding. list_body.go's S4 fallback branch
// ("This override is only a fallback for the case where Wave-2 enrichment
// has landed in the enrichment-store map but has NOT yet been mutated onto
// r.Findings") is a DISTINCT code path from phase03_view_reads_test.go's
// TestViews_ListStatusColumn_Wave2OverridesLifecycle, which drives the
// resource's own r.Findings directly — this test drives the finding only
// through ApplyEnrichmentState's separate map, and had zero other coverage
// (hasWave2Finding/statusCol fallback never appeared in any test file).
// Reuses loadListController from phase03_view_reads_test.go (same package).
// ===========================================================================

func TestListStatusColumn_EnrichmentMapOnlyFinding_OverridesRawState(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "wave3-s4-status-test",
		Name:      "S4 Status Test",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 30},
		},
	}
	c := loadListController(t, td, []resource.Resource{
		{ID: "i-flagged-1", Name: "billing-db-01", Fields: map[string]string{"name": "billing-db-01", "state": "available"}},
	})

	findings := map[string][]domain.Finding{
		"i-flagged-1": {{Code: "dbi.no-backups", Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave2:dbi"}},
	}
	c.ApplyEnrichmentState(td.ShortName, 0, false, findings, nil)

	body := *c.Snapshot().Body.List
	if len(body.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(body.Rows))
	}
	joined := strings.Join(body.Rows[0].Cells, "|")
	if !strings.Contains(joined, "no automated backups") {
		t.Errorf("row must show the enrichment-map finding's Phrase in the status cell (r.Findings not yet mutated); got: %q", joined)
	}
	if strings.Contains(joined, "available") {
		t.Errorf("row must NOT still show the raw AWS state once an enrichment-map finding exists for it; got: %q", joined)
	}
}

// ===========================================================================
// 10. buildListFooterHints's "t" (CloudTrail) hint gate — port of
// ct_events_t_key_test.go's TestResourceList_TKey_NoHintOnCtEventsList /
// TestResourceList_TKey_SuppressedOnChildList (dead
// ResourceListModel.BottomHints()). Those fixtures used ct-events/s3_objects,
// neither of which sets CloudTrailKey, so they never actually exercised the
// "ls.ParentContext == nil" half of the guard (core/app/footer.go:72) — the
// hint was absent for the trivial reason (empty CloudTrailKey) in both
// cases. This port uses a synthetic type WITH CloudTrailKey set and checks
// both halves: the hint appears at top level and is suppressed once the
// screen carries a ParentContext, proving the ParentContext check (not just
// CloudTrailKey) drives the suppression.
// ===========================================================================

func hasTKeyHint(hints []app.KeyHint) bool {
	for _, h := range hints {
		if h.Key == "t" {
			return true
		}
	}
	return false
}

func TestListFooterHints_CloudTrailTKey_GatedByParentContext(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName:     "wave3-ct-hint-test",
		Name:          "CT Hint Test",
		CloudTrailKey: "ResourceName:ID",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
		},
	}

	t.Run("top-level list: CloudTrailKey set, no parent context → hint present", func(t *testing.T) {
		c := newTestController(t)
		c.RegisterFallbackTypeDef(td)
		c.PushChildListScreen(td.ShortName)

		footer := c.Snapshot().Footer
		if !hasTKeyHint(footer) {
			t.Errorf("footer = %+v, want a %q hint (CloudTrailKey set, ParentContext nil)", footer, "t")
		}
	})

	t.Run("child list: CloudTrailKey set but ParentContext non-nil → hint suppressed", func(t *testing.T) {
		c := newTestController(t)
		c.RegisterFallbackTypeDef(td)
		c.PushChildListScreen(td.ShortName)
		c.PatchListParentContext(map[string]string{"bucket": "my-bucket"})

		footer := c.Snapshot().Footer
		if hasTKeyHint(footer) {
			t.Errorf("footer = %+v, must NOT contain a %q hint once ParentContext is set (child list)", footer, "t")
		}
	})
}
