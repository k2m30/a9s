package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
)

// ACMFixtures holds typed fixture data for ACM.
type ACMFixtures struct {
	Certificates []acmtypes.CertificateSummary
}

const (
	ProdACMCertARN1 = "arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111"
	ProdACMCertARN2 = "arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222"
)

func mustParseACMTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewACMFixtures constructs ACMFixtures from the canonical demo data.
func NewACMFixtures() *ACMFixtures {
	return &ACMFixtures{
		Certificates: []acmtypes.CertificateSummary{
			{
				DomainName:     aws.String("acme-corp.com"),
				CertificateArn: aws.String(ProdACMCertARN1),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(mustParseACMTime("2027-04-15T23:59:59+00:00")),
				NotBefore:      aws.Time(mustParseACMTime("2025-04-15T00:00:00+00:00")),
				IssuedAt:       aws.Time(time.Date(2025, 4, 15, 10, 0, 0, 0, time.UTC)),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Date(2025, 4, 15, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"acme-corp.com",
					"www.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityEligible,
			},
			{
				DomainName:     aws.String("*.acme-corp.com"),
				CertificateArn: aws.String(ProdACMCertARN2),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(mustParseACMTime("2027-06-20T23:59:59+00:00")),
				NotBefore:      aws.Time(mustParseACMTime("2025-06-20T00:00:00+00:00")),
				IssuedAt:       aws.Time(time.Date(2025, 6, 20, 14, 0, 0, 0, time.UTC)),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Date(2025, 6, 20, 14, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"*.acme-corp.com",
					"acme-corp.com",
					"assets.acme-corp.com",
					"api.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityEligible,
			},
			{
				DomainName:     aws.String("staging.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/c3d4e5f6-7890-12ab-cdef-333333333333"),
				Status:         acmtypes.CertificateStatusPendingValidation,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				InUse:          aws.Bool(false),
				CreatedAt:      aws.Time(time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"staging.acme-corp.com",
					"*.staging.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
			{
				DomainName:     aws.String("legacy.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/d4e5f6a7-8901-23ab-cdef-444444444444"),
				Status:         acmtypes.CertificateStatusExpired,
				Type:           acmtypes.CertificateTypeImported,
				NotAfter:       aws.Time(mustParseACMTime("2025-12-31T23:59:59+00:00")),
				InUse:          aws.Bool(false),
				ImportedAt:     aws.Time(time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"legacy.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
		},
	}
}
