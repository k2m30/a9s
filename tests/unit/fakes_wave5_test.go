// fakes_wave5_test.go contains lightweight fake implementations of AWS service
// client interfaces used by the coverage-wave-5 zero-hit branch tests.
// Covered: CWLogsAPI+CWLogsDescribeSubscriptionFiltersAPI (logs→kinesis/s3),
// GlueAPI+GlueGetTagsAPI (glue→cfn), GlueAPI+GlueGetSecurityConfigurationAPI
// (glue→kms), LambdaAPI with ListEventSourceMappings (ddb→lambda).
// All types are in package unit_test (external test package).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// fakeCWLogsWithSubFilters — implements CWLogsAPI + CWLogsDescribeSubscriptionFiltersAPI.
// Only DescribeSubscriptionFilters is controllable; all other methods return
// safe empty stubs so the compile-time assertion below passes.
// ---------------------------------------------------------------------------

type fakeCWLogsWithSubFilters struct {
	subFilters []cloudwatchlogstypes.SubscriptionFilter
	subFilErr  error
}

func (f *fakeCWLogsWithSubFilters) DescribeLogGroups(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (f *fakeCWLogsWithSubFilters) DescribeLogStreams(_ context.Context, _ *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	return &cloudwatchlogs.DescribeLogStreamsOutput{}, nil
}

func (f *fakeCWLogsWithSubFilters) GetLogEvents(_ context.Context, _ *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	return &cloudwatchlogs.GetLogEventsOutput{}, nil
}

func (f *fakeCWLogsWithSubFilters) FilterLogEvents(_ context.Context, _ *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	return &cloudwatchlogs.FilterLogEventsOutput{}, nil
}

func (f *fakeCWLogsWithSubFilters) DescribeMetricFilters(_ context.Context, _ *cloudwatchlogs.DescribeMetricFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error) {
	return &cloudwatchlogs.DescribeMetricFiltersOutput{}, nil
}

func (f *fakeCWLogsWithSubFilters) DescribeSubscriptionFilters(_ context.Context, _ *cloudwatchlogs.DescribeSubscriptionFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error) {
	if f.subFilErr != nil {
		return nil, f.subFilErr
	}
	return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{SubscriptionFilters: f.subFilters}, nil
}

// Compile-time checks: satisfies both aggregates required by ServiceClients.
var _ awsclient.CWLogsAPI = (*fakeCWLogsWithSubFilters)(nil)
var _ awsclient.CWLogsDescribeSubscriptionFiltersAPI = (*fakeCWLogsWithSubFilters)(nil)

// newFakeCWLogsWithSubFilters returns a fake with the given subscription filter list.
func newFakeCWLogsWithSubFilters(filters []cloudwatchlogstypes.SubscriptionFilter) *fakeCWLogsWithSubFilters {
	return &fakeCWLogsWithSubFilters{subFilters: filters}
}

// ---------------------------------------------------------------------------
// fakeGlueWithTags — implements GlueAPI + GlueGetTagsAPI.
// GetTags is controllable; GetJobs / GetJobRuns return empty stubs.
// ---------------------------------------------------------------------------

type fakeGlueWithTags struct {
	tags    map[string]string
	tagsErr error
}

func (f *fakeGlueWithTags) GetJobs(_ context.Context, _ *glue.GetJobsInput, _ ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	return &glue.GetJobsOutput{}, nil
}

func (f *fakeGlueWithTags) GetJobRuns(_ context.Context, _ *glue.GetJobRunsInput, _ ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	return &glue.GetJobRunsOutput{}, nil
}

func (f *fakeGlueWithTags) GetTags(_ context.Context, _ *glue.GetTagsInput, _ ...func(*glue.Options)) (*glue.GetTagsOutput, error) {
	if f.tagsErr != nil {
		return nil, f.tagsErr
	}
	return &glue.GetTagsOutput{Tags: f.tags}, nil
}

// Compile-time checks.
var _ awsclient.GlueAPI = (*fakeGlueWithTags)(nil)
var _ awsclient.GlueGetTagsAPI = (*fakeGlueWithTags)(nil)

// newFakeGlueWithStackTag returns a fakeGlueWithTags whose GetTags returns
// a single aws:cloudformation:stack-name tag entry.
func newFakeGlueWithStackTag(stackName string) *fakeGlueWithTags {
	return &fakeGlueWithTags{
		tags: map[string]string{"aws:cloudformation:stack-name": stackName},
	}
}

// newFakeGlueWithNoStackTag returns a fakeGlueWithTags whose GetTags returns
// no cloudformation tag (stack-name will be empty string).
func newFakeGlueWithNoStackTag() *fakeGlueWithTags {
	return &fakeGlueWithTags{tags: map[string]string{"env": "prod"}}
}

// ---------------------------------------------------------------------------
// fakeGlueWithSecurityConfig — implements GlueAPI + GlueGetSecurityConfigurationAPI.
// GetSecurityConfiguration is controllable; GetJobs / GetJobRuns return empty stubs.
// ---------------------------------------------------------------------------

type fakeGlueWithSecurityConfig struct {
	secCfgOut *glue.GetSecurityConfigurationOutput
	secCfgErr error
}

func (f *fakeGlueWithSecurityConfig) GetJobs(_ context.Context, _ *glue.GetJobsInput, _ ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	return &glue.GetJobsOutput{}, nil
}

func (f *fakeGlueWithSecurityConfig) GetJobRuns(_ context.Context, _ *glue.GetJobRunsInput, _ ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	return &glue.GetJobRunsOutput{}, nil
}

func (f *fakeGlueWithSecurityConfig) GetSecurityConfiguration(_ context.Context, _ *glue.GetSecurityConfigurationInput, _ ...func(*glue.Options)) (*glue.GetSecurityConfigurationOutput, error) {
	if f.secCfgErr != nil {
		return nil, f.secCfgErr
	}
	if f.secCfgOut != nil {
		return f.secCfgOut, nil
	}
	return &glue.GetSecurityConfigurationOutput{}, nil
}

// Compile-time checks.
var _ awsclient.GlueAPI = (*fakeGlueWithSecurityConfig)(nil)
var _ awsclient.GlueGetSecurityConfigurationAPI = (*fakeGlueWithSecurityConfig)(nil)

// newFakeGlueWithKMSConfig returns a fakeGlueWithSecurityConfig whose
// GetSecurityConfiguration returns a security config with CloudWatch KMS ARN.
func newFakeGlueWithKMSConfig(kmsKeyARN string) *fakeGlueWithSecurityConfig {
	return &fakeGlueWithSecurityConfig{
		secCfgOut: &glue.GetSecurityConfigurationOutput{
			SecurityConfiguration: &gluetypes.SecurityConfiguration{
				EncryptionConfiguration: &gluetypes.EncryptionConfiguration{
					CloudWatchEncryption: &gluetypes.CloudWatchEncryption{
						KmsKeyArn: &kmsKeyARN,
					},
				},
			},
		},
	}
}

// newFakeGlueWithEmptyEncryption returns a fake where GetSecurityConfiguration
// returns a security config with no encryption entries (empty EncryptionConfiguration).
func newFakeGlueWithEmptyEncryption() *fakeGlueWithSecurityConfig {
	return &fakeGlueWithSecurityConfig{
		secCfgOut: &glue.GetSecurityConfigurationOutput{
			SecurityConfiguration: &gluetypes.SecurityConfiguration{
				EncryptionConfiguration: &gluetypes.EncryptionConfiguration{},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// fakeLambdaWithEventSourceMappings — implements LambdaAPI.
// ListEventSourceMappings is controllable; all other methods return stubs.
// ---------------------------------------------------------------------------

type fakeLambdaWithESM struct {
	esmOutput *lambdapkg.ListEventSourceMappingsOutput
	esmErr    error
}

func (f *fakeLambdaWithESM) ListFunctions(_ context.Context, _ *lambdapkg.ListFunctionsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListFunctionsOutput, error) {
	return &lambdapkg.ListFunctionsOutput{}, nil
}

func (f *fakeLambdaWithESM) ListEventSourceMappings(_ context.Context, _ *lambdapkg.ListEventSourceMappingsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListEventSourceMappingsOutput, error) {
	if f.esmErr != nil {
		return nil, f.esmErr
	}
	if f.esmOutput != nil {
		return f.esmOutput, nil
	}
	return &lambdapkg.ListEventSourceMappingsOutput{}, nil
}

func (f *fakeLambdaWithESM) GetFunction(_ context.Context, _ *lambdapkg.GetFunctionInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	return &lambdapkg.GetFunctionOutput{}, nil
}

func (f *fakeLambdaWithESM) ListTags(_ context.Context, _ *lambdapkg.ListTagsInput, _ ...func(*lambdapkg.Options)) (*lambdapkg.ListTagsOutput, error) {
	return &lambdapkg.ListTagsOutput{}, nil
}

// Compile-time check.
var _ awsclient.LambdaAPI = (*fakeLambdaWithESM)(nil)

// newFakeLambdaWithESMFunctions returns a fake whose ListEventSourceMappings
// returns a mapping for each given function ARN.
func newFakeLambdaWithESMFunctions(functionARNs []string) *fakeLambdaWithESM {
	mappings := make([]lambdatypes.EventSourceMappingConfiguration, 0, len(functionARNs))
	for i := range functionARNs {
		arn := functionARNs[i]
		mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{FunctionArn: &arn})
	}
	return &fakeLambdaWithESM{
		esmOutput: &lambdapkg.ListEventSourceMappingsOutput{EventSourceMappings: mappings},
	}
}
