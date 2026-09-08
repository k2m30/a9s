// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_key_column_width_test.go — the detail key column is measured, and
// there is one function that measures it.
//
// The width was counted in bytes, so a key with any multi-byte character
// reserved more of the row than it paints: a key of twelve CJK characters
// takes twenty-four terminal columns and was given thirty-seven, pushing every
// value on the screen a dozen columns to the right of where it belongs.
package unit

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// detailKeyFloor is the narrowest the key column ever gets, mirrored from the
// measure under test so the pin fails loudly if the floor is changed rather
// than quietly following it.
const detailKeyFloor = 22

// keyColumnStart renders the rows and reports the terminal column each value
// begins at, keyed by its key.
func keyColumnStart(t *testing.T, rows []app.FieldRow) map[string]int {
	t.Helper()
	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(20))
	m := views.NewTransientDetail(120, 20, vp)
	screen := ansi.Strip(m.RenderDetail(app.DetailBody{Fields: rows, FieldCursor: -1}))

	out := make(map[string]int, len(rows))
	for _, r := range rows {
		for _, line := range strings.Split(screen, "\n") {
			i := strings.Index(line, r.Key+":")
			if i < 0 {
				continue
			}
			v := strings.Index(line[i:], r.Value)
			if v < 0 {
				continue
			}
			out[r.Key] = text.Width(line[:i+v])
			break
		}
	}
	if len(out) != len(rows) {
		t.Fatalf("only %d of %d rows reached the screen:\n%s", len(out), len(rows), screen)
	}
	return out
}

// TestDetailKeyColumn_IsMeasuredNotCounted pins that the key column is as wide
// as the widest key paints, not as many bytes as it takes to store.
func TestDetailKeyColumn_IsMeasuredNotCounted(t *testing.T) {
	// Twelve double-width characters: 24 terminal columns, 36 bytes.
	const wideKey = "日本語のとても長い項目名前"

	rows := []app.FieldRow{
		{Key: wideKey, Value: "wide"},
		{Key: "Status", Value: "ok"},
	}
	starts := keyColumnStart(t, rows)

	// The column is sized by the widest key, so both values start together.
	if starts[wideKey] != starts["Status"] {
		t.Errorf("the two values start at columns %d and %d; every row shares one key column",
			starts[wideKey], starts["Status"])
	}
	// One column of indent, then the key column: the widest key plus its colon,
	// never narrower than the floor.
	want := 1 + max(detailKeyFloor, text.Width(wideKey)+1)
	if got := starts[wideKey]; got != want {
		t.Errorf("the value starts at column %d; the widest key paints %d columns, so the value belongs at %d",
			got, text.Width(wideKey), want)
	}
}

// TestDetailKeyColumn_ShortKeysKeepTheFloor pins the floor the surviving
// measure carries: a resource whose longest key is short still gets a key
// column wide enough that the values line up down the screen rather than
// crowding the left edge.
func TestDetailKeyColumn_ShortKeysKeepTheFloor(t *testing.T) {
	rows := []app.FieldRow{
		{Key: "ID", Value: "i-0abc123def4567890"},
		{Key: "AZ", Value: "eu-west-1a"},
	}
	starts := keyColumnStart(t, rows)

	if starts["ID"] != starts["AZ"] {
		t.Errorf("the two values start at columns %d and %d; every row shares one key column", starts["ID"], starts["AZ"])
	}
	if got, want := starts["ID"], 1+detailKeyFloor; got != want {
		t.Errorf("with a longest key of two characters the value starts at column %d, want the floored column %d", got, want)
	}
}
