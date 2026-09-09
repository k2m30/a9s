// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_hostile_aws_strings_test.go — an AWS-supplied string cannot steer the
// terminal.
//
// A tag value, a description, a CloudTrail user agent and a resource name are
// all attacker-influenced: anyone who can tag an instance can put an ESC and a
// CSI payload in the value a9s paints. The painter must keep ESC, because that
// is how a9s's own styling reaches the screen, so the control bytes have to be
// gone before the string is a value the app owns. These pins say that the
// strings the app hands to any surface — a list cell, a detail row, an
// attention row, the filter, the clipboard — carry no control byte at all, on
// every lane a string can enter by: a loaded page of rows, an enricher result,
// a detail opened directly, and a value read back off the SDK struct.
//
// The last pin is the counterpart: a9s's own SGR sequences still survive the
// painter, so sanitising the data must not be done by stripping ESC from what
// the painter produces.
package unit

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// tui6HostileTag is a tag value AWS would accept and hand back verbatim: an
// ESC opening an SGR sequence that is never closed, and a BEL. Painted raw it
// turns the rest of the row red and costs the cell a column.
const tui6HostileTag = "web\x1b[31m-prod\x07"

// tui6PrintableOf is what the operator must still be able to read once the
// controls are gone, whichever way the boundary removes them.
const tui6PrintableHead = "web"
const tui6PrintableTail = "-prod"

// tui6Controls returns the control runes in s: every C0 including ESC, DEL,
// and the C1 block. None of them may reach a surface.
func tui6Controls(s string) []rune {
	var out []rune
	for _, r := range s {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			out = append(out, r)
		}
	}
	return out
}

// tui6AssertInert fails when s still carries a control byte, or when the CSI
// payload survived as literal text (which is what dropping the ESC byte alone
// would leave), or when the printable words the operator reads are gone.
func tui6AssertInert(t *testing.T, where, s string) {
	t.Helper()
	if ctrls := tui6Controls(s); len(ctrls) > 0 {
		t.Errorf("%s carries control runes %q: %q", where, ctrls, s)
	}
	if strings.Contains(s, "[31m") {
		t.Errorf("%s kept the CSI payload as literal text: %q", where, s)
	}
	if !strings.Contains(s, tui6PrintableHead) || !strings.Contains(s, tui6PrintableTail) {
		t.Errorf("%s lost the readable part of the value: %q", where, s)
	}
}

// tui6HostileRows is a loaded page carrying one hostile instance and one
// healthy twin. The twin is what the hostile row must end up costing the same
// number of columns as.
func tui6HostileRows() []resource.Resource {
	return []resource.Resource{
		{ID: "i-0123456789abcdef0", Name: tui6HostileTag, Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"name":        tui6HostileTag,
				"state":       "running",
			}},
		{ID: "i-0aaaaaaaaaaaaaaa1", Name: "web-prod", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0aaaaaaaaaaaaaaa1",
				"name":        "web-prod",
				"state":       "running",
			}},
	}
}

// TestHostileAWSString_ListCellIsInert pins the fetch-result lane: a page of
// rows handed to the controller reaches the list body with the controls
// already gone, so no renderer has to know the value was hostile.
func TestHostileAWSString_ListCellIsInert(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", tui6HostileRows(), nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body after a page of rows landed")
	}
	if len(body.Rows) != 2 {
		t.Fatalf("want the 2 loaded rows, got %d", len(body.Rows))
	}
	for _, cell := range body.Rows[0].Cells {
		if ctrls := tui6Controls(cell); len(ctrls) > 0 {
			t.Errorf("a list cell carries control runes %q: %q", ctrls, cell)
		}
	}
	tui6AssertInert(t, "the identity cell", body.Rows[0].Cells[body.IdentityCol])
}

// TestHostileAWSString_RenderedRowKeepsItsWidth pins the consequence the
// operator sees: the hostile row and its healthy twin paint the same number of
// columns. An unpainted ESC sequence in the cell makes the row longer than the
// header it sits under and slides every column right of it.
func TestHostileAWSString_RenderedRowKeepsItsWidth(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", tui6HostileRows(), nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body after a page of rows landed")
	}
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 is not a registered resource type")
	}
	m := views.NewTransientResourceList(*td, 160, 30)
	lines := strings.Split(m.RenderList(*body), "\n")
	var hostile, healthy string
	for _, line := range lines {
		switch {
		case strings.Contains(ansi.Strip(line), "i-0123456789abcdef0"):
			hostile = line
		case strings.Contains(ansi.Strip(line), "i-0aaaaaaaaaaaaaaa1"):
			healthy = line
		}
	}
	if hostile == "" || healthy == "" {
		t.Fatalf("both rows must be on screen; hostile=%q healthy=%q", hostile, healthy)
	}
	if hw, cw := text.Width(hostile), text.Width(healthy); hw != cw {
		t.Errorf("the hostile row paints %d columns, its healthy twin %d — the columns to its right no longer line up\nhostile: %q", hw, cw, hostile)
	}
}

// TestHostileAWSString_FilterMatchesWhatTheCellShows pins the filter: the
// operator types what is on the screen. While the control bytes are still in
// the value, the word the terminal paints as one run matches nothing.
func TestHostileAWSString_FilterMatchesWhatTheCellShows(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", tui6HostileRows(), nil, false)
	shown := c.Snapshot().Body.List.Rows[0].Cells[c.Snapshot().Body.List.IdentityCol]
	tui6AssertInert(t, "the identity cell the filter is typed against", shown)

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: strings.TrimSpace(shown)})
	got := c.Snapshot().Body.List
	if len(got.Rows) != 1 {
		t.Errorf("filtering by the cell's own text %q kept %d rows, want the 1 row that shows it", shown, len(got.Rows))
	}

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "\x1b[31m"})
	if n := len(c.Snapshot().Body.List.Rows); n != 0 {
		t.Errorf("filtering by the escape sequence kept %d rows; no cell shows it", n)
	}
}

// tui6HostileDetail opens a detail screen on a resource whose name, finding
// phrase, finding detail and attention row all carry the hostile value — the
// enricher lane and the fetch lane land on the same rows.
func tui6HostileDetail(t *testing.T) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	res := resource.Resource{
		ID: "i-0123456789abcdef0", Name: tui6HostileTag, Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0123456789abcdef0",
			"name":        tui6HostileTag,
			"description": "owner " + tui6HostileTag,
		},
		Findings: []domain.Finding{{
			Code:     "ec2.public-ip",
			Phrase:   "tagged " + tui6HostileTag,
			Detail:   "The instance is tagged " + tui6HostileTag + ".",
			Severity: domain.SevBroken,
			Source:   "wave1",
		}},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			"ec2.public-ip": {Rows: []domain.DetailRow{{Label: "Name tag", Value: tui6HostileTag}}},
		},
	}
	c.EnsureDetailState(res, "ec2")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(120)
	return c
}

// TestHostileAWSString_DetailRowsAreInert pins the detail lane, including the
// attention rows an enricher writes: no key and no value on the built body
// carries a control byte.
func TestHostileAWSString_DetailRowsAreInert(t *testing.T) {
	c := tui6HostileDetail(t)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body")
	}
	var sawValue bool
	for i, f := range body.Fields {
		if ctrls := tui6Controls(f.Key); len(ctrls) > 0 {
			t.Errorf("detail row %d key carries control runes %q: %q", i, ctrls, f.Key)
		}
		if ctrls := tui6Controls(f.Value); len(ctrls) > 0 {
			t.Errorf("detail row %d value carries control runes %q: %q", i, ctrls, f.Value)
		}
		if strings.Contains(f.Value, tui6PrintableTail) {
			sawValue = true
		}
	}
	if !sawValue {
		t.Fatalf("the hostile value reached no detail row, so this pin proves nothing; rows: %d", len(body.Fields))
	}
}

// TestHostileAWSString_CopyIsInert pins the clipboard: whatever row the cursor
// rests on, what leaves the app for the operator's shell is text, not a
// terminal command.
func TestHostileAWSString_CopyIsInert(t *testing.T) {
	c := tui6HostileDetail(t)
	rows := len(c.Snapshot().Body.Detail.Fields)
	if rows == 0 {
		t.Fatal("no detail rows to walk")
	}
	var sawValue bool
	for i := range rows {
		content, label := c.CopyContent()
		if ctrls := tui6Controls(content); len(ctrls) > 0 {
			t.Errorf("copy at cursor row %d carries control runes %q: %q", i, ctrls, content)
		}
		if ctrls := tui6Controls(label); len(ctrls) > 0 {
			t.Errorf("copy label at cursor row %d carries control runes %q: %q", i, ctrls, label)
		}
		if strings.Contains(content, tui6PrintableTail) {
			sawValue = true
		}
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	if !sawValue {
		t.Fatal("no cursor row copied the hostile value, so this pin proves nothing")
	}
}

// TestHostileAWSString_EnricherPhraseIsInert pins the second writer the row
// says must pass the boundary: an enricher result. Its phrase becomes the
// row's Status cell, so a hostile phrase steers the terminal from a lane the
// loaded page never touches.
func TestHostileAWSString_EnricherPhraseIsInert(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{{
		ID: "i-0123456789abcdef0", Name: "web-prod", Type: "ec2",
		Fields: map[string]string{"instance_id": "i-0123456789abcdef0", "name": "web-prod", "state": "running"},
	}}, nil, false)
	c.ApplyEnrichmentState("ec2", 1, false,
		map[string][]domain.Finding{"i-0123456789abcdef0": {{
			Code:     "ec2.public-ip",
			Phrase:   "tagged " + tui6HostileTag,
			Detail:   "The instance is tagged " + tui6HostileTag + ".",
			Severity: domain.SevBroken,
			Source:   "wave2",
		}}},
		map[string]map[domain.FindingCode]domain.AttentionDetail{
			"i-0123456789abcdef0": {"ec2.public-ip": {Rows: []domain.DetailRow{{Label: "Name tag", Value: tui6HostileTag}}}},
		})

	body := c.Snapshot().Body.List
	if body == nil || len(body.Rows) != 1 {
		t.Fatalf("want the one enriched row, got %+v", body)
	}
	var sawPhrase bool
	for i, cell := range body.Rows[0].Cells {
		if ctrls := tui6Controls(cell); len(ctrls) > 0 {
			t.Errorf("cell %d written by the enricher carries control runes %q: %q", i, ctrls, cell)
		}
		if strings.Contains(cell, "tagged") {
			sawPhrase = true
		}
	}
	if !sawPhrase {
		t.Fatal("the enricher's phrase reached no cell, so this pin proves nothing")
	}
}

// TestHostileAWSString_FrameTitleIsInert pins the breadcrumb: the detail
// screen titles itself with the resource's name, which is a tag value AWS
// hands back verbatim.
func TestHostileAWSString_FrameTitleIsInert(t *testing.T) {
	c := tui6HostileDetail(t)
	title := c.Snapshot().FrameTitle
	if !strings.Contains(title, tui6PrintableHead) {
		t.Fatalf("the frame title does not name the resource (%q), so this pin proves nothing", title)
	}
	tui6AssertInert(t, "the frame title", title)
}

// TestPainterKeepsA9sOwnStyling is the counterpart to every pin above: the
// escape sequences a9s itself writes are how colour reaches the screen, so the
// painter must go on passing them through and measuring around them. A fix
// that sanitised the painter's output instead of the incoming data would fail
// here.
func TestPainterKeepsA9sOwnStyling(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Render("running")
	if !strings.Contains(styled, "\x1b[") {
		t.Fatal("lipgloss rendered no escape sequence, so this pin proves nothing")
	}
	got := text.PadOrTrunc(styled, 20)
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("the painter dropped a9s's own styling: %q", got)
	}
	if w := text.Width(got); w != 20 {
		t.Errorf("the painter produced %d columns, want 20: %q", w, got)
	}
	if plain := ansi.Strip(got); strings.TrimRight(plain, " ") != "running" {
		t.Errorf("the painter changed the styled text to %q", plain)
	}
}
