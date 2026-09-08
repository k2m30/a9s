// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// app_detail_cursor_record_test.go — the cursor keeps the row it is reading
// whatever the rebuild does to the layout, and two rows that read the same
// are still two rows.
//
// The pins in app_detail_attention_cursor_test.go cover a cursor INSIDE the
// Attention block. These are the same rule for the two cases that block left
// open: a cursor on a content field below the block (which used to be moved
// by the change in the block's size alone, so a rebuild that also changed the
// content list carried it onto a different field), and a cursor on a wrapped
// Detail sentence (which used to be named by its own prose, so two findings
// whose sentences wrap identically gave that name two matches).
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// contentRowKeyAt returns the Key of the field row at idx, failing when the
// row is not a real content row.
func contentRowKeyAt(t *testing.T, body *app.DetailBody, idx int) string {
	t.Helper()
	row := fieldRowAt(t, body, idx)
	if row.IsSection || row.IsSpacer || row.Path == "Attention" {
		t.Fatalf("precondition: row %d is not a content field (Key=%q Path=%q)", idx, row.Key, row.Path)
	}
	return row.Key
}

// TestDetailCursor_ContentFieldFollowsItsOwnRowWhenTheFieldsChange: an
// on-demand detail enrichment replaces the resource, so the content list
// itself can grow above the cursor. The cursor must land on the row it was
// reading, not on whatever index the old one left behind.
func TestDetailCursor_ContentFieldFollowsItsOwnRowWhenTheFieldsChange(t *testing.T) {
	row := resource.Resource{
		ID: "i-0eee555555555555e", Name: "batch-runner", Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0eee555555555555e",
			"name":        "batch-runner",
			"state":       "running",
			"zzz_tail":    "last-field",
		},
	}
	c, _ := openDetailWithSweptRow(t, row)

	// Park the cursor on the last content row — the one furthest from the
	// Attention block, so any drift is visible.
	c.Apply(app.Action{Kind: app.ActionMoveBottom})
	before := c.Snapshot().Body.Detail
	preCursor := before.FieldCursor
	preKey := contentRowKeyAt(t, before, preCursor)

	// The enricher returns the same resource with one more field. Nothing
	// about the Attention block changes: the block is empty before and after,
	// so a cursor moved by the block's size alone does not move at all.
	enriched := row
	enriched.Fields = map[string]string{}
	for k, v := range row.Fields {
		enriched.Fields[k] = v
	}
	enriched.Fields["aaa_head"] = "an extra field the enricher found"
	c.ApplyDetailEnrichmentForResource("ec2", row.ID, enriched, nil, nil)

	after := c.Snapshot().Body.Detail
	moved := -1
	for i, f := range after.Fields {
		if f.Key == preKey && f.Path != "Attention" {
			moved = i
			break
		}
	}
	if moved == preCursor {
		t.Fatalf("fixture no longer moves the row (%q still at index %d) — a cursor that never had to move pins nothing", preKey, preCursor)
	}
	postRow := fieldRowAt(t, after, after.FieldCursor)
	if postRow.Key != preKey {
		t.Errorf(
			"the cursor left the field the operator was reading when the enricher added a row above it:\n"+
				"  before: FieldCursor=%d Key=%q\n"+
				"  after:  FieldCursor=%d Key=%q\n"+
				"want the cursor still on %q — the row is relocated by what it is, not by "+
				"the change in the Attention block's size.",
			preCursor, preKey, after.FieldCursor, postRow.Key, preKey,
		)
	}
}

// TestDetailCursor_TwoEntriesOfOneCodeAreTwoRows: the finding code is the
// stem of an Attention row's identity, and a resource can carry two findings
// under one code (two ports, two rules). Their rows are still two rows, and
// the cursor on the second one must not be relocated onto the first.
func TestDetailCursor_TwoEntriesOfOneCodeAreTwoRows(t *testing.T) {
	const secondPhrase = "second phrase"
	row := resource.Resource{
		ID: "i-0777777777777777a", Name: "batch-runner", Type: "ec2",
		Fields: map[string]string{"instance_id": "i-0777777777777777a", "state": "running"},
		Findings: []domain.Finding{
			{Code: "ec2.open-ingress", Phrase: "first phrase", Detail: "Sentence one.", Severity: domain.SevWarn, Source: "wave1"},
			{Code: "ec2.open-ingress", Phrase: secondPhrase, Detail: "Sentence two.", Severity: domain.SevWarn, Source: "wave1"},
		},
	}
	c, core := openDetailWithSweptRow(t, row)
	preCursor := seekAttentionEntry(t, c, secondPhrase)

	deliverEnrichment(t, c, core, row.ID, false, []domain.Finding{{
		Code: "ec2.instance-status-impaired", Phrase: "impaired: system checks failing",
		Detail: "The instance failed its system status check.", Severity: domain.SevBroken, Source: "wave2:ec2",
	}})

	after := c.Snapshot().Body.Detail
	postRow := fieldRowAt(t, after, after.FieldCursor)
	if postRow.Key != secondPhrase {
		t.Errorf(
			"the cursor moved from the second entry of a code to another row when a finding arrived above them:\n"+
				"  before: FieldCursor=%d Key=%q\n"+
				"  after:  FieldCursor=%d Key=%q\n"+
				"two entries sharing a code are two rows, and each keeps its own identity.",
			preCursor, secondPhrase, after.FieldCursor, postRow.Key,
		)
	}
}

// identicalDetailRow carries two findings whose Detail sentences are word for
// word the same, so their wrapped lines are indistinguishable as prose.
func identicalDetailRow(id string) resource.Resource {
	const sameSentence = "This resource is reachable from the whole internet."
	return resource.Resource{
		ID: id, Name: "batch-runner", Type: "ec2",
		Fields: map[string]string{"instance_id": id, "name": "batch-runner", "state": "running"},
		Findings: []domain.Finding{
			{
				Code: "ec2.open-ssh", Phrase: "public ingress on port 22 from 0.0.0.0/0",
				Detail: sameSentence, Severity: domain.SevWarn, Source: "wave1",
			},
			{
				Code: "ec2.open-rdp", Phrase: "public ingress on port 3389 from 0.0.0.0/0",
				Detail: sameSentence, Severity: domain.SevWarn, Source: "wave1",
			},
		},
	}
}

// attentionLineAfter returns the index of the Attention row directly below
// the entry whose line reads phrase — the first wrapped line of that entry's
// Detail sentence.
func attentionLineAfter(t *testing.T, body *app.DetailBody, phrase string) int {
	t.Helper()
	for i, f := range body.Fields {
		if f.Path == "Attention" && f.Key == phrase {
			if i+1 >= len(body.Fields) || body.Fields[i+1].Path != "Attention" {
				t.Fatalf("the entry %q has no Detail line under it", phrase)
			}
			return i + 1
		}
	}
	t.Fatalf("no Attention entry reading %q; rows: %v", phrase, attentionKeys(body))
	return -1
}

// attentionKeys lists the Attention rows' keys, for failure messages.
func attentionKeys(body *app.DetailBody) []string {
	var out []string
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			out = append(out, f.Key)
		}
	}
	return out
}

// TestDetailCursor_DetailLineKeepsItsOwnFindingWhenTwoSentencesMatch: an
// Attention row was named by the text painted on it, and a wrapped Detail
// sentence is prose two findings can share word for word. The cursor then had
// two rows answering to the same name and took the first, silently moving the
// operator from the finding they were reading to another one.
func TestDetailCursor_DetailLineKeepsItsOwnFindingWhenTwoSentencesMatch(t *testing.T) {
	const secondPhrase = "public ingress on port 3389 from 0.0.0.0/0"
	row := identicalDetailRow("i-0fff666666666666f")
	c, core := openDetailWithSweptRow(t, row)

	before := c.Snapshot().Body.Detail
	target := attentionLineAfter(t, before, secondPhrase)
	first := attentionLineAfter(t, before, "public ingress on port 22 from 0.0.0.0/0")
	if before.Fields[target].Value != before.Fields[first].Value {
		t.Fatalf("fixture no longer wraps the two sentences identically: %q vs %q",
			before.Fields[first].Value, before.Fields[target].Value)
	}
	c.Apply(app.Action{Kind: app.ActionMoveTop})
	for range target {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	if got := c.Snapshot().Body.Detail.FieldCursor; got != target {
		t.Fatalf("precondition: cursor is at %d, wanted the second finding's Detail line at %d", got, target)
	}

	// A more severe finding lands above both, so every row below it moves.
	deliverEnrichment(t, c, core, row.ID, false, []domain.Finding{{
		Code: "ec2.instance-status-impaired", Phrase: "impaired: system checks failing",
		Detail: "The instance failed its system status check.", Severity: domain.SevBroken, Source: "wave2:ec2",
	}})

	after := c.Snapshot().Body.Detail
	want := attentionLineAfter(t, after, secondPhrase)
	if after.FieldCursor != want {
		other := attentionLineAfter(t, after, "public ingress on port 22 from 0.0.0.0/0")
		where := "somewhere else"
		if after.FieldCursor == other {
			where = "the other finding's Detail line, which reads the same"
		}
		t.Errorf(
			"the cursor left the Detail line of %q for %s:\n"+
				"  want FieldCursor=%d, got %d (rows: %s)\n"+
				"an Attention line is identified by the finding it belongs to and its "+
				"position in that finding, never by the prose painted on it.",
			secondPhrase, where, want, after.FieldCursor, strings.Join(attentionKeys(after), " | "),
		)
	}
}
