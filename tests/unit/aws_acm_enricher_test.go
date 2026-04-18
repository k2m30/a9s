package unit

// aws_acm_enricher_test.go — Behavioral tests for EnrichACMCertificate.
//
// Contract assertions:
//   - DescribeCertificate is called once per ACM resource (keyed by cert ARN).
//   - NotAfter > now+30d AND InUseBy non-empty AND Status=ISSUED → 0 findings.
//   - NotAfter within 30d → 1 finding sev "!" "expires" for that cert.
//   - NotAfter in the past (expired) → 1 finding sev "!" for that cert.
//   - Status=ISSUED AND InUseBy=[] → 1 finding sev "~" (orphan) for that cert.
//   - clients.ACM == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// acmDescribeCertificateFake implements ACMAPI for enrichment testing.
// It embeds the interface and overrides only DescribeCertificate.
// The results map is keyed by CertificateArn so the fake can serve different
// responses per resource.
type acmDescribeCertificateFake struct {
	awsclient.ACMAPI
	// results maps CertificateArn → CertificateDetail.
	results map[string]*acmtypes.CertificateDetail
	// errByArn maps CertificateArn → error; overrides results when set.
	errByArn map[string]error
}

func (f *acmDescribeCertificateFake) DescribeCertificate(
	_ context.Context,
	in *acm.DescribeCertificateInput,
	_ ...func(*acm.Options),
) (*acm.DescribeCertificateOutput, error) {
	arn := ""
	if in != nil && in.CertificateArn != nil {
		arn = *in.CertificateArn
	}
	if f.errByArn != nil {
		if err, ok := f.errByArn[arn]; ok {
			return nil, err
		}
	}
	detail, ok := f.results[arn]
	if !ok {
		return &acm.DescribeCertificateOutput{}, nil
	}
	return &acm.DescribeCertificateOutput{Certificate: detail}, nil
}

// Compile-time check: acmDescribeCertificateFake satisfies ACMAPI.
var _ awsclient.ACMAPI = (*acmDescribeCertificateFake)(nil)

// acmCertResources returns a slice of ACM Resource stubs with the given ARNs.
func acmCertResources(arns ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(arns))
	for _, arn := range arns {
		domain := "example-" + arn[len(arn)-8:] + ".com"
		res = append(res, resource.Resource{
			ID:     arn,
			Name:   domain,
			Status: "ISSUED",
			Fields: map[string]string{
				"domain_name": domain,
				"status":      "ISSUED",
				"type":        "AMAZON_ISSUED",
				"not_after":   "",
				"in_use":      "true",
			},
		})
	}
	return res
}

// acmCertDetail builds a CertificateDetail with the provided NotAfter offset and InUseBy slice.
func acmCertDetail(arn string, notAfterOffset time.Duration, inUseBy []string, status acmtypes.CertificateStatus) *acmtypes.CertificateDetail {
	notAfter := time.Now().Add(notAfterOffset)
	domain := "example-" + arn[len(arn)-8:] + ".com"
	return &acmtypes.CertificateDetail{
		CertificateArn: aws.String(arn),
		DomainName:     aws.String(domain),
		NotAfter:       &notAfter,
		InUseBy:        inUseBy,
		Status:         status,
	}
}

const (
	acmARN1 = "arn:aws:acm:us-east-1:123456789012:certificate/aaaaaaaa-1111-2222-3333-444444444444"
	acmARN2 = "arn:aws:acm:us-east-1:123456789012:certificate/bbbbbbbb-1111-2222-3333-444444444444"
	acmARN3 = "arn:aws:acm:us-east-1:123456789012:certificate/cccccccc-1111-2222-3333-444444444444"
)

// TestEnrichACMCertificate_ValidInUseProducesNoFindings verifies that when all certs
// have NotAfter > now+30d, are in use (InUseBy non-empty), and Status=ISSUED, no
// findings are produced.
func TestEnrichACMCertificate_ValidInUseProducesNoFindings(t *testing.T) {
	fake := &acmDescribeCertificateFake{
		results: map[string]*acmtypes.CertificateDetail{
			acmARN1: acmCertDetail(acmARN1, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-1/aabbccdd"}, acmtypes.CertificateStatusIssued),
			acmARN2: acmCertDetail(acmARN2, 90*24*time.Hour, []string{"arn:aws:cloudfront::123456789012:distribution/EDFDVBD6EXAMPLE"}, acmtypes.CertificateStatusIssued),
			acmARN3: acmCertDetail(acmARN3, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-3/aabbccdd"}, acmtypes.CertificateStatusIssued),
		},
	}
	clients := &awsclient.ServiceClients{ACM: fake}
	resources := acmCertResources(acmARN1, acmARN2, acmARN3)

	result, err := awsclient.EnrichACMCertificate(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichACMCertificate_ExpiringSoonProducesFindingSevBang verifies that when
// cert-1 expires within 30 days (NotAfter=now+10d), a finding with severity "!" and
// a summary containing "expires" is produced for cert-1 only.
func TestEnrichACMCertificate_ExpiringSoonProducesFindingSevBang(t *testing.T) {
	fake := &acmDescribeCertificateFake{
		results: map[string]*acmtypes.CertificateDetail{
			acmARN1: acmCertDetail(acmARN1, 10*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-1/aabbccdd"}, acmtypes.CertificateStatusIssued),
			acmARN2: acmCertDetail(acmARN2, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"}, acmtypes.CertificateStatusIssued),
			acmARN3: acmCertDetail(acmARN3, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-3/aabbccdd"}, acmtypes.CertificateStatusIssued),
		},
	}
	clients := &awsclient.ServiceClients{ACM: fake}
	resources := acmCertResources(acmARN1, acmARN2, acmARN3)

	result, err := awsclient.EnrichACMCertificate(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[acmARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", acmARN1)
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if !strings.Contains(strings.ToLower(f.Summary), "expires") {
		t.Errorf("summary %q must contain \"expires\"", f.Summary)
	}
	if _, ok := result.Findings[acmARN2]; ok {
		t.Error("cert-2 must NOT appear in Findings — it is not expiring soon")
	}
	if _, ok := result.Findings[acmARN3]; ok {
		t.Error("cert-3 must NOT appear in Findings — it is not expiring soon")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichACMCertificate_ExpiredProducesFindingSevBang verifies that when cert-1
// has already expired (NotAfter=now-1d), a finding with severity "!" is produced for
// cert-1 only.
func TestEnrichACMCertificate_ExpiredProducesFindingSevBang(t *testing.T) {
	fake := &acmDescribeCertificateFake{
		results: map[string]*acmtypes.CertificateDetail{
			acmARN1: acmCertDetail(acmARN1, -24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-1/aabbccdd"}, acmtypes.CertificateStatusExpired),
			acmARN2: acmCertDetail(acmARN2, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"}, acmtypes.CertificateStatusIssued),
			acmARN3: acmCertDetail(acmARN3, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-3/aabbccdd"}, acmtypes.CertificateStatusIssued),
		},
	}
	clients := &awsclient.ServiceClients{ACM: fake}
	resources := acmCertResources(acmARN1, acmARN2, acmARN3)

	result, err := awsclient.EnrichACMCertificate(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[acmARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", acmARN1)
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if _, ok := result.Findings[acmARN2]; ok {
		t.Error("cert-2 must NOT appear in Findings — it is valid")
	}
	if _, ok := result.Findings[acmARN3]; ok {
		t.Error("cert-3 must NOT appear in Findings — it is valid")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichACMCertificate_OrphanIssuedProducesFindingSevTilde verifies that when
// cert-1 is Status=ISSUED but InUseBy is empty (orphan), a finding with severity "~"
// is produced for cert-1 only. cert-2 and cert-3 are in use and produce no finding.
func TestEnrichACMCertificate_OrphanIssuedProducesFindingSevTilde(t *testing.T) {
	fake := &acmDescribeCertificateFake{
		results: map[string]*acmtypes.CertificateDetail{
			acmARN1: acmCertDetail(acmARN1, 90*24*time.Hour, []string{}, acmtypes.CertificateStatusIssued),
			acmARN2: acmCertDetail(acmARN2, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"}, acmtypes.CertificateStatusIssued),
			acmARN3: acmCertDetail(acmARN3, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-3/aabbccdd"}, acmtypes.CertificateStatusIssued),
		},
	}
	clients := &awsclient.ServiceClients{ACM: fake}
	resources := acmCertResources(acmARN1, acmARN2, acmARN3)

	result, err := awsclient.EnrichACMCertificate(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[acmARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (orphan cert)", acmARN1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings[acmARN2]; ok {
		t.Error("cert-2 must NOT appear in Findings — it is in use")
	}
	if _, ok := result.Findings[acmARN3]; ok {
		t.Error("cert-3 must NOT appear in Findings — it is in use")
	}
	// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichACMCertificate_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.ACM is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichACMCertificate_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{ACM: nil}

	result, err := awsclient.EnrichACMCertificate(context.Background(), clients, acmCertResources(acmARN1, acmARN2, acmARN3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when ACM client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichACMCertificate_APIErrorSetsTruncatedNoError verifies that when the API
// call for cert-1 returns an error, the enricher sets Truncated=true, produces 0
// findings for that cert, and does not propagate the error.
func TestEnrichACMCertificate_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("acm: DescribeCertificate throttled")
	fake := &acmDescribeCertificateFake{
		errByArn: map[string]error{
			acmARN1: apiErr,
		},
		results: map[string]*acmtypes.CertificateDetail{
			acmARN2: acmCertDetail(acmARN2, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"}, acmtypes.CertificateStatusIssued),
			acmARN3: acmCertDetail(acmARN3, 90*24*time.Hour, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-3/aabbccdd"}, acmtypes.CertificateStatusIssued),
		},
	}
	clients := &awsclient.ServiceClients{ACM: fake}
	resources := acmCertResources(acmARN1, acmARN2, acmARN3)

	result, err := awsclient.EnrichACMCertificate(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
