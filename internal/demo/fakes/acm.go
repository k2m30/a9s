package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

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

// DescribeCertificate returns InUseBy + DomainValidationOptions for known demo
// certs so the acm:elb / acm:apigw / acm:r53 related-panel checkers (which
// call this API directly) resolve real witnesses instead of an empty stub.
// ARN validation is still enforced so that callers passing a bare certificate
// name are caught early.
func (f *ACMFake) DescribeCertificate(_ context.Context, input *acm.DescribeCertificateInput, _ ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
	if input == nil || input.CertificateArn == nil {
		return &acm.DescribeCertificateOutput{}, nil
	}
	certARN := *input.CertificateArn
	if err := validateARN(certARN); err != nil {
		return nil, err
	}
	inUseBy, ok := f.fix.InUseBy[certARN]
	if !ok {
		return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{CertificateArn: &certARN}}, nil
	}
	return &acm.DescribeCertificateOutput{
		Certificate: &acmtypes.CertificateDetail{
			CertificateArn:          &certARN,
			InUseBy:                 inUseBy,
			DomainValidationOptions: f.fix.DomainValidationOptions[certARN],
		},
	}, nil
}
