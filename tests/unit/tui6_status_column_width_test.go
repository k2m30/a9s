// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_status_column_width_test.go — the status column is as wide as the body
// says it is.
//
// Every other column on a list publishes its width on the built body, and the
// painter fills exactly that. The status column does not: the body publishes
// the type's declared width and the terminal renderer widens it again, from
// the row cells, on the way to the screen. That is the same shape the detail
// key width had — a layout decision taken where only one lane can see it — and
// it has the same two consequences. A lane with no renderer of its own reads a
// width the screen never uses, and a caller that sets the width cannot make it
// stick.
package unit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tui6LongStatus is a Wave-2 phrase wider than any status column a type
// declares, so the column has to grow for the whole phrase to be readable.
const tui6LongStatus = "a very long status phrase indeed"

// tui6StatusListBody opens an ec2 list with two rows and gives the second one
// a status phrase of statusText, returning the built body.
func tui6StatusListBody(t *testing.T, statusText string) (*app.ListBody, *app.Controller) {
	t.Helper()
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-0aaaaaaaaaaaaaaa1", Name: "web-a", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaaaaaaaaaaaaaa1", "name": "web-a", "state": "running"}},
		{ID: "i-0aaaaaaaaaaaaaaa2", Name: "web-b", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaaaaaaaaaaaaaa2", "name": "web-b", "state": "running"}},
	}, nil, false)
	c.ApplyEnrichmentState("ec2", 1, false, map[string][]domain.Finding{
		"i-0aaaaaaaaaaaaaaa2": {{
			Code: "ec2.public-ip", Phrase: statusText,
			Severity: domain.SevBroken, Source: "wave2",
		}},
	}, nil)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body")
	}
	if body.StatusCol < 0 {
		t.Fatal("ec2 has no status column, so these pins prove nothing")
	}
	return body, c
}

// tui6PaintedColumnWidth returns the width the rendered header gives the
// column titled title: the distance to the next column's header, less the two
// spaces between them.
func tui6PaintedColumnWidth(t *testing.T, screen, title, nextTitle string) int {
	t.Helper()
	header := ansi.Strip(strings.SplitN(screen, "\n", 2)[0])
	at := strings.Index(header, title)
	next := strings.Index(header, nextTitle)
	if at < 0 || next < 0 || next <= at {
		t.Fatalf("cannot locate %q and %q in the header: %q", title, nextTitle, header)
	}
	return next - at - 2
}

// tui6RenderList paints body at a terminal wide enough that no column is
// scrolled off.
func tui6RenderList(t *testing.T, body app.ListBody) string {
	t.Helper()
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 is not a registered resource type")
	}
	m := views.NewTransientResourceList(*td, 220, 30)
	return m.RenderList(body)
}

// TestStatusColumnWidth_IsAFactOfTheBuiltBody pins where the decision is made:
// the width that fits the widest status cell is on the body, like every other
// column's width, not discovered again by whichever lane happens to paint.
func TestStatusColumnWidth_IsAFactOfTheBuiltBody(t *testing.T) {
	// The guard reads the width the TYPE declares, not the width this body
	// published. Reading the published width would be the assertion's own
	// answer, and the pin would report "proves nothing" exactly when the
	// widening it exists for is working.
	declared := tui6DeclaredStatusWidth(t)
	body, _ := tui6StatusListBody(t, tui6LongStatus)

	var widest int
	for _, row := range body.Rows {
		if body.StatusCol < len(row.Cells) {
			widest = max(widest, text.Width(row.Cells[body.StatusCol]))
		}
	}
	if widest <= declared {
		t.Fatalf("no status cell (%d columns) exceeds the type's declared width (%d), so this pin proves nothing", widest, declared)
	}
	if got := body.Columns[body.StatusCol].Width; got != widest {
		t.Errorf("the body publishes a status column of %d columns while its own widest status cell needs %d; the screen shows %d, so the body's number describes nothing", got, widest, widest)
	}
}

// TestStatusColumnWidth_RendererReadsTheBodys pins that the painter consumes
// the decision instead of retaking it: a width the body sets below what the
// cells need is the caller's decision, and the screen must honour it the way
// it honours every other column's.
func TestStatusColumnWidth_RendererReadsTheBodys(t *testing.T) {
	body, _ := tui6StatusListBody(t, tui6LongStatus)
	const narrow = 18
	body.Columns[body.StatusCol].Width = narrow

	screen := tui6RenderList(t, *body)
	if got := tui6PaintedColumnWidth(t, screen, "2:Status", "3:Health"); got != narrow {
		t.Errorf("the body reserved %d columns for the status column, the screen paints %d — the width is being decided twice", narrow, got)
	}
}

// TestStatusColumnWidth_UnchangedWhenNoCellNeedsMore is the counterpart: a
// list whose status cells all fit keeps the type's declared width, so moving
// the decision must not widen every list by default.
func TestStatusColumnWidth_UnchangedWhenNoCellNeedsMore(t *testing.T) {
	declared := tui6DeclaredStatusWidth(t)
	body, _ := tui6StatusListBody(t, "stopped")

	if got := body.Columns[body.StatusCol].Width; got != declared {
		t.Errorf("the body widened the status column to %d with nothing in it needing more than the declared %d", got, declared)
	}
	screen := tui6RenderList(t, *body)
	if got := tui6PaintedColumnWidth(t, screen, "2:Status", "3:Health"); got != declared {
		t.Errorf("the screen paints a status column of %d, the body and the type both say %d", got, declared)
	}
}

// tui6DeclaredStatusWidth is the width the type asks for, read from a list
// whose rows carry no status text at all.
func tui6DeclaredStatusWidth(t *testing.T) int {
	t.Helper()
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{{
		ID: "i-0aaaaaaaaaaaaaaa1", Name: "web-a", Type: "ec2",
		Fields: map[string]string{"instance_id": "i-0aaaaaaaaaaaaaaa1", "name": "web-a"},
	}}, nil, false)
	body := c.Snapshot().Body.List
	if body == nil || body.StatusCol < 0 {
		t.Fatal("no list body with a status column")
	}
	return body.Columns[body.StatusCol].Width
}
