// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// wrapPanelWidths sweeps a cramped panel, a comfortable one and a wide one.
// One width alone cannot tell a wrap that follows the panel from one that
// wraps at a fixed column and happens to fit.
var wrapPanelWidths = []int{60, 120, 200}

// wrapPanelWidth is the width the single-width pins use.
const wrapPanelWidth = 120

// catalogFinding returns the registered finding for a code as a live resource
// would carry it. Taking the phrase and sentence from the catalog rather than
// from a literal means these pins follow the prose instead of freezing one
// wording of it.
func catalogFinding(t *testing.T, shortName string, code domain.FindingCode) domain.Finding {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("resource type %q is not registered", shortName)
	}
	for _, f := range td.Findings {
		if f.Code != code {
			continue
		}
		if f.Detail == "" {
			t.Fatalf("%s carries no Detail sentence, so there is nothing to wrap", code)
		}
		return domain.Finding{Code: f.Code, Phrase: f.Phrase, Severity: f.Severity, Source: f.Source, Detail: f.Detail}
	}
	t.Fatalf("%s is not registered on %s", code, shortName)
	return domain.Finding{}
}

// longUnwrappedValue is a plain field value wider than the panel. It is the
// control: the wrap toggle owns ordinary values, and a fix that wraps the
// Attention sentence must not start wrapping these too.
var longUnwrappedValue = "arn:aws:iam::123456789012:role/service-role/" +
	strings.Repeat("very-long-path-segment-", 8) + "tail-token"

// detailBodyWithFinding opens a detail for a resource carrying f and returns
// the body the renderer paints. The related panel is hidden so the field
// panel spans the whole width, which makes the width reported to the
// controller the width the renderer paints into — otherwise the test would
// have to re-derive the renderer's own column arithmetic.
func detailBodyWithFinding(t *testing.T, shortName string, f domain.Finding, width int) *app.DetailBody {
	t.Helper()
	c := newTestController(t)
	res := resource.Resource{
		ID:       "wrap-witness",
		Name:     "wrap-witness",
		Type:     shortName,
		Findings: []domain.Finding{f},
		Fields:   map[string]string{"arn": longUnwrappedValue},
	}
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, shortName)
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(width)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body on the snapshot")
	}
	if body.Wrap {
		t.Fatal("this pin is about the default, wrap-off state")
	}
	if body.RelatedVisible {
		t.Fatal("the related panel is showing, so the field panel is narrower than the reported width")
	}
	return body
}

// attentionSentenceRows returns the Attention rows carrying sentence text,
// i.e. the ones whose value is prose rather than a glyph line or a label.
func attentionSentenceRows(body *app.DetailBody) []string {
	var out []string
	for _, f := range body.Fields {
		if f.Path == "Attention" && !f.IsSection && !f.IsSpacer {
			out = append(out, f.Value)
		}
	}
	return out
}

// The Attention sentence is the one thing on the detail screen a reader must
// act on, so it cannot depend on the reader finding the wrap toggle first.
// No Attention row may be wider than the panel: a row that is stays cut at
// the panel edge, with no ellipsis and no continuation line, and the words
// past the cut never reach anyone. Swept across widths, because a wrap that
// ignores the panel and breaks at a fixed column passes at one width only by
// luck.
func TestDetailAttention_SentenceWrapsToThePanelWithWrapOff(t *testing.T) {
	cases := []struct {
		shortName string
		code      domain.FindingCode
	}{
		{"role", "role.trust.confused-deputy"},
		{"kms", "kms.public-policy"},
	}
	for _, tt := range cases {
		for _, width := range wrapPanelWidths {
			t.Run(fmt.Sprintf("%s@%d", tt.code, width), func(t *testing.T) {
				f := catalogFinding(t, tt.shortName, tt.code)
				body := detailBodyWithFinding(t, tt.shortName, f, width)

				rows := attentionSentenceRows(body)
				for _, v := range rows {
					if w := ansi.StringWidth(v); w > width {
						t.Errorf("Attention row is %d columns wide at a %d-column panel, so it is cut: %q",
							w, width, v)
					}
				}

				// Every word must survive the trip, in order: wrapping that
				// drops or reorders text is not wrapping.
				joined := strings.Join(strings.Fields(strings.Join(rows, " ")), " ")
				if !strings.Contains(joined, strings.Join(strings.Fields(f.Detail), " ")) {
					t.Errorf("the Detail sentence is not recoverable in full from the Attention rows.\nwant: %q\ngot:  %q",
						f.Detail, joined)
				}
			})
		}
	}
}

// The same sentence through the renderer that actually paints the screen. The
// headless body and this must agree, which is what the deleted TUI-side
// injector used to be asked to guarantee by hand.
func TestDetailAttention_SentenceRendersInFullInTheTUIAtNormalWidth(t *testing.T) {
	f := catalogFinding(t, "role", "role.trust.confused-deputy")
	for _, width := range wrapPanelWidths {
		t.Run(fmt.Sprintf("%d", width), func(t *testing.T) {
			body := detailBodyWithFinding(t, "role", f, width)
			vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(80))
			m := views.NewTransientDetail(width, 80, vp)
			flat := strings.Join(strings.Fields(ansi.Strip(m.RenderDetail(*body))), " ")

			var missing []string
			for _, word := range strings.Fields(f.Detail) {
				if !strings.Contains(flat, word) {
					missing = append(missing, word)
				}
			}
			if len(missing) > 0 {
				t.Errorf("%d of the %d words of the Detail sentence never reach the screen at %d columns, from %q onwards",
					len(missing), len(strings.Fields(f.Detail)), width, missing[0])
			}
		})
	}
}

// The control. An ordinary field value wider than the panel keeps the wrap
// toggle's behaviour: it stays one row in the body and is cut on screen. A
// fix that wraps everything would make the toggle meaningless and would
// reflow every ARN and policy document on the screen.
func TestDetailAttention_OrdinaryFieldValueStillObeysTheWrapToggle(t *testing.T) {
	f := catalogFinding(t, "role", "role.trust.confused-deputy")
	body := detailBodyWithFinding(t, "role", f, wrapPanelWidth)

	var found bool
	for _, fr := range body.Fields {
		if fr.Path != "Attention" && fr.Value == longUnwrappedValue {
			found = true
		}
	}
	if !found {
		t.Fatalf("the long non-Attention field value was not carried into the body intact, so it was reflowed")
	}

	vp := viewport.New(viewport.WithWidth(wrapPanelWidth), viewport.WithHeight(80))
	m := views.NewTransientDetail(wrapPanelWidth, 80, vp)
	flat := strings.Join(strings.Fields(ansi.Strip(m.RenderDetail(*body))), " ")
	if strings.Contains(flat, "tail-token") {
		t.Errorf("the tail of an ordinary over-wide field value reached the screen with wrap off, so the toggle no longer governs it")
	}
}

// The field cursor is held steady across an enrichment by a count of the rows
// the Attention block prepends. A sentence that wraps into several rows must
// be counted as several, or the cursor jumps by the difference the first time
// a wrapped finding lands. A narrow panel makes the miscount unmissable.
func TestDetailAttention_WrappedSentenceKeepsTheFieldCursorSteady(t *testing.T) {
	res := resource.Resource{
		ID:     "i-0aaa111111111111a",
		Name:   "web-server",
		Type:   "ec2",
		Fields: map[string]string{"instance_id": "i-0aaa111111111111a", "state": "running"},
	}
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, "ec2")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(60)
	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	before := c.Snapshot().Body.Detail
	preRow := before.Fields[before.FieldCursor]
	if preRow.Path == "Attention" || preRow.IsSection || preRow.IsSpacer {
		t.Fatalf("the cursor must start on an ordinary content field, not %+v", preRow)
	}

	wrapped := catalogFinding(t, "role", "role.trust.confused-deputy")
	wrapped.Code = "ec2.wrapped-witness"
	c.ApplyDetailFinding(&wrapped, nil)

	after := c.Snapshot().Body.Detail
	postRow := after.Fields[after.FieldCursor]
	if postRow.Key != preRow.Key || postRow.Path != preRow.Path {
		t.Errorf("the cursor moved to a different field when a wrapped finding landed:\n  before: %d %q\n  after:  %d %q",
			before.FieldCursor, preRow.Key, after.FieldCursor, postRow.Key)
	}
	if after.FieldCursor-before.FieldCursor < 4 {
		t.Errorf("the sentence wrapped into %d fewer rows than expected at a 60-column panel; the pin is not exercising a multi-row sentence",
			after.FieldCursor-before.FieldCursor)
	}
}

// The wrap measures what the terminal draws, not how many bytes the sentence
// occupies. A wide rune costs two columns and three bytes, so a byte budget
// never overflows the panel — it wastes it, stopping a third short of the
// edge and spreading the sentence over more rows than the reader's terminal
// needs. Filling the panel is the behaviour, so that is what is pinned.
func TestDetailAttention_WideRunesWrapByDisplayWidth(t *testing.T) {
	f := catalogFinding(t, "role", "role.trust.confused-deputy")
	f.Detail = strings.TrimSpace(strings.Repeat("この条件は呼び出し元を絞り込みません ", 6))
	body := detailBodyWithFinding(t, "role", f, wrapPanelWidth)

	rows := attentionSentenceRows(body)
	var wrappedAny bool
	for _, v := range rows {
		if w := ansi.StringWidth(v); w > wrapPanelWidth {
			t.Errorf("Attention row is %d columns wide at a %d-column panel: %q", w, wrapPanelWidth, v)
		}
		if strings.Contains(v, "この条件") {
			wrappedAny = true
		}
	}
	if !wrappedAny {
		t.Fatal("the wide-rune sentence never reached the Attention rows")
	}
	var widest int
	for _, v := range rows {
		if strings.Contains(v, "この条件") {
			widest = max(widest, ansi.StringWidth(v))
		}
	}
	if widest <= wrapPanelWidth*2/3 {
		t.Errorf("the widest wide-rune row is %d columns of a %d-column panel, so the wrap is budgeting bytes rather than columns",
			widest, wrapPanelWidth)
	}
}
