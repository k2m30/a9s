package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// ACM - Test FetchACMCertificates response parsing
// ---------------------------------------------------------------------------

func TestFetchACMCertificates_ParsesMultipleCertificates(t *testing.T) {
	notAfter := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	notBefore := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2025, 6, 14, 12, 0, 0, 0, time.UTC)

	mock := &mockACMListCertificatesClient{
		output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{
				{
					DomainName:         aws.String("api.example.com"),
					Status:             acmtypes.CertificateStatusIssued,
					Type:               acmtypes.CertificateTypeAmazonIssued,
					NotAfter:           &notAfter,
					NotBefore:          &notBefore,
					InUse:              aws.Bool(true),
					CertificateArn:     aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc12345-1234-1234-1234-abcdef123456"),
					CreatedAt:          &createdAt,
					RenewalEligibility: acmtypes.RenewalEligibilityEligible,
					KeyAlgorithm:       acmtypes.KeyAlgorithmRsa2048,
				},
				{
					DomainName:     aws.String("staging.example.com"),
					Status:         acmtypes.CertificateStatusPendingValidation,
					Type:           acmtypes.CertificateTypeImported,
					NotAfter:       &notAfter,
					InUse:          aws.Bool(false),
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/def67890-5678-5678-5678-fedcba654321"),
					KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				},
			},
		},
	}

	resources, err := awsclient.FetchACMCertificates(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"domain_name", "status", "type", "not_after", "in_use"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first certificate
	r0 := resources[0]
	if r0.ID != "api.example.com" {
		t.Errorf("resource[0].ID: expected %q, got %q", "api.example.com", r0.ID)
	}
	if r0.Name != "api.example.com" {
		t.Errorf("resource[0].Name: expected %q, got %q", "api.example.com", r0.Name)
	}
	if r0.Status != "ISSUED" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ISSUED", r0.Status)
	}
	if r0.Fields["domain_name"] != "api.example.com" {
		t.Errorf("resource[0].Fields[\"domain_name\"]: expected %q, got %q", "api.example.com", r0.Fields["domain_name"])
	}
	if r0.Fields["status"] != "ISSUED" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "ISSUED", r0.Fields["status"])
	}
	if r0.Fields["type"] != "AMAZON_ISSUED" {
		t.Errorf("resource[0].Fields[\"type\"]: expected %q, got %q", "AMAZON_ISSUED", r0.Fields["type"])
	}
	if r0.Fields["not_after"] == "" {
		t.Error("resource[0].Fields[\"not_after\"] should not be empty")
	}
	if r0.Fields["in_use"] != "true" {
		t.Errorf("resource[0].Fields[\"in_use\"]: expected %q, got %q", "true", r0.Fields["in_use"])
	}

	// Verify second certificate
	r1 := resources[1]
	if r1.ID != "staging.example.com" {
		t.Errorf("resource[1].ID: expected %q, got %q", "staging.example.com", r1.ID)
	}
	if r1.Status != "PENDING_VALIDATION" {
		t.Errorf("resource[1].Status: expected %q, got %q", "PENDING_VALIDATION", r1.Status)
	}
	if r1.Fields["type"] != "IMPORTED" {
		t.Errorf("resource[1].Fields[\"type\"]: expected %q, got %q", "IMPORTED", r1.Fields["type"])
	}
	if r1.Fields["in_use"] != "false" {
		t.Errorf("resource[1].Fields[\"in_use\"]: expected %q, got %q", "false", r1.Fields["in_use"])
	}
}

func TestFetchACMCertificates_ErrorResponse(t *testing.T) {
	mock := &mockACMListCertificatesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchACMCertificates(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchACMCertificates_EmptyResponse(t *testing.T) {
	mock := &mockACMListCertificatesClient{
		output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{},
		},
	}

	resources, err := awsclient.FetchACMCertificates(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
