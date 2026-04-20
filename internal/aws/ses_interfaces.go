package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

// SESv2ListEmailIdentitiesAPI defines the interface for the SES v2 ListEmailIdentities operation.
type SESv2ListEmailIdentitiesAPI interface {
	ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, optFns ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error)
}

// SESv2GetAccountAPI defines the interface for the SES v2 GetAccount operation.
type SESv2GetAccountAPI interface {
	GetAccount(ctx context.Context, params *sesv2.GetAccountInput, optFns ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error)
}

// SESv2GetConfigurationSetEventDestinationsAPI defines the interface for the SES v2 GetConfigurationSetEventDestinations operation.
type SESv2GetConfigurationSetEventDestinationsAPI interface {
	GetConfigurationSetEventDestinations(ctx context.Context, params *sesv2.GetConfigurationSetEventDestinationsInput, optFns ...func(*sesv2.Options)) (*sesv2.GetConfigurationSetEventDestinationsOutput, error)
}

// SESv2GetEmailIdentityAPI for ses→{kms, role} (GetEmailIdentity returns DkimAttributes,
// ConfigurationSetName, Tags, MailFromAttributes).
type SESv2GetEmailIdentityAPI interface {
	GetEmailIdentity(ctx context.Context, params *sesv2.GetEmailIdentityInput, optFns ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error)
}

// SESv2API is the aggregate interface covering all SESv2 operations used by a9s fetchers.
// *sesv2.Client structurally satisfies this interface.
type SESv2API interface {
	SESv2ListEmailIdentitiesAPI
	SESv2GetAccountAPI
	SESv2GetEmailIdentityAPI
	SESv2GetConfigurationSetEventDestinationsAPI
}
