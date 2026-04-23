package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ses"
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
	return f.fix.GetAccountDefault, nil
}

// GetEmailIdentity returns the configured output for the requested identity, or an empty
// output when the identity has no configuration set. This satisfies the eb-rule, kinesis,
// and sns related-panel pivots for the graph-root identity "acme-corp.com".
func (f *SESFake) GetEmailIdentity(_ context.Context, in *sesv2.GetEmailIdentityInput, _ ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	if in != nil && in.EmailIdentity != nil {
		if out, ok := f.fix.GetEmailIdentityByName[*in.EmailIdentity]; ok {
			return out, nil
		}
	}
	return &sesv2.GetEmailIdentityOutput{}, nil
}

// GetConfigurationSetEventDestinations returns the event destinations for the requested
// configuration set. Returns an empty output when the config set is unknown.
func (f *SESFake) GetConfigurationSetEventDestinations(_ context.Context, in *sesv2.GetConfigurationSetEventDestinationsInput, _ ...func(*sesv2.Options)) (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
	if in != nil && in.ConfigurationSetName != nil {
		if out, ok := f.fix.EventDestinationsByConfigSet[*in.ConfigurationSetName]; ok {
			return out, nil
		}
	}
	return &sesv2.GetConfigurationSetEventDestinationsOutput{}, nil
}

// SESV1Fake implements aws.SESV1API against fixture data loaded at construction time.
// It handles the SES v1 DescribeActiveReceiptRuleSet call used by the lambda and s3
// related-panel pivots (inbound receipt rules).
type SESV1Fake struct {
	fix *fixtures.SESFixtures
}

// NewSESV1 constructs a SESV1Fake backed by fixture data from the fixtures package.
func NewSESV1() *SESV1Fake {
	return &SESV1Fake{fix: fixtures.NewSESFixtures()}
}

// DescribeActiveReceiptRuleSet returns the demo active receipt rule set, which contains
// one rule with S3Action and LambdaAction wired to the graph-root identity.
func (f *SESV1Fake) DescribeActiveReceiptRuleSet(_ context.Context, _ *ses.DescribeActiveReceiptRuleSetInput, _ ...func(*ses.Options)) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	return f.fix.ActiveReceiptRuleSet, nil
}
