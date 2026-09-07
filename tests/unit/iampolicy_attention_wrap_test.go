// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
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

// wrapPanelWidth is a comfortable terminal, not a narrow one: the sentence
// must be readable without the reader widening the window or discovering the
// wrap toggle.
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

func detailBodyWithFinding(t *testing.T, shortName string, f domain.Finding) *app.DetailBody {
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
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body on the snapshot")
	}
	if body.Wrap {
		t.Fatal("this pin is about the default, wrap-off state")
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
// past the cut never reach anyone.
func TestDetailAttention_SentenceWrapsToThePanelWithWrapOff(t *testing.T) {
	cases := []struct {
		shortName string
		code      domain.FindingCode
	}{
		{"role", "role.trust.confused-deputy"},
		{"kms", "kms.public-policy"},
	}
	for _, tt := range cases {
		t.Run(string(tt.code), func(t *testing.T) {
			f := catalogFinding(t, tt.shortName, tt.code)
			body := detailBodyWithFinding(t, tt.shortName, f)

			rows := attentionSentenceRows(body)
			for _, v := range rows {
				if w := ansi.StringWidth(v); w > wrapPanelWidth {
					t.Errorf("Attention row is %d columns wide at a %d-column panel, so it is cut: %q",
						w, wrapPanelWidth, v)
				}
			}

			// Every word must survive the trip, in order: wrapping that drops
			// or reorders text is not wrapping.
			joined := strings.Join(strings.Fields(strings.Join(rows, " ")), " ")
			if !strings.Contains(joined, strings.Join(strings.Fields(f.Detail), " ")) {
				t.Errorf("the Detail sentence is not recoverable in full from the Attention rows.\nwant: %q\ngot:  %q",
					f.Detail, joined)
			}
		})
	}
}

// The same sentence through the renderer that actually paints the screen. The
// headless body and this must agree, which is what the two mirrors' comments
// require of each other.
func TestDetailAttention_SentenceRendersInFullInTheTUIAtNormalWidth(t *testing.T) {
	f := catalogFinding(t, "role", "role.trust.confused-deputy")
	body := detailBodyWithFinding(t, "role", f)

	vp := viewport.New(viewport.WithWidth(wrapPanelWidth), viewport.WithHeight(60))
	m := views.NewTransientDetail(wrapPanelWidth, 60, vp)
	rendered := ansi.Strip(m.RenderDetail(*body))
	flat := strings.Join(strings.Fields(rendered), " ")

	var missing []string
	for _, word := range strings.Fields(f.Detail) {
		if !strings.Contains(flat, word) {
			missing = append(missing, word)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d of the %d words of the Detail sentence never reach the screen at %d columns, from %q onwards",
			len(missing), len(strings.Fields(f.Detail)), wrapPanelWidth, missing[0])
	}
}

// The control. An ordinary field value wider than the panel keeps the wrap
// toggle's behaviour: it stays one row in the body and is cut on screen. A
// fix that wraps everything would make the toggle meaningless and would
// reflow every ARN and policy document on the screen.
func TestDetailAttention_OrdinaryFieldValueStillObeysTheWrapToggle(t *testing.T) {
	f := catalogFinding(t, "role", "role.trust.confused-deputy")
	body := detailBodyWithFinding(t, "role", f)

	var found bool
	for _, fr := range body.Fields {
		if fr.Path != "Attention" && fr.Value == longUnwrappedValue {
			found = true
		}
	}
	if !found {
		t.Fatalf("the long non-Attention field value was not carried into the body intact, so it was reflowed")
	}

	vp := viewport.New(viewport.WithWidth(wrapPanelWidth), viewport.WithHeight(60))
	m := views.NewTransientDetail(wrapPanelWidth, 60, vp)
	flat := strings.Join(strings.Fields(ansi.Strip(m.RenderDetail(*body))), " ")
	if strings.Contains(flat, "tail-token") {
		t.Errorf("the tail of an ordinary over-wide field value reached the screen with wrap off, so the toggle no longer governs it")
	}
}
