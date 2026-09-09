// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_search_offset_unit_test.go — a search offset says what unit it is in.
//
// A text screen publishes each match as ColStart/ColEnd on app.SearchMatch,
// and serialises them as col_start/col_end. A reader that believes the name
// and places its highlight at that column lands somewhere else the moment the
// line holds a rune wider or longer than one column: a Japanese resource name
// costs three bytes and paints two columns, so the byte offset, the rune
// offset and the column offset are three different numbers on the same line.
//
// These pins say the published offsets are display columns, matching what
// they are called, and that the terminal's own highlight still lands on the
// match text after the conversion — the terminal computes its highlight from
// the query rather than from these offsets, so a change here must not be
// allowed to quietly move it.
package unit

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tui6WideLine is a YAML line as the detail-to-YAML view produces it for a
// resource named in Japanese. "東京" is two runes, six bytes and four columns,
// so every candidate unit gives a different offset for the match after it.
const tui6WideLine = "Name: 東京-web-01-production"
const tui6WideQuery = "production"

// tui6TextMatches opens a YAML screen over lines, searches for query and
// returns the published matches.
func tui6TextMatches(t *testing.T, lines []string, query string) []app.SearchMatch {
	t.Helper()
	c := newTextScreenController(runtime.ScreenYAML, lines)
	c.Apply(app.Action{Kind: app.ActionSearch, Arg: query})
	body := c.Snapshot().Body.Text
	if body == nil {
		t.Fatal("no text body on the snapshot")
	}
	return body.SearchMatches
}

// TestSearchOffsets_AreDisplayColumnsAfterADoubleWidthRune pins the unit the
// field name claims. The three candidate numbers are spelled out in the
// failure message so a reader can see which one the offset actually is.
func TestSearchOffsets_AreDisplayColumnsAfterADoubleWidthRune(t *testing.T) {
	prefix := tui6WideLine[:strings.Index(tui6WideLine, tui6WideQuery)]
	wantStart := text.Width(prefix)
	wantEnd := wantStart + text.Width(tui6WideQuery)

	byteStart := len(prefix)
	runeStart := len([]rune(prefix))
	if wantStart == byteStart || wantStart == runeStart {
		t.Fatalf("the fixture line does not separate the three units (cols=%d bytes=%d runes=%d), so this pin proves nothing", wantStart, byteStart, runeStart)
	}

	matches := tui6TextMatches(t, []string{tui6WideLine, "State: running"}, tui6WideQuery)
	if len(matches) != 1 {
		t.Fatalf("want 1 match for %q, got %d: %+v", tui6WideQuery, len(matches), matches)
	}
	got := matches[0]
	if got.Line != 0 {
		t.Errorf("match is on line %d, want 0", got.Line)
	}
	if got.ColStart != wantStart {
		t.Errorf("ColStart is %d; the match starts at display column %d (byte offset %d, rune offset %d)", got.ColStart, wantStart, byteStart, runeStart)
	}
	if got.ColEnd != wantEnd {
		t.Errorf("ColEnd is %d; the match ends at display column %d", got.ColEnd, wantEnd)
	}
}

// TestSearchOffsets_UnchangedOnAPlainASCIILine is the counterpart: on a line
// where all three units agree, the published offsets are the same numbers they
// have always been. A conversion that shifted every match by a constant would
// fail here.
func TestSearchOffsets_UnchangedOnAPlainASCIILine(t *testing.T) {
	const line = "Name: web-01-production"
	matches := tui6TextMatches(t, []string{line}, tui6WideQuery)
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d: %+v", len(matches), matches)
	}
	wantStart := strings.Index(line, tui6WideQuery)
	if matches[0].ColStart != wantStart || matches[0].ColEnd != wantStart+len(tui6WideQuery) {
		t.Errorf("ASCII match published as [%d,%d), want [%d,%d)", matches[0].ColStart, matches[0].ColEnd, wantStart, wantStart+len(tui6WideQuery))
	}
}

// TestSearchOffsets_EveryMatchOnALineIsPublished pins the second and third
// occurrence too: a per-match conversion applied only to the first one would
// leave the rest in the old unit.
func TestSearchOffsets_EveryMatchOnALineIsPublished(t *testing.T) {
	const line = "Tags: 東京=production, env=production"
	matches := tui6TextMatches(t, []string{line}, tui6WideQuery)
	if len(matches) != 2 {
		t.Fatalf("want 2 matches, got %d: %+v", len(matches), matches)
	}
	first := strings.Index(line, tui6WideQuery)
	second := strings.Index(line[first+len(tui6WideQuery):], tui6WideQuery) + first + len(tui6WideQuery)
	for i, want := range []int{text.Width(line[:first]), text.Width(line[:second])} {
		if matches[i].ColStart != want {
			t.Errorf("match %d starts at %d, want display column %d", i, matches[i].ColStart, want)
		}
	}
}

// TestSearchHighlight_LandsOnTheMatchAfterADoubleWidthRune pins the terminal
// lane: whatever unit the published offsets end up in, the painted highlight
// still covers exactly the matched text.
func TestSearchHighlight_LandsOnTheMatchAfterADoubleWidthRune(t *testing.T) {
	c := newTextScreenController(runtime.ScreenYAML, []string{tui6WideLine, "State: running"})
	c.Apply(app.Action{Kind: app.ActionSearch, Arg: tui6WideQuery})
	body := c.Snapshot().Body.Text
	if body == nil {
		t.Fatal("no text body on the snapshot")
	}

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	m := views.NewTransientYAML(80, 10, vp)
	out := m.RenderText(*body)

	want := styles.SearchCurrentStyle.Render(tui6WideQuery)
	if !strings.Contains(out, want) {
		t.Errorf("the highlight does not cover %q on a line with a double-width rune\nrendered: %q", tui6WideQuery, out)
	}
	for _, wrong := range []string{"-production", "roduction", "production,"} {
		if strings.Contains(out, styles.SearchCurrentStyle.Render(wrong)) {
			t.Errorf("the highlight covers %q instead of the match", wrong)
		}
	}
}

// TestSearchHighlight_PaintsTheBodysMatchSetAndComputesNone pins the shape of
// row 2: the match set is computed once, by the controller, and the terminal
// paints the set it is handed. A renderer that computes its own would find the
// second occurrence on this line, which the body deliberately does not carry.
func TestSearchHighlight_PaintsTheBodysMatchSetAndComputesNone(t *testing.T) {
	const line = "Tags: env=production, role=production"
	c := newTextScreenController(runtime.ScreenYAML, []string{line})
	c.Apply(app.Action{Kind: app.ActionSearch, Arg: tui6WideQuery})
	body := c.Snapshot().Body.Text
	if body == nil {
		t.Fatal("no text body on the snapshot")
	}
	if len(body.SearchMatches) != 2 {
		t.Fatalf("the fixture line must hold 2 occurrences, the body published %d", len(body.SearchMatches))
	}

	// Hand the renderer a body that names only the first occurrence.
	body.SearchMatches = body.SearchMatches[:1]

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(6))
	m := views.NewTransientYAML(80, 6, vp)
	out := m.RenderText(*body)

	current := styles.SearchCurrentStyle.Render(tui6WideQuery)
	if n := strings.Count(out, current); n != 1 {
		t.Errorf("the body named 1 match; the screen paints %d highlighted runs of %q", n, tui6WideQuery)
	}
	if other := styles.SearchOtherStyle.Render(tui6WideQuery); strings.Contains(out, other) {
		t.Errorf("the screen highlights an occurrence the body did not name, so the renderer computed a match set of its own:\n%q", out)
	}
}

// tui6FoldLine holds a rune whose lowercase form is a different length in
// bytes: "İ" is two bytes and lowercases to three. Matching that folds the
// line before searching it, and then reads the offset back against the
// unfolded line, shifts every later match by the difference.
const tui6FoldLine = "Name: İstanbul-production"

// TestSearchOffsets_UnshiftedByALengthChangingFold pins row 4 on the published
// offsets: the match is where the operator sees it, not where it sits in a
// folded copy of the line nobody paints.
func TestSearchOffsets_UnshiftedByALengthChangingFold(t *testing.T) {
	if len(strings.ToLower(tui6FoldLine)) == len(tui6FoldLine) {
		t.Fatal("the fixture line's fold does not change its byte length, so this pin proves nothing")
	}
	prefix := tui6FoldLine[:strings.Index(tui6FoldLine, tui6WideQuery)]
	wantStart := text.Width(prefix)

	matches := tui6TextMatches(t, []string{tui6FoldLine}, tui6WideQuery)
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d: %+v", len(matches), matches)
	}
	if matches[0].ColStart != wantStart {
		t.Errorf("ColStart is %d; the match starts at display column %d — the fold made the line one byte longer and every offset after it moved", matches[0].ColStart, wantStart)
	}
	if want := wantStart + text.Width(tui6WideQuery); matches[0].ColEnd != want {
		t.Errorf("ColEnd is %d, want display column %d", matches[0].ColEnd, want)
	}
}

// TestSearchHighlight_LandsOnTheMatchAfterALengthChangingFold pins row 4 on
// the painted surface: the highlight covers the word, not the word shifted by
// the fold's byte difference.
func TestSearchHighlight_LandsOnTheMatchAfterALengthChangingFold(t *testing.T) {
	c := newTextScreenController(runtime.ScreenYAML, []string{tui6FoldLine})
	c.Apply(app.Action{Kind: app.ActionSearch, Arg: tui6WideQuery})
	body := c.Snapshot().Body.Text
	if body == nil {
		t.Fatal("no text body on the snapshot")
	}
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(6))
	m := views.NewTransientYAML(80, 6, vp)
	out := m.RenderText(*body)

	if !strings.Contains(out, styles.SearchCurrentStyle.Render(tui6WideQuery)) {
		t.Errorf("the highlight does not cover %q\nrendered: %q", tui6WideQuery, out)
	}
	for _, wrong := range []string{"-productio", "roduction", "n-producti"} {
		if strings.Contains(out, styles.SearchCurrentStyle.Render(wrong)) {
			t.Errorf("the highlight covers %q instead of the match", wrong)
		}
	}
}
