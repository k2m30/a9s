package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/acm"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ACMFake implements aws.ACMAPI against fixture data loaded at construction time.
type ACMFake struct {
	fix *fixtures.ACMFixtures
}

// NewACM constructs an ACMFake backed by fixture data from the fixtures package.
func NewACM() *ACMFake {
	return &ACMFake{fix: fixtures.NewACMFixtures()}
}

func (f *ACMFake) ListCertificates(_ context.Context, _ *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return &acm.ListCertificatesOutput{CertificateSummaryList: f.fix.Certificates}, nil
}

// DescribeCertificate is a no-op stub — the demo transport does not exercise Wave 2 enrichment.
func (f *ACMFake) DescribeCertificate(_ context.Context, _ *acm.DescribeCertificateInput, _ ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
	return &acm.DescribeCertificateOutput{}, nil
}
