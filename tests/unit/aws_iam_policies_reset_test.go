package unit

// aws_iam_policies_reset_test.go — pins for the PolicyStore-based IAM policy
// cache.

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/session"
)

// countingListPoliciesAPI is an IAMAPI mock that returns a single policy from
// ListPolicies (counting calls) and stubs ListGroups empty — FetchIAMPoliciesByIDsFull
// always runs the inline group-policy sweep too, so a bare ListPolicies-only
// mock would panic through the embedded nil IAMAPI on the first ListGroups call.
type countingListPoliciesAPI struct {
	awsclient.IAMAPI
	calls      atomic.Int64
	policyName string
}

func (f *countingListPoliciesAPI) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	f.calls.Add(1)
	return &iam.ListPoliciesOutput{
		Policies: []iamtypes.Policy{
			{
				PolicyName:      aws.String(f.policyName),
				Arn:             aws.String("arn:aws:iam::123456789012:policy/" + f.policyName),
				AttachmentCount: aws.Int32(0),
				IsAttachable:    true,
				Path:            aws.String("/"),
			},
		},
		IsTruncated: false,
	}, nil
}

func (f *countingListPoliciesAPI) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

var _ awsclient.IAMAPI = (*countingListPoliciesAPI)(nil)

// TestPolicyStore_ClearForcesRebuild verifies that after store.Clear() is
// called, the next FetchIAMPoliciesByIDs call rebuilds the cache from the
// new API client — not from the stale prior call.
func TestPolicyStore_ClearForcesRebuild(t *testing.T) {
	store := session.NewPolicyStore()

	mock1 := &countingListPoliciesAPI{policyName: "policy-A"}

	res1, err := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), mock1, []string{"policy-A"}, store, "aws")
	if err != nil {
		t.Fatalf("FetchIAMPoliciesByIDs (mock1, first call): unexpected error: %v", err)
	}
	if mock1.calls.Load() != 1 {
		t.Errorf("mock1.calls after first call: want 1, got %d", mock1.calls.Load())
	}
	if len(res1) != 1 || res1[0].ID != "arn:aws:iam::123456789012:policy/policy-A" {
		t.Errorf("first call: want [{ID:arn:aws:iam::123456789012:policy/policy-A}], got %v", res1)
	}

	res2, err := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), mock1, []string{"policy-A"}, store, "aws")
	if err != nil {
		t.Fatalf("FetchIAMPoliciesByIDs (mock1, second call): unexpected error: %v", err)
	}
	if mock1.calls.Load() != 1 {
		t.Errorf("mock1.calls after second call (cache hit): want 1, got %d — cache not working", mock1.calls.Load())
	}
	if len(res2) != 1 || res2[0].ID != "arn:aws:iam::123456789012:policy/policy-A" {
		t.Errorf("second call: want [{ID:arn:aws:iam::123456789012:policy/policy-A}], got %v", res2)
	}

	store.Clear()

	mock2 := &countingListPoliciesAPI{policyName: "policy-B"}
	res3, err := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), mock2, []string{"policy-B"}, store, "aws")
	if err != nil {
		t.Fatalf("FetchIAMPoliciesByIDs (mock2, post-clear): unexpected error: %v", err)
	}
	if mock2.calls.Load() != 1 {
		t.Errorf("mock2.calls after clear+fetch: want 1, got %d", mock2.calls.Load())
	}
	if len(res3) != 1 || res3[0].ID != "arn:aws:iam::123456789012:policy/policy-B" {
		t.Errorf("post-clear call: want [{ID:arn:aws:iam::123456789012:policy/policy-B}], got %v — stale cache from mock1 not cleared", res3)
	}
}

// TestPolicyStore_ClearIdempotent verifies that calling store.Clear() twice
// in a row without an intervening fetch does not panic or produce an error.
func TestPolicyStore_ClearIdempotent(t *testing.T) {
	store := session.NewPolicyStore()

	store.Clear()
	store.Clear()
}
