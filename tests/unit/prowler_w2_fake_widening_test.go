package unit

// prowler_w2_fake_widening_test.go — posture-neutral stubs that keep the
// pre-existing fakes satisfying the aggregate interfaces the w2 Prowler batch
// widened (S3API/S3FullAPI gained the five bucket-posture reads, RDSAPI and
// DocDBAPI gained the snapshot attribute read, RedshiftAPI gained the cluster
// parameter read).
//
// Every stub answers the healthy value for its condition, so the fakes that
// carry them keep producing exactly the findings their own tests were written
// to assert. A stub that returned a misconfigured value would silently add a
// finding to fixtures those tests believe are clean.

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// ---------------------------------------------------------------------------
// healthy answers, shared by every S3 stub below
// ---------------------------------------------------------------------------

func w2HealthyPolicyStatus() *s3.GetBucketPolicyStatusOutput {
	return &s3.GetBucketPolicyStatusOutput{PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(false)}}
}

func w2HealthyVersioning() *s3.GetBucketVersioningOutput {
	return &s3.GetBucketVersioningOutput{
		Status:    s3types.BucketVersioningStatusEnabled,
		MFADelete: s3types.MFADeleteStatusEnabled,
	}
}

func w2HealthyLogging() *s3.GetBucketLoggingOutput {
	return &s3.GetBucketLoggingOutput{
		LoggingEnabled: &s3types.LoggingEnabled{TargetBucket: aws.String("acme-access-logs")},
	}
}

func w2HealthyLifecycle() *s3.GetBucketLifecycleConfigurationOutput {
	return &s3.GetBucketLifecycleConfigurationOutput{
		Rules: []s3types.LifecycleRule{{ID: aws.String("expire-old"), Status: s3types.ExpirationStatusEnabled}},
	}
}

func w2HealthyObjectLock() *s3.GetObjectLockConfigurationOutput {
	return &s3.GetObjectLockConfigurationOutput{
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{ObjectLockEnabled: s3types.ObjectLockEnabledEnabled},
	}
}

func w2NoRDSClusterSnapshotShares() *rdstypes.DBClusterSnapshotAttributesResult {
	return &rdstypes.DBClusterSnapshotAttributesResult{
		DBClusterSnapshotAttributes: []rdstypes.DBClusterSnapshotAttribute{{AttributeName: aws.String("restore")}},
	}
}

func w2NoDocDBClusterSnapshotShares() *docdbtypes.DBClusterSnapshotAttributesResult {
	return &docdbtypes.DBClusterSnapshotAttributesResult{
		DBClusterSnapshotAttributes: []docdbtypes.DBClusterSnapshotAttribute{{AttributeName: aws.String("restore")}},
	}
}

func w2RequireSSLOn() *redshift.DescribeClusterParametersOutput {
	return &redshift.DescribeClusterParametersOutput{
		Parameters: []redshifttypes.Parameter{
			{ParameterName: aws.String("require_ssl"), ParameterValue: aws.String("true")},
		},
	}
}

// ---------------------------------------------------------------------------
// s3
// ---------------------------------------------------------------------------

func (f *coalesceS3Fake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2HealthyPolicyStatus(), nil
}

func (f *coalesceS3Fake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2HealthyVersioning(), nil
}

func (f *coalesceS3Fake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2HealthyObjectLock(), nil
}

func (f *enrichS3Fake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2HealthyPolicyStatus(), nil
}

func (f *enrichS3Fake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2HealthyVersioning(), nil
}

func (f *enrichS3Fake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2HealthyLogging(), nil
}

func (f *enrichS3Fake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2HealthyObjectLock(), nil
}

func (f *enrichS3FakeNoBucketAPIs) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2HealthyPolicyStatus(), nil
}

func (f *enrichS3FakeNoBucketAPIs) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2HealthyVersioning(), nil
}

func (f *enrichS3FakeNoBucketAPIs) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2HealthyLogging(), nil
}

func (f *enrichS3FakeNoBucketAPIs) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2HealthyLifecycle(), nil
}

func (f *enrichS3FakeNoBucketAPIs) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2HealthyObjectLock(), nil
}

func (f *s3PABFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return w2HealthyPolicyStatus(), nil
}

func (f *s3PABFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return w2HealthyVersioning(), nil
}

func (f *s3PABFake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return w2HealthyLogging(), nil
}

func (f *s3PABFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return w2HealthyLifecycle(), nil
}

func (f *s3PABFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return w2HealthyObjectLock(), nil
}

// The two cross-region fakes deliberately answer the cross-region error on
// every call, so the enricher's "one redirect marks the bucket unknown and
// skips the rest" rule is exercised on the new calls too.
func (f *s3CrossRegionFake) GetBucketPolicyStatus(ctx context.Context, in *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyPolicyStatus(), nil
}

func (f *s3CrossRegionFake) GetBucketVersioning(ctx context.Context, in *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyVersioning(), nil
}

func (f *s3CrossRegionFake) GetBucketLogging(ctx context.Context, in *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyLogging(), nil
}

func (f *s3CrossRegionFake) GetBucketLifecycleConfiguration(ctx context.Context, in *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyLifecycle(), nil
}

func (f *s3CrossRegionFake) GetObjectLockConfiguration(ctx context.Context, in *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyObjectLock(), nil
}

func (f *fakeS3IllegalLocation) GetBucketPolicyStatus(ctx context.Context, in *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyPolicyStatus(), nil
}

func (f *fakeS3IllegalLocation) GetBucketVersioning(ctx context.Context, in *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyVersioning(), nil
}

func (f *fakeS3IllegalLocation) GetBucketLogging(ctx context.Context, in *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyLogging(), nil
}

func (f *fakeS3IllegalLocation) GetBucketLifecycleConfiguration(ctx context.Context, in *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyLifecycle(), nil
}

func (f *fakeS3IllegalLocation) GetObjectLockConfiguration(ctx context.Context, in *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	if _, err := f.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: in.Bucket}); err != nil {
		return nil, err
	}
	return w2HealthyObjectLock(), nil
}

// ---------------------------------------------------------------------------
// rds / docdb / redshift
// ---------------------------------------------------------------------------

func (m *dbcMaintenanceFake) DescribeDBClusterSnapshotAttributes(_ context.Context, in *docdb.DescribeDBClusterSnapshotAttributesInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotAttributesOutput, error) {
	res := w2NoDocDBClusterSnapshotShares()
	res.DBClusterSnapshotIdentifier = in.DBClusterSnapshotIdentifier
	return &docdb.DescribeDBClusterSnapshotAttributesOutput{DBClusterSnapshotAttributesResult: res}, nil
}

func (m *dbcSnapDocDBMock) DescribeDBClusterSnapshotAttributes(_ context.Context, in *docdb.DescribeDBClusterSnapshotAttributesInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotAttributesOutput, error) {
	res := w2NoDocDBClusterSnapshotShares()
	res.DBClusterSnapshotIdentifier = in.DBClusterSnapshotIdentifier
	return &docdb.DescribeDBClusterSnapshotAttributesOutput{DBClusterSnapshotAttributesResult: res}, nil
}

func (m *fullDocDBMock) DescribeDBClusterSnapshotAttributes(_ context.Context, in *docdb.DescribeDBClusterSnapshotAttributesInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotAttributesOutput, error) {
	res := w2NoDocDBClusterSnapshotShares()
	res.DBClusterSnapshotIdentifier = in.DBClusterSnapshotIdentifier
	return &docdb.DescribeDBClusterSnapshotAttributesOutput{DBClusterSnapshotAttributesResult: res}, nil
}

func (m *dbcSnapRDSMock) DescribeDBClusterSnapshotAttributes(_ context.Context, in *rds.DescribeDBClusterSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	res := w2NoRDSClusterSnapshotShares()
	res.DBClusterSnapshotIdentifier = in.DBClusterSnapshotIdentifier
	return &rds.DescribeDBClusterSnapshotAttributesOutput{DBClusterSnapshotAttributesResult: res}, nil
}

func (m *fullRDSMock) DescribeDBClusterSnapshotAttributes(_ context.Context, in *rds.DescribeDBClusterSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	res := w2NoRDSClusterSnapshotShares()
	res.DBClusterSnapshotIdentifier = in.DBClusterSnapshotIdentifier
	return &rds.DescribeDBClusterSnapshotAttributesOutput{DBClusterSnapshotAttributesResult: res}, nil
}

func (f *fakeRedshiftClient) DescribeClusterParameters(_ context.Context, _ *redshift.DescribeClusterParametersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterParametersOutput, error) {
	return w2RequireSSLOn(), nil
}

func w2AvailableEngineVersions(in *rds.DescribeDBEngineVersionsInput) *rds.DescribeDBEngineVersionsOutput {
	return &rds.DescribeDBEngineVersionsOutput{
		DBEngineVersions: []rdstypes.DBEngineVersion{{
			Engine:        in.Engine,
			EngineVersion: in.EngineVersion,
			Status:        aws.String("available"),
		}},
	}
}

func w2NoSnapshotShares(in *rds.DescribeDBSnapshotAttributesInput) *rds.DescribeDBSnapshotAttributesOutput {
	return &rds.DescribeDBSnapshotAttributesOutput{
		DBSnapshotAttributesResult: &rdstypes.DBSnapshotAttributesResult{
			DBSnapshotIdentifier: in.DBSnapshotIdentifier,
			DBSnapshotAttributes: []rdstypes.DBSnapshotAttribute{{AttributeName: aws.String("restore")}},
		},
	}
}

func (m *dbcSnapRDSMock) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *fullRDSMock) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *dbcSnapRDSMock) DescribeDBSnapshotAttributes(_ context.Context, in *rds.DescribeDBSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	return w2NoSnapshotShares(in), nil
}

func (m *fullRDSMock) DescribeDBSnapshotAttributes(_ context.Context, in *rds.DescribeDBSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	return w2NoSnapshotShares(in), nil
}

// The dbi maintenance fake predates the engine-version call the enricher now
// makes; answering "available" keeps its cases about pending maintenance.
func (m *dbiMaintenanceFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *noCWMaintenanceFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *noCWFetchRDSClient) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *enrichRDSFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *dbiStackedFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *rdsMaintenanceBugFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *rdsFindingKeyFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *rdsUnboundedMaintenanceFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *invRDSFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *dbiMaintOverdueFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	return w2AvailableEngineVersions(in), nil
}

func (m *ddbContinuousBackupsFake) GetResourcePolicy(_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "PolicyNotFoundException", Message: "no resource policy"}
}

func (m *mockDDBKinesisClient) GetResourcePolicy(_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetResourcePolicyOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "PolicyNotFoundException", Message: "no resource policy"}
}

func (m *efsMountTargetFake) DescribeFileSystemPolicy(_ context.Context, _ *efs.DescribeFileSystemPolicyInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemPolicyOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "PolicyNotFound", Message: "no policy"}
}

func (m *efsMountTargetFake) DescribeBackupPolicy(_ context.Context, _ *efs.DescribeBackupPolicyInput, _ ...func(*efs.Options)) (*efs.DescribeBackupPolicyOutput, error) {
	return &efs.DescribeBackupPolicyOutput{BackupPolicy: &efstypes.BackupPolicy{Status: efstypes.StatusEnabled}}, nil
}
