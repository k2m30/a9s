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
// The published offset is a BYTE offset into the line as it is painted, with
// its styling stripped: a display-column unit cannot be mapped back — a
// combining mark occupies no column, so two different positions in the text
// share one column, and a painter asked to highlight column 1 of "éx" covers
// the mark as well as the x, while a search for the mark itself highlights
// nothing at all. The painter needs the exact position.
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
	c := newTextScreenController(t, runtime.ScreenYAML, lines)
	c.Apply(app.Action{Kind: app.ActionSearch, Arg: query})
	body := c.Snapshot().Body.Text
	if body == nil {
		t.Fatal("no text body on the snapshot")
	}
	return body.SearchMatches
}

// TestSearchOffsets_AreByteOffsetsAfterADoubleWidthRune pins the unit: the
// exact position in the painted line. The three candidate numbers are spelled
// out in the failure message so a reader can see which one the offset is.
func TestSearchOffsets_AreByteOffsetsAfterADoubleWidthRune(t *testing.T) {
	prefix := tui6WideLine[:strings.Index(tui6WideLine, tui6WideQuery)]
	wantStart := len(prefix)
	wantEnd := wantStart + len(tui6WideQuery)

	colStart := text.Width(prefix)
	runeStart := len([]rune(prefix))
	if wantStart == colStart || wantStart == runeStart {
		t.Fatalf("the fixture line does not separate the three units (bytes=%d cols=%d runes=%d), so this pin proves nothing", wantStart, colStart, runeStart)
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
		t.Errorf("ColStart is %d; the match starts at byte %d of the painted line (display column %d, rune offset %d)", got.ColStart, wantStart, colStart, runeStart)
	}
	if got.ColEnd != wantEnd {
		t.Errorf("ColEnd is %d; the match ends at byte %d", got.ColEnd, wantEnd)
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
	for i, want := range []int{first, second} {
		if matches[i].ColStart != want {
			t.Errorf("match %d starts at %d, want byte %d of the painted line", i, matches[i].ColStart, want)
		}
	}
}

// TestSearchHighlight_LandsOnTheMatchAfterADoubleWidthRune pins the terminal
// lane: whatever unit the published offsets end up in, the painted highlight
// still covers exactly the matched text.
func TestSearchHighlight_LandsOnTheMatchAfterADoubleWidthRune(t *testing.T) {
	c := newTextScreenController(t, runtime.ScreenYAML, []string{tui6WideLine, "State: running"})
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
// row 2 on both text lanes: the match set is computed once, by the controller,
// and the terminal paints the set it is handed. A renderer that computes its
// own would find the second occurrence on this line, which the body
// deliberately does not carry.
func TestSearchHighlight_PaintsTheBodysMatchSetAndComputesNone(t *testing.T) {
	const line = "Tags: env=production, role=production"
	for _, lane := range []struct {
		name   string
		screen runtime.ScreenID
		render func(vp viewport.Model, body app.TextBody) string
	}{
		{"yaml", runtime.ScreenYAML, func(vp viewport.Model, body app.TextBody) string {
			m := views.NewTransientYAML(80, 6, vp)
			return m.RenderText(body)
		}},
		{"json", runtime.ScreenJSON, func(vp viewport.Model, body app.TextBody) string {
			m := views.NewTransientJSON(80, 6, vp)
			return m.RenderText(body)
		}},
	} {
		t.Run(lane.name, func(t *testing.T) {
			c := newTextScreenController(t, lane.screen, []string{line})
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

			out := lane.render(viewport.New(viewport.WithWidth(80), viewport.WithHeight(6)), *body)

			current := styles.SearchCurrentStyle.Render(tui6WideQuery)
			if n := strings.Count(out, current); n != 1 {
				t.Errorf("the body named 1 match; the screen paints %d highlighted runs of %q", n, tui6WideQuery)
			}
			if other := styles.SearchOtherStyle.Render(tui6WideQuery); strings.Contains(out, other) {
				t.Errorf("the screen highlights an occurrence the body did not name, so the renderer computed a match set of its own:\n%q", out)
			}
		})
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
	wantStart := len(prefix)

	matches := tui6TextMatches(t, []string{tui6FoldLine}, tui6WideQuery)
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d: %+v", len(matches), matches)
	}
	if matches[0].ColStart != wantStart {
		t.Errorf("ColStart is %d; the match starts at byte %d — the fold made the line one byte longer and every offset after it moved", matches[0].ColStart, wantStart)
	}
	if want := wantStart + len(tui6WideQuery); matches[0].ColEnd != want {
		t.Errorf("ColEnd is %d, want byte %d", matches[0].ColEnd, want)
	}
}

// TestSearchHighlight_LandsOnTheMatchAfterALengthChangingFold pins row 4 on
// the painted surface: the highlight covers the word, not the word shifted by
// the fold's byte difference.
func TestSearchHighlight_LandsOnTheMatchAfterALengthChangingFold(t *testing.T) {
	c := newTextScreenController(t, runtime.ScreenYAML, []string{tui6FoldLine})
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

// tui6MarkLine holds a combining acute after its base letter. The mark paints
// no column of its own, so two distinct positions in this line share one
// display column — which is why a column offset cannot be mapped back to a
// position, and why the published offset is a byte.
const tui6MarkLine = "Name: e\u0301x-production"

// TestSearchOffsets_ExactAroundACombiningMark pins the case the column unit
// could not express: a search for the letter after the mark, and a search for
// the mark itself.
func TestSearchOffsets_ExactAroundACombiningMark(t *testing.T) {
	t.Run("the letter after the mark", func(t *testing.T) {
		matches := tui6TextMatches(t, []string{tui6MarkLine}, "x")
		if len(matches) != 1 {
			t.Fatalf("want 1 match for %q, got %d: %+v", "x", len(matches), matches)
		}
		want := strings.Index(tui6MarkLine, "x")
		if matches[0].ColStart != want || matches[0].ColEnd != want+1 {
			t.Errorf("the match is published as [%d,%d); the letter sits at bytes [%d,%d) of the painted line, and a display column cannot name it — the mark before it shares that column", matches[0].ColStart, matches[0].ColEnd, want, want+1)
		}
	})

	t.Run("the mark itself", func(t *testing.T) {
		matches := tui6TextMatches(t, []string{tui6MarkLine}, "\u0301")
		if len(matches) != 1 {
			t.Fatalf("searching the combining mark found %d matches, want the 1 that is there: %+v", len(matches), matches)
		}
		want := strings.Index(tui6MarkLine, "\u0301")
		if matches[0].ColStart != want {
			t.Errorf("the mark is published at %d, want byte %d — it occupies no column, so a column offset has nowhere to put it", matches[0].ColStart, want)
		}
	})
}

// TestSearchHighlight_CoversExactlyTheMatchedBytes pins the painter across
// every shape that separates the units: a combining mark before the match, a
// combining mark inside it, a double-width pair, and a fold whose lowercase
// form is longer than what it folds.
func TestSearchHighlight_CoversExactlyTheMatchedBytes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		line  string
		query string
		wrong []string
	}{
		{"a combining mark before the match", tui6MarkLine, "x-production", []string{"\u0301x-production", "-production"}},
		{"a combining mark inside the match", "Name: production-e\u0301nv", "production-e\u0301nv", nil},
		{"a double-width pair before the match", tui6WideLine, tui6WideQuery, []string{"-production", "roduction"}},
		{"a fold that changes byte length", tui6FoldLine, tui6WideQuery, []string{"-productio", "roduction"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTextScreenController(t, runtime.ScreenYAML, []string{tc.line})
			c.Apply(app.Action{Kind: app.ActionSearch, Arg: tc.query})
			body := c.Snapshot().Body.Text
			if body == nil || len(body.SearchMatches) == 0 {
				t.Fatalf("no match published for %q in %q", tc.query, tc.line)
			}
			vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(6))
			m := views.NewTransientYAML(80, 6, vp)
			out := m.RenderText(*body)

			if !strings.Contains(out, styles.SearchCurrentStyle.Render(tc.query)) {
				t.Errorf("the highlight does not cover exactly %q\nrendered: %q", tc.query, out)
			}
			for _, wrong := range tc.wrong {
				if strings.Contains(out, styles.SearchCurrentStyle.Render(wrong)) {
					t.Errorf("the highlight covers %q instead of the match", wrong)
				}
			}
		})
	}
}

// TestSearchMatches_ComputedOncePerContentAndQuery pins the cost. A text
// screen is a document, and scanning it is not free: the match set was
// recomputed by the action that set the query, by every snapshot, and again by
// the renderer, so pressing n on a six-thousand-line document rescanned it
// three times to move a highlight down one line.
//
// The observable is the published set itself. Computing it again yields an
// equal slice in a new allocation; handing back the one already decided for
// this content and this query yields the same slice.
func TestSearchMatches_ComputedOncePerContentAndQuery(t *testing.T) {
	lines := make([]string, 0, 64)
	for i := range 64 {
		lines = append(lines, "Name: web-0"+string(rune('0'+i%10))+"-production")
	}
	c := newTextScreenController(t, runtime.ScreenYAML, lines)
	c.Apply(app.Action{Kind: app.ActionSearch, Arg: tui6WideQuery})

	first := c.Snapshot().Body.Text.SearchMatches
	if len(first) < 2 {
		t.Fatalf("the fixture must publish several matches, it published %d", len(first))
	}

	// A keypress that changes nothing about the content or the query, then two
	// navigations: three more snapshots, no new scan.
	same := c.Snapshot().Body.Text.SearchMatches
	c.Apply(app.Action{Kind: app.ActionSearchNext})
	afterNext := c.Snapshot().Body.Text.SearchMatches
	c.Apply(app.Action{Kind: app.ActionSearchPrev})
	afterPrev := c.Snapshot().Body.Text.SearchMatches

	for name, got := range map[string][]app.SearchMatch{
		"a second snapshot":  same,
		"the next match":     afterNext,
		"the previous match": afterPrev,
	} {
		if len(got) != len(first) {
			t.Fatalf("%s published %d matches, the first snapshot published %d", name, len(got), len(first))
		}
		if &got[0] != &first[0] {
			t.Errorf("%s rescanned the document: the match set is a fresh slice, not the one already decided for this content and this query", name)
		}
	}
}
