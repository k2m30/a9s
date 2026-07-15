package unit

// qa_acm_uses_arn_from_fields_test.go — Regression: EnrichACMCertificate must
// call DescribeCertificate with the certificate ARN from
// r.Fields["certificate_arn"], NOT the bare domain in r.ID.
//
// Same shape as the tg/sfn/elb bugs: the acm fetcher (acm.go) sets
// `ID: domainName` and stores the ARN in Fields["certificate_arn"].
// Passing r.ID directly as CertificateArn fails against real AWS with
// ValidationError.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// strictACMFake mirrors AWS: rejects DescribeCertificate when CertificateArn
// is not a valid ARN. mu guards calledWith, which is written concurrently:
// EnrichACMCertificate fans out DescribeCertificate calls per resource via
// core/aws.ForEachParallel (EnrichmentParallelism goroutines).
type strictACMFake struct {
	awsclient.ACMAPI
	mu         sync.Mutex
	calledWith string
}

func (f *strictACMFake) DescribeCertificate(
	_ context.Context,
	input *acm.DescribeCertificateInput,
	_ ...func(*acm.Options),
) (*acm.DescribeCertificateOutput, error) {
	got := aws.ToString(input.CertificateArn)
	f.mu.Lock()
	f.calledWith = got
	f.mu.Unlock()
	if !strings.HasPrefix(got, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "ValidationError",
			Message: "'" + got + "' is not a valid ARN",
		}
	}
	return &acm.DescribeCertificateOutput{
		Certificate: &acmtypes.CertificateDetail{CertificateArn: &got},
	}, nil
}

// calledWithSafe returns calledWith, safe for concurrent use with
// DescribeCertificate.
func (f *strictACMFake) calledWithSafe() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calledWith
}

// TestEnrichACMCertificate_UsesARNFromFields verifies the enricher passes
// r.Fields["certificate_arn"] to DescribeCertificate, not r.ID (the bare
// domain name set by the acm fetcher).
func TestEnrichACMCertificate_UsesARNFromFields(t *testing.T) {
	const domain = "example.com"
	const certARN = "arn:aws:acm:us-east-1:123456789012:certificate/abc123-def456-7890-1234-56789abcdef0"

	fake := &strictACMFake{}
	clients := &awsclient.ServiceClients{ACM: fake}
	resources := []resource.Resource{{
		ID:     domain,
		Name:   domain,
		Fields: map[string]string{"certificate_arn": certARN},
	}}

	_, err := awsclient.EnrichACMCertificate(context.Background(), clients, resources, nil)
	if err != nil && strings.Contains(err.Error(), "ValidationError") {
		t.Fatalf("enricher passed bare domain to AWS instead of ARN; got: %v", err)
	}
	if got := fake.calledWithSafe(); got != certARN {
		t.Errorf("DescribeCertificate was called with %q, want %q (ARN from Fields[\"certificate_arn\"])",
			got, certARN)
	}
}
