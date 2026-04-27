package unit

// aws_iam_policies_reset_test.go — Regression pins for the PolicyStore-based
// IAM policy cache (replaces the former package-level ResetIAMPoliciesCache).
//
// Two Scenarios verified:
//   TestPolicyStore_ClearForcesRebuild — proves the cache is rebuilt
//     from a new mock after store.Clear(), not from stale prior data.
//   TestPolicyStore_ClearIdempotent — calling Clear twice in a row must not panic.

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/session"
)

// countingListPoliciesAPI is a minimal IAMListPoliciesAPI mock that returns
// a single policy and counts how many times ListPolicies is called.
type countingListPoliciesAPI struct {
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

// Compile-time: countingListPoliciesAPI satisfies IAMListPoliciesAPI.
var _ awsclient.IAMListPoliciesAPI = (*countingListPoliciesAPI)(nil)

// TestPolicyStore_ClearForcesRebuild verifies that after store.Clear() is
// called, the next FetchIAMPoliciesByIDs call rebuilds the cache from the
// new API client — not from the stale prior call.
//
// Steps:
//  1. Construct a fresh store per test (no global pollution).
//  2. Call FetchIAMPoliciesByIDs with mock1 (returns "policy-A").
//     Assert mock1.calls == 1, result contains "policy-A".
//  3. Call again with the same mock1. Assert mock1.calls still == 1 (cache hit).
//  4. Call store.Clear().
//  5. Call FetchIAMPoliciesByIDs with mock2 (returns "policy-B").
//     Assert mock2.calls == 1, result contains "policy-B" (not stale "policy-A").
func TestPolicyStore_ClearForcesRebuild(t *testing.T) {
	store := session.NewPolicyStore()

	mock1 := &countingListPoliciesAPI{policyName: "policy-A"}

	// Step 2: First call — must build the cache (ListPolicies called once).
	res1, err := awsclient.FetchIAMPoliciesByIDs(context.Background(), mock1, []string{"policy-A"}, store)
	if err != nil {
		t.Fatalf("FetchIAMPoliciesByIDs (mock1, first call): unexpected error: %v", err)
	}
	if mock1.calls.Load() != 1 {
		t.Errorf("mock1.calls after first call: want 1, got %d", mock1.calls.Load())
	}
	if len(res1) != 1 || res1[0].ID != "policy-A" {
		t.Errorf("first call: want [{ID:policy-A}], got %v", res1)
	}

	// Step 3: Second call with same mock — must be a cache hit (no extra API call).
	res2, err := awsclient.FetchIAMPoliciesByIDs(context.Background(), mock1, []string{"policy-A"}, store)
	if err != nil {
		t.Fatalf("FetchIAMPoliciesByIDs (mock1, second call): unexpected error: %v", err)
	}
	if mock1.calls.Load() != 1 {
		t.Errorf("mock1.calls after second call (cache hit): want 1, got %d — cache not working", mock1.calls.Load())
	}
	if len(res2) != 1 || res2[0].ID != "policy-A" {
		t.Errorf("second call: want [{ID:policy-A}], got %v", res2)
	}

	// Step 4: Clear the store.
	store.Clear()

	// Step 5: Call with mock2 — must rebuild from mock2 (returns "policy-B").
	mock2 := &countingListPoliciesAPI{policyName: "policy-B"}
	res3, err := awsclient.FetchIAMPoliciesByIDs(context.Background(), mock2, []string{"policy-B"}, store)
	if err != nil {
		t.Fatalf("FetchIAMPoliciesByIDs (mock2, post-clear): unexpected error: %v", err)
	}
	if mock2.calls.Load() != 1 {
		t.Errorf("mock2.calls after clear+fetch: want 1, got %d", mock2.calls.Load())
	}
	if len(res3) != 1 || res3[0].ID != "policy-B" {
		t.Errorf("post-clear call: want [{ID:policy-B}], got %v — stale cache from mock1 not cleared", res3)
	}
}

// TestPolicyStore_ClearIdempotent verifies that calling store.Clear() twice
// in a row without an intervening fetch does not panic or produce an error.
//
// Regression: if Clear had non-idempotent cleanup (e.g., double-free of a map)
// it would panic here.
func TestPolicyStore_ClearIdempotent(t *testing.T) {
	store := session.NewPolicyStore()

	// Two clears in a row — must not panic.
	store.Clear()
	store.Clear()
}
