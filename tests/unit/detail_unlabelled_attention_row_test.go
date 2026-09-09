// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_unlabelled_attention_row_test.go — an Attention supporting row with
// no label is a whole line, and it is the renderer that knows so.
//
// The projection leaves the key empty rather than copying the value into it
// so the renderer's "key equals value" test would catch it: that would put
// the knowledge of how a line is painted a layer above the painter, and
// leave the body carrying a label no operator asked for.
package unit

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// unlabelledRowValue is the closing line of a capped supporting list — the
// one Attention row shipped today that has no label. No colon in it, so a
// colon on its line can only have come from an empty label.
const unlabelledRowValue = "and 3 more"

// TestDetailAttention_UnlabelledSupportingRowIsAWholeLine drives a finding
// whose supporting rows are one labelled fact and one bare line, and reads
// both the body and the painted screen back.
func TestDetailAttention_UnlabelledSupportingRowIsAWholeLine(t *testing.T) {
	const code domain.FindingCode = "sg.public-ingress"
	res := resource.Resource{
		ID: "sg-0aaa111111111111a", Name: "public-web", Type: "sg",
		Fields: map[string]string{"group_id": "sg-0aaa111111111111a"},
		Findings: []domain.Finding{{
			Code: code, Phrase: "public ingress on 4 ports",
			Detail: "Four ports accept traffic from the whole internet.", Severity: domain.SevBroken,
			Source: "wave1",
		}},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			code: {Rows: []domain.DetailRow{
				{Label: "Port", Value: "22 from 0.0.0.0/0", Tier: "!"},
				{Label: "", Value: unlabelledRowValue, Tier: "!"},
			}},
		},
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

	var row app.FieldRow
	found := false
	for _, f := range body.Fields {
		if f.Path == "Attention" && f.Value == unlabelledRowValue {
			row, found = f, true
			break
		}
	}
	if !found {
		t.Fatalf("the unlabelled supporting row never reached the body; Attention rows: %v", attentionRowValues(body))
	}
	if row.Key != "" {
		t.Errorf(
			"the body carries %q as the label of a row the finding gave no label:\n"+
				"the projection must pass the empty label through and let the renderer "+
				"decide that a row without one is a whole line.",
			row.Key,
		)
	}

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(80))
	m := views.NewTransientDetail(120, 80, vp)
	var painted string
	for _, line := range strings.Split(ansi.Strip(m.RenderDetail(*body)), "\n") {
		if strings.Contains(line, unlabelledRowValue) {
			painted = line
			break
		}
	}
	if painted == "" {
		t.Fatalf("the unlabelled row never reached the screen:\n%s", ansi.Strip(m.RenderDetail(*body)))
	}
	if strings.Contains(painted, ":") {
		t.Errorf("a row with no label is painted behind a bare colon: %q", painted)
	}
	if strings.TrimSpace(painted) != unlabelledRowValue {
		t.Errorf("the unlabelled row is painted as %q, want just %q", strings.TrimSpace(painted), unlabelledRowValue)
	}
}

// attentionRowValues lists the Attention rows' values, for failure messages.
func attentionRowValues(body *app.DetailBody) []string {
	var out []string
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			out = append(out, f.Value)
		}
	}
	return out
}
