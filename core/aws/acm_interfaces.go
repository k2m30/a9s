package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/acm"
)

// ACMListCertificatesAPI defines the interface for the ACM ListCertificates operation.
type ACMListCertificatesAPI interface {
	ListCertificates(ctx context.Context, params *acm.ListCertificatesInput, optFns ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
}

// ACMDescribeCertificateAPI defines the interface for the ACM DescribeCertificate operation.
type ACMDescribeCertificateAPI interface {
	DescribeCertificate(ctx context.Context, params *acm.DescribeCertificateInput, optFns ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error)
}

// ACMAPI is the aggregate interface covering all ACM operations used by a9s fetchers.
// *acm.Client structurally satisfies this interface.
type ACMAPI interface {
	ACMListCertificatesAPI
	ACMDescribeCertificateAPI
}
