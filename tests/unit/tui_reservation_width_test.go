// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui_reservation_width_test.go — a cell paints exactly the columns it reserved.
//
// Every fixed-width cell in the TUI is laid out by reserving w columns and
// handing the content to text.PadOrTrunc. When the content is wider than w and
// the cut lands in the middle of a double-width rune, the truncation used to
// come back one column short, so everything painted to the right of that cell
// slid left by one and the row stopped lining up with its header.
package unit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// visibleLines strips styling and splits, so a pin reads the columns a
// terminal would paint rather than the escape sequences around them.
func visibleLines(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

// TestPadOrTrunc_PaintsExactlyTheReservedWidth pins the measure both sides of
// a reservation share: whatever w a caller reserves, the returned cell occupies
// w terminal columns — padded, truncated, ASCII or double-width.
func TestPadOrTrunc_PaintsExactlyTheReservedWidth(t *testing.T) {
	cases := []struct {
		name string
		s    string
		w    int
	}{
		{"ascii shorter than the reservation", "ec2", 15},
		{"ascii exactly the reservation", "ec2", 3},
		{"ascii wider than the reservation", "elastic compute cloud", 9},
		{"wide runes cut between two of them", "日本語エイリアス名前", 15},
		{"wide runes cut through one of them", "日本語エイリアス名前", 14},
		{"wide runes cut through the first one", "日本語", 4},
		{"a single wide rune in an odd cell", "日本語エイ", 5},
		{"mixed ascii and wide runes", "ab日本語", 4},
		{"one wide rune wider than the whole cell", "日", 1},
		{"a tab inside a cell the content overflows", "ab\tcd", 4},
		{"a tab inside a cell with room to spare", "ab\tcd", 9},
		{"a vertical tab", "ab\vcd", 9},
		{"a form feed", "ab\fcd", 9},
		{"a bell", "ab\acd", 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := text.PadOrTrunc(tc.s, tc.w)
			if w := text.Width(got); w != tc.w {
				t.Errorf("PadOrTrunc(%q, %d) paints %d columns, not the %d it was asked to fill: %q",
					tc.s, tc.w, w, tc.w, got)
			}
		})
	}
}

// TestPadOrTrunc_NeverPanicsOnAnUnsplittableCluster pins the other side of the
// same reservation. A cell is filled by padding out to the reserved width, so
// content the cut cannot narrow to that width — a flag emoji, a family built
// out of joined runes, a mark that combines with the rune before it — would
// otherwise ask for a negative number of spaces and take the renderer down.
// Such a cluster paints at most one column past its cell; it is never trimmed
// into a broken half-glyph and never crashes.
func TestPadOrTrunc_NeverPanicsOnAnUnsplittableCluster(t *testing.T) {
	clusters := []string{
		"\U0001f1ef\U0001f1f5tokyo",
		"\U0001f468\u200d\U0001f469\u200d\U0001f467 family",
		"e\u0301\u0301\u0301 combining",
		"\u3030\ufe0f wavy",
		"a\ufe0f\U0001f1ef\U0001f1f5\U0001f467\u2192",
		"\U0001f534\U0001f534\U0001f534",
	}
	for _, s := range clusters {
		for w := 1; w <= 12; w++ {
			got := text.PadOrTrunc(s, w)
			if gw := text.Width(got); gw > w+1 {
				t.Errorf("PadOrTrunc(%q, %d) paints %d columns, more than one past its cell: %q", s, w, gw, got)
			}
		}
	}
}

// TestMainMenuRenderBody_WideAliasRowIsAsWideAsAnAsciiOne pins the menu: a type
// whose alias is written in double-width runes still ends its row where every
// other row ends, so the alias column stays a column. Both compared rows are
// unselected — the selected row is padded to the terminal width by its own
// highlight and would hide the difference.
func TestMainMenuRenderBody_WideAliasRowIsAsWideAsAnAsciiOne(t *testing.T) {
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 40)
	out := m.RenderBody(app.MenuBody{
		Selected: 0,
		Entries: []app.MenuEntry{
			{ShortName: "rds", Display: "RDS Instances", Alias: "rds"},
			{ShortName: "ec2", Display: "EC2 Instances", Alias: "ec2"},
			{ShortName: "s3", Display: "S3 Buckets", Alias: "a日本語エイリアス名前"},
		},
	})

	lines := visibleLines(out)
	if len(lines) != 3 {
		t.Fatalf("expected one line per menu entry, got %d:\n%q", len(lines), out)
	}
	if a, b := text.Width(lines[1]), text.Width(lines[2]); a != b {
		t.Errorf("the row with a double-width alias is %d columns wide and the ascii one is %d:\n%q\n%q", b, a, lines[1], lines[2])
	}
}

// TestRenderCosts_WideLabelRowStaysAlignedWithItsHeader pins the cost grid: a
// service label in double-width runes is cut to the label column's budget and
// the amounts to its right stay under the month they belong to.
func TestRenderCosts_WideLabelRowStaysAlignedWithItsHeader(t *testing.T) {
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	out := views.RenderCosts(app.CostsBody{
		Pivot:   "SERVICE",
		Columns: []app.CostColumn{{Label: "Jul"}, {Label: "Aug"}},
		Rows: []app.CostRow{
			{Label: "Elastic Compute Cloud", Cells: []app.CostCell{{Amount: "1.00"}, {Amount: "2.00"}}},
			{Label: "日本語のとても長いサービスの名前です", Cells: []app.CostCell{{Amount: "3.00"}, {Amount: "4.00"}}},
		},
		Totals: []app.CostCell{{Amount: "4.00"}, {Amount: "6.00"}},
	}, 81, 20)

	lines := visibleLines(out)
	if len(lines) < 4 {
		t.Fatalf("expected a header, two rows and a TOTAL, got %d lines:\n%q", len(lines), out)
	}
	// endColumn reports the terminal column the first occurrence of sub ends at,
	// measured rather than counted so a double-width label does not read as one
	// column per rune.
	endColumn := func(line, sub string) int {
		i := strings.Index(line, sub)
		if i < 0 {
			return -1
		}
		return text.Width(line[:i+len(sub)])
	}
	augEnd := endColumn(lines[0], "Aug")
	if augEnd < 0 {
		t.Fatalf("the header row does not name the Aug column:\n%q", lines[0])
	}
	for i, want := range map[int]string{1: "2.00", 2: "4.00", 3: "6.00"} {
		if got := endColumn(lines[i], want); got != augEnd {
			t.Errorf("row %d ends its Aug amount %q at column %d, the header ends Aug at column %d:\n%q\n%q",
				i, want, got, augEnd, lines[0], lines[i])
		}
	}
}

// TestHelpLegend_ColumnsAreMeasuredFromTheirContent pins the CloudTrail legend:
// every description starts at the same column as every other, whatever glyph
// or tier label is in front of it.
func TestHelpLegend_ColumnsAreMeasuredFromTheirContent(t *testing.T) {
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	h := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceList, "ct-events")
	h.SetSize(120, 40)
	lines := visibleLines(h.View())

	sameStart := func(t *testing.T, marker string, wants []string) {
		t.Helper()
		at := -1
		for i, l := range lines {
			if strings.HasSuffix(strings.TrimRight(l, " "), marker) {
				at = i
			}
		}
		if at < 0 {
			t.Fatalf("the legend has no %q heading:\n%s", marker, strings.Join(lines, "\n"))
		}
		col := -1
		for _, w := range wants {
			var line string
			for _, l := range lines[at:] {
				if strings.Contains(l, w) {
					line = l
					break
				}
			}
			if line == "" {
				t.Fatalf("the %q block has no row containing %q:\n%s", marker, w, strings.Join(lines, "\n"))
			}
			c := text.Width(line[:strings.Index(line, w)])
			if col < 0 {
				col = c
			} else if c != col {
				t.Errorf("under %s, %q starts at column %d while the first row's text starts at %d:\n%q", marker, w, c, col, line)
			}
		}
	}
	sameStart(t, "VERB GLYPHS", []string{"Read  (", "Write (", "Destructive ("})
	sameStart(t, "SEVERITY TIERS", []string{"routine reads", "worth a glance", "worth investigating"})
}
