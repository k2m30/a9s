package unit

// qa_iam_policy_lazy_retry_test.go — Regression pin for the IAM policy lazy-add
// inline retry contract (Group H).
//
// Bug (fixed at HEAD): FetchIAMPoliciesByIDsFull previously set
// allInlinePoliciesBuilt=true even when fetchInlineGroupPolicies returned an
// error. This meant a transient ListGroupPolicies throttle permanently prevented
// inline policy resolution on subsequent calls within the same session.
//
// Contract after fix (internal/aws/iam_policies.go):
//   - When fetchInlineGroupPolicies returns (inlines, inlineErr) with inlineErr != nil,
//     store.InlineBuilt() remains false → next call retries the inline fetch.
//   - When fetchInlineGroupPolicies succeeds, store.MarkInlineBuilt() is called → cached.
//   - Partial inline results from the first (errored) call are incorporated AND
//     the composite error is propagated (never-silent-skip).
//   - The second call (after retry succeeds) finds the inline policy by name.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/session"
)

// iamPolicyRetryFake implements IAMAPI for the inline retry test.
// It embeds IAMAPI and overrides ListPolicies, ListGroups, ListGroupPolicies.
// listGroupPoliciesErr controls whether ListGroupPolicies fails.
type iamPolicyRetryFake struct {
	awsclient.IAMAPI

	// managedPolicies is returned by ListPolicies.
	managedPolicies []iamtypes.Policy

	// groups is returned by ListGroups.
	groups []iamtypes.Group

	// inlinePoliciesByGroup maps GroupName → inline policy names.
	inlinePoliciesByGroup map[string][]string

	// listGroupPoliciesErr is returned by ListGroupPolicies if non-nil.
	listGroupPoliciesErr error
}

// Compile-time interface check.
var _ awsclient.IAMAPI = (*iamPolicyRetryFake)(nil)

func (f *iamPolicyRetryFake) ListPolicies(
	_ context.Context,
	in *iam.ListPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{
		Policies:    f.managedPolicies,
		IsTruncated: false,
	}, nil
}

func (f *iamPolicyRetryFake) ListGroups(
	_ context.Context,
	_ *iam.ListGroupsInput,
	_ ...func(*iam.Options),
) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{Groups: f.groups}, nil
}

func (f *iamPolicyRetryFake) ListGroupPolicies(
	_ context.Context,
	in *iam.ListGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	if f.listGroupPoliciesErr != nil {
		return nil, f.listGroupPoliciesErr
	}
	groupName := ""
	if in != nil && in.GroupName != nil {
		groupName = *in.GroupName
	}
	return &iam.ListGroupPoliciesOutput{
		PolicyNames: f.inlinePoliciesByGroup[groupName],
	}, nil
}

// TestFetchIAMPoliciesByIDsFull_InlineRetryOnError verifies the retry contract:
// first call fails inline fetch → store.InlineBuilt() stays false;
// second call succeeds → inline policy found in result.
//
// Fails pre-fix: InlineBuilt was set true on error, so the second
// call skipped inline fetch and never found the inline policy by name.
// Passes post-fix: InlineBuilt remains false → second call retries.
func TestFetchIAMPoliciesByIDsFull_InlineRetryOnError(t *testing.T) {
	store := session.NewPolicyStore()

	const (
		managedPolicyID   = "arn:aws:iam::123456789012:policy/MyManagedPolicy"
		managedPolicyName = "MyManagedPolicy"
		inlineGroupName   = "ops-team"
		inlinePolicyName  = "inline-ops-policy"
	)

	// Build the fake: managed list always succeeds; inline fails first, then succeeds.
	fake := &iamPolicyRetryFake{
		managedPolicies: []iamtypes.Policy{
			{
				Arn:             aws.String(managedPolicyID),
				PolicyName:      aws.String(managedPolicyName),
				AttachmentCount: aws.Int32(1),
				IsAttachable:    true,
				Path:            aws.String("/"),
			},
		},
		groups: []iamtypes.Group{
			{GroupName: aws.String(inlineGroupName)},
		},
		inlinePoliciesByGroup: map[string][]string{
			inlineGroupName: {inlinePolicyName},
		},
		listGroupPoliciesErr: errors.New("throttled: rate exceeded"),
	}

	ctx := context.Background()

	// ── Call 1: inline fetch fails ──────────────────────────────────────────────
	// Contract:
	//   - Managed policy is found (buildAllManagedPolicies succeeds).
	//   - Inline fetch fails → composite error returned alongside partial results.
	//   - store.InlineBuilt() must remain false.
	results1, err1 := awsclient.FetchIAMPoliciesByIDsFull(ctx, fake, []string{managedPolicyID}, store)

	// Managed policy should be found even with inline failure.
	if len(results1) == 0 {
		t.Fatalf("call 1: expected managed policy in results even with inline error; got none (err: %v)", err1)
	}
	// Resources are indexed by PolicyName (not ARN), so check by name.
	foundManaged := false
	for _, r := range results1 {
		if r.ID == managedPolicyName || r.Name == managedPolicyName {
			foundManaged = true
		}
	}
	if !foundManaged {
		t.Errorf("call 1: managed policy %q not found in results (by name); results: %v", managedPolicyName, results1)
	}
	if err1 == nil {
		t.Error("call 1: expected composite error (inline fetch throttled), got nil — " +
			"partial failures must be surfaced (never-silent-skip)")
	}

	// ── Call 2: fix inline fake to succeed ────────────────────────────────────
	// Now remove the inline error so retry can succeed.
	fake.listGroupPoliciesErr = nil

	results2, err2 := awsclient.FetchIAMPoliciesByIDsFull(ctx, fake, []string{inlinePolicyName}, store)

	if err2 != nil {
		t.Errorf("call 2: expected no error after inline retry; got %v", err2)
	}
	foundInline := false
	for _, r := range results2 {
		if r.ID == inlinePolicyName || r.Name == inlinePolicyName {
			foundInline = true
		}
	}
	if !foundInline {
		// CONTRACT ASSERTION: this fails pre-fix (InlineBuilt=true meant
		// inline fetch never retried, so inlinePolicyName was never added to cache).
		t.Errorf("call 2: inline policy %q not found — "+
			"PRE-FIX BUG: InlineBuilt was set true on error, preventing retry; "+
			"InlineBuilt must stay false on inline error so next call retries",
			inlinePolicyName)
	}
}

// TestFetchIAMPoliciesByIDsFull_InlineCachedOnSuccess verifies that when
// inline fetch succeeds, store.InlineBuilt() is set to true, and subsequent
// calls do NOT re-invoke ListGroupPolicies (cache hit).
func TestFetchIAMPoliciesByIDsFull_InlineCachedOnSuccess(t *testing.T) {
	store := session.NewPolicyStore()

	const inlinePolicyName = "inline-cached-policy"
	const inlineGroupName = "dev-team"

	listGroupPoliciesCallCount := 0
	fake := &iamPolicyRetryFake{
		managedPolicies: []iamtypes.Policy{},
		groups:          []iamtypes.Group{{GroupName: aws.String(inlineGroupName)}},
		inlinePoliciesByGroup: map[string][]string{
			inlineGroupName: {inlinePolicyName},
		},
	}

	// Wrap ListGroupPolicies to count calls.
	original := fake
	countingFake := &countingIAMPolicyFake{
		iamPolicyRetryFake: original,
		onListGroupPolicies: func() {
			listGroupPoliciesCallCount++
		},
	}

	ctx := context.Background()

	// Call 1: inline fetch succeeds → should call ListGroupPolicies.
	_, err1 := awsclient.FetchIAMPoliciesByIDsFull(ctx, countingFake, []string{inlinePolicyName}, store)
	if err1 != nil {
		t.Errorf("call 1: unexpected error: %v", err1)
	}
	if listGroupPoliciesCallCount == 0 {
		t.Error("call 1: expected ListGroupPolicies to be called at least once")
	}
	callsAfterFirst := listGroupPoliciesCallCount

	// Call 2: inline cache should be warm — ListGroupPolicies must NOT be called again.
	_, err2 := awsclient.FetchIAMPoliciesByIDsFull(ctx, countingFake, []string{inlinePolicyName}, store)
	if err2 != nil {
		t.Errorf("call 2: unexpected error: %v", err2)
	}
	if listGroupPoliciesCallCount > callsAfterFirst {
		t.Errorf("call 2: ListGroupPolicies called %d times total, want %d (cache should be warm after successful call 1)",
			listGroupPoliciesCallCount, callsAfterFirst)
	}
}

// countingIAMPolicyFake wraps iamPolicyRetryFake to count ListGroupPolicies calls.
type countingIAMPolicyFake struct {
	*iamPolicyRetryFake
	onListGroupPolicies func()
}

var _ awsclient.IAMAPI = (*countingIAMPolicyFake)(nil)

func (f *countingIAMPolicyFake) ListGroupPolicies(
	ctx context.Context,
	in *iam.ListGroupPoliciesInput,
	optFns ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	f.onListGroupPolicies()
	return f.iamPolicyRetryFake.ListGroupPolicies(ctx, in, optFns...)
}
