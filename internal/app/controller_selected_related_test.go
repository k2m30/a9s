// controller_selected_related_test.go — regression test for SelectedRelatedRow,
// the accessor the TUI Enter/yank path uses after the stack-lift removed the
// right-column widget ResourceID sync. Related navigation must source
// ResourceIDs from controller state (DetailState.RelatedRows), not the widget.
package app_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
)

// TestSelectedRelatedRow_SourcesIDsFromControllerState verifies that the focused
// related row (TargetType, DisplayName, ResourceIDs) is readable from controller
// state and only when the related panel has focus. Pre-lift the Enter path read
// these IDs from the right-column widget (fed by an adapter sync); that sync is
// gone, so a regression here would break keyboard related-navigation.
func TestSelectedRelatedRow_SourcesIDsFromControllerState(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(res, "ec2")

	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{
			TargetType:  "sg",
			DisplayName: "Security Groups",
			Count:       2,
			ResourceIDs: []string{"sg-aaa", "sg-bbb"},
		},
	})

	// Related panel unfocused: nothing is selectable.
	if _, ok := c.SelectedRelatedRow(); ok {
		t.Fatal("SelectedRelatedRow returned ok=true before the related panel had focus")
	}

	// Force the panel visible (width-independent) so Tab engages ds.RelatedFocus,
	// which SelectedRelatedRow reads directly (the snapshot's RelatedFocused is
	// additionally width-gated and stays false without a terminal size).
	c.SetDetailRelatedVisible(true, false)
	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus asserted via SelectedRelatedRow

	row, ok := c.SelectedRelatedRow()
	if !ok {
		t.Fatal("SelectedRelatedRow returned ok=false with a focused actionable row")
	}
	if row.TargetType != "sg" {
		t.Errorf("TargetType = %q, want %q", row.TargetType, "sg")
	}
	if row.DisplayName != "Security Groups" {
		t.Errorf("DisplayName = %q, want %q", row.DisplayName, "Security Groups")
	}
	if len(row.ResourceIDs) != 2 || row.ResourceIDs[0] != "sg-aaa" || row.ResourceIDs[1] != "sg-bbb" {
		t.Errorf("ResourceIDs = %v, want [sg-aaa sg-bbb] — Enter path must source IDs from controller state", row.ResourceIDs)
	}
}
