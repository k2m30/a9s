// aws_ses_test.go — Fetcher tests for FetchSESIdentities and FetchSESIdentitiesPage.
//
// Contract assertions:
//   - Fields["identity_name"], Fields["identity_type"], Fields["sending_enabled"],
//     Fields["verification_status"] are populated from the raw IdentityInfo struct.
//   - Fields["verification_status"] is the raw SDK enum string (e.g. "SUCCESS").
//   - Status is the computed human-readable phrase from computeSESStatusAndIssues.
//     Healthy identities (SUCCESS + sending enabled) → Status = "".
//   - Issues slice mirrors the computed phrase(s).
//   - RawStruct is set to the sesv2types.IdentityInfo value on every resource.
//   - ID and Name both equal the identity name.
//   - Empty API response → 0 resources, no error.
//   - API error → error propagated.
//   - Truncated when NextToken present; not truncated when absent.
//   - Fixture-based: all 8 fixture identities map expected human-readable Status values.
package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ---------------------------------------------------------------------------
// Field-mapping tests
// ---------------------------------------------------------------------------

// TestFetchSESIdentitiesPage_DomainIdentityFieldMapping verifies that a DOMAIN
// identity is mapped to all four field-map keys with exact values.
// Fields["verification_status"] is the raw enum string; Status is "".
func TestFetchSESIdentitiesPage_DomainIdentityFieldMapping(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("acme-corp.com"),
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     true,
					VerificationStatus: sesv2types.VerificationStatusSuccess,
				},
			},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.ID != "acme-corp.com" {
		t.Errorf("ID = %q, want %q", r.ID, "acme-corp.com")
	}
	if r.Name != "acme-corp.com" {
		t.Errorf("Name = %q, want %q", r.Name, "acme-corp.com")
	}
	// Healthy identity (SUCCESS + sending enabled): Status = ""
	if r.Status != "" {
		t.Errorf("Status = %q, want %q (healthy identity)", r.Status, "")
	}
	if r.Fields["identity_name"] != "acme-corp.com" {
		t.Errorf("Fields[identity_name] = %q, want %q", r.Fields["identity_name"], "acme-corp.com")
	}
	if r.Fields["identity_type"] != "DOMAIN" {
		t.Errorf("Fields[identity_type] = %q, want %q", r.Fields["identity_type"], "DOMAIN")
	}
	if r.Fields["sending_enabled"] != "true" {
		t.Errorf("Fields[sending_enabled] = %q, want %q", r.Fields["sending_enabled"], "true")
	}
	// Fields["verification_status"] is the raw SDK enum.
	if r.Fields["verification_status"] != "SUCCESS" {
		t.Errorf("Fields[verification_status] = %q, want %q", r.Fields["verification_status"], "SUCCESS")
	}
	if r.RawStruct == nil {
		t.Error("RawStruct must not be nil")
	}
}

// TestFetchSESIdentitiesPage_EmailAddressIdentityFieldMapping verifies that an
// EMAIL_ADDRESS identity maps identity_type and sending_enabled correctly.
// A PENDING identity produces a non-empty Status phrase.
func TestFetchSESIdentitiesPage_EmailAddressIdentityFieldMapping(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("noreply@acme-corp.com"),
					IdentityType:       sesv2types.IdentityTypeEmailAddress,
					SendingEnabled:     false,
					VerificationStatus: sesv2types.VerificationStatusPending,
				},
			},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.ID != "noreply@acme-corp.com" {
		t.Errorf("ID = %q, want %q", r.ID, "noreply@acme-corp.com")
	}
	if r.Fields["identity_type"] != "EMAIL_ADDRESS" {
		t.Errorf("Fields[identity_type] = %q, want %q", r.Fields["identity_type"], "EMAIL_ADDRESS")
	}
	if r.Fields["sending_enabled"] != "false" {
		t.Errorf("Fields[sending_enabled] = %q, want %q", r.Fields["sending_enabled"], "false")
	}
	// PENDING + sending disabled → multiple issues; status phrase is the top phrase.
	if r.Status == "" {
		t.Error("Status = empty, want non-empty (PENDING + sending disabled identity)")
	}
	// Fields["verification_status"] is the raw SDK enum.
	if r.Fields["verification_status"] != "PENDING" {
		t.Errorf("Fields[verification_status] = %q, want %q", r.Fields["verification_status"], "PENDING")
	}
}

// ---------------------------------------------------------------------------
// Status phrase mapping — human-readable, not raw enum
// ---------------------------------------------------------------------------

// TestFetchSESIdentitiesPage_StatusPhraseMapping verifies the human-readable
// Status phrase for each verification status that produces a non-empty phrase.
// Healthy identities (SUCCESS + sending enabled) produce Status = "".
func TestFetchSESIdentitiesPage_StatusPhraseMapping(t *testing.T) {
	cases := []struct {
		status         sesv2types.VerificationStatus
		sendingEnabled bool
		wantStatus     string
	}{
		// Healthy: SUCCESS + enabled → empty Status
		{sesv2types.VerificationStatusSuccess, true, ""},
		// PENDING verification
		{sesv2types.VerificationStatusPending, true, "pending verification"},
		// FAILED verification
		{sesv2types.VerificationStatusFailed, true, "verification failed"},
		// TEMPORARY_FAILURE
		{sesv2types.VerificationStatusTemporaryFailure, true, "verify: temp failure"},
		// NOT_STARTED
		{sesv2types.VerificationStatusNotStarted, true, "verification not started"},
		// Verified but sending disabled
		{sesv2types.VerificationStatusSuccess, false, "sending disabled"},
	}

	for _, tc := range cases {
		tc := tc
		name := string(tc.status)
		if !tc.sendingEnabled {
			name += "/sending-disabled"
		}
		t.Run(name, func(t *testing.T) {
			mock := &mockSESv2Client{
				output: &sesv2.ListEmailIdentitiesOutput{
					EmailIdentities: []sesv2types.IdentityInfo{
						{
							IdentityName:       aws.String("test@example.com"),
							IdentityType:       sesv2types.IdentityTypeEmailAddress,
							SendingEnabled:     tc.sendingEnabled,
							VerificationStatus: tc.status,
						},
					},
				},
			}

			result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}
			if result.Resources[0].Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", result.Resources[0].Status, tc.wantStatus)
			}
		})
	}
}

// TestFetchSESIdentitiesPage_MultipleIssuesSuffixBumped verifies that an identity
// with FAILED verification AND sending disabled produces a Status phrase with the
// "(+N)" suffix (BumpFindingSuffix behavior from computeSESStatusAndIssues).
func TestFetchSESIdentitiesPage_MultipleIssuesSuffixBumped(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("broken.acme-corp.com"),
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     false, // second issue
					VerificationStatus: sesv2types.VerificationStatusFailed, // first issue
				},
			},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	// FAILED + sending-disabled → top phrase is "verification failed (+1)"
	expected := "verification failed (+1)"
	if r.Status != expected {
		t.Errorf("Status = %q, want %q (multi-issue suffix)", r.Status, expected)
	}
	// Issues slice contains both phrases.
	if len(r.Issues) != 2 {
		t.Errorf("Issues = %v, want 2 entries", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// Issues slice
// ---------------------------------------------------------------------------

// TestFetchSESIdentitiesPage_HealthyIdentityHasNilIssues verifies that a healthy
// identity (SUCCESS + enabled) produces a nil or empty Issues slice (not an empty
// string entry).
func TestFetchSESIdentitiesPage_HealthyIdentityHasNilIssues(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("healthy.acme-corp.com"),
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     true,
					VerificationStatus: sesv2types.VerificationStatusSuccess,
				},
			},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if len(r.Issues) != 0 {
		t.Errorf("Issues = %v, want empty for healthy identity", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// Pagination tests
// ---------------------------------------------------------------------------

// TestFetchSESIdentitiesPage_NotTruncatedWhenNoNextToken verifies that when
// ListEmailIdentities returns no NextToken, the result is not truncated.
func TestFetchSESIdentitiesPage_NotTruncatedWhenNoNextToken(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("example.com"),
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     true,
					VerificationStatus: sesv2types.VerificationStatusSuccess,
				},
			},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("IsTruncated = true, want false (no NextToken)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken = %q, want empty", result.Pagination.NextToken)
	}
}

// TestFetchSESIdentitiesPage_TruncatedWhenNextTokenPresent verifies that when
// ListEmailIdentities returns a NextToken, IsTruncated=true and NextToken is set.
func TestFetchSESIdentitiesPage_TruncatedWhenNextTokenPresent(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       aws.String("example.com"),
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     true,
					VerificationStatus: sesv2types.VerificationStatusSuccess,
				},
			},
			NextToken: aws.String("page2-token"),
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("IsTruncated = false, want true (NextToken present)")
	}
	if result.Pagination.NextToken != "page2-token" {
		t.Errorf("NextToken = %q, want %q", result.Pagination.NextToken, "page2-token")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

// TestFetchSESIdentitiesPage_EmptyResponseReturnsZeroResources verifies that
// an empty identity list returns 0 resources and no error.
func TestFetchSESIdentitiesPage_EmptyResponseReturnsZeroResources(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchSESIdentitiesPage_NilIdentityNameUsesEmptyString verifies that an
// identity with nil IdentityName produces a resource with ID and Name = "".
func TestFetchSESIdentitiesPage_NilIdentityNameUsesEmptyString(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{
				{
					IdentityName:       nil,
					IdentityType:       sesv2types.IdentityTypeDomain,
					SendingEnabled:     true,
					VerificationStatus: sesv2types.VerificationStatusSuccess,
				},
			},
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "" {
		t.Errorf("ID = %q, want empty string for nil IdentityName", result.Resources[0].ID)
	}
	if result.Resources[0].Name != "" {
		t.Errorf("Name = %q, want empty string for nil IdentityName", result.Resources[0].Name)
	}
}

// TestFetchSESIdentitiesPage_APIErrorPropagated verifies that an API error is
// returned without panic and without resources.
func TestFetchSESIdentitiesPage_APIErrorPropagated(t *testing.T) {
	mock := &mockSESv2Client{
		err: &mockAPIError{code: "TooManyRequestsException", message: "rate limit exceeded"},
	}

	_, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

// TestFetchSESIdentitiesPage_ContinuationTokenAccepted verifies that a non-empty
// continuation token does not cause an error (the page function does not reject it).
func TestFetchSESIdentitiesPage_ContinuationTokenAccepted(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: []sesv2types.IdentityInfo{},
		},
	}

	_, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "some-token")
	if err != nil {
		t.Fatalf("unexpected error with continuation token: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FetchSESIdentities (multi-page aggregator)
// ---------------------------------------------------------------------------

// TestFetchSESIdentities_NilEmailIdentitiesSliceReturnsZero verifies that when
// the API returns nil EmailIdentities, the result is 0 resources without error.
func TestFetchSESIdentities_NilEmailIdentitiesSliceReturnsZero(t *testing.T) {
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: nil,
		},
	}

	resources, err := awsclient.FetchSESIdentities(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources for nil identities, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// Fixture-based tests
// ---------------------------------------------------------------------------

// TestFetchSESIdentitiesPage_FixtureGraphRootIsHealthy verifies that the graph-root
// identity (SESGraphRootIdentity = "acme-corp.com") maps to Status = "" (healthy)
// with identity_type = "DOMAIN" and sending_enabled = "true".
func TestFetchSESIdentitiesPage_FixtureGraphRootIsHealthy(t *testing.T) {
	f := fixtures.NewSESFixtures()
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: f.Identities,
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var found bool
	for _, r := range result.Resources {
		if r.ID != fixtures.SESGraphRootIdentity {
			continue
		}
		found = true
		// Graph-root is SUCCESS + sending enabled → Status = ""
		if r.Status != "" {
			t.Errorf("graph-root Status = %q, want empty string (healthy)", r.Status)
		}
		if r.Fields["identity_type"] != "DOMAIN" {
			t.Errorf("graph-root Fields[identity_type] = %q, want %q", r.Fields["identity_type"], "DOMAIN")
		}
		if r.Fields["sending_enabled"] != "true" {
			t.Errorf("graph-root Fields[sending_enabled] = %q, want %q", r.Fields["sending_enabled"], "true")
		}
		if r.Fields["verification_status"] != "SUCCESS" {
			t.Errorf("graph-root Fields[verification_status] = %q, want %q", r.Fields["verification_status"], "SUCCESS")
		}
	}

	if !found {
		t.Errorf("graph-root identity %q not found in fixture output", fixtures.SESGraphRootIdentity)
	}
}

// TestFetchSESIdentitiesPage_FixtureBrokenIdentitiesHaveNonEmptyStatus verifies
// that the fixture contains identities with non-empty Status phrases, covering
// the broken/warning categories.
func TestFetchSESIdentitiesPage_FixtureBrokenIdentitiesHaveNonEmptyStatus(t *testing.T) {
	f := fixtures.NewSESFixtures()
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: f.Identities,
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count resources with non-empty Status.
	var nonEmpty int
	for _, r := range result.Resources {
		if r.Status != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Error("expected at least one resource with non-empty Status in fixture set")
	}
}

// TestFetchSESIdentitiesPage_FixtureMultiIssueIdentityHasSuffix verifies that
// "broken.acme-corp.com" (FAILED + sending-disabled) maps to Status "verification failed (+1)".
func TestFetchSESIdentitiesPage_FixtureMultiIssueIdentityHasSuffix(t *testing.T) {
	f := fixtures.NewSESFixtures()
	mock := &mockSESv2Client{
		output: &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: f.Identities,
		},
	}

	result, err := awsclient.FetchSESIdentitiesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range result.Resources {
		if r.ID == "broken.acme-corp.com" {
			expected := "verification failed (+1)"
			if r.Status != expected {
				t.Errorf("broken.acme-corp.com Status = %q, want %q", r.Status, expected)
			}
			if len(r.Issues) != 2 {
				t.Errorf("broken.acme-corp.com Issues = %v, want 2", r.Issues)
			}
			return
		}
	}
	t.Error("broken.acme-corp.com not found in fixture output")
}
