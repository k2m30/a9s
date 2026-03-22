package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestQA_IAMPolicies_FetchSuccess(t *testing.T) {
	now := time.Now()
	mock := &mockIAMListPoliciesClient{
		output: &iam.ListPoliciesOutput{
			Policies: []iamtypes.Policy{
				{
					PolicyName:      aws.String("my-custom-policy"),
					PolicyId:        aws.String("ANPA123456789"),
					Arn:             aws.String("arn:aws:iam::123456789012:policy/my-custom-policy"),
					Path:            aws.String("/"),
					AttachmentCount: aws.Int32(3),
					CreateDate:      &now,
				},
				{
					PolicyName:      aws.String("dev-access"),
					PolicyId:        aws.String("ANPA987654321"),
					Arn:             aws.String("arn:aws:iam::123456789012:policy/teams/dev-access"),
					Path:            aws.String("/teams/"),
					AttachmentCount: aws.Int32(0),
					CreateDate:      &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMPolicies(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "my-custom-policy" {
		t.Errorf("expected ID 'my-custom-policy', got %q", r.ID)
	}
	if r.Name != "my-custom-policy" {
		t.Errorf("expected Name 'my-custom-policy', got %q", r.Name)
	}
	if r.Status != "" {
		t.Errorf("expected empty status, got %q", r.Status)
	}
	if r.Fields["policy_name"] != "my-custom-policy" {
		t.Errorf("expected policy_name 'my-custom-policy', got %q", r.Fields["policy_name"])
	}
	if r.Fields["policy_id"] != "ANPA123456789" {
		t.Errorf("expected policy_id 'ANPA123456789', got %q", r.Fields["policy_id"])
	}
	if r.Fields["attachment_count"] != "3" {
		t.Errorf("expected attachment_count '3', got %q", r.Fields["attachment_count"])
	}
	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_IAMPolicies_FetchEmpty(t *testing.T) {
	mock := &mockIAMListPoliciesClient{
		output: &iam.ListPoliciesOutput{
			Policies: []iamtypes.Policy{},
		},
	}

	resources, err := awsclient.FetchIAMPolicies(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_IAMPolicies_FetchError(t *testing.T) {
	mock := &mockIAMListPoliciesClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchIAMPolicies(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_IAMPolicies_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("policy")
	if rt == nil {
		t.Fatal("resource type 'policy' not found")
	}
	if rt.Name != "IAM Policies" {
		t.Errorf("expected Name 'IAM Policies', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"policy_name", "Policy Name"},
		{"policy_id", "Policy ID"},
		{"attachment_count", "Attached"},
		{"path", "Path"},
		{"create_date", "Created"},
	}
	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}
	for i, want := range expected {
		if rt.Columns[i].Key != want.key {
			t.Errorf("column %d: expected key %q, got %q", i, want.key, rt.Columns[i].Key)
		}
		if rt.Columns[i].Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, rt.Columns[i].Title)
		}
	}
}

func TestQA_IAMPolicies_Aliases(t *testing.T) {
	for _, alias := range []string{"policy", "policies", "iam-policies"} {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("alias %q should resolve to policy resource type", alias)
		}
	}
}
