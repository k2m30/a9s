package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// SESFake implements aws.SESv2API against fixture data loaded at construction time.
type SESFake struct {
	fix *fixtures.SESFixtures
}

// NewSES constructs a SESFake backed by fixture data from the fixtures package.
func NewSES() *SESFake {
	return &SESFake{fix: fixtures.NewSESFixtures()}
}

func (f *SESFake) ListEmailIdentities(_ context.Context, _ *sesv2.ListEmailIdentitiesInput, _ ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	return &sesv2.ListEmailIdentitiesOutput{EmailIdentities: f.fix.Identities}, nil
}

func (f *SESFake) GetAccount(_ context.Context, _ *sesv2.GetAccountInput, _ ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error) {
	return &sesv2.GetAccountOutput{SendingEnabled: true}, nil
}

// GetEmailIdentity returns an empty identity — demo mode does not model SES identity details.
func (f *SESFake) GetEmailIdentity(_ context.Context, _ *sesv2.GetEmailIdentityInput, _ ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	return &sesv2.GetEmailIdentityOutput{}, nil
}

// GetConfigurationSetEventDestinations is a no-op stub satisfying SESv2GetConfigurationSetEventDestinationsAPI.
// Demo mode does not model SES configuration set event destinations.
func (f *SESFake) GetConfigurationSetEventDestinations(_ context.Context, _ *sesv2.GetConfigurationSetEventDestinationsInput, _ ...func(*sesv2.Options)) (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
	return &sesv2.GetConfigurationSetEventDestinationsOutput{}, nil
}
