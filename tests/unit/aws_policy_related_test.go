package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	internalaws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// mockIAMListEntitiesAPI is a minimal implementation of IAMListEntitiesForPolicyAPI
// used by policy related-checker tests.
type mockIAMListEntitiesAPI struct {
	out *iam.ListEntitiesForPolicyOutput
	err error
}

func (m *mockIAMListEntitiesAPI) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	return m.out, m.err
}

// policyResource constructs a Resource with an ARN in Fields["arn"] so the
// checkers can resolve the policy ARN without a RawStruct.
func policyResource(arn string) resource.Resource {
	return resource.Resource{
		ID:     arn,
		Name:   "test-policy",
		Fields: map[string]string{"arn": arn},
	}
}

// --- Navigable Fields ---

func TestNavigableFields_Policy_None(t *testing.T) {
	fields := resource.GetNavigableFields("policy")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for policy, got %d: %v", len(fields), fields)
	}
}

// --- checkPolicyRole ---

func TestRelated_Policy_Role_ReturnsRoleNames(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyRoles: []iamtypes.PolicyRole{
				{RoleName: new("admin-role")},
				{RoleName: new("readonly-role")},
			},
		},
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/role-test-01")
	checker := checkerByTarget(t, "policy", "role")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	wantIDs := map[string]bool{"admin-role": false, "readonly-role": false}
	for _, id := range result.ResourceIDs {
		wantIDs[id] = true
	}
	for name, found := range wantIDs {
		if !found {
			t.Errorf("ResourceIDs missing %q; got %v", name, result.ResourceIDs)
		}
	}
}

func TestRelated_Policy_Role_ReturnsZeroWhenNoRoles(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyRoles: []iamtypes.PolicyRole{},
		},
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/role-test-02")
	checker := checkerByTarget(t, "policy", "role")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

func TestRelated_Policy_Role_PropagatesAPIError(t *testing.T) {
	apiErr := errors.New("iam: access denied")
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: nil,
		err: apiErr,
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/role-test-03")
	checker := checkerByTarget(t, "policy", "role")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 on API error", result.Count)
	}
	if result.Err == nil {
		t.Error("Err should not be nil on API error")
	}
}

func TestRelated_Policy_Role_NilClientsReturnsNegOne(t *testing.T) {
	res := policyResource("arn:aws:iam::111122223333:policy/role-test-04")
	checker := checkerByTarget(t, "policy", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

// --- checkPolicyUser ---

func TestRelated_Policy_User_ReturnsUserNames(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyUsers: []iamtypes.PolicyUser{
				{UserName: new("alice")},
				{UserName: new("bob")},
			},
		},
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/user-test-01")
	checker := checkerByTarget(t, "policy", "iam-user")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.TargetType != "iam-user" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-user")
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Policy_User_ReturnsZeroWhenNoUsers(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyUsers: []iamtypes.PolicyUser{},
		},
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/user-test-02")
	checker := checkerByTarget(t, "policy", "iam-user")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Policy_User_NilClientsReturnsNegOne(t *testing.T) {
	res := policyResource("arn:aws:iam::111122223333:policy/user-test-03")
	checker := checkerByTarget(t, "policy", "iam-user")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "iam-user" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-user")
	}
}

// --- checkPolicyGroup ---

func TestRelated_Policy_Group_ReturnsGroupNames(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyGroups: []iamtypes.PolicyGroup{
				{GroupName: new("developers")},
				{GroupName: new("ops")},
			},
		},
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/group-test-01")
	checker := checkerByTarget(t, "policy", "iam-group")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.TargetType != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-group")
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Policy_Group_ReturnsZeroWhenNoGroups(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyGroups: []iamtypes.PolicyGroup{},
		},
	})
	defer restore()

	res := policyResource("arn:aws:iam::111122223333:policy/group-test-02")
	checker := checkerByTarget(t, "policy", "iam-group")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Policy_Group_NilClientsReturnsNegOne(t *testing.T) {
	res := policyResource("arn:aws:iam::111122223333:policy/group-test-03")
	checker := checkerByTarget(t, "policy", "iam-group")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-group")
	}
}

// --- ARN fallback ---

// TestRelated_Policy_Role_ARNFallbackFromID verifies that when Fields["arn"] is empty
// the checker returns Count 0 (no ARN resolvable), not -1 (error).
func TestRelated_Policy_Role_ARNFallbackFromID(t *testing.T) {
	restore := internalaws.SetIAMListEntitiesAPIForTest(&mockIAMListEntitiesAPI{
		out: &iam.ListEntitiesForPolicyOutput{},
	})
	defer restore()

	// Resource with no "arn" in Fields and no RawStruct — policyARNFromResource returns "".
	res := resource.Resource{
		ID:     "no-arn-policy",
		Name:   "no-arn-policy",
		Fields: map[string]string{},
	}
	checker := checkerByTarget(t, "policy", "role")
	result := checker(context.Background(), &internalaws.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ARN resolvable)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

// --- TTL cache ---

// TestRelated_Policy_TTLCache verifies that two checkers called with the same ARN
// within policyEntitiesTTL share a cached result (the mock is only called once).
func TestRelated_Policy_TTLCache(t *testing.T) {
	// Use a unique ARN to guarantee no stale entry from other tests.
	arn := "arn:aws:iam::111122223333:policy/ttl-cache-test-" + time.Now().Format("20060102150405.999999999")

	callCount := 0
	mock := &mockIAMListEntitiesAPICounter{
		out: &iam.ListEntitiesForPolicyOutput{
			PolicyRoles: []iamtypes.PolicyRole{{RoleName: new("cached-role")}},
		},
		counter: &callCount,
	}
	restore := internalaws.SetIAMListEntitiesAPIForTest(mock)
	defer restore()

	clients := &internalaws.ServiceClients{}
	res := policyResource(arn)
	checker := checkerByTarget(t, "policy", "role")

	result1 := checker(context.Background(), clients, res, resource.ResourceCache{})
	result2 := checker(context.Background(), clients, res, resource.ResourceCache{})

	if callCount != 1 {
		t.Errorf("API called %d times, want 1 (second call should hit cache)", callCount)
	}
	if result1.Count != 1 || result2.Count != 1 {
		t.Errorf("Count = %d / %d, want 1 / 1", result1.Count, result2.Count)
	}
}

// mockIAMListEntitiesAPICounter is a mock that increments a counter on each call.
type mockIAMListEntitiesAPICounter struct {
	out     *iam.ListEntitiesForPolicyOutput
	counter *int
}

func (m *mockIAMListEntitiesAPICounter) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	*m.counter++
	return m.out, nil
}
