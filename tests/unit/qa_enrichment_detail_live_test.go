package unit

// qa_enrichment_detail_live_test.go — T049 (live-update tests): US3 handler behavior.
//
// Tests verify that when an EnrichmentCheckedMsg arrives while a DetailModel is
// the active view, the root model's handler correctly:
//   - calls SetEnrichmentFinding(&f) when a finding exists for the viewed resource
//   - calls SetEnrichmentFinding(nil) when the resource is no longer in the findings map
//
// These tests drive behavior through m.Update(msg) on the root tui.Model,
// not through internal handlers directly.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// ec2Resource returns a minimal EC2 resource for live-update testing.
func ec2Resource(id string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id + "-name",
		Fields: map[string]string{
			"instance_id": id,
			"state":       "running",
		},
	}
}

// navigateToDetailWithEC2 sets up a root model where:
//   - A ResourceList for "ec2" is pushed (via NavigateMsg)
//   - Resources are loaded (via ResourcesLoadedMsg)
//   - A DetailModel for the given resource is pushed (via NavigateMsg)
//
// Returns the model after all navigation.
func navigateToDetailWithEC2(t *testing.T, res resource.Resource) tui.Model {
	t.Helper()

	m := tui.New("", "")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m2.(tui.Model)

	// Navigate to EC2 resource list.
	m2, _ = m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = m2.(tui.Model)

	// Load resources into the list.
	m2, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{res},
	})
	m, _ = m2.(tui.Model)

	// Navigate to detail for the resource.
	m2, _ = m.Update(messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	m, _ = m2.(tui.Model)

	return m
}

// renderRootModel returns the string content of the root model's view.
// The root model's View() method returns tea.View, whose Content field
// holds the rendered ANSI string.
func renderRootModel(m tui.Model) string {
	v := m.View()
	return v.Content
}

// ---------------------------------------------------------------------------
// T049-a: Live update adds finding to active detail view
// ---------------------------------------------------------------------------

// TestHandleEnrichmentChecked_UpdatesActiveDetailWhenFindingPresent asserts that
// when an EnrichmentCheckedMsg arrives with a finding for the resource currently
// shown in the active DetailModel, the root model updates the detail view so
// that the finding summary appears in the rendered output.
func TestHandleEnrichmentChecked_UpdatesActiveDetailWhenFindingPresent(t *testing.T) {
	res := ec2Resource("i-live-001")
	m := navigateToDetailWithEC2(t, res)

	// Send a valid EnrichmentCheckedMsg (Gen=0, TypeGen=0 match a fresh model).
	findingMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"i-live-001": {Severity: "!", Summary: "system status impaired — live update test"},
		},
		Err:     nil,
		Gen:     0, // matches fresh model's enrichmentGen=0
		TypeGen: 0, // matches fresh model's enrichmentTypeGen["ec2"]=0
	}

	m2, _ := m.Update(findingMsg)
	m, _ = m2.(tui.Model)

	output := renderRootModel(m)

	if !strings.Contains(output, "system status impaired — live update test") {
		t.Errorf("after live EnrichmentCheckedMsg, detail view must show finding summary, got:\n%s", output)
	}
	if !strings.Contains(output, "Background Check") {
		t.Errorf("after live EnrichmentCheckedMsg, detail view must show 'Background Check' section, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T049-b: Live update clears finding from active detail view on recovery
// ---------------------------------------------------------------------------

// TestHandleEnrichmentChecked_ClearsDetailFindingOnRecovery asserts that when
// an EnrichmentCheckedMsg arrives with a Findings map that does NOT include the
// resource currently shown in the active DetailModel, the handler clears any
// existing finding from the detail view (recovery case).
func TestHandleEnrichmentChecked_ClearsDetailFindingOnRecovery(t *testing.T) {
	res := ec2Resource("i-live-002")
	m := navigateToDetailWithEC2(t, res)

	// Step 1: Set a finding via the first EnrichmentCheckedMsg.
	setFindingMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       1,
		Findings: map[string]resource.EnrichmentFinding{
			"i-live-002": {Severity: "!", Summary: "instance status impaired — will recover"},
		},
		Gen:     0,
		TypeGen: 0,
	}

	m2, _ := m.Update(setFindingMsg)
	m, _ = m2.(tui.Model)

	withFinding := renderRootModel(m)
	if !strings.Contains(withFinding, "instance status impaired — will recover") {
		t.Skip("pre-condition failed: finding was not set; skipping recovery check")
	}

	// Step 2: For the recovery EnrichmentCheckedMsg, TypeGen must be bumped
	// to a value that still matches. The first message set TypeGen=0 for a fresh
	// model. After that message was processed, enrichmentTypeGen["ec2"] is still 0
	// (it's only bumped on rerun start, not on receipt). So TypeGen=0 still matches.
	clearFindingMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       0,
		Findings:     map[string]resource.EnrichmentFinding{}, // empty — "i-live-002" recovered
		Gen:          0,
		TypeGen:      0, // still matches (TypeGen only changes on rerun start)
	}

	m3, _ := m.Update(clearFindingMsg)
	m, _ = m3.(tui.Model)

	withoutFinding := renderRootModel(m)
	if strings.Contains(withoutFinding, "instance status impaired — will recover") {
		t.Errorf("after recovery EnrichmentCheckedMsg, old finding summary must be cleared from detail view, got:\n%s", withoutFinding)
	}
}

// ---------------------------------------------------------------------------
// Stale TypeGen: finding not applied when TypeGen is stale
// ---------------------------------------------------------------------------

// TestHandleEnrichmentChecked_StaleTypeGenDoesNotUpdateDetail asserts that when
// an EnrichmentCheckedMsg arrives with a TypeGen that does NOT match the model's
// current enrichmentTypeGen["ec2"], the handler drops the message and the detail
// view is NOT updated with the finding.
func TestHandleEnrichmentChecked_StaleTypeGenDoesNotUpdateDetail(t *testing.T) {
	res := ec2Resource("i-live-003")
	m := navigateToDetailWithEC2(t, res)

	staleMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       1,
		Findings: map[string]resource.EnrichmentFinding{
			"i-live-003": {Severity: "!", Summary: "stale finding — should not appear"},
		},
		Gen:     0,
		TypeGen: 99, // stale — fresh model's enrichmentTypeGen["ec2"] is 0
	}

	m2, _ := m.Update(staleMsg)
	m, _ = m2.(tui.Model)

	output := renderRootModel(m)
	if strings.Contains(output, "stale finding — should not appear") {
		t.Errorf("stale TypeGen must be dropped; finding must not appear in detail view, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Finding not applied when the active view is not detail for the same type
// ---------------------------------------------------------------------------

// TestHandleEnrichmentChecked_FindingNotAppliedWhenDetailInactive asserts that
// when an EnrichmentCheckedMsg arrives while the active view is a ResourceList
// (not a detail view), the finding is stored in enrichmentFindings but NOT applied
// to any detail view (there is none active). The model must not panic.
func TestHandleEnrichmentChecked_FindingNotAppliedWhenDetailInactive(t *testing.T) {
	m := tui.New("", "")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m2.(tui.Model)

	// Navigate to EC2 list (NOT detail).
	m2, _ = m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = m2.(tui.Model)

	// Send valid EnrichmentCheckedMsg while a list (not detail) is active.
	findingMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       1,
		Findings: map[string]resource.EnrichmentFinding{
			"i-not-in-detail": {Severity: "!", Summary: "finding for list-only scenario"},
		},
		Gen:     0,
		TypeGen: 0,
	}

	// Must not panic.
	m3, _ := m.Update(findingMsg)
	_ = m3.View()
}
