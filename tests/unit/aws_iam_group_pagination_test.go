package unit

// aws_iam_group_pagination_test.go — Failing tests for EnrichIAMGroup pagination.
//
// These tests document the REQUIRED behavior after the coder implements
// pagination for GetGroup, ListAttachedGroupPolicies, and ListGroupPolicies.
//
// All three operations use IAM's Marker/IsTruncated pagination pattern.
// After pagination, counts are exact (not approximate) unless the walk is
// capped at PerParentPageCap = 10 pages, in which case the value carries
// a "+" suffix to signal approximate.
//
// Contract assertions:
//   - GetGroup returns 2 pages (100+50 users) → Fields["member_count"] == "150"
//   - ListAttachedGroupPolicies returns 2 pages (60+30) → no "no policies" finding
//   - ListGroupPolicies returns 2 pages (20+15) → correct aggregation
//   - GetGroup always truncated (huge group) → capped at PerParentPageCap pages, "1000+"
//   - GetGroup returns 0 users across all pages → "0", finding "group has no members"

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Pagination-aware fakes
// ---------------------------------------------------------------------------

// iamGroupPaginatedFake handles GetGroup with Marker/IsTruncated pagination.
// Each call to GetGroup advances an internal call counter per group name,
// returning the corresponding page from the pages map.
//
// Keyed by group name → ordered list of pages. Each call reads the next page.
type iamGroupPaginatedFake struct {
	awsclient.IAMAPI

	// getGroupPages maps groupName → ordered pages of GetGroupOutput.
	// Each successive GetGroup call (matching by Marker) returns the next page.
	getGroupPages map[string][]*iam.GetGroupOutput

	// attachedPoliciesPages maps groupName → ordered pages of ListAttachedGroupPoliciesOutput.
	attachedPoliciesPages map[string][]*iam.ListAttachedGroupPoliciesOutput

	// inlinePoliciesPages maps groupName → ordered pages of ListGroupPoliciesOutput.
	inlinePoliciesPages map[string][]*iam.ListGroupPoliciesOutput

	// call counters (per group)
	getGroupCalls         map[string]int
	attachedPoliciesCalls map[string]int
	inlinePoliciesCalls   map[string]int
}

func newIAMGroupPaginatedFake() *iamGroupPaginatedFake {
	return &iamGroupPaginatedFake{
		getGroupPages:         make(map[string][]*iam.GetGroupOutput),
		attachedPoliciesPages: make(map[string][]*iam.ListAttachedGroupPoliciesOutput),
		inlinePoliciesPages:   make(map[string][]*iam.ListGroupPoliciesOutput),
		getGroupCalls:         make(map[string]int),
		attachedPoliciesCalls: make(map[string]int),
		inlinePoliciesCalls:   make(map[string]int),
	}
}

func (f *iamGroupPaginatedFake) GetGroup(
	_ context.Context,
	in *iam.GetGroupInput,
	_ ...func(*iam.Options),
) (*iam.GetGroupOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	pages := f.getGroupPages[name]
	idx := f.getGroupCalls[name]
	f.getGroupCalls[name] = idx + 1
	if idx >= len(pages) {
		// No more pages defined — return a final empty page.
		return &iam.GetGroupOutput{
			Group: &iamtypes.Group{GroupName: aws.String(name)},
			Users: []iamtypes.User{},
		}, nil
	}
	return pages[idx], nil
}

func (f *iamGroupPaginatedFake) ListAttachedGroupPolicies(
	_ context.Context,
	in *iam.ListAttachedGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListAttachedGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	pages := f.attachedPoliciesPages[name]
	idx := f.attachedPoliciesCalls[name]
	f.attachedPoliciesCalls[name] = idx + 1
	if idx >= len(pages) {
		return &iam.ListAttachedGroupPoliciesOutput{
			AttachedPolicies: []iamtypes.AttachedPolicy{},
		}, nil
	}
	return pages[idx], nil
}

func (f *iamGroupPaginatedFake) ListGroupPolicies(
	_ context.Context,
	in *iam.ListGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	pages := f.inlinePoliciesPages[name]
	idx := f.inlinePoliciesCalls[name]
	f.inlinePoliciesCalls[name] = idx + 1
	if idx >= len(pages) {
		return &iam.ListGroupPoliciesOutput{PolicyNames: []string{}}, nil
	}
	return pages[idx], nil
}

// Compile-time check: iamGroupPaginatedFake satisfies IAMAPI.
var _ awsclient.IAMAPI = (*iamGroupPaginatedFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeUsers builds a slice of n minimal iamtypes.User stubs.
func makeUsers(n int) []iamtypes.User {
	users := make([]iamtypes.User, n)
	for i := range users {
		users[i] = iamtypes.User{UserName: aws.String(fmt.Sprintf("user-%d", i))}
	}
	return users
}

// makeAttachedPolicies builds a slice of n minimal AttachedPolicy stubs.
func makeAttachedPolicies(n int) []iamtypes.AttachedPolicy {
	policies := make([]iamtypes.AttachedPolicy, n)
	for i := range policies {
		policies[i] = iamtypes.AttachedPolicy{
			PolicyArn:  aws.String(fmt.Sprintf("arn:aws:iam::aws:policy/Policy%d", i)),
			PolicyName: aws.String(fmt.Sprintf("Policy%d", i)),
		}
	}
	return policies
}

// makeInlinePolicyNames builds a slice of n inline policy name strings.
func makeInlinePolicyNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("inline-policy-%d", i)
	}
	return names
}

// ---------------------------------------------------------------------------
// Test: GetGroup pagination — 2 pages (100 + 50 users)
// ---------------------------------------------------------------------------

// TestEnrichIAMGroup_PaginatesGetGroupMembers verifies that the enricher
// follows Marker/IsTruncated across two GetGroup pages and writes the total
// count (150) to Fields["member_count"].
func TestEnrichIAMGroup_PaginatesGetGroupMembers(t *testing.T) {
	const groupName = "dev-team"

	fake := newIAMGroupPaginatedFake()

	// Page 1: 100 users, IsTruncated=true, Marker="m1"
	fake.getGroupPages[groupName] = []*iam.GetGroupOutput{
		{
			Group:       &iamtypes.Group{GroupName: aws.String(groupName)},
			Users:       makeUsers(100),
			IsTruncated: true,
			Marker:      aws.String("m1"),
		},
		// Page 2: 50 users, IsTruncated=false
		{
			Group:       &iamtypes.Group{GroupName: aws.String(groupName)},
			Users:       makeUsers(50),
			IsTruncated: false,
		},
	}
	// ListAttachedGroupPolicies: 1 page, 1 policy (so no "no policies" finding)
	fake.attachedPoliciesPages[groupName] = []*iam.ListAttachedGroupPoliciesOutput{
		{AttachedPolicies: makeAttachedPolicies(1), IsTruncated: false},
	}
	// ListGroupPolicies: 1 page, empty
	fake.inlinePoliciesPages[groupName] = []*iam.ListGroupPoliciesOutput{
		{PolicyNames: []string{}, IsTruncated: false},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(groupName)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify member_count reflects both pages
	updates, ok := result.FieldUpdates[groupName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", groupName)
	}
	wantCount := "150"
	if updates["member_count"] != wantCount {
		t.Errorf("member_count = %q, want %q", updates["member_count"], wantCount)
	}

	// GetGroup must have been called twice (once per page)
	calls := fake.getGroupCalls[groupName]
	if calls != 2 {
		t.Errorf("GetGroup called %d times, want 2", calls)
	}

	// No findings expected (group has members and policies)
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

// ---------------------------------------------------------------------------
// Test: ListAttachedGroupPolicies pagination — 2 pages (60 + 30 policies)
// ---------------------------------------------------------------------------

// TestEnrichIAMGroup_PaginatesAttachedPolicies verifies that the enricher
// aggregates attached policies across two pages and suppresses the
// "no policies" finding when the total is non-zero.
func TestEnrichIAMGroup_PaginatesAttachedPolicies(t *testing.T) {
	const groupName = "ops-team"

	fake := newIAMGroupPaginatedFake()

	// GetGroup: 1 page, 0 members (to keep this test focused on policies only)
	fake.getGroupPages[groupName] = []*iam.GetGroupOutput{
		{
			Group:       &iamtypes.Group{GroupName: aws.String(groupName)},
			Users:       makeUsers(1), // 1 member so "no members" finding is not emitted
			IsTruncated: false,
		},
	}
	// ListAttachedGroupPolicies: 2 pages, 60+30 policies
	fake.attachedPoliciesPages[groupName] = []*iam.ListAttachedGroupPoliciesOutput{
		{
			AttachedPolicies: makeAttachedPolicies(60),
			IsTruncated:      true,
			Marker:           aws.String("ap-marker"),
		},
		{
			AttachedPolicies: makeAttachedPolicies(30),
			IsTruncated:      false,
		},
	}
	// ListGroupPolicies: 1 page, empty
	fake.inlinePoliciesPages[groupName] = []*iam.ListGroupPoliciesOutput{
		{PolicyNames: []string{}, IsTruncated: false},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(groupName)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Because total attached policies (90) > 0, no "no policies" finding
	if f, ok := result.Findings[groupName]; ok {
		if strings.Contains(f.Summary, "no policies") {
			t.Errorf("must not emit 'no policies' finding when attached policies span 2 pages; got: %q", f.Summary)
		}
	}

	// ListAttachedGroupPolicies must have been called twice
	calls := fake.attachedPoliciesCalls[groupName]
	if calls != 2 {
		t.Errorf("ListAttachedGroupPolicies called %d times, want 2", calls)
	}
}

// ---------------------------------------------------------------------------
// Test: ListGroupPolicies pagination — 2 pages (20 + 15 policy names)
// ---------------------------------------------------------------------------

// TestEnrichIAMGroup_PaginatesInlinePolicies verifies that the enricher
// aggregates inline policy names across two pages and suppresses the
// "no policies" finding when the total is non-zero.
func TestEnrichIAMGroup_PaginatesInlinePolicies(t *testing.T) {
	const groupName = "sec-team"

	fake := newIAMGroupPaginatedFake()

	// GetGroup: 1 page, 1 member
	fake.getGroupPages[groupName] = []*iam.GetGroupOutput{
		{
			Group:       &iamtypes.Group{GroupName: aws.String(groupName)},
			Users:       makeUsers(1),
			IsTruncated: false,
		},
	}
	// ListAttachedGroupPolicies: 1 page, empty
	fake.attachedPoliciesPages[groupName] = []*iam.ListAttachedGroupPoliciesOutput{
		{AttachedPolicies: []iamtypes.AttachedPolicy{}, IsTruncated: false},
	}
	// ListGroupPolicies: 2 pages, 20+15 policy names
	fake.inlinePoliciesPages[groupName] = []*iam.ListGroupPoliciesOutput{
		{
			PolicyNames: makeInlinePolicyNames(20),
			IsTruncated: true,
			Marker:      aws.String("inline-marker"),
		},
		{
			PolicyNames: makeInlinePolicyNames(15),
			IsTruncated: false,
		},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(groupName)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Because total inline policies (35) > 0, no "no policies" finding
	if f, ok := result.Findings[groupName]; ok {
		if strings.Contains(f.Summary, "no policies") {
			t.Errorf("must not emit 'no policies' finding when inline policies span 2 pages; got: %q", f.Summary)
		}
	}

	// ListGroupPolicies must have been called twice
	calls := fake.inlinePoliciesCalls[groupName]
	if calls != 2 {
		t.Errorf("ListGroupPolicies called %d times, want 2", calls)
	}
}

// ---------------------------------------------------------------------------
// Test: GetGroup pagination capped at PerParentPageCap → "1000+"
// ---------------------------------------------------------------------------

// TestEnrichIAMGroup_CappedAtPerParentPageCap verifies that when GetGroup
// always returns IsTruncated=true (simulating a huge group), the enricher
// stops after PerParentPageCap pages and writes "1000+" to member_count.
func TestEnrichIAMGroup_CappedAtPerParentPageCap(t *testing.T) {
	const groupName = "huge-group"

	fake := newIAMGroupPaginatedFake()

	// Build PerParentPageCap+2 pages, all with IsTruncated=true, 100 users each.
	// The enricher should stop at exactly PerParentPageCap pages.
	pages := make([]*iam.GetGroupOutput, awsclient.PerParentPageCap+2)
	for i := range pages {
		pages[i] = &iam.GetGroupOutput{
			Group:       &iamtypes.Group{GroupName: aws.String(groupName)},
			Users:       makeUsers(100),
			IsTruncated: true,
			Marker:      aws.String(fmt.Sprintf("marker-%d", i+1)),
		}
	}
	fake.getGroupPages[groupName] = pages

	// ListAttachedGroupPolicies: single page, 1 policy
	fake.attachedPoliciesPages[groupName] = []*iam.ListAttachedGroupPoliciesOutput{
		{AttachedPolicies: makeAttachedPolicies(1), IsTruncated: false},
	}
	// ListGroupPolicies: single page, empty
	fake.inlinePoliciesPages[groupName] = []*iam.ListGroupPoliciesOutput{
		{PolicyNames: []string{}, IsTruncated: false},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(groupName)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GetGroup must be called exactly PerParentPageCap times
	calls := fake.getGroupCalls[groupName]
	if calls != awsclient.PerParentPageCap {
		t.Errorf("GetGroup called %d times, want exactly %d (PerParentPageCap)", calls, awsclient.PerParentPageCap)
	}

	// member_count must carry "+" suffix to indicate approximate
	updates, ok := result.FieldUpdates[groupName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", groupName)
	}
	mc := updates["member_count"]
	if !strings.HasSuffix(mc, "+") {
		t.Errorf("member_count = %q, want suffix \"+\" (approximate)", mc)
	}
	// The numeric part must be PerParentPageCap * 100 = 1000
	wantPrefix := fmt.Sprintf("%d+", awsclient.PerParentPageCap*100)
	if mc != wantPrefix {
		t.Errorf("member_count = %q, want %q", mc, wantPrefix)
	}
}

// ---------------------------------------------------------------------------
// Test: zero members across all pages → "0", finding emitted
// ---------------------------------------------------------------------------

// TestEnrichIAMGroup_ZeroMembersAcrossPages verifies that when all pages
// return 0 users, member_count is "0" and the "group has no members" finding
// is emitted.
func TestEnrichIAMGroup_ZeroMembersAcrossPages(t *testing.T) {
	const groupName = "empty-group"

	fake := newIAMGroupPaginatedFake()

	// GetGroup: 1 page, 0 users, not truncated
	fake.getGroupPages[groupName] = []*iam.GetGroupOutput{
		{
			Group:       &iamtypes.Group{GroupName: aws.String(groupName)},
			Users:       []iamtypes.User{},
			IsTruncated: false,
		},
	}
	// ListAttachedGroupPolicies: 1 policy (so no "no policies" finding)
	fake.attachedPoliciesPages[groupName] = []*iam.ListAttachedGroupPoliciesOutput{
		{AttachedPolicies: makeAttachedPolicies(1), IsTruncated: false},
	}
	// ListGroupPolicies: empty
	fake.inlinePoliciesPages[groupName] = []*iam.ListGroupPoliciesOutput{
		{PolicyNames: []string{}, IsTruncated: false},
	}

	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources(groupName)

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// member_count must be "0"
	updates, ok := result.FieldUpdates[groupName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", groupName)
	}
	if updates["member_count"] != "0" {
		t.Errorf("member_count = %q, want \"0\"", updates["member_count"])
	}

	// Finding "group has no members (orphan)" must be emitted
	f, ok := result.Findings[groupName]
	if !ok {
		t.Fatalf("expected finding for %q (no members), but none was produced", groupName)
	}
	if !strings.Contains(f.Summary, "no members") {
		t.Errorf("finding summary %q must contain \"no members\"", f.Summary)
	}
}
