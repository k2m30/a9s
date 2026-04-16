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
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
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

// TestIAMPolicyList_IncludesInlinePolicies reveals that FetchIAMPoliciesPage only
// calls ListPolicies(Scope: Local) and never fetches inline group policies.
// Inline policies from IAM groups (AllowAssumeRole, AllowChangeOwnPassword,
// DenyS3Delete) are absent from the policy resource list, causing the "IAM Policies"
// related panel to navigate to an empty or incomplete list.
//
// This test WILL FAIL until FetchIAMPoliciesPage (or a companion fetcher) also
// calls ListGroupPolicies for each group and synthesises inline policy resources
// with Fields["policy_type"] == "inline".
//
// Fixture inline policies (internal/demo/fixtures/iam.go):
//
//	developers: ["AllowAssumeRole", "AllowChangeOwnPassword"]
//	readonly:   ["DenyS3Delete"]
func TestIAMPolicyList_IncludesInlinePolicies(t *testing.T) {
	m := newDemoColdCacheApp(t)
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	// Navigate to the policy resource list.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "policy",
	})
	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{policy}, got nil")
	}

	// Drain the fetch command to get ResourcesLoadedMsg.
	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	loaded := raw.(messages.ResourcesLoadedMsg)

	// Collect inline policy names from the returned resources.
	var inlineNames []string
	for _, r := range loaded.Resources {
		if r.Fields["policy_type"] == "inline" {
			inlineNames = append(inlineNames, r.Name)
		}
	}

	// These inline policy names must appear once the fetcher is fixed.
	wantInline := []string{"AllowAssumeRole", "AllowChangeOwnPassword", "DenyS3Delete"}

	if len(inlineNames) == 0 {
		t.Errorf("policy list contains no resources with Fields[\"policy_type\"]==\"inline\"; "+
			"BUG: FetchIAMPoliciesPage never calls ListGroupPolicies — "+
			"inline policies are missing from the list (internal/aws/iam_policies.go); "+
			"expected inline policies: %v", wantInline)
		return
	}

	// Verify each expected inline policy name is present.
	nameSet := make(map[string]bool, len(inlineNames))
	for _, n := range inlineNames {
		nameSet[n] = true
	}
	for _, want := range wantInline {
		if !nameSet[want] {
			t.Errorf("inline policy %q not found in policy list; got inline names: %v",
				want, inlineNames)
		}
	}
}

// ---------------------------------------------------------------------------
// fetchInlineGroupPolicies error-path tests
//
// fetchInlineGroupPolicies is unexported. We drive it via the registered
// paginated fetcher for "policy", which calls FetchIAMPoliciesPage then
// fetchInlineGroupPolicies. We construct a *awsclient.ServiceClients with a
// controlled IAM stub that exercises the three defensive branches:
//   - ListGroups error → function returns nil
//   - group with nil GroupName → skipped, no resource emitted
//   - ListGroupPolicies error → continue to next group
// ---------------------------------------------------------------------------

// stubGroupPolicyIAM satisfies awsclient.IAMAPI. Only ListGroups,
// ListGroupPolicies, ListPolicies, and ListAttachedGroupPolicies are called by
// the policy fetcher path; all other methods panic (they must never be called).
type stubGroupPolicyIAM struct {
	listGroupsOut *iam.ListGroupsOutput
	listGroupsErr error

	listGroupPoliciesErr error

	// ListPolicies must succeed (returns empty) so FetchIAMPoliciesPage doesn't
	// short-circuit with an error before fetchInlineGroupPolicies is called.
	listPoliciesOut *iam.ListPoliciesOutput
}

func (s *stubGroupPolicyIAM) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	if s.listGroupsErr != nil {
		return nil, s.listGroupsErr
	}
	if s.listGroupsOut != nil {
		return s.listGroupsOut, nil
	}
	return &iam.ListGroupsOutput{}, nil
}

func (s *stubGroupPolicyIAM) ListGroupPolicies(_ context.Context, _ *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	if s.listGroupPoliciesErr != nil {
		return nil, s.listGroupPoliciesErr
	}
	return &iam.ListGroupPoliciesOutput{PolicyNames: []string{"inline-pol"}}, nil
}

func (s *stubGroupPolicyIAM) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	if s.listPoliciesOut != nil {
		return s.listPoliciesOut, nil
	}
	return &iam.ListPoliciesOutput{}, nil
}

func (s *stubGroupPolicyIAM) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}

// Stub methods that are part of IAMAPI but never called by the policy fetcher.
func (s *stubGroupPolicyIAM) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	panic("stubGroupPolicyIAM.ListRoles called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	panic("stubGroupPolicyIAM.ListUsers called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	panic("stubGroupPolicyIAM.ListAttachedRolePolicies called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	panic("stubGroupPolicyIAM.ListRolePolicies called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListAttachedUserPolicies(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	panic("stubGroupPolicyIAM.ListAttachedUserPolicies called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	panic("stubGroupPolicyIAM.ListGroupsForUser called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	panic("stubGroupPolicyIAM.ListEntitiesForPolicy called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	panic("stubGroupPolicyIAM.ListAccountAliases called unexpectedly")
}
func (s *stubGroupPolicyIAM) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	panic("stubGroupPolicyIAM.GetGroup called unexpectedly")
}
func (s *stubGroupPolicyIAM) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	panic("stubGroupPolicyIAM.GetPolicy called unexpectedly")
}
func (s *stubGroupPolicyIAM) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	panic("stubGroupPolicyIAM.GetPolicyVersion called unexpectedly")
}
func (s *stubGroupPolicyIAM) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	panic("stubGroupPolicyIAM.GetRolePolicy called unexpectedly")
}
func (s *stubGroupPolicyIAM) GetLoginProfile(_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	panic("stubGroupPolicyIAM.GetLoginProfile called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	panic("stubGroupPolicyIAM.ListMFADevices called unexpectedly")
}
func (s *stubGroupPolicyIAM) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	panic("stubGroupPolicyIAM.ListAccessKeys called unexpectedly")
}

// compile-time check
var _ awsclient.IAMAPI = (*stubGroupPolicyIAM)(nil)

// callPolicyFetcher invokes the registered "policy" paginated fetcher with the
// given IAM stub wired into ServiceClients.
func callPolicyFetcher(t *testing.T, stub *stubGroupPolicyIAM) ([]resource.Resource, error) {
	t.Helper()
	fetcher := resource.GetPaginatedFetcher("policy")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for 'policy' — internal/aws not imported?")
	}
	clients := &awsclient.ServiceClients{IAM: stub}
	result, err := fetcher(context.Background(), clients, "")
	return result.Resources, err
}

// TestFetchInlineGroupPolicies_ListGroupsError verifies that when ListGroups
// returns an error, fetchInlineGroupPolicies returns nil (no inline resources).
// The overall policy fetch still succeeds (ListPolicies returned empty), so
// the function returns an empty list rather than an error.
func TestFetchInlineGroupPolicies_ListGroupsError(t *testing.T) {
	stub := &stubGroupPolicyIAM{
		listGroupsErr: fmt.Errorf("iam: ListGroups access denied"),
	}
	resources, err := callPolicyFetcher(t, stub)
	if err != nil {
		t.Errorf("unexpected error from fetcher: %v", err)
	}
	// ListGroups failed → fetchInlineGroupPolicies returns nil → no inline resources
	for _, r := range resources {
		if r.Fields["policy_type"] == "inline" {
			t.Errorf("expected no inline resources when ListGroups fails, got: %v", r.Name)
		}
	}
}

// TestFetchInlineGroupPolicies_NilGroupName verifies that groups with a nil
// GroupName are skipped and produce no inline policy resources.
func TestFetchInlineGroupPolicies_NilGroupName(t *testing.T) {
	stub := &stubGroupPolicyIAM{
		listGroupsOut: &iam.ListGroupsOutput{
			Groups: []iamtypes.Group{
				{GroupName: nil, GroupId: aws.String("AGPA123"), Arn: aws.String("arn:aws:iam::123:group/nil-name"), Path: aws.String("/")},
			},
		},
	}
	resources, err := callPolicyFetcher(t, stub)
	if err != nil {
		t.Errorf("unexpected error from fetcher: %v", err)
	}
	// nil GroupName group is skipped → ListGroupPolicies never called → no inline resources
	for _, r := range resources {
		if r.Fields["policy_type"] == "inline" {
			t.Errorf("expected no inline resources for nil-GroupName group, got: %v", r.Name)
		}
	}
}

// TestFetchInlineGroupPolicies_ListGroupPoliciesError verifies that when
// ListGroupPolicies returns an error for a group, the function continues to
// the next group rather than returning nil for everything.
func TestFetchInlineGroupPolicies_ListGroupPoliciesError(t *testing.T) {
	stub := &stubGroupPolicyIAM{
		listGroupsOut: &iam.ListGroupsOutput{
			Groups: []iamtypes.Group{
				// Two groups: first will hit ListGroupPolicies error, second will succeed.
				// Since our stub returns the same error for all calls, we verify that
				// neither group panics and the function returns nil (all fail → continue).
				{GroupName: aws.String("group-a"), GroupId: aws.String("AGPA001"), Arn: aws.String("arn:aws:iam::123:group/group-a"), Path: aws.String("/")},
				{GroupName: aws.String("group-b"), GroupId: aws.String("AGPA002"), Arn: aws.String("arn:aws:iam::123:group/group-b"), Path: aws.String("/")},
			},
		},
		listGroupPoliciesErr: fmt.Errorf("iam: ListGroupPolicies throttled"),
	}
	resources, err := callPolicyFetcher(t, stub)
	if err != nil {
		t.Errorf("unexpected error from fetcher: %v", err)
	}
	// ListGroupPolicies errors → continue for both groups → no inline resources emitted
	for _, r := range resources {
		if r.Fields["policy_type"] == "inline" {
			t.Errorf("expected no inline resources when ListGroupPolicies errors, got: %v", r.Name)
		}
	}
}

// TestFetchInlineGroupPolicies_HappyPath verifies the success path: a group
// with a valid GroupName and successful ListGroupPolicies produces inline
// policy resources with correct field values.
func TestFetchInlineGroupPolicies_HappyPath(t *testing.T) {
	stub := &stubGroupPolicyIAM{
		listGroupsOut: &iam.ListGroupsOutput{
			Groups: []iamtypes.Group{
				{GroupName: aws.String("dev-group"), GroupId: aws.String("AGPA999"), Arn: aws.String("arn:aws:iam::123:group/dev-group"), Path: aws.String("/")},
			},
		},
		// listGroupPoliciesErr is nil → stub returns ["inline-pol"]
	}
	resources, err := callPolicyFetcher(t, stub)
	if err != nil {
		t.Errorf("unexpected error from fetcher: %v", err)
	}

	var found *resource.Resource
	for i := range resources {
		if resources[i].Name == "inline-pol" && resources[i].Fields["policy_type"] == "inline" {
			found = &resources[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected inline policy 'inline-pol' in results, not found")
	}
	if found.Fields["path"] != "inline/dev-group" {
		t.Errorf("Fields[\"path\"] = %q, want \"inline/dev-group\"", found.Fields["path"])
	}
	if found.Fields["attachment_count"] != "" {
		t.Errorf("Fields[\"attachment_count\"] = %q, want \"\" (inline policies have no count)", found.Fields["attachment_count"])
	}
}

// TestInlinePolicy_DetailShowsParentGroup reveals that checkPolicyGroup returns
// Count=0 for inline policies because policyARNFromResource returns "" for them
// (inline policies have no ARN). The checker exits early at line 117 of
// internal/aws/iam_policies_related.go without inspecting Fields["path"].
//
// For an inline policy with Fields["path"] == "inline/developers", the related
// panel must show "IAM Groups (1)" pointing to the developers group.
//
// This test WILL FAIL until checkPolicyGroup is fixed to extract the group name
// from Fields["path"] when the policy ARN is empty.
//
// NOTE: This test also requires TestIAMPolicyList_IncludesInlinePolicies to pass
// first (i.e. the fetcher must emit inline policy resources). If that bug is still
// present, this test will fail at the "find inline policy" step instead.
func TestInlinePolicy_DetailShowsParentGroup(t *testing.T) {
	m := newDemoColdCacheApp(t)
	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReadyMsg{Clients: clients, Gen: 0})

	// Fetch the policy list.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "policy",
	})
	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{policy}, got nil")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	loaded := raw.(messages.ResourcesLoadedMsg)
	*m, _ = rootApplyMsg(*m, loaded)

	// Find an inline policy with a known parent group.
	// AllowAssumeRole belongs to "developers" (Fields["path"] == "inline/developers").
	inlineIdx := -1
	for i, r := range loaded.Resources {
		if r.Fields["policy_type"] == "inline" && r.Name == "AllowAssumeRole" {
			inlineIdx = i
			break
		}
	}
	if inlineIdx == -1 {
		t.Fatal("inline policy 'AllowAssumeRole' not found in policy list; " +
			"fix TestIAMPolicyList_IncludesInlinePolicies bug first " +
			"(FetchIAMPoliciesPage must call ListGroupPolicies)")
	}

	inlinePolicy := loaded.Resources[inlineIdx]

	// Verify the path field encodes the parent group.
	if inlinePolicy.Fields["path"] != "inline/developers" {
		t.Fatalf("expected Fields[\"path\"] == \"inline/developers\", got %q",
			inlinePolicy.Fields["path"])
	}

	// Open detail for the inline policy — triggers related-check + enrichment commands.
	var batchCmd tea.Cmd
	*m, batchCmd = rootApplyMsg(*m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		Resource:     &inlinePolicy,
		ResourceType: "policy",
	})
	if batchCmd == nil {
		t.Fatal("expected cmd after opening inline policy detail; " +
			"are RelatedDefs/Enrichers registered for policy?")
	}

	// The returned cmd may be a batch (enrichment + related check).
	// Drain it to find the RelatedCheckStartedMsg.
	batchMsg := batchCmd()
	var started messages.RelatedCheckStartedMsg
	switch msg := batchMsg.(type) {
	case messages.RelatedCheckStartedMsg:
		started = msg
	case tea.BatchMsg:
		found := false
		for _, sub := range msg {
			if sub == nil {
				continue
			}
			subMsg := sub()
			if s, ok := subMsg.(messages.RelatedCheckStartedMsg); ok {
				started = s
				found = true
				break
			}
		}
		if !found {
			t.Fatal("batch did not contain RelatedCheckStartedMsg")
		}
	default:
		t.Fatalf("expected RelatedCheckStartedMsg or BatchMsg, got %T", batchMsg)
	}

	var checkCmds tea.Cmd
	*m, checkCmds = rootApplyMsg(*m, started)
	if checkCmds == nil {
		t.Fatal("handleRelatedCheckStarted returned nil cmd for inline policy")
	}

	runChecker := func(c tea.Cmd) (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = nil
			}
		}()
		return c()
	}

	var groupResult messages.RelatedCheckResultMsg
	var found bool

	rawCheck := runChecker(checkCmds)
	switch v := rawCheck.(type) {
	case messages.RelatedCheckResultMsg:
		if v.Result.TargetType == "iam-group" {
			groupResult = v
			found = true
		}
	case tea.BatchMsg:
		for _, subCmd := range v {
			if subCmd == nil {
				continue
			}
			sub := runChecker(subCmd)
			if r, ok2 := sub.(messages.RelatedCheckResultMsg); ok2 && r.Result.TargetType == "iam-group" {
				groupResult = r
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatal("no 'iam-group' RelatedCheckResultMsg found for inline policy detail; " +
			"is checkPolicyGroup registered as a RelatedDef for policy?")
	}

	// Fails until checkPolicyGroup extracts the group from Fields["path"]:
	// policyARNFromResource returns "" for inline policies → checker returns Count=0 at line 117.
	if groupResult.Result.Count < 1 {
		t.Errorf("inline policy 'AllowAssumeRole' (path=inline/developers) got IAM Groups Count=%d, want >=1; "+
			"BUG: checkPolicyGroup returns early when ARN is empty — "+
			"must extract group name from Fields[\"path\"] for inline policies "+
			"(internal/aws/iam_policies_related.go:116-117)",
			groupResult.Result.Count)
	}
}
