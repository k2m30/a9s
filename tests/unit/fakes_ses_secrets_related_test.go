// fakes_ses_secrets_related_test.go contains lightweight fake implementations of AWS
// service client interfaces used by the ses and secrets related-panel
// checker tests.
// Covered: SESv2API (ses→eb-rule, ses→kinesis, ses→sns via GetConfigurationSetEventDestinations),
// SecretsManagerAPI (secrets→role via GetResourcePolicy),
// LambdaAPI (secrets→logs, secrets→role, secrets→sns via GetFunction).
// All types are in package unit_test (external test package).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// ---------------------------------------------------------------------------
// fakeSESv2Checker — implements SESv2API
// Controllable method: GetConfigurationSetEventDestinations — keyed by config set name.
// GetEmailIdentity is also controllable to return the ConfigurationSetName.
// All other methods return safe empty stubs.
// ---------------------------------------------------------------------------

type fakeSESv2Checker struct {
	getConfigSetEventDestsFn func(*sesv2.GetConfigurationSetEventDestinationsInput) (*sesv2.GetConfigurationSetEventDestinationsOutput, error)
	getEmailIdentityFn       func(*sesv2.GetEmailIdentityInput) (*sesv2.GetEmailIdentityOutput, error)
}

func (f *fakeSESv2Checker) ListEmailIdentities(_ context.Context, _ *sesv2.ListEmailIdentitiesInput, _ ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	return &sesv2.ListEmailIdentitiesOutput{}, nil
}

func (f *fakeSESv2Checker) GetAccount(_ context.Context, _ *sesv2.GetAccountInput, _ ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error) {
	return &sesv2.GetAccountOutput{}, nil
}

func (f *fakeSESv2Checker) GetConfigurationSetEventDestinations(_ context.Context, input *sesv2.GetConfigurationSetEventDestinationsInput, _ ...func(*sesv2.Options)) (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
	if f.getConfigSetEventDestsFn != nil {
		return f.getConfigSetEventDestsFn(input)
	}
	return &sesv2.GetConfigurationSetEventDestinationsOutput{}, nil
}

func (f *fakeSESv2Checker) GetEmailIdentity(_ context.Context, input *sesv2.GetEmailIdentityInput, _ ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	if f.getEmailIdentityFn != nil {
		return f.getEmailIdentityFn(input)
	}
	return &sesv2.GetEmailIdentityOutput{}, nil
}

// Compile-time check: fakeSESv2Checker satisfies SESv2API.
var _ awsclient.SESv2API = (*fakeSESv2Checker)(nil)

// newFakeSESv2WithEventDestinations returns a fakeSESv2Checker whose
// GetEmailIdentity returns configSetName for identityName, and whose
// GetConfigurationSetEventDestinations returns the given event destinations.
func newFakeSESv2WithEventDestinations(identityName, configSetName string, dests []sesv2types.EventDestination) *fakeSESv2Checker {
	return &fakeSESv2Checker{
		getEmailIdentityFn: func(input *sesv2.GetEmailIdentityInput) (*sesv2.GetEmailIdentityOutput, error) {
			name := ""
			if input.EmailIdentity != nil {
				name = *input.EmailIdentity
			}
			if name == identityName {
				return &sesv2.GetEmailIdentityOutput{
					ConfigurationSetName: aws.String(configSetName),
				}, nil
			}
			return &sesv2.GetEmailIdentityOutput{}, nil
		},
		getConfigSetEventDestsFn: func(_ *sesv2.GetConfigurationSetEventDestinationsInput) (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
			return &sesv2.GetConfigurationSetEventDestinationsOutput{
				EventDestinations: dests,
			}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeSecretsManagerChecker — implements SecretsManagerAPI
// Controllable method: GetResourcePolicy.
// ---------------------------------------------------------------------------

type fakeSecretsManagerChecker struct {
	getResourcePolicyOutput *secretsmanager.GetResourcePolicyOutput
	getResourcePolicyErr    error
}

func (f *fakeSecretsManagerChecker) ListSecrets(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	return &secretsmanager.ListSecretsOutput{}, nil
}

func (f *fakeSecretsManagerChecker) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return &secretsmanager.GetSecretValueOutput{}, nil
}

func (f *fakeSecretsManagerChecker) GetResourcePolicy(_ context.Context, _ *secretsmanager.GetResourcePolicyInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetResourcePolicyOutput, error) {
	if f.getResourcePolicyErr != nil {
		return nil, f.getResourcePolicyErr
	}
	if f.getResourcePolicyOutput != nil {
		return f.getResourcePolicyOutput, nil
	}
	return &secretsmanager.GetResourcePolicyOutput{}, nil
}

// Compile-time check: fakeSecretsManagerChecker satisfies SecretsManagerAPI.
var _ awsclient.SecretsManagerAPI = (*fakeSecretsManagerChecker)(nil)

// newFakeSecretsManagerWithResourcePolicy returns a fakeSecretsManagerChecker
// whose GetResourcePolicy returns the given IAM policy JSON.
func newFakeSecretsManagerWithResourcePolicy(policyJSON string) *fakeSecretsManagerChecker {
	return &fakeSecretsManagerChecker{
		getResourcePolicyOutput: &secretsmanager.GetResourcePolicyOutput{
			ResourcePolicy: aws.String(policyJSON),
		},
	}
}

// newFakeSecretsManagerNoPolicy returns a fakeSecretsManagerChecker whose
// GetResourcePolicy returns nil (no policy attached to secret).
func newFakeSecretsManagerNoPolicy() *fakeSecretsManagerChecker {
	return &fakeSecretsManagerChecker{
		getResourcePolicyOutput: &secretsmanager.GetResourcePolicyOutput{
			ResourcePolicy: nil,
		},
	}
}

// ---------------------------------------------------------------------------
// fakeLambdaForSecrets — implements LambdaAPI
// Controllable method: GetFunction — returns FunctionConfiguration per invocation.
// ---------------------------------------------------------------------------

type fakeLambdaForSecrets struct {
	getFunctionFn func(*lambdapkg.GetFunctionInput) (*lambdapkg.GetFunctionOutput, error)
}

func (f *fakeLambdaForSecrets) ListFunctions(_ context.Context, _ *lambdapkg.ListFunctionsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListFunctionsOutput, error) {
	return &lambdapkg.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaForSecrets) ListEventSourceMappings(_ context.Context, _ *lambdapkg.ListEventSourceMappingsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListEventSourceMappingsOutput, error) {
	return &lambdapkg.ListEventSourceMappingsOutput{}, nil
}

func (f *fakeLambdaForSecrets) GetFunction(_ context.Context, input *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	if f.getFunctionFn != nil {
		return f.getFunctionFn(input)
	}
	return &lambdapkg.GetFunctionOutput{}, nil
}

func (f *fakeLambdaForSecrets) ListTags(_ context.Context, _ *lambdapkg.ListTagsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListTagsOutput, error) {
	return &lambdapkg.ListTagsOutput{}, nil
}

// Compile-time check: fakeLambdaForSecrets satisfies LambdaAPI.
var _ awsclient.LambdaAPI = (*fakeLambdaForSecrets)(nil)

// newFakeLambdaWithFunctionConfig returns a fakeLambdaForSecrets whose GetFunction
// returns the given FunctionConfiguration regardless of which function is requested.
func newFakeLambdaWithFunctionConfig(cfg *lambdatypes.FunctionConfiguration) *fakeLambdaForSecrets {
	return &fakeLambdaForSecrets{
		getFunctionFn: func(_ *lambdapkg.GetFunctionInput) (*lambdapkg.GetFunctionOutput, error) {
			return &lambdapkg.GetFunctionOutput{Configuration: cfg}, nil
		},
	}
}

// newFakeLambdaWithDLQ returns a fakeLambdaForSecrets whose GetFunction
// returns a FunctionConfiguration with the given DeadLetterConfig.TargetArn.
func newFakeLambdaWithDLQ(functionName, dlqArn string) *fakeLambdaForSecrets {
	return &fakeLambdaForSecrets{
		getFunctionFn: func(_ *lambdapkg.GetFunctionInput) (*lambdapkg.GetFunctionOutput, error) {
			return &lambdapkg.GetFunctionOutput{
				Configuration: &lambdatypes.FunctionConfiguration{
					FunctionName: aws.String(functionName),
					DeadLetterConfig: &lambdatypes.DeadLetterConfig{
						TargetArn: aws.String(dlqArn),
					},
				},
			}, nil
		},
	}
}
