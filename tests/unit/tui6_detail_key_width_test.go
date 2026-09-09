// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_detail_key_width_test.go — the detail key column is one decision.
//
// The width of the key column decides where every value on a detail screen
// starts. It was decided in the terminal renderer, from the renderer's own
// full width, on every frame — while the body build, which knows the viewport
// the fields are actually laid out in, published no width at all. Two lanes
// reading the same body therefore indent it differently, and the lane with no
// renderer of its own has nothing to read.
//
// These pins say the width is a field of the built body, computed once from
// the viewport the body was built for, measured in terminal columns; that the
// terminal renderer reads that field rather than recomputing one; and that the
// serialised body carries the same number, so the second lane is reading the
// decision instead of guessing at it.
package unit

import (
	"encoding/json"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tui6DetailBodyAt opens a detail screen on a resource carrying fields and
// returns the body built for a viewport of viewportW columns.
func tui6DetailBodyAt(t *testing.T, fields map[string]string, viewportW int) *app.DetailBody {
	t.Helper()
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(resource.Resource{
		ID: "i-0123456789abcdef0", Name: "web-prod", Type: "ec2", Fields: fields,
	}, "ec2")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(viewportW)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body on the snapshot")
	}
	return body
}

// TestDetailKeyWidth_BodyCarriesTheFloor pins the narrow end: a resource whose
// field names are all short still reserves the standing minimum, so values
// start in the same place from one screen to the next.
func TestDetailKeyWidth_BodyCarriesTheFloor(t *testing.T) {
	body := tui6DetailBodyAt(t, map[string]string{
		"instance_id": "i-0123456789abcdef0",
		"state":       "running",
	}, 200)
	if body.KeyWidth != 22 {
		t.Errorf("KeyWidth is %d, want the 22-column floor for a resource with only short field names", body.KeyWidth)
	}
}

// TestDetailKeyWidth_BoundedByTheViewportTheBodyWasBuiltFor pins the wide end
// against the width the fields are laid out in — the viewport the controller
// was told about, not the whole terminal. A field name wider than the bound
// must not reserve the line and push the value off the right edge.
func TestDetailKeyWidth_BoundedByTheViewportTheBodyWasBuiltFor(t *testing.T) {
	body := tui6DetailBodyAt(t, map[string]string{
		"instance_id": "i-0123456789abcdef0",
		"a_very_long_field_name_that_exceeds_the_floor_easily": "v",
	}, 87)
	if body.KeyWidth != 34 {
		t.Errorf("KeyWidth is %d, want 34 — two fifths of the 87-column viewport the body was built for", body.KeyWidth)
	}
}

// TestDetailKeyWidth_MeasuredInColumnsNotRunes pins the measure. A Japanese
// field name costs two columns per rune, and the padding under the key fills
// columns, so a rune-count measure leaves the values on that row starting
// short of every other row's.
func TestDetailKeyWidth_MeasuredInColumnsNotRunes(t *testing.T) {
	const wideKey = "東京リージョンの名前です" // 12 runes, 24 columns
	body := tui6DetailBodyAt(t, map[string]string{
		"instance_id": "i-0123456789abcdef0",
		wideKey:       "tokyo",
	}, 200)
	if body.KeyWidth != 25 {
		t.Errorf("KeyWidth is %d, want 25 — the 24 columns %q paints plus its colon; a rune count would have given 13 and left the floor of 22", body.KeyWidth, wideKey)
	}
}

// TestDetailKeyWidth_RendererReadsTheBodysWidth pins the renderer to the
// body's decision. The width handed in is one no renderer would arrive at on
// its own from this field set, so a renderer still computing its own fails.
func TestDetailKeyWidth_RendererReadsTheBodysWidth(t *testing.T) {
	body := tui6DetailBodyAt(t, map[string]string{
		"instance_id":  "i-0123456789abcdef0",
		"marker_field": "MARKERVALUE",
	}, 87)
	body.KeyWidth = 30

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(40))
	m := views.NewTransientDetail(120, 40, vp)
	out := ansi.Strip(m.RenderDetail(*body))

	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "MARKERVALUE") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("the marker row never reached the screen:\n%s", out)
	}
	if !strings.HasPrefix(strings.TrimLeft(line, " "), "marker_field:") {
		t.Fatalf("the marker row is not the key/value row this pin measures: %q", line)
	}
	// One leading space, then the key padded to the body's width, then the value.
	if got := strings.Index(line, "MARKERVALUE"); got != 1+body.KeyWidth {
		t.Errorf("the value starts at column %d; the body reserved %d columns for the key, so it must start at %d\nline: %q", got, body.KeyWidth, 1+body.KeyWidth, line)
	}
}

// TestDetailKeyWidth_SerialisedBodyCarriesTheSameValue pins the second lane:
// the number it reads off the body is the number the body decided, under the
// name the body publishes it by.
func TestDetailKeyWidth_SerialisedBodyCarriesTheSameValue(t *testing.T) {
	body := tui6DetailBodyAt(t, map[string]string{
		"instance_id": "i-0123456789abcdef0",
		"a_very_long_field_name_that_exceeds_the_floor_easily": "v",
	}, 87)
	if body.KeyWidth == 0 {
		t.Fatal("the body decided no key width, so this pin proves nothing")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling the detail body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding the serialised body: %v", err)
	}
	got, ok := decoded["key_width"]
	if !ok {
		t.Fatalf("the serialised body publishes no key_width, so the second lane has nothing to read: %s", raw)
	}
	if n, isNum := got.(float64); !isNum || int(n) != body.KeyWidth {
		t.Errorf("the serialised body says key_width=%v, the built body decided %d", got, body.KeyWidth)
	}
}
