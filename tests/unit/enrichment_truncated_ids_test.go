package unit

// enrichment_truncated_ids_test.go — Contract tests for EnricherResult.TruncatedIDs.
//
// TruncatedIDs is a per-resource truncation map (map[string]bool) signalling a
// per-row "?" coverage gap (API error or cap hit) independent of the aggregate
// Truncated bool. When an enricher bails on a specific resource, it MUST set
// result.TruncatedIDs[resourceID] = true. The aggregate result.Truncated only
// follows when the coverage gap could hide a severity "!" (SevBroken) finding
// — i.e. the enricher's IssueCount is a lower bound. Enrichers that emit only
// "~" (informational) findings, like iam-group, MUST leave Truncated false
// regardless of TruncatedIDs: a coverage gap in informational-only data never
// lower-bounds the issue badge (see core/aws/issue_enrichment.go
// IssueEnricherResult.Truncated godoc).
//
// Tests use existing fake infrastructure from aws_iam_group_enricher_test.go
// and aws_eventbridge_pagination_test.go (same package unit).
//
// Tests:
//   1. TestEnrichIAMGroup_TruncatedIDsPopulatedOnPerResourceErrorNotBadge:
//      GetGroup errors on the second group → TruncatedIDs["second-group"] == true,
//      TruncatedIDs["first-group"] == false (first succeeded), Truncated == false
//      (iam-group is "~"-only — the gap never lower-bounds the issue badge).
//   2. TestEnrichEventBridgeRuleTargets_TruncatedIDsPopulatedOnCapHit:
//      NextToken always set → after PerParentPageCap pages, TruncatedIDs[ruleID] == true.
//      (eb-rule can emit SevBroken findings, so its Truncated follows IssueCount
//      lower-bound rules independently of this file's iam-group case.)
//   3. TestEnricher_TruncatedIDs_IsSubsetOfResourceIDs:
//      Every key in TruncatedIDs must have been in the input resource IDs. No phantom keys.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// ---------------------------------------------------------------------------
// Error-on-second-call IAM fake
// ---------------------------------------------------------------------------

// iamGroupErrorOnSecondFake returns a successful GetGroup for the first group
// and an error for the second. Used to trigger per-resource truncation.
type iamGroupErrorOnSecondFake struct {
	awsclient.IAMAPI

	// mu guards callOrder, which is written concurrently: EnrichIAMGroup fans
	// out GetGroup calls per resource via core/aws.ForEachParallel
	// (EnrichmentParallelism goroutines).
	mu sync.Mutex
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
	f.mu.Lock()
	f.callOrder = append(f.callOrder, name)
	f.mu.Unlock()
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

// TestEnrichIAMGroup_TruncatedIDsPopulatedOnPerResourceErrorNotBadge verifies
// that when GetGroup returns an error for a specific group,
// TruncatedIDs[groupID] is true for that group, TruncatedIDs[otherGroupID]
// is false (succeeded), and the global Truncated flag stays false — iam-group
// is a "~"-only enricher (IssueCount always 0), so a per-resource coverage
// gap must never lower-bound the aggregate issue badge.
func TestEnrichIAMGroup_TruncatedIDsPopulatedOnPerResourceErrorNotBadge(t *testing.T) {
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

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}

	// Global Truncated must stay false: iam-group is a "~"-only enricher, so
	// a per-resource API error marks the row via TruncatedIDs, never the
	// aggregate issue badge.
	if result.Truncated {
		t.Error("Truncated must stay false when a per-resource API call errors — the gap is signalled via TruncatedIDs, not the badge")
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

	result, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rules, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pagination was capped.
	calls := fake.callsFor(ruleName)
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

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources, nil)
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
