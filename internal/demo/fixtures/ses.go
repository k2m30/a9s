package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESFixtures holds typed fixture data for SESv2.
type SESFixtures struct {
	Identities []sesv2types.IdentityInfo
}

// NewSESFixtures constructs SESFixtures from the canonical demo data.
func NewSESFixtures() *SESFixtures {
	return &SESFixtures{
		Identities: []sesv2types.IdentityInfo{
			{
				IdentityName:       aws.String("acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeDomain,
				SendingEnabled:     true,
				VerificationStatus: sesv2types.VerificationStatusSuccess,
			},
			{
				IdentityName:       aws.String("noreply@acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeEmailAddress,
				SendingEnabled:     true,
				VerificationStatus: sesv2types.VerificationStatusSuccess,
			},
			{
				IdentityName:       aws.String("alerts@acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeEmailAddress,
				SendingEnabled:     false,
				VerificationStatus: sesv2types.VerificationStatusPending,
			},
			// Issue: VerificationStatus=Pending → Warning (verification DNS record not yet detected)
			{
				IdentityName:       aws.String("staging.acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeDomain,
				SendingEnabled:     false,
				VerificationStatus: sesv2types.VerificationStatusPending,
			},
			// Issue: VerificationStatus=Failed → Broken (domain verification definitively failed)
			{
				IdentityName:       aws.String("ses-failed.acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeDomain,
				SendingEnabled:     false,
				VerificationStatus: sesv2types.VerificationStatusFailed,
			},
			// Issue: Success but SendingEnabled=false → Warning (verified but sending disabled)
			{
				IdentityName:       aws.String("suppressed@acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeEmailAddress,
				SendingEnabled:     false,
				VerificationStatus: sesv2types.VerificationStatusSuccess,
			},
		},
	}
}
