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
