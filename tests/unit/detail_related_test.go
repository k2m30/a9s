package unit_test

// detail_related_test.go tests the DetailModel's right-column toggle behavior
// and RelatedCheckStartedMsg dispatch.
//
// Design spec: docs/design/related-resources.md v4.3
// QA stories:  docs/qa/related-resources-stories.md
//
// Tests here focus on the cmd returned by Update() when ToggleRelated is
// pressed — specifically whether a RelatedCheckStartedMsg is dispatched —
// and the RelatedCheckResultMsg path that updates the right column display.
//
// Overlap avoidance: rightcolumn_test.go covers View()-output assertions for
// RELATED header, loading state, count rendering, error state, and the
// auto-show-on-entry behavior. This file covers the cmd dispatch semantics
// and edge cases around the toggle state machine.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// makeDetailForToggleTest creates a DetailModel with a Fields-only resource at
// the given width, ready for toggle testing.
func makeDetailForToggleTest(t *testing.T, width int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-toggle123",
		Name: "toggle-test",
		Fields: map[string]string{
			"instance_id": "i-toggle123",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(width, 30)
	return d
}

// pressToggleRelated sends the ToggleRelated ('r') key to d and returns both
// the updated model and the cmd.
func pressToggleRelated(d views.DetailModel) (views.DetailModel, func() interface{}) {
	updated, cmd := d.Update(detailKeyPress("r"))
	if cmd == nil {
		return updated, nil
	}
	return updated, func() interface{} { return cmd() }
}

// ---------------------------------------------------------------------------
// TestDetail_ToggleRelated_SetsVisible
// Given: width=140, RelatedDefs registered for "ec2"
// When:  ToggleRelated key pressed
// Then:  returned cmd produces a RelatedCheckStartedMsg with ResourceType="ec2"
// ---------------------------------------------------------------------------

func TestDetail_ToggleRelated_SetsVisible(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForToggleTest(t, 140)
	_, rawCmd := pressToggleRelated(d)

	if rawCmd == nil {
		t.Fatal("ToggleRelated at width=140 with registered defs should return a non-nil cmd")
	}

	result := rawCmd()
	startMsg, ok := result.(messages.RelatedCheckStartedMsg)
	if !ok {
		t.Fatalf("cmd() should produce RelatedCheckStartedMsg; got %T", result)
	}
	if startMsg.ResourceType != "ec2" {
		t.Errorf("RelatedCheckStartedMsg.ResourceType expected %q, got %q", "ec2", startMsg.ResourceType)
	}
}

// ---------------------------------------------------------------------------
// TestDetail_ToggleRelated_SecondPressHides
// Given: width=140, RelatedDefs registered for "ec2", toggle pressed once
// When:  toggle pressed a second time
// Then:  returned cmd is nil (no checker dispatch on close)
// ---------------------------------------------------------------------------

func TestDetail_ToggleRelated_SecondPressHides(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForToggleTest(t, 140)

	// First press: auto-shown → explicitly on (or no-auto, explicitly on).
	// Either way, a cmd is returned and the column is visible.
	var firstCmd func() interface{}
	d, firstCmd = pressToggleRelated(d)
	if firstCmd == nil {
		t.Fatal("first ToggleRelated press should dispatch a cmd (checker); got nil")
	}

	// Second press: hide the column — no checker dispatch expected.
	_, secondCmd := pressToggleRelated(d)
	if secondCmd != nil {
		t.Errorf("second ToggleRelated press (hide) should return nil cmd; got non-nil")
	}
}

// ---------------------------------------------------------------------------
// TestDetail_ToggleRelated_NarrowTerminal
// Given: width=80 (below the 100-column threshold)
// When:  ToggleRelated key pressed
// Then:  returned cmd is nil (silently ignored)
// ---------------------------------------------------------------------------

func TestDetail_ToggleRelated_NarrowTerminal(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForToggleTest(t, 80)
	_, rawCmd := pressToggleRelated(d)

	if rawCmd != nil {
		t.Errorf("ToggleRelated at width=80 (below 100-col threshold) should return nil cmd; got non-nil")
	}
}

// ---------------------------------------------------------------------------
// TestDetail_RelatedCheckResult_UpdatesRightCol
// Given: width=140, RelatedDef with TargetType="tg" registered for "ec2",
//        right column visible (auto-show on SetSize)
// When:  RelatedCheckResultMsg{ResourceType:"ec2", Result:{TargetType:"tg", Count:3}} sent
// Then:  View() output contains "(3)"
// ---------------------------------------------------------------------------

func TestDetail_RelatedCheckResult_UpdatesRightCol(t *testing.T) {
	ensureNoColor(t)

	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	// SetSize with width=140 and registered defs auto-shows the right column.
	d := makeDetailForToggleTest(t, 140)

	// Deliver the async check result.
	d, _ = d.Update(messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       3,
			ResourceIDs: []string{"tg-1", "tg-2", "tg-3"},
			Err:         nil,
		},
	})

	view := d.View()
	if !strings.Contains(view, "(3)") {
		t.Errorf("after RelatedCheckResultMsg with Count=3, View() should contain \"(3)\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestDetail_ToggleRelated_NoDefsRegistered
// Given: width=140, NO RelatedDefs registered for "ec2"
// When:  ToggleRelated key pressed
// Then:  toggle still opens (right column visible), cmd dispatches
//        RelatedCheckStartedMsg (with empty defs list — root model handles no-ops)
// ---------------------------------------------------------------------------

func TestDetail_ToggleRelated_NoDefsRegistered(t *testing.T) {
	// Guarantee no defs are registered for "ec2".
	resource.UnregisterRelated("ec2")

	d := makeDetailForToggleTest(t, 140)
	_, rawCmd := pressToggleRelated(d)

	// The detail view should dispatch a cmd regardless of whether defs exist —
	// the root model is responsible for deciding what to do with an empty defs list.
	if rawCmd == nil {
		t.Fatal("ToggleRelated with no registered defs should still dispatch a cmd; got nil")
	}

	result := rawCmd()
	startMsg, ok := result.(messages.RelatedCheckStartedMsg)
	if !ok {
		t.Fatalf("cmd() should produce RelatedCheckStartedMsg even with no defs; got %T", result)
	}
	if startMsg.ResourceType != "ec2" {
		t.Errorf("RelatedCheckStartedMsg.ResourceType expected %q, got %q", "ec2", startMsg.ResourceType)
	}
}
