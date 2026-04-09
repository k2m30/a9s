package unit

// Tests for the IAM group related checker covering both managed (attached) and
// inline group policies. See internal/aws/iam_groups_related.go:48.
//
// Bug: checkGroupPolicy only calls ListAttachedGroupPolicies (managed policies).
// Groups with only inline policies (ListGroupPolicies) show "IAM Policies (0)".
//
// TestIAMGroup_ManagedPolicies_RelatedCount — verifies existing managed-policy
// path works. Should PASS immediately.
//
// TestIAMGroup_InlinePoliciesOnly_RelatedCount — reveals the bug. WILL FAIL
// until the coder:
//  1. Adds ListGroupPolicies call to checkGroupPolicy (iam_groups_related.go:48)
//  2. Adds InlineGroupPolicies map to IAMFixtures (internal/demo/fixtures/iam.go)
//  3. Adds ListGroupPolicies method to IAMFake (internal/demo/fakes/iam.go)
//  4. Populates inline policies for "readonly" group in buildIAMRelations

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// runIAMGroupRelatedCheck drives the cold-cache app to the detail view for the
// given IAM group name and returns the RelatedCheckResultMsg for TargetType "policy".
// Fails the test if the group is not found or no policy check result is produced.
func runIAMGroupRelatedCheck(t *testing.T, groupName string) messages.RelatedCheckResultMsg {
	t.Helper()

	m := newDemoColdCacheApp(t)
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	// Navigate to IAM groups list.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "iam-group",
	})
	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{iam-group}, got nil")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	loaded := raw.(messages.ResourcesLoadedMsg)

	if len(loaded.Resources) == 0 {
		t.Fatal("IAM groups fixture returned zero groups")
	}
	*m, _ = rootApplyMsg(*m, loaded)

	// Locate the target group.
	targetIdx := -1
	for i, r := range loaded.Resources {
		if r.ID == groupName || r.Name == groupName {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		groupIDs := make([]string, len(loaded.Resources))
		for i, r := range loaded.Resources {
			groupIDs[i] = r.ID
		}
		t.Fatalf("fixture does not contain group %q; available: %v", groupName, groupIDs)
	}

	targetGroup := loaded.Resources[targetIdx]

	// Open detail — triggers related-check commands.
	var relatedCmd tea.Cmd
	*m, relatedCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		Resource:     &targetGroup,
		ResourceType: "iam-group",
	})
	if relatedCmd == nil {
		t.Fatalf("expected related-check cmd after opening detail for group %q; "+
			"are RelatedDefs registered for iam-group?", groupName)
	}

	// Execute to get RelatedCheckStartedMsg.
	relatedMsg := relatedCmd()
	started, ok := relatedMsg.(messages.RelatedCheckStartedMsg)
	if !ok {
		t.Fatalf("expected RelatedCheckStartedMsg after detail nav, got %T", relatedMsg)
	}

	// Dispatch so checkers run.
	var checkCmds tea.Cmd
	*m, checkCmds = rootApplyMsg(*m, started)
	if checkCmds == nil {
		t.Fatalf("handleRelatedCheckStarted returned nil cmd for group %q", groupName)
	}

	// Execute checker batch; recover from panics on unrelated checkers.
	runChecker := func(c tea.Cmd) (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = nil
			}
		}()
		return c()
	}

	rawCheck := runChecker(checkCmds)
	switch v := rawCheck.(type) {
	case messages.RelatedCheckResultMsg:
		if v.Result.TargetType == "policy" {
			return v
		}
	case tea.BatchMsg:
		for _, subCmd := range v {
			if subCmd == nil {
				continue
			}
			sub := runChecker(subCmd)
			if r, ok2 := sub.(messages.RelatedCheckResultMsg); ok2 && r.Result.TargetType == "policy" {
				return r
			}
		}
	}

	t.Fatalf("no 'policy' RelatedCheckResultMsg found for group %q; "+
		"is checkGroupPolicy registered as a RelatedDef for iam-group?", groupName)
	return messages.RelatedCheckResultMsg{}
}

// TestIAMGroup_ManagedPolicies_RelatedCount verifies that a group with attached
// customer-managed policies shows Count > 0 in the related panel.
//
// The "developers" fixture group has acme-s3-read-only and acme-deploy-policy
// attached (both customer-managed). checkGroupPolicy filters to customer-managed
// only via customerManagedAttachedPolicyNames, so Count should be 2.
// Exercises the existing ListAttachedGroupPolicies path — should PASS.
func TestIAMGroup_ManagedPolicies_RelatedCount(t *testing.T) {
	result := runIAMGroupRelatedCheck(t, "developers")

	if result.Result.Count <= 0 {
		t.Errorf("developers group (acme-s3-read-only + acme-deploy-policy attached) got Count=%d, want >0; "+
			"checkGroupPolicy may not be calling ListAttachedGroupPolicies correctly",
			result.Result.Count)
	}
}

// TestIAMGroup_InlinePoliciesOnly_RelatedCount reveals the missing ListGroupPolicies
// call in checkGroupPolicy (internal/aws/iam_groups_related.go:48).
//
// The "readonly" fixture group has no attached managed policies. After the coder
// adds InlineGroupPolicies support to fixtures/fakes and populates inline policies
// for "readonly", the checker MUST count them via ListGroupPolicies.
//
// This test WILL FAIL (Count=0) until the bug is fixed:
//   - checkGroupPolicy must call ListGroupPolicies in addition to ListAttachedGroupPolicies
//   - InlineGroupPolicies["readonly"] must be non-empty in demo fixtures
func TestIAMGroup_InlinePoliciesOnly_RelatedCount(t *testing.T) {
	result := runIAMGroupRelatedCheck(t, "readonly")

	// Fails until checkGroupPolicy calls ListGroupPolicies:
	if result.Result.Count <= 0 {
		t.Errorf("readonly group (inline policies only) got Count=%d, want >0; "+
			"BUG: checkGroupPolicy does not call ListGroupPolicies — "+
			"inline policies are never counted (internal/aws/iam_groups_related.go:48)",
			result.Result.Count)
	}
}
