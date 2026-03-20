package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-WAF-001 - Test WAF Web ACLs response parsing
// ---------------------------------------------------------------------------

func TestFetchWAFWebACLs_ParsesMultipleACLs(t *testing.T) {
	mock := &mockWAFv2Client{
		output: &wafv2.ListWebACLsOutput{
			WebACLs: []wafv2types.WebACLSummary{
				{
					Name:        aws.String("my-web-acl"),
					Id:          aws.String("acl-001"),
					ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-web-acl/acl-001"),
					Description: aws.String("Primary web ACL"),
					LockToken:   aws.String("token-1"),
				},
				{
					Name:        aws.String("backup-acl"),
					Id:          aws.String("acl-002"),
					ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/backup-acl/acl-002"),
					Description: aws.String("Backup ACL"),
				},
			},
		},
	}

	resources, err := awsclient.FetchWAFWebACLs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "my-web-acl" {
		t.Errorf("expected Name 'my-web-acl', got %q", r.Name)
	}
	if r.ID != "acl-001" {
		t.Errorf("expected ID 'acl-001', got %q", r.ID)
	}
	if r.Fields["name"] != "my-web-acl" {
		t.Errorf("expected Fields[name] 'my-web-acl', got %q", r.Fields["name"])
	}
	if r.Fields["id"] != "acl-001" {
		t.Errorf("expected Fields[id] 'acl-001', got %q", r.Fields["id"])
	}
	if r.Fields["description"] != "Primary web ACL" {
		t.Errorf("expected Fields[description] 'Primary web ACL', got %q", r.Fields["description"])
	}
	if r.Fields["arn"] == "" {
		t.Error("expected Fields[arn] to be non-empty")
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestFetchWAFWebACLs_ScopeRegional(t *testing.T) {
	mock := &mockWAFv2CaptureClient{
		output: &wafv2.ListWebACLsOutput{
			WebACLs: []wafv2types.WebACLSummary{},
		},
	}

	_, err := awsclient.FetchWAFWebACLs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.capturedInput == nil {
		t.Fatal("expected input to be captured")
	}

	if mock.capturedInput.Scope != wafv2types.ScopeRegional {
		t.Errorf("expected Scope REGIONAL, got %q", mock.capturedInput.Scope)
	}
}

func TestFetchWAFWebACLs_EmptyResponse(t *testing.T) {
	mock := &mockWAFv2Client{
		output: &wafv2.ListWebACLsOutput{
			WebACLs: []wafv2types.WebACLSummary{},
		},
	}

	resources, err := awsclient.FetchWAFWebACLs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchWAFWebACLs_APIError(t *testing.T) {
	mock := &mockWAFv2Client{
		err: &mockAPIError{code: "WAFInternalErrorException", message: "internal error"},
	}

	_, err := awsclient.FetchWAFWebACLs(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
