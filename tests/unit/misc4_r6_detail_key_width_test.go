// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r6_detail_key_width_test.go — misc4 row 6.
//
// The detail key column was sized from the widest field name alone, with no
// upper bound. A field name as wide as the terminal reserved the whole line
// for the label and pushed every value off the right edge, which is the one
// thing the reader opened the detail view for.

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// misc4RenderAt renders rows into a viewport of the given width.
func misc4RenderAt(width int, fields []app.FieldRow) string {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(20))
	m := views.NewTransientDetail(width, 20, vp)
	return m.RenderDetail(app.DetailBody{Fields: fields, FieldCursor: -1})
}

// TestDetailValueSurvivesAKeyAsWideAsTheViewport pins that a 120-column field
// name in a 120-column viewport still leaves the value on screen.
func TestDetailValueSurvivesAKeyAsWideAsTheViewport(t *testing.T) {
	const width = 120
	key := strings.Repeat("K", width)
	got := misc4RenderAt(width, []app.FieldRow{{Key: key, Value: "the-value"}})

	if !strings.Contains(got, "the-value") {
		t.Fatalf("the value was pushed off screen by a %d-column key:\n%s", width, got)
	}
	for _, line := range strings.Split(got, "\n") {
		if !strings.Contains(line, "the-value") {
			continue
		}
		if at := text.Width(line[:strings.Index(line, "the-value")]); at > width*2/5+1 {
			t.Errorf("value starts at column %d; the key column may take at most 40%% of %d",
				at, width)
		}
	}
}

// TestDetailKeyFloorHoldsWhereTheViewportAllows pins that capping the key
// column did not move the ordinary case: a short field name in a wide
// viewport still pads to the 22-column floor.
func TestDetailKeyFloorHoldsWhereTheViewportAllows(t *testing.T) {
	got := misc4RenderAt(120, []app.FieldRow{{Key: "Status", Value: "ok"}})
	if !strings.Contains(got, text.PadOrTrunc("Status:", 22)) {
		t.Errorf("the 22-column key floor no longer holds in a 120-column viewport:\n%s", got)
	}
}
