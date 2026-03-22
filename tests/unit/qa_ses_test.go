package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T-SES-001 - Test SES Email Identities response parsing
// ---------------------------------------------------------------------------

func TestFetchSESIdentities_ParsesMultipleIdentities(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("example.com"),
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     true,
					VerificationStatus: sesv2types.VerificationStatusSuccess,
				},
				{
					IdentityName:       aws.String("user@example.com"),
					IdentityType:       sesv2types.IdentityTypeEmailAddress,
					SendingEnabled:     false,
					VerificationStatus: sesv2types.VerificationStatusPending,
				},
			},
		},
	}

	resources, err := awsclient.FetchSESIdentities(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "example.com" {
		t.Errorf("expected Name 'example.com', got %q", r.Name)
	}
	if r.ID != "example.com" {
		t.Errorf("expected ID 'example.com', got %q", r.ID)
	}
	if r.Fields["identity_name"] != "example.com" {
		t.Errorf("expected Fields[identity_name] 'example.com', got %q", r.Fields["identity_name"])
	}
	if r.Fields["identity_type"] != "DOMAIN" {
		t.Errorf("expected Fields[identity_type] 'DOMAIN', got %q", r.Fields["identity_type"])
	}
	if r.Fields["sending_enabled"] != "true" {
		t.Errorf("expected Fields[sending_enabled] 'true', got %q", r.Fields["sending_enabled"])
	}
	if r.Fields["verification_status"] != "SUCCESS" {
		t.Errorf("expected Fields[verification_status] 'SUCCESS', got %q", r.Fields["verification_status"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

	// Second identity
	r2 := resources[1]
	if r2.Fields["identity_type"] != "EMAIL_ADDRESS" {
		t.Errorf("expected Fields[identity_type] 'EMAIL_ADDRESS', got %q", r2.Fields["identity_type"])
	}
	if r2.Fields["sending_enabled"] != "false" {
		t.Errorf("expected Fields[sending_enabled] 'false', got %q", r2.Fields["sending_enabled"])
	}
}

func TestFetchSESIdentities_EmptyResponse(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{},
		},
	}

	resources, err := awsclient.FetchSESIdentities(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchSESIdentities_APIError(t *testing.T) {
	mock := &mockSESv2Client{
		err: &mockAPIError{code: "TooManyRequestsException", message: "throttled"},
	}

	_, err := awsclient.FetchSESIdentities(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
