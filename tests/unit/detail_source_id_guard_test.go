package unit

// detail_source_id_guard_test.go — Tests for the SourceResourceID guard bug
// in DetailModel.Update's RelatedCheckResultMsg handler.
//
// Bug (detail.go:92-97): the guard only checks msg.ResourceType != m.resourceType.
// It does NOT check msg.SourceResourceID against the resource being viewed.
// If two EC2 instances are opened back-to-back, the first one's late async
// results can update the second one's right column.
//
// TestDetail_RelatedCheckResult_IgnoresWrongSourceID — FAILS with current code.
// TestDetail_RelatedCheckResult_AcceptsCorrectSourceID — PASSES with current code.

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// makeEC2DetailForSourceIDTest creates a DetailModel for an EC2 instance with
// the given resource ID, at width=140 so the right column auto-shows.
func makeEC2DetailForSourceIDTest(t *testing.T, resourceID string) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   resourceID,
		Name: "test-instance-" + resourceID,
		Fields: map[string]string{
			"instance_id": resourceID,
			"state":       "running",
			"type":        "t3.micro",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(140, 30)
	return d
}

// TestDetail_RelatedCheckResult_IgnoresWrongSourceID verifies that a
// RelatedCheckResultMsg for a different source resource is ignored even when
// the ResourceType matches.
//
// Scenario: Detail is showing instance "i-111". A stale async result arrives
// with SourceResourceID "i-WRONG" (from a previously-opened instance).
// The right column must NOT update — it should stay in loading/zero state.
//
// This test FAILS with the current code because the guard only checks
// ResourceType and does not compare SourceResourceID.
func TestDetail_RelatedCheckResult_IgnoresWrongSourceID(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Skip("no ec2 related defs registered — import internal/aws to register them")
	}

	d := makeEC2DetailForSourceIDTest(t, "i-111")

	// Precondition: right column must be visible (auto-shown at width=140).
	viewBefore := stripANSI(d.View())
	if !strings.Contains(viewBefore, "RELATED") {
		t.Fatal("precondition failed: right column should be auto-shown at width=140 for ec2 with registered related defs")
	}

	// Deliver a result for the WRONG source resource ID.
	// Count=5 is distinct — if this leaks into the view, it will be obvious.
	wrongIDMsg := messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: "i-WRONG",
		Result: resource.RelatedCheckResult{
			TargetType:  defs[0].TargetType,
			Count:       5,
			ResourceIDs: []string{"tg-1", "tg-2", "tg-3", "tg-4", "tg-5"},
		},
	}
	d, _ = d.Update(wrongIDMsg)

	viewAfter := stripANSI(d.View())

	// The wrong-source result must NOT appear in the right column.
	// Current bug: the guard only checks ResourceType so "(5)" would appear.
	if strings.Contains(viewAfter, "(5)") {
		t.Fatalf("BUG: right column shows '(5)' after receiving RelatedCheckResultMsg "+
			"for SourceResourceID=%q but detail is showing resource %q. "+
			"The guard must also check SourceResourceID.\nView:\n%s",
			"i-WRONG", "i-111", viewAfter)
	}
}

// TestDetail_RelatedCheckResult_AcceptsCorrectSourceID verifies that a
// RelatedCheckResultMsg with a matching SourceResourceID DOES update the
// right column.
//
// This test PASSES with the current code (correct existing behavior).
func TestDetail_RelatedCheckResult_AcceptsCorrectSourceID(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Skip("no ec2 related defs registered — import internal/aws to register them")
	}

	d := makeEC2DetailForSourceIDTest(t, "i-111")

	// Precondition: right column must be visible.
	viewBefore := stripANSI(d.View())
	if !strings.Contains(viewBefore, "RELATED") {
		t.Fatal("precondition failed: right column should be auto-shown at width=140 for ec2 with registered related defs")
	}

	// Deliver a result with the CORRECT source resource ID.
	correctIDMsg := messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: "i-111",
		Result: resource.RelatedCheckResult{
			TargetType:  defs[0].TargetType,
			Count:       5,
			ResourceIDs: []string{"tg-1", "tg-2", "tg-3", "tg-4", "tg-5"},
		},
	}
	d, _ = d.Update(correctIDMsg)

	viewAfter := stripANSI(d.View())

	// The correct-source result MUST appear as "(5)" in the right column.
	if !strings.Contains(viewAfter, "(5)") {
		t.Fatalf("right column should show '(5)' after receiving RelatedCheckResultMsg "+
			"with matching SourceResourceID=%q for resource %q.\nView:\n%s",
			"i-111", "i-111", viewAfter)
	}
}
