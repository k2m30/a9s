// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
)

// ACMFixtures holds typed fixture data for ACM.
type ACMFixtures struct {
	Certificates []acmtypes.CertificateSummary
	// InUseBy maps a certificate ARN to the ARNs of resources using it, mirroring
	// acm:DescribeCertificate.Certificate.InUseBy. Backs the acm→elb and acm→apigw
	// related-panel pivots (checkACMELB / checkACMAPIGW).
	InUseBy map[string][]string
	// DomainValidationOptions maps a certificate ARN to its DNS validation
	// records, mirroring acm:DescribeCertificate.Certificate.DomainValidationOptions.
	// Backs the acm→r53 related-panel pivot (checkACMR53).
	DomainValidationOptions map[string][]acmtypes.DomainValidation
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
var sharedACMFixtures = sync.OnceValue(func() *ACMFixtures {
	return &ACMFixtures{
		Certificates: []acmtypes.CertificateSummary{
			{
				// ACMWeakKey: the one certificate on a key below 2048 bits.
				// Every other certificate fixture is RSA 2048, so nothing else
				// trips acm.weak-key.
				DomainName:     aws.String(ACMWeakKey),
				CertificateArn: aws.String(ProdACMCertARN1),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(mustParseACMTime("2027-04-15T23:59:59+00:00")),
				NotBefore:      aws.Time(mustParseACMTime("2025-04-15T00:00:00+00:00")),
				IssuedAt:       aws.Time(time.Date(2025, 4, 15, 10, 0, 0, 0, time.UTC)),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Date(2025, 4, 15, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa1024,
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
			// Issue: Status=INACTIVE → Dim (imported cert not currently in use for TLS)
			{
				DomainName:     aws.String("inactive.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/c9d0e1f2-3456-78ab-cdef-999999999999"),
				Status:         acmtypes.CertificateStatusInactive,
				Type:           acmtypes.CertificateTypeImported,
				NotAfter:       aws.Time(mustParseACMTime("2027-01-01T23:59:59+00:00")),
				InUse:          aws.Bool(false),
				ImportedAt:     aws.Time(time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"inactive.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityIneligible,
			},
			// Issue: ISSUED but NotAfter in the past → Broken (acm.expires-critical)
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
			// Issue: ISSUED, NotAfter ~20 days out → Warning (acm.expires-soon —
			// inside the 30d window, outside the 7d critical threshold). Computed
			// relative to time.Now() so the fixture stays in the warn window
			// regardless of when the demo runs.
			{
				DomainName:     aws.String("renewal-window.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/0d0d0d0d-0d0d-0d0d-0d0d-0d0d0d0d0d0d"),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(time.Now().Add(20 * 24 * time.Hour)),
				NotBefore:      aws.Time(time.Now().Add(-345 * 24 * time.Hour)),
				IssuedAt:       aws.Time(time.Now().Add(-345 * 24 * time.Hour)),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Now().Add(-345 * 24 * time.Hour)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"renewal-window.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityEligible,
			},
			// Issue: ISSUED, healthy expiry, InUse=false → Warning (acm.orphan).
			{
				DomainName:     aws.String("orphaned-cert.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/0e0e0e0e-0e0e-0e0e-0e0e-0e0e0e0e0e0e"),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(time.Now().Add(200 * 24 * time.Hour)),
				NotBefore:      aws.Time(time.Now().Add(-165 * 24 * time.Hour)),
				IssuedAt:       aws.Time(time.Now().Add(-165 * 24 * time.Hour)),
				InUse:          aws.Bool(false),
				CreatedAt:      aws.Time(time.Now().Add(-165 * 24 * time.Hour)),
				KeyAlgorithm:   acmtypes.KeyAlgorithmRsa2048,
				SubjectAlternativeNameSummaries: []string{
					"orphaned-cert.acme-corp.com",
				},
				RenewalEligibility: acmtypes.RenewalEligibilityEligible,
			},
		},
		// InUseBy — backs the acm→elb and acm→apigw related-panel pivots.
		// ProdACMCertARN1 (acme-corp.com) is attached to the prod ALB
		// (elb.go fixtProdELBARN) and to the public API Gateway custom
		// domain (apigw.go PublicAPIGWID) via a realistic InUseBy ARN set.
		InUseBy: map[string][]string{
			ProdACMCertARN1: {
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef",
				"arn:aws:apigateway:us-east-1::/restapis/abc123def4",
			},
		},
		// DomainValidationOptions — backs the acm→r53 related-panel pivot.
		// The validation CNAME for ProdACMCertARN1 lives under
		// acme-corp.com, matching the public zone Z0123456789ABCDEFGHIJ
		// (r53.go) by longest-suffix match.
		DomainValidationOptions: map[string][]acmtypes.DomainValidation{
			ProdACMCertARN1: {
				{
					DomainName: aws.String("acme-corp.com"),
					ResourceRecord: &acmtypes.ResourceRecord{
						Name:  aws.String("_a1b2c3d4e5f6.acme-corp.com."),
						Type:  acmtypes.RecordTypeCname,
						Value: aws.String("_f6e5d4c3b2a1.acm-validations.aws."),
					},
					ValidationStatus: acmtypes.DomainStatusSuccess,
				},
			},
		},
	}
})

func NewACMFixtures() *ACMFixtures {
	return sharedACMFixtures()
}

// ACMWeakKey is the ONE demo certificate with an RSA key below 2048 bits for
// the w6a batch. Every other certificate fixture carries RSA 2048 or an
// elliptic-curve algorithm.
const ACMWeakKey = "acme-corp.com"

func init() {
	Register(Pin{ShortName: "acm", Rows: 12, Issues: 9})
}
