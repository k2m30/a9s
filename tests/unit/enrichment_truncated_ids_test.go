package unit

// enrichment_truncated_ids_test.go — Contract tests for EnricherResult.TruncatedIDs.
//
// TruncatedIDs is a per-resource truncation map (map[string]bool) that replaces the
// coarse global Truncated bool for fine-grained UI resolution. When an enricher
// bails on a specific resource (API error or cap hit), it MUST set:
//   - result.TruncatedIDs[resourceID] = true
//   - result.Truncated = true  (both signals survive)
//
// Tests use existing fake infrastructure from aws_iam_group_enricher_test.go
// and aws_eventbridge_pagination_test.go (same package unit).
//
// Tests:
//   1. TestEnrichIAMGroup_TruncatedIDsPopulatedOnPerResourceError:
//      GetGroup errors on the second group → TruncatedIDs["second-group"] == true,
//      TruncatedIDs["first-group"] == false (first succeeded), Truncated == true.
//   2. TestEnrichEventBridgeRuleTargets_TruncatedIDsPopulatedOnCapHit:
//      NextToken always set → after PerParentPageCap pages, TruncatedIDs[ruleID] == true.
//   3. TestEnricher_TruncatedIDs_IsSubsetOfResourceIDs:
//      Every key in TruncatedIDs must have been in the input resource IDs. No phantom keys.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Error-on-second-call IAM fake
// ---------------------------------------------------------------------------

// iamGroupErrorOnSecondFake returns a successful GetGroup for the first group
// and an error for the second. Used to trigger per-resource truncation.
type iamGroupErrorOnSecondFake struct {
	awsclient.IAMAPI

	// callOrder records the order of GetGroup calls by group name.
	callOrder []string

	// errorOnGroup is the group name that triggers an error.
	errorOnGroup string

	// usersByGroup maps group name → users returned on success.
	usersByGroup map[string][]iamtypes.User

	// attachedPoliciesByGroup maps group name → attached policies.
	attachedPoliciesByGroup map[string][]iamtypes.AttachedPolicy
}

func (f *iamGroupErrorOnSecondFake) GetGroup(
	_ context.Context,
	in *iam.GetGroupInput,
	_ ...func(*iam.Options),
) (*iam.GetGroupOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	f.callOrder = append(f.callOrder, name)
	if name == f.errorOnGroup {
		return nil, errors.New("simulated GetGroup API error for " + name)
	}
	users := f.usersByGroup[name]
	return &iam.GetGroupOutput{
		Group: &iamtypes.Group{GroupName: aws.String(name)},
		Users: users,
	}, nil
}

func (f *iamGroupErrorOnSecondFake) ListAttachedGroupPolicies(
	_ context.Context,
	in *iam.ListAttachedGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListAttachedGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	policies := f.attachedPoliciesByGroup[name]
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: policies}, nil
}

func (f *iamGroupErrorOnSecondFake) ListGroupPolicies(
	_ context.Context,
	in *iam.ListGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{PolicyNames: []string{}}, nil
}

// Compile-time check.
var _ awsclient.IAMAPI = (*iamGroupErrorOnSecondFake)(nil)

// ---------------------------------------------------------------------------
// Test 1: per-resource error → TruncatedIDs populated
// ---------------------------------------------------------------------------

// TestEnrichIAMGroup_TruncatedIDsPopulatedOnPerResourceError verifies that when
// GetGroup returns an error for a specific group, TruncatedIDs[groupID] is true
// for that group, TruncatedIDs[otherGroupID] is false (succeeded), and the global
// Truncated flag is also true.
func TestEnrichIAMGroup_TruncatedIDsPopulatedOnPerResourceError(t *testing.T) {
	const firstGroup = "dev-team"
	const secondGroup = "ops-team"

	fake := &iamGroupErrorOnSecondFake{
		errorOnGroup: secondGroup,
		usersByGroup: map[string][]iamtypes.User{
			firstGroup: {iamGroupUser("alice")},
		},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{
			firstGroup: {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
		},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(firstGroup, secondGroup)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}

	// Global Truncated must be true because at least one resource was skipped.
	if !result.Truncated {
		t.Error("Truncated must be true when a per-resource API call errors")
	}

	// TruncatedIDs must carry a true entry for the failing group.
	if result.TruncatedIDs == nil {
		t.Fatal("TruncatedIDs must not be nil when a per-resource error occurs")
	}
	if !result.TruncatedIDs[secondGroup] {
		t.Errorf("TruncatedIDs[%q] = false, want true (GetGroup returned error)", secondGroup)
	}

	// The first group succeeded; it must NOT appear in TruncatedIDs (or be false).
	if result.TruncatedIDs[firstGroup] {
		t.Errorf("TruncatedIDs[%q] = true, want false (GetGroup succeeded)", firstGroup)
	}
}

// ---------------------------------------------------------------------------
// Test 2: pagination cap hit → TruncatedIDs populated for the capped rule
// ---------------------------------------------------------------------------

// TestEnrichEventBridgeRuleTargets_TruncatedIDsPopulatedOnCapHit verifies that when
// ListTargetsByRule always returns a NextToken (simulating a huge rule), after
// PerParentPageCap pages the enricher marks the rule as truncated:
//   - result.TruncatedIDs[ruleID] == true
//   - result.Truncated == true
func TestEnrichEventBridgeRuleTargets_TruncatedIDsPopulatedOnCapHit(t *testing.T) {
	const ruleName = "huge-rule-truncated"

	fake := newEBPaginatedFake()

	// Build PerParentPageCap+2 pages, all with NextToken set.
	pages := make([]*eventbridge.ListTargetsByRuleOutput, awsclient.PerParentPageCap+2)
	for i := range pages {
		pages[i] = &eventbridge.ListTargetsByRuleOutput{
			Targets:   makeEBTargetsWithDLQ(100),
			NextToken: aws.String(fmt.Sprintf("token-%d", i+1)),
		}
	}
	fake.pages[ruleName] = pages

	rules := ebRuleResources(struct {
		name  string
		state string
		bus   string
	}{ruleName, "ENABLED", "default"})

	clients := &awsclient.ServiceClients{EventBridge: fake}

	result, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pagination was capped.
	calls := fake.callCounts[ruleName]
	if calls != awsclient.PerParentPageCap {
		t.Errorf("ListTargetsByRule called %d times, want %d (PerParentPageCap)", calls, awsclient.PerParentPageCap)
	}

	// Global Truncated must be true.
	if !result.Truncated {
		t.Error("Truncated must be true when pagination cap is hit")
	}

	// TruncatedIDs must carry a true entry for the capped rule.
	if result.TruncatedIDs == nil {
		t.Fatal("TruncatedIDs must not be nil when per-rule pagination cap is hit")
	}
	if !result.TruncatedIDs[ruleName] {
		t.Errorf("TruncatedIDs[%q] = false, want true (pagination cap hit)", ruleName)
	}
}

// ---------------------------------------------------------------------------
// Test 3: every key in TruncatedIDs must be in input resource IDs
// ---------------------------------------------------------------------------

// TestEnricher_TruncatedIDs_IsSubsetOfResourceIDs asserts the invariant that
// no key in TruncatedIDs is a phantom — every key must correspond to an ID in
// the input resources slice (or be the empty string, which no enricher should
// produce). This guards against enrichers accidentally keying on derived strings
// (ARNs, service names) instead of resource.Resource.ID.
//
// We exercise EnrichIAMGroup with a controlled truncation scenario.
func TestEnricher_TruncatedIDs_IsSubsetOfResourceIDs(t *testing.T) {
	const firstGroup = "team-alpha"
	const secondGroup = "team-beta"

	fake := &iamGroupErrorOnSecondFake{
		errorOnGroup: secondGroup,
		usersByGroup: map[string][]iamtypes.User{
			firstGroup: {iamGroupUser("user1")},
		},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{
			firstGroup: {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
		},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(firstGroup, secondGroup)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build the input ID set.
	inputIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		inputIDs[r.ID] = true
	}

	// Every key in TruncatedIDs must be in the input ID set.
	for id := range result.TruncatedIDs {
		if id == "" {
			t.Error("TruncatedIDs contains an empty-string key — enricher must key by resource.Resource.ID")
			continue
		}
		if !inputIDs[id] {
			t.Errorf("TruncatedIDs[%q] is a phantom — %q was not in the input resources slice", id, id)
		}
	}
}
