// fakes_boundary_test.go — minimal fake AWS client implementations used by
// aws_boundary_test.go (US3 boundary semantic tests).
//
// All fakes are in package unit_test so they are isolated from the shared
// state in mocks_test.go (package unit).
package unit_test

import (
	"context"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/smithy-go"
)

// ---------------------------------------------------------------------------
// boundaryAPIError — implements smithy.APIError for access-denied / throttle
// testing.
// ---------------------------------------------------------------------------

type boundaryAPIError struct {
	code    string
	message string
}

func (e *boundaryAPIError) Error() string                 { return e.message }
func (e *boundaryAPIError) ErrorCode() string             { return e.code }
func (e *boundaryAPIError) ErrorMessage() string          { return e.message }
func (e *boundaryAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// newAccessDeniedError returns an error that ClassifyAWSError maps to
// retryable=false with code "AccessDeniedException".
func newAccessDeniedError() error {
	return &boundaryAPIError{code: "AccessDeniedException", message: "access denied by IAM policy"}
}

// newThrottleError returns an error that ClassifyAWSError maps to
// retryable=true with code "Throttling".
func newThrottleError() error {
	return &boundaryAPIError{code: "Throttling", message: "rate exceeded"}
}

// ---------------------------------------------------------------------------
// fakeEC2BoundaryAccessDenied — EC2 fake that returns AccessDenied on
// DescribeSubnets. All other methods return safe empty responses.
// Implements EC2API.
// ---------------------------------------------------------------------------

type fakeEC2BoundaryAccessDenied struct{}

func (f fakeEC2BoundaryAccessDenied) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return nil, newAccessDeniedError()
}
func (f fakeEC2BoundaryAccessDenied) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeTransitGatewayAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayAttachmentsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeImages(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeVolumeStatus(_ context.Context, _ *ec2.DescribeVolumeStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return &ec2.DescribeVolumeStatusOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeTransitGatewayVpcAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayVpcAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeTransitGatewayRouteTables(_ context.Context, _ *ec2.DescribeTransitGatewayRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error) {
	return &ec2.DescribeTransitGatewayRouteTablesOutput{}, nil
}
func (f fakeEC2BoundaryAccessDenied) DescribeLaunchTemplateVersions(_ context.Context, _ *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}

// ---------------------------------------------------------------------------
// fakeEC2BoundaryThrottle — EC2 fake whose DescribeSubnets returns a throttle
// error on the first call and succeeds with the given VPC IDs on subsequent
// calls. Implements EC2API. The call counter is atomic for goroutine-safety.
// ---------------------------------------------------------------------------

type fakeEC2BoundaryThrottle struct {
	calls  atomic.Int32
	vpcIDs []string // returned on the successful (2nd+) call
}

func (f *fakeEC2BoundaryThrottle) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	n := f.calls.Add(1)
	if n == 1 {
		return nil, newThrottleError()
	}
	subnets := make([]ec2types.Subnet, 0, len(f.vpcIDs))
	for _, vid := range f.vpcIDs {
		vid := vid // capture loop variable
		subnets = append(subnets, ec2types.Subnet{VpcId: &vid})
	}
	return &ec2.DescribeSubnetsOutput{Subnets: subnets}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeTransitGatewayAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayAttachmentsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeImages(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeVolumeStatus(_ context.Context, _ *ec2.DescribeVolumeStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return &ec2.DescribeVolumeStatusOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeTransitGatewayVpcAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayVpcAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeTransitGatewayRouteTables(_ context.Context, _ *ec2.DescribeTransitGatewayRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error) {
	return &ec2.DescribeTransitGatewayRouteTablesOutput{}, nil
}
func (f *fakeEC2BoundaryThrottle) DescribeLaunchTemplateVersions(_ context.Context, _ *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}

// ---------------------------------------------------------------------------
// fakeBackupBoundaryAccessDenied — Backup fake whose
// ListRecoveryPointsByResource returns AccessDenied. Implements BackupAPI.
// ---------------------------------------------------------------------------

type fakeBackupBoundaryAccessDenied struct{}

func (f fakeBackupBoundaryAccessDenied) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{}, nil
}
func (f fakeBackupBoundaryAccessDenied) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{}, nil
}
func (f fakeBackupBoundaryAccessDenied) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	return &backup.GetBackupPlanOutput{}, nil
}
func (f fakeBackupBoundaryAccessDenied) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return &backup.ListBackupSelectionsOutput{}, nil
}
func (f fakeBackupBoundaryAccessDenied) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	return &backup.DescribeBackupVaultOutput{}, nil
}
func (f fakeBackupBoundaryAccessDenied) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}
func (f fakeBackupBoundaryAccessDenied) ListRecoveryPointsByResource(_ context.Context, _ *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	return nil, newAccessDeniedError()
}

// ---------------------------------------------------------------------------
// fakeBackupBoundaryThrottle — Backup fake whose
// ListRecoveryPointsByResource returns a throttle error on the first call and
// succeeds with one recovery point ARN on subsequent calls.
// ---------------------------------------------------------------------------

type fakeBackupBoundaryThrottle struct {
	calls         atomic.Int32
	recoveryPoint string // ARN to return on successful call
}

func (f *fakeBackupBoundaryThrottle) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{}, nil
}
func (f *fakeBackupBoundaryThrottle) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{}, nil
}
func (f *fakeBackupBoundaryThrottle) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	return &backup.GetBackupPlanOutput{}, nil
}
func (f *fakeBackupBoundaryThrottle) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return &backup.ListBackupSelectionsOutput{}, nil
}
func (f *fakeBackupBoundaryThrottle) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	return &backup.DescribeBackupVaultOutput{}, nil
}
func (f *fakeBackupBoundaryThrottle) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}
func (f *fakeBackupBoundaryThrottle) ListRecoveryPointsByResource(_ context.Context, _ *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	n := f.calls.Add(1)
	if n == 1 {
		return nil, newThrottleError()
	}
	rp := backuptypes.RecoveryPointByResource{RecoveryPointArn: &f.recoveryPoint}
	return &backup.ListRecoveryPointsByResourceOutput{RecoveryPoints: []backuptypes.RecoveryPointByResource{rp}}, nil
}

// ---------------------------------------------------------------------------
// fakeKMSBoundaryAccessDenied — KMS fake whose GetKeyPolicy returns
// AccessDenied. ListGrants returns an empty response so the second call
// inside checkKMSRole doesn't interfere. Implements KMSAPI.
// ---------------------------------------------------------------------------

type fakeKMSBoundaryAccessDenied struct{}

func (f fakeKMSBoundaryAccessDenied) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{}, nil
}
func (f fakeKMSBoundaryAccessDenied) DescribeKey(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	return &kms.DescribeKeyOutput{}, nil
}
func (f fakeKMSBoundaryAccessDenied) ListAliases(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{}, nil
}
func (f fakeKMSBoundaryAccessDenied) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}
func (f fakeKMSBoundaryAccessDenied) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}
func (f fakeKMSBoundaryAccessDenied) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return nil, newAccessDeniedError()
}

// ---------------------------------------------------------------------------
// fakeDynamoDBBoundaryAccessDenied — DynamoDB fake whose
// DescribeKinesisStreamingDestination returns AccessDenied. Used to test
// checkDdbKinesis access-denied path. Implements DynamoDBAPI.
// ---------------------------------------------------------------------------

type fakeDynamoDBBoundaryAccessDenied struct{}

func (f fakeDynamoDBBoundaryAccessDenied) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{}, nil
}
func (f fakeDynamoDBBoundaryAccessDenied) DescribeTable(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, nil
}
func (f fakeDynamoDBBoundaryAccessDenied) DescribeContinuousBackups(_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{}, nil
}
func (f fakeDynamoDBBoundaryAccessDenied) DescribeKinesisStreamingDestination(_ context.Context, _ *dynamodb.DescribeKinesisStreamingDestinationInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	return nil, newAccessDeniedError()
}
