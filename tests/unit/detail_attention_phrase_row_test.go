// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_attention_phrase_row_test.go — an Attention phrase is painted once.
//
// The phrase row reaches the renderer carrying the finding's raw wording as
// its label and the glyph-and-capital display form as its value. Only the
// display form is painted; there is no flag that turns it into "raw phrase:
// display form", since nothing in the app takes a plain-text copy of the
// screen through this renderer.
package unit

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestDetailAttention_PhraseRowIsPaintedOnceInItsDisplayForm(t *testing.T) {
	const rawPhrase = "public ingress on 4 ports"
	const code domain.FindingCode = "sg.public-ingress"

	res := resource.Resource{
		ID: "sg-0aaa111111111111a", Name: "public-web", Type: "sg",
		Fields: map[string]string{"group_id": "sg-0aaa111111111111a"},
		Findings: []domain.Finding{{
			Code: code, Phrase: rawPhrase,
			Detail:   "Four ports accept traffic from the whole internet.",
			Severity: domain.SevBroken, Source: "wave1",
		}},
	}

	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, "sg")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(120)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body on the snapshot")
	}

	var display string
	for _, f := range body.Fields {
		if f.Path == "Attention" && f.IndentLevel == 1 && f.Key == rawPhrase {
			display = f.Value
			break
		}
	}
	if display == "" {
		t.Fatalf("the phrase row never reached the body; Attention rows: %v", attentionRowValues(body))
	}
	if display == rawPhrase {
		t.Fatalf("the body's phrase row carries the raw wording as its display form (%q), so this pin cannot tell the two apart", display)
	}

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(80))
	m := views.NewTransientDetail(120, 80, vp)
	screen := ansi.Strip(m.RenderDetail(*body))

	var painted string
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, display) {
			painted = line
			break
		}
	}
	if painted == "" {
		t.Fatalf("the phrase row never reached the screen:\n%s", screen)
	}
	if strings.TrimSpace(painted) != strings.TrimSpace(display) {
		t.Errorf("the phrase row is painted as %q, want just its display form %q", strings.TrimSpace(painted), strings.TrimSpace(display))
	}
	if strings.Contains(painted, rawPhrase+":") {
		t.Errorf("the phrase row is painted with the raw wording in front of the display form: %q", painted)
	}
}
