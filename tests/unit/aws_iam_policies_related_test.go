package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkerByTarget returns the RelatedChecker for the given source→target pair.
// Fatals if the def is not found or the checker is nil.
func checkerByTarget(t *testing.T, source, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("%s→%s checker is nil", source, target)
			}
			return def.Checker
		}
	}
	t.Fatalf("%s→%s related def not found", source, target)
	return nil
}

// --- policy→role ---

func TestRelated_Policy_Role_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "policy", "role")
	// checkerByTarget fatals if checker is nil — reaching here means it's non-nil.
	_ = checker
}

func TestRelated_Policy_Role_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:iam::111122223333:policy/test-policy",
		Name: "test-policy",
		Fields: map[string]string{
			"policy_name": "test-policy",
			"arn":         "arn:aws:iam::111122223333:policy/test-policy",
		},
		RawStruct: iamtypes.Policy{
			Arn:        aws.String("arn:aws:iam::111122223333:policy/test-policy"),
			PolicyName: aws.String("test-policy"),
		},
	}
	checker := checkerByTarget(t, "policy", "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no clients)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

func TestRelated_Policy_Role_EmptyARN(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := checkerByTarget(t, "policy", "role")
	// nil clients: must return -1, not panic
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- policy→iam-user ---

func TestRelated_Policy_IAMUser_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "policy", "iam-user")
	_ = checker
}

func TestRelated_Policy_IAMUser_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:iam::111122223333:policy/test-policy",
		Name: "test-policy",
		Fields: map[string]string{
			"policy_name": "test-policy",
			"arn":         "arn:aws:iam::111122223333:policy/test-policy",
		},
		RawStruct: iamtypes.Policy{
			Arn:        aws.String("arn:aws:iam::111122223333:policy/test-policy"),
			PolicyName: aws.String("test-policy"),
		},
	}
	checker := checkerByTarget(t, "policy", "iam-user")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no clients)", result.Count)
	}
	if result.TargetType != "iam-user" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-user")
	}
}

func TestRelated_Policy_IAMUser_EmptyARN(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := checkerByTarget(t, "policy", "iam-user")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- policy→iam-group ---

func TestRelated_Policy_IAMGroup_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "policy", "iam-group")
	_ = checker
}

func TestRelated_Policy_IAMGroup_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:iam::111122223333:policy/test-policy",
		Name: "test-policy",
		Fields: map[string]string{
			"policy_name": "test-policy",
			"arn":         "arn:aws:iam::111122223333:policy/test-policy",
		},
		RawStruct: iamtypes.Policy{
			Arn:        aws.String("arn:aws:iam::111122223333:policy/test-policy"),
			PolicyName: aws.String("test-policy"),
		},
	}
	checker := checkerByTarget(t, "policy", "iam-group")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no clients)", result.Count)
	}
	if result.TargetType != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-group")
	}
}

func TestRelated_Policy_IAMGroup_EmptyARN(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := checkerByTarget(t, "policy", "iam-group")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
