// SPDX-License-Identifier: GPL-3.0-or-later

// coverage_live_gaps_whitebox_test.go — white-box tests for live,
// production-reachable functions that carried low coverage: styleForCostCell,
// decodeRune, searchReadClipboard, enterChildFor, and the three
// refreshViewportContent methods (DetailModel/YAMLModel/JSONModel). All are
// unexported, so they are exercised directly from package views.
package views

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// styleForCostCell (costs.go)
// ---------------------------------------------------------------------------

func TestLiveGap_StyleForCostCell(t *testing.T) {
	cases := []struct {
		name     string
		cell     app.CostCell
		selected bool
		want     lipgloss.Style
	}{
		{"selected wins over every other flag", app.CostCell{Estimated: true, Negative: true, DeltaTag: "growth-strong"}, true, styles.RowSelected},
		{"estimated", app.CostCell{Estimated: true}, false, styles.DimText},
		{"negative", app.CostCell{Negative: true}, false, styles.StatusCheckOk},
		{"growth-soft", app.CostCell{DeltaTag: "growth-soft"}, false, styles.CostGrowthSoft},
		{"growth-strong", app.CostCell{DeltaTag: "growth-strong"}, false, styles.CostGrowthStrong},
		{"drop-soft", app.CostCell{DeltaTag: "drop-soft"}, false, styles.CostDropSoft},
		{"drop-strong", app.CostCell{DeltaTag: "drop-strong"}, false, styles.CostDropStrong},
		{"default (no flags)", app.CostCell{}, false, styles.RowNormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := styleForCostCell(tc.cell, tc.selected)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("styleForCostCell(%+v, selected=%v) = %#v, want %#v", tc.cell, tc.selected, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// decodeRune (search.go)
// ---------------------------------------------------------------------------

func TestLiveGap_DecodeRune(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantR    rune
		wantSize int
	}{
		{"empty input", "", 0, 0},
		{"ascii", "A", 'A', 1},
		{"two-byte utf8", "é", 'é', 2},
		{"three-byte utf8", "€", '€', 3},
		{"four-byte utf8", "😀", '😀', 4},
		{"truncated two-byte lead falls back to single byte", string([]byte{0xC3}), rune(0xC3), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, size := decodeRune(tc.in)
			if r != tc.wantR || size != tc.wantSize {
				t.Errorf("decodeRune(%q) = (%q, %d), want (%q, %d)", tc.in, r, size, tc.wantR, tc.wantSize)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// searchReadClipboard (search.go)
// ---------------------------------------------------------------------------

// TestLiveGap_SearchReadClipboard writes a known value to the host clipboard
// and reads it back exactly once via searchReadClipboard(), rather than
// calling clipboard.ReadAll() a second time to compute "want" — a second,
// separate read is flaky whenever anything else on the host (another test,
// a user, a background app) changes the clipboard between the two reads.
// Writing the fixture value first also removes the dependency on whatever
// pre-existing clipboard content happened to be present in this environment.
//
// searchReadClipboard (search.go) calls clipboard.ReadAll() directly with no
// injectable reader seam, so exercising it live is the only way to cover it
// at all — but doing so mutates whatever is actually in the host clipboard
// at the time the suite runs. Opt-in only (A9S_CLIPBOARD_INTEGRATION=1): the
// normal `make test`/`make test-race` run never touches the host clipboard,
// so a developer's real clipboard contents are never at risk of being
// permanently overwritten by CI/local test runs that crash before the
// t.Cleanup restore below fires.
func TestLiveGap_SearchReadClipboard(t *testing.T) {
	if os.Getenv("A9S_CLIPBOARD_INTEGRATION") != "1" {
		t.Skip("set A9S_CLIPBOARD_INTEGRATION=1 to run this host-clipboard-mutating test")
	}

	original, readErr := clipboard.ReadAll()
	if readErr != nil {
		t.Skip("cannot read and safely restore the system clipboard")
	}
	if err := clipboard.WriteAll("a9s-livegap-clipboard-fixture"); err != nil {
		t.Skip("no system clipboard available in this environment")
	}
	t.Cleanup(func() {
		_ = clipboard.WriteAll(original) //nolint:errcheck // best-effort restore of the host clipboard
	})

	msg := searchReadClipboard()
	pasted, ok := msg.(searchPasteMsg)
	if !ok {
		t.Fatalf("searchReadClipboard() = %#v (%T), want searchPasteMsg", msg, msg)
	}
	if string(pasted) != "a9s-livegap-clipboard-fixture" {
		t.Errorf("searchReadClipboard() = %q, want %q (the fixture value just written)", string(pasted), "a9s-livegap-clipboard-fixture")
	}
}

// ---------------------------------------------------------------------------
// enterChildFor (resourcelist_helpers.go)
// ---------------------------------------------------------------------------

func TestLiveGap_EnterChildFor_NoEnterChildRegistered_ReturnsNil(t *testing.T) {
	td := resource.ResourceTypeDef{ShortName: "livegap-nochild"}
	m := NewResourceList(td, nil, keys.Default())

	if got := m.enterChildFor(resource.Resource{ID: "x"}); got != nil {
		t.Errorf("enterChildFor() = %+v, want nil (no \"enter\" child registered)", got)
	}
}

func TestLiveGap_EnterChildFor_DrillConditionVetoes_ReturnsNil(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "livegap-vetoed",
		Children: []resource.ChildViewDef{
			{
				Key:       "enter",
				ChildType: "livegap-child",
				DrillCondition: func(r resource.Resource) bool {
					return r.Fields["state"] == "running"
				},
			},
		},
	}
	m := NewResourceList(td, nil, keys.Default())

	got := m.enterChildFor(resource.Resource{ID: "x", Fields: map[string]string{"state": "stopped"}})
	if got != nil {
		t.Errorf("enterChildFor() = %+v, want nil (DrillCondition vetoed the row)", got)
	}
}

func TestLiveGap_EnterChildFor_DrillConditionPasses_ReturnsChild(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "livegap-allowed",
		Children: []resource.ChildViewDef{
			{
				Key:       "enter",
				ChildType: "livegap-child",
				DrillCondition: func(r resource.Resource) bool {
					return r.Fields["state"] == "running"
				},
			},
		},
	}
	m := NewResourceList(td, nil, keys.Default())

	got := m.enterChildFor(resource.Resource{ID: "x", Fields: map[string]string{"state": "running"}})
	if got == nil || got.ChildType != "livegap-child" {
		t.Errorf("enterChildFor() = %+v, want a non-nil ChildViewDef with ChildType=%q", got, "livegap-child")
	}
}

func TestLiveGap_EnterChildFor_NoDrillCondition_AlwaysReturnsChild(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "livegap-nodrillcond",
		Children: []resource.ChildViewDef{
			{Key: "enter", ChildType: "livegap-child2"},
		},
	}
	m := NewResourceList(td, nil, keys.Default())

	got := m.enterChildFor(resource.Resource{ID: "x"})
	if got == nil || got.ChildType != "livegap-child2" {
		t.Errorf("enterChildFor() = %+v, want a non-nil ChildViewDef with ChildType=%q", got, "livegap-child2")
	}
}

// ---------------------------------------------------------------------------
// refreshViewportContent — DetailModel, YAMLModel, JSONModel
//
// All three share the same contract: when the search widget is active with a
// non-empty query, the re-rendered content is scanned for the first matching
// line and the viewport scrolls to it (GotoTop + SetYOffset(matchLine)). The
// expected line is computed the same way production does (ANSI-stripped
// content, first line containing the query) rather than hardcoded, so the
// assertion tracks the real render output instead of a guessed layout.
// ---------------------------------------------------------------------------

// livegapFindLine returns the 0-based index of the first line in content
// (after stripping ANSI) that contains substr, or -1 if none does.
func livegapFindLine(content, substr string) int {
	for i, l := range strings.Split(ansi.Strip(content), "\n") {
		if strings.Contains(l, substr) {
			return i
		}
	}
	return -1
}

// TestLiveGap_DetailModel_RenderDetail_SearchActiveScrollsToMatch drives the
// search-scroll-to-match behavior through the LIVE RenderDetail method — which
// calls the (live) DetailModel.refreshViewportContent internally — via a
// hand-built app.DetailBody + NewTransientDetail. This replaces the retired
// stateful NewDetail(...) lifecycle constructor (deleted in
// specs/022-codebase-cleanup/wave3), matching RenderDetail's own
// body.Search/body.SearchCursor handling (detail_helpers.go).
func TestLiveGap_DetailModel_RenderDetail_SearchActiveScrollsToMatch(t *testing.T) {
	// 3 fields sorted alphabetically by key so the marker lands on the
	// middle line (index 1) — a viewport height of 1 (< 3 total lines)
	// forces a real, non-clamped-to-zero scroll when the search jumps to it.
	const marker = "zzz-detail-marker-zzz"
	body := app.DetailBody{
		Fields: []app.FieldRow{
			{Key: "AFirst", Value: "first-field"},
			{Key: "MarkerField", Value: marker},
			{Key: "ZLast", Value: "last-field"},
		},
		KeyWidth: 12,
	}
	// wantLine is fixed by construction (Fields is a hand-built, explicitly
	// ordered slice, not map-iteration-derived) — the marker row is index 1,
	// non-zero so the assertion below proves real scrolling, not a
	// zero-clamped false pass.
	const wantLine = 1

	vp := viewport.New(viewport.WithWidth(100), viewport.WithHeight(1))
	m := NewTransientDetail(100, 1, vp)

	body.Search = marker
	m.RenderDetail(body)

	vpAfter := m.Viewport()
	if got := vpAfter.YOffset(); got != wantLine {
		t.Errorf("RenderDetail() with active search matching a field value: viewport YOffset = %d, want %d", got, wantLine)
	}
}

// TestLiveGap_DetailModel_RenderDetail_SearchActiveNoMatch_LeavesScrollAtZero
// drives the no-match case through the LIVE RenderDetail seam (which uses the
// live DetailModel.refreshViewportContent), replacing the retired stateful
// NewDetail(...) path.
func TestLiveGap_DetailModel_RenderDetail_SearchActiveNoMatch_LeavesScrollAtZero(t *testing.T) {
	body := app.DetailBody{
		Fields:   []app.FieldRow{{Key: "Other", Value: "value"}},
		Search:   "no-such-substring-in-this-resource",
		KeyWidth: 12,
	}
	vp := viewport.New(viewport.WithWidth(100), viewport.WithHeight(10))
	m := NewTransientDetail(100, 10, vp)
	m.RenderDetail(body)

	vpAfter := m.Viewport()
	if got := vpAfter.YOffset(); got != 0 {
		t.Errorf("RenderDetail() with a non-matching search query: viewport YOffset = %d, want 0 (no scroll)", got)
	}
}

func TestLiveGap_YAMLModel_RefreshViewportContent_SearchActiveScrollsToMatch(t *testing.T) {
	// Same non-zero-line + clamped-height setup as the DetailModel case above.
	const marker = "zzz-yaml-marker-zzz"
	res := resource.Resource{Fields: map[string]string{
		"AFirst": "first-field",
		"Marker": marker,
		"ZLast":  "last-field",
	}}
	m := NewYAMLWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(80, 1)

	wantLine := livegapFindLine(m.renderContent(), marker)
	if wantLine <= 0 {
		t.Fatalf("precondition: want marker on a non-zero line (to prove real scrolling, not a zero-clamped false pass), got line %d in:\n%s", wantLine, ansi.Strip(m.renderContent()))
	}

	m.search.Activate()
	m.search.SetQuery(marker)
	m.refreshViewportContent()

	vp := m.Viewport()
	if got := vp.YOffset(); got != wantLine {
		t.Errorf("refreshViewportContent() with active search: viewport YOffset = %d, want %d", got, wantLine)
	}
}

func TestLiveGap_JSONModel_RefreshViewportContent_SearchActiveScrollsToMatch(t *testing.T) {
	// 5 keys -> 7 JSON lines ("{", 5 entries, "}"); marker sorts to the
	// middle. Viewport height 3 (< 7 total lines) forces a real scroll.
	const marker = "zzz-json-marker-zzz"
	res := resource.Resource{Fields: map[string]string{
		"A":      "1",
		"B":      "2",
		"Marker": marker,
		"Y":      "3",
		"Z":      "4",
	}}
	m := NewJSONWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(80, 3)

	wantLine := livegapFindLine(m.renderContent(), marker)
	if wantLine <= 0 {
		t.Fatalf("precondition: want marker on a non-zero line (to prove real scrolling, not a zero-clamped false pass), got line %d in:\n%s", wantLine, ansi.Strip(m.renderContent()))
	}

	m.search.Activate()
	m.search.SetQuery(marker)
	m.refreshViewportContent()

	if got := m.viewport.YOffset(); got != wantLine {
		t.Errorf("refreshViewportContent() with active search: viewport YOffset = %d, want %d", got, wantLine)
	}
}
