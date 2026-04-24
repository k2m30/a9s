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
			// Issue: Status=REVOKED → Broken (certificate revoked by CA)
			{
				DomainName:     aws.String("revoked.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/e5f6a7b8-9012-34ab-cdef-555555555555"),
				Status:         acmtypes.CertificateStatusRevoked,
				Type:           acmtypes.CertificateTypeImported,
				NotAfter:       aws.Time(mustParseACMTime("2026-06-01T23:59:59+00:00")),
				NotBefore:      aws.Time(mustParseACMTime("2025-06-01T00:00:00+00:00")),
				InUse:          aws.Bool(false),
				ImportedAt:     aws.Time(time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"revoked.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
			// Issue: Status=FAILED → Broken (DNS/email validation failed)
			{
				DomainName:     aws.String("validation-failed.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/f6a7b8c9-0123-45ab-cdef-666666666666"),
				Status:         acmtypes.CertificateStatusFailed,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				InUse:          aws.Bool(false),
				CreatedAt:      aws.Time(time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"validation-failed.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
			// OpenSearch graph-root custom endpoint cert — required for opensearch→acm pivot.
			// checkOpenSearchACM calls DescribeDomainConfig and reads
			// DomainEndpointOptions.Options.CustomEndpointCertificateArn = OpenSearchACMCertARN.
			// The checker strips the ARN to the bare cert ID = OpenSearchACMCertID and looks it up here.
			{
				DomainName:     aws.String("acme-logs.internal.com"),
				CertificateArn: aws.String(OpenSearchACMCertARN),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(mustParseACMTime("2028-06-01T23:59:59+00:00")),
				NotBefore:      aws.Time(mustParseACMTime("2026-06-01T00:00:00+00:00")),
				IssuedAt:       aws.Time(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"acme-logs.internal.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityEligible,
			},
			// Issue: Status=VALIDATION_TIMED_OUT → Broken (DNS record never added)
			{
				DomainName:     aws.String("timeout.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/a7b8c9d0-1234-56ab-cdef-777777777777"),
				Status:         acmtypes.CertificateStatusValidationTimedOut,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				InUse:          aws.Bool(false),
				CreatedAt:      aws.Time(time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmEcSecp384r1,
				SubjectAlternativeNameSummaries: []string{
					"timeout.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
			// Issue: ISSUED but NotAfter in ~5 days → Broken (imminent expiry)
			{
				DomainName:     aws.String("expiring-soon.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/b8c9d0e1-2345-67ab-cdef-888888888888"),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeImported,
				NotAfter:       aws.Time(mustParseACMTime("2026-04-23T23:59:59+00:00")),
				NotBefore:      aws.Time(mustParseACMTime("2025-04-23T00:00:00+00:00")),
				IssuedAt:       aws.Time(time.Date(2025, 4, 23, 10, 0, 0, 0, time.UTC)),
				InUse:          aws.Bool(true),
				ImportedAt:     aws.Time(time.Date(2025, 4, 23, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"expiring-soon.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
		},
	}
}
