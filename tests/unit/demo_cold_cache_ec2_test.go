package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestDemoColdCacheEC2_ListPopulates drives the real message flow starting from
// a cold resource cache (no preloading). It navigates to the EC2 resource list,
// executes the fetch command through the fake EC2 client, and verifies at least
// one EC2 instance appears in the rendered view.
//
// Expected to FAIL initially with a panic from EC2Fake.DescribeInstances because
// that method is still a stub ("not yet implemented"). The coder's task (T013) is
// to implement the fake and provide fixture data — at that point this test passes.
func TestDemoColdCacheEC2_ListPopulates(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	// Size the model so View() renders.
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Wire the pre-supplied fake clients by injecting a ClientsReadyMsg so that
	// m.clients is set before any fetch commands run. ConnectGen seeds at 1
	// (session.New()) and this model is never rotated, so Gen=1 matches it.
	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	// Navigate to the EC2 resource list. handleNavigate pushes the list and
	// returns a batch cmd containing the resource list's Init + fetchResources("ec2").
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{ec2}, got nil")
	}

	// extractMsg walks tea.BatchMsg to find ResourcesLoadedMsg. This triggers the
	// EC2Fake.DescribeInstances call — which panics with "not yet implemented".
	// That panic is the expected initial failure (the test correctly fails before T013).
	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})

	result := raw.(messages.ResourcesLoaded)

	if len(result.Resources) == 0 {
		t.Fatal("expected at least one EC2 instance in fixture data, got zero")
	}

	// Deliver the resources to the model.
	*m, _ = rootApplyMsg(*m, result)

	// Verify the rendered list contains at least one instance ID or name.
	plain := stripANSI(rootViewContent(*m))
	hasInstance := false
	for _, r := range result.Resources {
		if strings.Contains(plain, r.ID) || strings.Contains(plain, r.Name) {
			hasInstance = true
			break
		}
	}
	if !hasInstance {
		t.Errorf("EC2 list view does not contain any instance ID or name from fixtures; view:\n%s", plain)
	}
}

// TestDemoColdCacheEC2_DetailRelatedPanels verifies that opening a detail view for
// an EC2 instance triggers the related-resource check path (VPC, SG, Subnet) via
// the real DetailOperation-driven task dispatch (Core.BeginDetailOperation +
// Core.DetailOperationTasks), not a demo shortcut. The related panels should
// populate with fixture data.
//
// Expected to FAIL initially because EC2Fake methods panic (T013 not yet done).
func TestDemoColdCacheEC2_DetailRelatedPanels(t *testing.T) {
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	// Navigate to EC2 list and extract the ResourcesLoadedMsg from the batch.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	if navCmd == nil {
		t.Fatal("expected cmd after NavigateMsg{ec2}")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)

	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero EC2 instances; cannot open detail")
	}

	// Deliver resources to the model.
	*m, _ = rootApplyMsg(*m, loaded)

	// Open detail for the first EC2 instance. handleNavigate dispatches a
	// related-check task command when related defs are registered for ec2.
	firstInstance := loaded.Resources[0]
	var relatedCmd tea.Cmd
	*m, relatedCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &firstInstance,
		ResourceType: "ec2",
	})

	if relatedCmd == nil {
		t.Fatal("expected a related-check command after opening EC2 detail, got nil — " +
			"are VPC/SG/Subnet RelatedDefs registered for ec2?")
	}

	// Detail-open for an enrichable type (ec2 now has a detail enricher)
	// dispatches enrich + related-check tasks directly off relatedCmd as a
	// (possibly nested) tea.Batch; each per-def related-check leaf carries a
	// RelatedCheckResult directly — RunRelatedDef already recovers
	// per-checker panics into a RelatedCheckResult with LazyAddError, so a
	// non-EC2-backed checker (e.g. ELBv2 target groups during the EC2-only
	// pilot) with a nil client surfaces as a negative Count (UnknownRelated)
	// rather than a raw panic. Look specifically for an EC2-backed result
	// (vpc/sg/subnet/eni etc, Count>=0).
	leaves := extractLeafMsgs(relatedCmd)
	var checkResult messages.RelatedCheckResult
	var found bool
	for _, leaf := range leaves {
		if r, ok := leaf.(messages.RelatedCheckResult); ok && r.Result.Count >= 0 {
			checkResult = r
			found = true
			break
		}
	}
	if !found {
		types := make([]string, len(leaves))
		for i, leaf := range leaves {
			types[i] = fmt.Sprintf("%T", leaf)
		}
		t.Fatalf("no EC2-backed related check produced a successful result; leaves=%v", types)
	}

	*m, _ = rootApplyMsg(*m, checkResult)

	if checkResult.Result.Count < 0 {
		t.Errorf("related check for %q returned Count=%d (error sentinel) — "+
			"fake client not wired or checker returned nil-client error",
			checkResult.DefDisplayName, checkResult.Result.Count)
	}

	// Verify the detail view rendered with the instance ID visible.
	plain := stripANSI(rootViewContent(*m))
	if !strings.Contains(plain, firstInstance.ID) {
		t.Errorf("detail view does not contain instance ID %q; view:\n%s", firstInstance.ID, plain)
	}

	// Verify at least one related panel label is visible.
	relatedPanelKeys := []string{"VPC", "Security Group", "Subnet"}
	foundAny := false
	for _, panel := range relatedPanelKeys {
		if strings.Contains(plain, panel) {
			foundAny = true
			break
		}
	}
	if !foundAny {
		t.Errorf("EC2 detail view missing related panel labels (VPC/Security Group/Subnet); view:\n%s", plain)
	}
}
