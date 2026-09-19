package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestFetchACMCertificates_ParsesMultipleCertificates(t *testing.T) {
	notAfter := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	notBefore := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2025, 6, 14, 12, 0, 0, 0, time.UTC)

	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
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

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	requiredFields := []string{"domain_name", "status", "type", "not_after", "in_use"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first certificate — ID is the certificate ARN (unique across
	// same-domain certs), Name stays the domain for display.
	r0 := resources[0]
	if r0.ID != "arn:aws:acm:us-east-1:123456789012:certificate/abc12345-1234-1234-1234-abcdef123456" {
		t.Errorf("resource[0].ID: expected the certificate ARN, got %q", r0.ID)
	}
	if r0.Name != "api.example.com" {
		t.Errorf("resource[0].Name: expected %q, got %q", "api.example.com", r0.Name)
	}
	// status/type are rendered in the acm list's cells, so both go through
	// domain.HumanizeStatusPhrase: no rendered cell shows a raw UPPER_SNAKE enum.
	if r0.Fields["status"] != "issued" {
		t.Errorf("resource[0].Status: expected %q, got %q", "issued", r0.Fields["status"])
	}
	if r0.Fields["domain_name"] != "api.example.com" {
		t.Errorf("resource[0].Fields[\"domain_name\"]: expected %q, got %q", "api.example.com", r0.Fields["domain_name"])
	}
	if r0.Fields["status"] != "issued" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "issued", r0.Fields["status"])
	}
	if r0.Fields["type"] != "amazon issued" {
		t.Errorf("resource[0].Fields[\"type\"]: expected %q, got %q", "amazon issued", r0.Fields["type"])
	}
	if r0.Fields["not_after"] == "" {
		t.Error("resource[0].Fields[\"not_after\"] should not be empty")
	}
	if r0.Fields["in_use"] != "true" {
		t.Errorf("resource[0].Fields[\"in_use\"]: expected %q, got %q", "true", r0.Fields["in_use"])
	}
	// ISSUED certs carry their expiry signal as a wave1 Finding read straight off
	// ListCertificates (docs/attention-signals.md, `acm`). This fixture's NotAfter
	// (2026-06-15) is in the past, so it lands in the expired bucket.
	if len(r0.Findings) != 1 {
		t.Fatalf("resource[0].Findings: expected 1 (expires-critical) for an expired ISSUED cert, got %d: %v", len(r0.Findings), r0.Findings)
	}
	// A cert past NotAfter carries its own code: "expired" and "expires in 3
	// days" are different things to do, and one code cannot declare both
	// wordings.
	if r0.Findings[0].Code != "acm.expired" {
		t.Errorf("resource[0].Findings[0].Code: expected %q, got %q", "acm.expired", r0.Findings[0].Code)
	}
	if r0.Findings[0].Severity != domain.SevBroken {
		t.Errorf("resource[0].Findings[0].Severity: expected %v, got %v", domain.SevBroken, r0.Findings[0].Severity)
	}
	// acmColor reads Fields["status"] directly for ISSUED, the only status with no
	// Finding for colorFromAnyFinding to use, so it must match the humanized
	// "issued". The fixture's NotAfter is past, so the ISSUED branch's expired case
	// gives ColorBroken, which the unmatched-status default does not.
	acmType := resource.FindResourceType("acm")
	if acmType == nil {
		t.Fatal("acm type not registered")
	}
	if got := acmType.Color(r0); got != resource.ColorBroken {
		t.Errorf("acm type.Color(resource[0]) = %v, want %v (issued, expired)", got, resource.ColorBroken)
	}

	r1 := resources[1]
	if r1.ID != "arn:aws:acm:us-east-1:123456789012:certificate/def67890-5678-5678-5678-fedcba654321" {
		t.Errorf("resource[1].ID: expected the certificate ARN, got %q", r1.ID)
	}
	if r1.Name != "staging.example.com" {
		t.Errorf("resource[1].Name: expected %q, got %q", "staging.example.com", r1.Name)
	}
	if r1.Fields["status"] != "pending validation" {
		t.Errorf("resource[1].Status: expected %q, got %q", "pending validation", r1.Fields["status"])
	}
	if r1.Fields["type"] != "imported" {
		t.Errorf("resource[1].Fields[\"type\"]: expected %q, got %q", "imported", r1.Fields["type"])
	}
	if r1.Fields["in_use"] != "false" {
		t.Errorf("resource[1].Fields[\"in_use\"]: expected %q, got %q", "false", r1.Fields["in_use"])
	}
	// acmStatusFindings switches on the certificate's raw status before
	// Fields["status"] is humanized, so PENDING_VALIDATION still emits its wave1
	// Finding (and ColorWarning via colorFromAnyFinding).
	if len(r1.Findings) != 1 {
		t.Fatalf("resource[1].Findings: expected 1 finding for PENDING_VALIDATION, got %d: %v", len(r1.Findings), r1.Findings)
	}
	if r1.Findings[0].Phrase != "pending validation" {
		t.Errorf("resource[1].Findings[0].Phrase: expected %q, got %q", "pending validation", r1.Findings[0].Phrase)
	}
	if got := acmType.Color(r1); got != resource.ColorWarning {
		t.Errorf("acm type.Color(resource[1]) = %v, want %v (pending validation)", got, resource.ColorWarning)
	}
}

// TestFetchACMCertificates_SameDomainDistinctARN_UniqueIDs pins that two
// certificates for the SAME domain (an expired cert and its active renewal)
// get DISTINCT resource IDs — the certificate ARN, not the shared domain name.
// A shared domain-name ID collides in the related-panel cache (keyed type:id),
// so the second cert's detail would replay the first's related panel.
func TestFetchACMCertificates_SameDomainDistinctARN_UniqueIDs(t *testing.T) {
	arnA := "arn:aws:acm:eu-central-1:123456789012:certificate/11111111-1111-1111-1111-111111111111"
	arnB := "arn:aws:acm:eu-central-1:123456789012:certificate/22222222-2222-2222-2222-222222222222"
	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{
				{DomainName: aws.String("artifacts.example.com"), Status: acmtypes.CertificateStatusExpired, CertificateArn: aws.String(arnA), InUse: aws.Bool(false)},
				{DomainName: aws.String("artifacts.example.com"), Status: acmtypes.CertificateStatusIssued, CertificateArn: aws.String(arnB), InUse: aws.Bool(true)},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	if resources[0].ID == resources[1].ID {
		t.Fatalf("same-domain certs share ID %q — the related-panel cache (type:id) would collide", resources[0].ID)
	}
	if resources[0].ID != arnA || resources[1].ID != arnB {
		t.Errorf("IDs = (%q, %q), want the ARNs (%q, %q)", resources[0].ID, resources[1].ID, arnA, arnB)
	}
	if resources[0].Name != "artifacts.example.com" || resources[1].Name != "artifacts.example.com" {
		t.Errorf("Names = (%q, %q), want both the domain %q", resources[0].Name, resources[1].Name, "artifacts.example.com")
	}
}

func TestFetchACMCertificates_ErrorResponse(t *testing.T) {
	mock := &fakeACMListCertificates{
		Err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchACMCertificates_EmptyResponse(t *testing.T) {
	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchACMCertificates_ExpiresWithin30Days_WarnFinding pins that an
// ISSUED cert with NotAfter 29 days out gets a "acm.expires-soon" Finding at
// SevWarn ("~"). 29d is inside the 30d window but outside the 7d Broken
// window.
func TestFetchACMCertificates_ExpiresWithin30Days_WarnFinding(t *testing.T) {
	notAfter := time.Now().Add(29 * 24 * time.Hour)
	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{
				{
					DomainName:     aws.String("soon.example.com"),
					Status:         acmtypes.CertificateStatusIssued,
					NotAfter:       &notAfter,
					InUse:          aws.Bool(true),
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/soon00000000-1111-2222-3333-444455556666"),
				},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 Finding (expires-soon), got %d: %v", len(r.Findings), r.Findings)
	}
	if r.Findings[0].Code != "acm.expires-soon" {
		t.Errorf("Findings[0].Code = %q, want %q", r.Findings[0].Code, "acm.expires-soon")
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity = %v, want %v (29d out is Warning, not Broken)", r.Findings[0].Severity, domain.SevWarn)
	}
	acmType := resource.FindResourceType("acm")
	if acmType == nil {
		t.Fatal("acm type not registered")
	}
	if got := acmType.Color(r); got != resource.ColorWarning {
		t.Errorf("acm type.Color(r) = %v, want %v (expires-soon)", got, resource.ColorWarning)
	}
}

// TestFetchACMCertificates_ExpiresWithin7Days_BrokenFinding pins that an
// ISSUED cert with NotAfter 6 days out escalates to "acm.expires-critical"
// at SevBroken ("!").
func TestFetchACMCertificates_ExpiresWithin7Days_BrokenFinding(t *testing.T) {
	notAfter := time.Now().Add(6 * 24 * time.Hour)
	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{
				{
					DomainName:     aws.String("critical.example.com"),
					Status:         acmtypes.CertificateStatusIssued,
					NotAfter:       &notAfter,
					InUse:          aws.Bool(true),
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/crit0000000-1111-2222-3333-444455556666"),
				},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 Finding (expires-critical), got %d: %v", len(r.Findings), r.Findings)
	}
	if r.Findings[0].Code != "acm.expires-critical" {
		t.Errorf("Findings[0].Code = %q, want %q", r.Findings[0].Code, "acm.expires-critical")
	}
	if r.Findings[0].Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity = %v, want %v (6d out is Broken)", r.Findings[0].Severity, domain.SevBroken)
	}
	acmType := resource.FindResourceType("acm")
	if acmType == nil {
		t.Fatal("acm type not registered")
	}
	if got := acmType.Color(r); got != resource.ColorBroken {
		t.Errorf("acm type.Color(r) = %v, want %v (expires-critical)", got, resource.ColorBroken)
	}
}

// TestFetchACMCertificates_OrphanNotExpired_WarnFinding pins that an ISSUED
// cert with InUse==false and a healthy (far-future) NotAfter gets an
// "acm.orphan" Finding at SevWarn ("~") — expiry takes priority over orphan,
// but this cert isn't expiring so orphan surfaces alone.
func TestFetchACMCertificates_OrphanNotExpired_WarnFinding(t *testing.T) {
	notAfter := time.Now().Add(90 * 24 * time.Hour)
	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{
				{
					DomainName:     aws.String("orphan.example.com"),
					Status:         acmtypes.CertificateStatusIssued,
					NotAfter:       &notAfter,
					InUse:          aws.Bool(false),
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/orph0000000-1111-2222-3333-444455556666"),
				},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 Finding (orphan), got %d: %v", len(r.Findings), r.Findings)
	}
	if r.Findings[0].Code != "acm.orphan" {
		t.Errorf("Findings[0].Code = %q, want %q", r.Findings[0].Code, "acm.orphan")
	}
	if r.Findings[0].Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity = %v, want %v", r.Findings[0].Severity, domain.SevWarn)
	}
	acmType := resource.FindResourceType("acm")
	if acmType == nil {
		t.Fatal("acm type not registered")
	}
	if got := acmType.Color(r); got != resource.ColorWarning {
		t.Errorf("acm type.Color(r) = %v, want %v (orphan)", got, resource.ColorWarning)
	}
}

// TestFetchACMCertificates_HealthyIssuedCert_NoFindings verifies the negative
// case: an ISSUED cert that is neither expiring soon nor orphaned (far-future
// NotAfter, InUse=true) produces zero Findings and colors Healthy.
func TestFetchACMCertificates_HealthyIssuedCert_NoFindings(t *testing.T) {
	notAfter := time.Now().Add(90 * 24 * time.Hour)
	mock := &fakeACMListCertificates{
		Output: &acm.ListCertificatesOutput{
			CertificateSummaryList: []acmtypes.CertificateSummary{
				{
					DomainName:     aws.String("healthy.example.com"),
					Status:         acmtypes.CertificateStatusIssued,
					NotAfter:       &notAfter,
					InUse:          aws.Bool(true),
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/hlth0000000-1111-2222-3333-444455556666"),
				},
			},
		},
	}
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchACMCertificatesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]
	if len(r.Findings) != 0 {
		t.Errorf("expected 0 Findings for a healthy ISSUED cert, got %d: %v", len(r.Findings), r.Findings)
	}
	acmType := resource.FindResourceType("acm")
	if acmType == nil {
		t.Fatal("acm type not registered")
	}
	if got := acmType.Color(r); got != resource.ColorHealthy {
		t.Errorf("acm type.Color(r) = %v, want %v", got, resource.ColorHealthy)
	}
}
