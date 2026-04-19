// fakes_coverage_restore_test.go contains fake AWS service client implementations
// used by the coverage-restoration tests across athena, eventbridge_rule, backup,
// redshift, tg, and pipeline related checkers.
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// fakeAthenaCR — implements AthenaAPI (ListWorkGroups + GetWorkGroup)
// Used by: athena coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeAthenaCR struct {
	workGroupOutput *athena.GetWorkGroupOutput
	err             error
}

func (f *fakeAthenaCR) ListWorkGroups(_ context.Context, _ *athena.ListWorkGroupsInput, _ ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	return &athena.ListWorkGroupsOutput{}, nil
}

func (f *fakeAthenaCR) GetWorkGroup(_ context.Context, _ *athena.GetWorkGroupInput, _ ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.workGroupOutput != nil {
		return f.workGroupOutput, nil
	}
	return &athena.GetWorkGroupOutput{}, nil
}

var _ awsclient.AthenaAPI = (*fakeAthenaCR)(nil)

// newFakeAthenaWithS3URI returns a fakeAthenaCR whose GetWorkGroup response
// carries an OutputLocation pointing at the given s3URI.
func newFakeAthenaWithS3URI(s3URI string) *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					ResultConfiguration: &athenatypes.ResultConfiguration{
						OutputLocation: aws.String(s3URI),
					},
				},
			},
		},
	}
}

// newFakeAthenaWithKMSKey returns a fakeAthenaCR whose GetWorkGroup response
// carries a KMS key ARN in the EncryptionConfiguration.
func newFakeAthenaWithKMSKey(kmsKeyARN string) *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					ResultConfiguration: &athenatypes.ResultConfiguration{
						EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
							KmsKey: aws.String(kmsKeyARN),
						},
					},
				},
			},
		},
	}
}

// newFakeAthenaWithCWLogsEnabled returns a fakeAthenaCR whose GetWorkGroup response
// has PublishCloudWatchMetricsEnabled=true.
func newFakeAthenaWithCWLogsEnabled() *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					PublishCloudWatchMetricsEnabled: aws.Bool(true),
				},
			},
		},
	}
}

// newFakeAthenaWithExecutionRole returns a fakeAthenaCR whose GetWorkGroup response
// carries an ExecutionRole ARN.
func newFakeAthenaWithExecutionRole(roleARN string) *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					ExecutionRole: aws.String(roleARN),
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// fakeEventBridgeCR — implements EventBridgeAPI with controllable ListTargetsByRule.
// Used by: eventbridge_rule coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeEventBridgeCR struct {
	targets []eventbridgetypes.Target
	err     error
}

func (f *fakeEventBridgeCR) ListRules(_ context.Context, _ *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	return &eventbridge.ListRulesOutput{}, nil
}

func (f *fakeEventBridgeCR) ListTargetsByRule(_ context.Context, _ *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &eventbridge.ListTargetsByRuleOutput{Targets: f.targets}, nil
}

func (f *fakeEventBridgeCR) ListRuleNamesByTarget(_ context.Context, _ *eventbridge.ListRuleNamesByTargetInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	return &eventbridge.ListRuleNamesByTargetOutput{}, nil
}

var _ awsclient.EventBridgeAPI = (*fakeEventBridgeCR)(nil)

// ---------------------------------------------------------------------------
// fakeBackupCR — implements BackupAPI with controllable ListBackupSelections,
// GetBackupPlan, DescribeBackupVault, GetBackupVaultNotifications.
// Used by: backup coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeBackupCR struct {
	listSelectionsOutput        *backup.ListBackupSelectionsOutput
	listSelectionsErr           error
	getBackupPlanOutput         *backup.GetBackupPlanOutput
	getBackupPlanErr            error
	describeVaultOutput         *backup.DescribeBackupVaultOutput
	describeVaultErr            error
	getVaultNotificationsOutput *backup.GetBackupVaultNotificationsOutput
	getVaultNotificationsErr    error
}

func (f *fakeBackupCR) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{}, nil
}

func (f *fakeBackupCR) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{}, nil
}

func (f *fakeBackupCR) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	if f.getBackupPlanErr != nil {
		return nil, f.getBackupPlanErr
	}
	if f.getBackupPlanOutput != nil {
		return f.getBackupPlanOutput, nil
	}
	return &backup.GetBackupPlanOutput{}, nil
}

func (f *fakeBackupCR) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	if f.listSelectionsErr != nil {
		return nil, f.listSelectionsErr
	}
	if f.listSelectionsOutput != nil {
		return f.listSelectionsOutput, nil
	}
	return &backup.ListBackupSelectionsOutput{}, nil
}

func (f *fakeBackupCR) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	if f.describeVaultErr != nil {
		return nil, f.describeVaultErr
	}
	if f.describeVaultOutput != nil {
		return f.describeVaultOutput, nil
	}
	return &backup.DescribeBackupVaultOutput{}, nil
}

func (f *fakeBackupCR) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	if f.getVaultNotificationsErr != nil {
		return nil, f.getVaultNotificationsErr
	}
	if f.getVaultNotificationsOutput != nil {
		return f.getVaultNotificationsOutput, nil
	}
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}

func (f *fakeBackupCR) ListRecoveryPointsByResource(_ context.Context, _ *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	return &backup.ListRecoveryPointsByResourceOutput{}, nil
}

var _ awsclient.BackupAPI = (*fakeBackupCR)(nil)

// newFakeBackupCRWithVaultKMS returns a fakeBackupCR configured for KMS key
// resolution: GetBackupPlan with a vault rule, DescribeBackupVault with key ARN.
func newFakeBackupCRWithVaultKMS(vaultName, kmsARN string) *fakeBackupCR {
	vaultName2 := vaultName
	return &fakeBackupCR{
		getBackupPlanOutput: &backup.GetBackupPlanOutput{
			BackupPlan: &backuptypes.BackupPlan{
				BackupPlanName: aws.String("test-plan"),
				Rules: []backuptypes.BackupRule{
					{
						RuleName:              aws.String("rule-1"),
						TargetBackupVaultName: &vaultName2,
					},
				},
			},
		},
		describeVaultOutput: &backup.DescribeBackupVaultOutput{
			BackupVaultName: aws.String(vaultName),
			EncryptionKeyArn: aws.String(kmsARN),
		},
	}
}

// ---------------------------------------------------------------------------
// fakeRedshiftCR — implements RedshiftAPI (DescribeClusters, DescribeLoggingStatus,
// DescribeClusterSubnetGroups).
// Used by: redshift coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeRedshiftCR struct {
	loggingOutput    *redshift.DescribeLoggingStatusOutput
	loggingErr       error
	subnetOutput     *redshift.DescribeClusterSubnetGroupsOutput
	subnetErr        error
}

func (f *fakeRedshiftCR) DescribeClusters(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return &redshift.DescribeClustersOutput{}, nil
}

func (f *fakeRedshiftCR) DescribeLoggingStatus(_ context.Context, _ *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	if f.loggingErr != nil {
		return nil, f.loggingErr
	}
	if f.loggingOutput != nil {
		return f.loggingOutput, nil
	}
	return &redshift.DescribeLoggingStatusOutput{}, nil
}

func (f *fakeRedshiftCR) DescribeClusterSubnetGroups(_ context.Context, _ *redshift.DescribeClusterSubnetGroupsInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error) {
	if f.subnetErr != nil {
		return nil, f.subnetErr
	}
	if f.subnetOutput != nil {
		return f.subnetOutput, nil
	}
	return &redshift.DescribeClusterSubnetGroupsOutput{}, nil
}

var _ awsclient.RedshiftAPI = (*fakeRedshiftCR)(nil)

// ---------------------------------------------------------------------------
// fakeELBv2CR — implements ELBv2API + ELBv2DescribeTagsAPI for TG checker tests.
// Used by: tg coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeELBv2CR struct {
	describeTagsOutput       *elbv2.DescribeTagsOutput
	describeTagsErr          error
	describeTargetHealthOutput *elbv2.DescribeTargetHealthOutput
	describeTargetHealthErr    error
}

func (f *fakeELBv2CR) DescribeLoadBalancers(_ context.Context, _ *elbv2.DescribeLoadBalancersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{}, nil
}

func (f *fakeELBv2CR) DescribeTargetGroups(_ context.Context, _ *elbv2.DescribeTargetGroupsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	return &elbv2.DescribeTargetGroupsOutput{}, nil
}

func (f *fakeELBv2CR) DescribeTargetHealth(_ context.Context, _ *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	if f.describeTargetHealthErr != nil {
		return nil, f.describeTargetHealthErr
	}
	if f.describeTargetHealthOutput != nil {
		return f.describeTargetHealthOutput, nil
	}
	return &elbv2.DescribeTargetHealthOutput{}, nil
}

func (f *fakeELBv2CR) DescribeListeners(_ context.Context, _ *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	return &elbv2.DescribeListenersOutput{}, nil
}

func (f *fakeELBv2CR) DescribeRules(_ context.Context, _ *elbv2.DescribeRulesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	return &elbv2.DescribeRulesOutput{}, nil
}

func (f *fakeELBv2CR) DescribeLoadBalancerAttributes(_ context.Context, _ *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}

// DescribeTags satisfies ELBv2DescribeTagsAPI (used by checkTGCFN via type assertion).
func (f *fakeELBv2CR) DescribeTags(_ context.Context, _ *elbv2.DescribeTagsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	if f.describeTagsErr != nil {
		return nil, f.describeTagsErr
	}
	if f.describeTagsOutput != nil {
		return f.describeTagsOutput, nil
	}
	return &elbv2.DescribeTagsOutput{}, nil
}

var _ awsclient.ELBv2API = (*fakeELBv2CR)(nil)

// newFakeELBv2CRWithCFNTag returns a fakeELBv2CR whose DescribeTags returns
// the aws:cloudformation:stack-name tag for the TG resource.
func newFakeELBv2CRWithCFNTag(stackName string) *fakeELBv2CR {
	return &fakeELBv2CR{
		describeTagsOutput: &elbv2.DescribeTagsOutput{
			TagDescriptions: []elbv2types.TagDescription{
				{
					Tags: []elbv2types.Tag{
						{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String(stackName)},
					},
				},
			},
		},
	}
}

// newFakeELBv2CRWithTargetHealth returns a fakeELBv2CR whose DescribeTargetHealth
// returns the given target health descriptions.
func newFakeELBv2CRWithTargetHealth(targets []elbv2types.TargetHealthDescription) *fakeELBv2CR {
	return &fakeELBv2CR{
		describeTargetHealthOutput: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: targets,
		},
	}
}

// ---------------------------------------------------------------------------
// fakeS3CR — implements S3GetBucketTaggingAPI, S3GetBucketEncryptionAPI,
// S3GetBucketLoggingAPI for S3 checker tests.
// Used by: s3 coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeS3CR struct {
	getBucketTaggingOutput    *s3.GetBucketTaggingOutput
	getBucketTaggingErr       error
	getBucketEncryptionOutput *s3.GetBucketEncryptionOutput
	getBucketEncryptionErr    error
	getBucketLoggingOutput    *s3.GetBucketLoggingOutput
	getBucketLoggingErr       error
}

func (f *fakeS3CR) GetBucketTagging(_ context.Context, _ *s3.GetBucketTaggingInput, _ ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
	if f.getBucketTaggingErr != nil {
		return nil, f.getBucketTaggingErr
	}
	if f.getBucketTaggingOutput != nil {
		return f.getBucketTaggingOutput, nil
	}
	return &s3.GetBucketTaggingOutput{}, nil
}

func (f *fakeS3CR) GetBucketEncryption(_ context.Context, _ *s3.GetBucketEncryptionInput, _ ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error) {
	if f.getBucketEncryptionErr != nil {
		return nil, f.getBucketEncryptionErr
	}
	if f.getBucketEncryptionOutput != nil {
		return f.getBucketEncryptionOutput, nil
	}
	return &s3.GetBucketEncryptionOutput{}, nil
}

func (f *fakeS3CR) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	if f.getBucketLoggingErr != nil {
		return nil, f.getBucketLoggingErr
	}
	if f.getBucketLoggingOutput != nil {
		return f.getBucketLoggingOutput, nil
	}
	return &s3.GetBucketLoggingOutput{}, nil
}

// S3API methods required for c.S3 assignment (S3ListBucketsAPI, S3ListObjectsV2API,
// S3GetBucketNotificationConfigurationAPI, S3GetPublicAccessBlockAPI).

func (f *fakeS3CR) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}

func (f *fakeS3CR) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func (f *fakeS3CR) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}

func (f *fakeS3CR) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return &s3.GetPublicAccessBlockOutput{}, nil
}

var _ awsclient.S3API = (*fakeS3CR)(nil)

// newFakeS3CRWithKMS returns a fakeS3CR whose GetBucketEncryption response
// carries an aws:kms rule with the given key ID.
func newFakeS3CRWithKMS(keyID string) *fakeS3CR {
	return &fakeS3CR{
		getBucketEncryptionOutput: &s3.GetBucketEncryptionOutput{
			ServerSideEncryptionConfiguration: &s3types.ServerSideEncryptionConfiguration{
				Rules: []s3types.ServerSideEncryptionRule{
					{
						ApplyServerSideEncryptionByDefault: &s3types.ServerSideEncryptionByDefault{
							SSEAlgorithm:   s3types.ServerSideEncryptionAwsKms,
							KMSMasterKeyID: aws.String(keyID),
						},
					},
				},
			},
		},
	}
}

// newFakeS3CRWithTagging returns a fakeS3CR whose GetBucketTagging response
// includes the given key/value tag pairs (alternating key, value, key, value…).
func newFakeS3CRWithTagging(kvPairs ...string) *fakeS3CR {
	var tags []s3types.Tag
	for i := 0; i+1 < len(kvPairs); i += 2 {
		k, v := kvPairs[i], kvPairs[i+1]
		tags = append(tags, s3types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return &fakeS3CR{
		getBucketTaggingOutput: &s3.GetBucketTaggingOutput{TagSet: tags},
	}
}

// newFakeS3CRWithLogging returns a fakeS3CR whose GetBucketLogging response
// identifies the given target bucket as the logging destination.
func newFakeS3CRWithLogging(targetBucket string) *fakeS3CR {
	return &fakeS3CR{
		getBucketLoggingOutput: &s3.GetBucketLoggingOutput{
			LoggingEnabled: &s3types.LoggingEnabled{
				TargetBucket: aws.String(targetBucket),
				TargetPrefix: aws.String("logs/"),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Backup test-data helpers
// These return pointer types matching fakeBackupCR field types.
// ---------------------------------------------------------------------------

// backupListSelectionsWithRole returns a *ListBackupSelectionsOutput containing
// one selection with the given IAM role ARN.
func backupListSelectionsWithRole(roleARN string) *backup.ListBackupSelectionsOutput {
	return &backup.ListBackupSelectionsOutput{
		BackupSelectionsList: []backuptypes.BackupSelectionsListMember{
			{
				IamRoleArn:  aws.String(roleARN),
				SelectionId: aws.String("sel-00000001"),
			},
		},
	}
}

// backupEmptyPlan returns a *GetBackupPlanOutput whose BackupPlan has no rules.
func backupEmptyPlan() *backup.GetBackupPlanOutput {
	return &backup.GetBackupPlanOutput{
		BackupPlan: &backuptypes.BackupPlan{
			BackupPlanName: aws.String("empty-plan"),
			Rules:          []backuptypes.BackupRule{},
		},
	}
}

// backupPlanWithVault returns a *GetBackupPlanOutput whose BackupPlan contains
// a single rule targeting the named vault.
func backupPlanWithVault(vaultName string) *backup.GetBackupPlanOutput {
	vn := vaultName
	return &backup.GetBackupPlanOutput{
		BackupPlan: &backuptypes.BackupPlan{
			BackupPlanName: aws.String("plan-with-vault"),
			Rules: []backuptypes.BackupRule{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: &vn,
				},
			},
		},
	}
}

// backupVaultNotificationWithSNS returns a *GetBackupVaultNotificationsOutput
// with the given SNS topic ARN.
func backupVaultNotificationWithSNS(topicARN string) *backup.GetBackupVaultNotificationsOutput {
	return &backup.GetBackupVaultNotificationsOutput{
		SNSTopicArn: aws.String(topicARN),
	}
}

// newFakeAthenaWithEmptyConfig returns a fakeAthenaCR whose GetWorkGroup returns a
// WorkGroup with a non-nil Configuration but no OutputLocation, KMS key,
// ExecutionRole, or PublishCloudWatchMetrics. Used by "no value" branch tests that
// need cfg != nil so checkers fall through to Count=0 (not Count=-1).
func newFakeAthenaWithEmptyConfig() *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{},
			},
		},
	}
}
