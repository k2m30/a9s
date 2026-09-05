package unit_test

// prowler_w2_fake_widening_ext_test.go — the package unit_test half of the
// posture-neutral stubs described in prowler_w2_fake_widening_test.go.
//
// Same rule: every stub answers the healthy value, so the related-checker and
// boundary tests that own these fakes keep asserting exactly what they were
// written to assert.

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// noResourcePolicy is what DynamoDB answers for a table nobody has shared.
func w2xNoResourcePolicy() error {
	return &smithy.GenericAPIError{Code: "PolicyNotFoundException", Message: "no resource policy"}
}

func w2xHealthyPolicyStatus() *s3.GetBucketPolicyStatusOutput {
	return &s3.GetBucketPolicyStatusOutput{PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(false)}}
}

func w2xHealthyVersioning() *s3.GetBucketVersioningOutput {
	return &s3.GetBucketVersioningOutput{
		Status:    s3types.BucketVersioningStatusEnabled,
		MFADelete: s3types.MFADeleteStatusEnabled,
	}
}

func w2xHealthyLogging() *s3.GetBucketLoggingOutput {
	return &s3.GetBucketLoggingOutput{
		LoggingEnabled: &s3types.LoggingEnabled{TargetBucket: aws.String("acme-access-logs")},
	}
}

func w2xHealthyLifecycle() *s3.GetBucketLifecycleConfigurationOutput {
	return &s3.GetBucketLifecycleConfigurationOutput{
		Rules: []s3types.LifecycleRule{{ID: aws.String("expire-old"), Status: s3types.ExpirationStatusEnabled}},
	}
}

func w2xHealthyObjectLock() *s3.GetObjectLockConfigurationOutput {
	return &s3.GetObjectLockConfigurationOutput{
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{ObjectLockEnabled: s3types.ObjectLockEnabledEnabled},
	}
}

// ---------------------------------------------------------------------------
// dynamodb
// ---------------------------------------------------------------------------

func (f *fakeDynamoDBForKinesis) GetResourcePolicy(_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	return nil, w2xNoResourcePolicy()
}

func (f fakeDynamoDBBoundaryAccessDenied) GetResourcePolicy(_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "access denied"}
}

// ---------------------------------------------------------------------------
// redshift
// ---------------------------------------------------------------------------

func (f *fakeRedshiftCR) DescribeClusterParameters(_ context.Context, _ *redshift.DescribeClusterParametersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterParametersOutput, error) {
	return &redshift.DescribeClusterParametersOutput{
		Parameters: []redshifttypes.Parameter{
			{ParameterName: aws.String("require_ssl"), ParameterValue: aws.String("true")},
		},
	}, nil
}

// ---------------------------------------------------------------------------
// s3
// ---------------------------------------------------------------------------

func (f *s3TaggingErrFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2xHealthyPolicyStatus(), nil
}

func (f *s3TaggingErrFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2xHealthyVersioning(), nil
}

func (f *s3TaggingErrFake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2xHealthyLogging(), nil
}

func (f *s3TaggingErrFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2xHealthyLifecycle(), nil
}

func (f *s3TaggingErrFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2xHealthyObjectLock(), nil
}

func (f *s3EncryptionErrFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2xHealthyPolicyStatus(), nil
}

func (f *s3EncryptionErrFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2xHealthyVersioning(), nil
}

func (f *s3EncryptionErrFake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2xHealthyLogging(), nil
}

func (f *s3EncryptionErrFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2xHealthyLifecycle(), nil
}

func (f *s3EncryptionErrFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2xHealthyObjectLock(), nil
}

func (f *s3LoggingErrFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2xHealthyPolicyStatus(), nil
}

func (f *s3LoggingErrFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2xHealthyVersioning(), nil
}

func (f *s3LoggingErrFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2xHealthyLifecycle(), nil
}

func (f *s3LoggingErrFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2xHealthyObjectLock(), nil
}

func (f *s3PolicyErrFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2xHealthyPolicyStatus(), nil
}

func (f *s3PolicyErrFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2xHealthyVersioning(), nil
}

func (f *s3PolicyErrFake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2xHealthyLogging(), nil
}

func (f *s3PolicyErrFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2xHealthyLifecycle(), nil
}

func (f *s3PolicyErrFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2xHealthyObjectLock(), nil
}

func (f *s3EncryptionGarbageKeyFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2xHealthyPolicyStatus(), nil
}

func (f *s3EncryptionGarbageKeyFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2xHealthyVersioning(), nil
}

func (f *s3EncryptionGarbageKeyFake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2xHealthyLogging(), nil
}

func (f *s3EncryptionGarbageKeyFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2xHealthyLifecycle(), nil
}

func (f *s3EncryptionGarbageKeyFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2xHealthyObjectLock(), nil
}
