package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestErrorWrapping_AllFetchers verifies that every fetcher wraps errors with
// descriptive context rather than returning bare errors.
// Each sub-test injects a sentinel error through the mock and checks that
// the returned error:
//   - wraps the original (errors.Is)
//   - contains a descriptive "fetching ..." prefix
func TestErrorWrapping_AllFetchers(t *testing.T) {
	sentinel := fmt.Errorf("sentinel-api-error")
	ctx := context.Background()

	tests := []struct {
		name     string
		contains string // substring that must appear in the wrapped error
		call     func() error
	}{
		{
			name:     "EC2 instances",
			contains: "fetching EC2 instances",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchEC2InstancesPage(ctx, &mockEC2Client{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "S3 buckets",
			contains: "fetching S3 buckets",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchS3BucketsPage(ctx, &mockS3ListBucketsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "S3 objects",
			contains: "fetching S3 objects",
			call: func() error {
				_, err := awsclient.FetchS3Objects(ctx, &mockS3ListObjectsV2Client{err: sentinel}, "test-bucket", "", "")
				return err
			},
		},
		{
			name:     "RDS instances",
			contains: "fetching RDS instances",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchRDSInstancesPage(ctx, &mockRDSClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Redis replication groups",
			contains: "fetching Redis replication groups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchRedisPage(ctx, &mockElastiCacheReplicationGroupsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "DocDB clusters",
			contains: "fetching DocumentDB clusters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchDocDBClustersPage(ctx, &mockDocDBClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "EKS clusters - list error",
			contains: "listing EKS clusters",
			call: func() error {
				_, err := awsclient.FetchEKSClusters(ctx,
					&mockEKSListClustersClient{err: sentinel},
					&mockEKSDescribeClusterClient{})
				return err
			},
		},
		{
			name:     "Secrets",
			contains: "fetching secrets",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSecretsPage(ctx, &mockSecretsManagerClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "RevealSecret",
			contains: "revealing secret",
			call: func() error {
				_, err := awsclient.RevealSecret(ctx, &mockSecretsManagerGetSecretValueClient{err: sentinel}, "test-secret")
				return err
			},
		},
		{
			name:     "VPCs",
			contains: "fetching VPCs",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchVPCsPage(ctx, &mockEC2DescribeVpcsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Security groups",
			contains: "fetching security groups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSecurityGroupsPage(ctx, &mockEC2DescribeSecurityGroupsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Node groups - list clusters error",
			contains: "listing EKS clusters",
			call: func() error {
				_, err := awsclient.FetchNodeGroups(ctx,
					&mockEKSListClustersClient{err: sentinel},
					&mockEKSListNodegroupsClient{},
					&mockEKSDescribeNodegroupClient{})
				return err
			},
		},
		{
			name:     "Subnets",
			contains: "fetching subnets",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSubnetsPage(ctx, &mockEC2DescribeSubnetsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Route tables",
			contains: "fetching route tables",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchRouteTablesPage(ctx, &mockEC2DescribeRouteTablesClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "NAT gateways",
			contains: "fetching NAT gateways",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchNatGatewaysPage(ctx, &mockEC2DescribeNatGatewaysClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Internet gateways",
			contains: "fetching internet gateways",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchInternetGatewaysPage(ctx, &mockEC2DescribeInternetGatewaysClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Lambda functions",
			contains: "fetching Lambda functions",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchLambdaFunctionsPage(ctx, &mockLambdaListFunctionsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CloudWatch alarms",
			contains: "fetching CloudWatch alarms",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCloudWatchAlarmsPage(ctx, &mockCloudWatchDescribeAlarmsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "SNS topics",
			contains: "fetching SNS topics",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSNSTopicsPage(ctx, &mockSNSListTopicsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "SQS queues - list error",
			contains: "listing SQS queues",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSQSQueuesPage(ctx,
						&mockSQSListQueuesClient{err: sentinel},
						&mockSQSGetQueueAttributesClient{}, token)
				})
				return err
			},
		},
		{
			name:     "Load balancers",
			contains: "fetching load balancers",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchLoadBalancersPage(ctx, &mockELBv2DescribeLoadBalancersClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Target groups",
			contains: "fetching target groups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchTargetGroupsPage(ctx, &mockELBv2DescribeTargetGroupsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "ECS clusters - list error",
			contains: "listing ECS clusters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchECSClustersPage(ctx,
						&mockECSListClustersClient{err: sentinel},
						&mockECSDescribeClustersClient{}, token)
				})
				return err
			},
		},
		{
			name:     "ECS services - list clusters error",
			contains: "listing ECS clusters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchECSServicesPage(ctx,
						&mockECSListClustersClient{err: sentinel},
						&mockECSListServicesClient{},
						&mockECSDescribeServicesClient{}, token)
				})
				return err
			},
		},
		{
			name:     "CloudFormation stacks",
			contains: "fetching CloudFormation stacks",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCloudFormationStacksPage(ctx, &mockCFNDescribeStacksClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "IAM roles",
			contains: "fetching IAM roles",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchIAMRolesPage(ctx, &mockIAMListRolesClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CloudWatch log groups",
			contains: "fetching CloudWatch log groups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCloudWatchLogGroupsPage(ctx, &mockCWLogsDescribeLogGroupsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "SSM parameters",
			contains: "fetching SSM parameters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSSMParametersPage(ctx, &mockSSMDescribeParametersClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "DynamoDB tables - list error",
			contains: "listing DynamoDB tables",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchDynamoDBTablesPage(ctx,
						&mockDDBListTablesClient{err: sentinel},
						&mockDDBDescribeTableClient{}, token)
				})
				return err
			},
		},
		{
			name:     "Elastic IPs",
			contains: "fetching Elastic IPs",
			call: func() error {
				_, err := awsclient.FetchElasticIPs(ctx, &mockEC2DescribeAddressesClient{err: sentinel})
				return err
			},
		},
		{
			name:     "ACM certificates",
			contains: "fetching ACM certificates",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchACMCertificatesPage(ctx, &mockACMListCertificatesClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Auto Scaling groups",
			contains: "fetching Auto Scaling groups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchAutoScalingGroupsPage(ctx, &mockASGDescribeAutoScalingGroupsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "IAM users",
			contains: "fetching IAM users",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchIAMUsersPage(ctx, &mockIAMListUsersClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "IAM groups",
			contains: "fetching IAM groups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchIAMGroupsPage(ctx, &mockIAMListGroupsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "RDS snapshots",
			contains: "fetching RDS snapshots",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchDBISnapshotsPage(ctx, &mockRDSDescribeDBSnapshotsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Transit gateways",
			contains: "fetching transit gateways",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchTransitGatewaysPage(ctx, &mockEC2DescribeTransitGatewaysClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "VPC endpoints",
			contains: "fetching VPC endpoints",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchVPCEndpointsPage(ctx, &mockEC2DescribeVpcEndpointsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Network interfaces",
			contains: "fetching network interfaces",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchNetworkInterfacesPage(ctx, &mockEC2DescribeNetworkInterfacesClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "SNS subscriptions",
			contains: "fetching SNS subscriptions",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSNSSubscriptionsPage(ctx, &mockSNSListSubscriptionsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "DocDB cluster snapshots",
			contains: "fetching DocumentDB cluster snapshots",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchDocDBClusterSnapshotsPage(ctx, &mockDocDBDescribeSnapshotsClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "ECS tasks - list clusters error",
			contains: "listing ECS clusters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchECSTasksPage(ctx,
						&mockECSListClustersClient{err: sentinel},
						&mockECSListTasksClient{},
						&mockECSDescribeTasksClient{}, token)
				})
				return err
			},
		},
		{
			name:     "IAM policies",
			contains: "fetching IAM policies",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchIAMPoliciesPage(ctx, &mockIAMListPoliciesClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CloudFront distributions",
			contains: "fetching CloudFront distributions",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCloudFrontDistributionsPage(ctx, &mockCloudFrontClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Route53 hosted zones",
			contains: "fetching Route53 hosted zones",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchHostedZonesPage(ctx, &mockRoute53Client{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "API Gateways",
			contains: "fetching API gateways",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchAPIGatewaysPage(ctx, &mockAPIGatewayV2Client{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "ECR repositories",
			contains: "fetching ECR repositories",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchECRRepositoriesPage(ctx, &mockECRClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "EFS file systems",
			contains: "fetching EFS file systems",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchEFSFileSystemsPage(ctx, &mockEFSClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "EventBridge rules",
			contains: "fetching EventBridge rules",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchEventBridgeRulesPage(ctx, &mockEventBridgeClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Step Functions",
			contains: "fetching Step Functions",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchStepFunctionsPage(ctx, &mockSFNClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CodePipeline pipelines",
			contains: "fetching CodePipeline pipelines",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCodePipelinesPage(ctx, &mockCodePipelineClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Kinesis streams",
			contains: "fetching Kinesis streams",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchKinesisStreamsPage(ctx, &mockKinesisClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "WAF web ACLs",
			contains: "fetching WAF web ACLs",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchWAFWebACLsPage(ctx, &mockWAFv2Client{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Glue jobs",
			contains: "fetching Glue jobs",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchGlueJobsPage(ctx, &mockGlueClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Elastic Beanstalk environments",
			contains: "fetching Elastic Beanstalk environments",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchEBEnvironmentsPage(ctx, &mockEBClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "SES identities",
			contains: "fetching SES identities",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchSESIdentitiesPage(ctx, &mockSESv2Client{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Redshift clusters",
			contains: "fetching Redshift clusters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchRedshiftClustersPage(ctx, &mockRedshiftClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CloudTrail trails",
			contains: "fetching CloudTrail trails",
			call: func() error {
				_, err := awsclient.FetchCloudTrailTrails(ctx, &mockCloudTrailClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Athena workgroups",
			contains: "fetching Athena workgroups",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchAthenaWorkgroupsPage(ctx, &mockAthenaClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CodeArtifact repositories",
			contains: "fetching CodeArtifact repositories",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCodeArtifactReposPage(ctx, &mockCodeArtifactClient{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "CodeBuild projects - list error",
			contains: "listing CodeBuild projects",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchCodeBuildProjectsPage(ctx,
						&mockCodeBuildListProjectsClient{err: sentinel},
						&mockCodeBuildBatchGetProjectsClient{}, token)
				})
				return err
			},
		},
		{
			name:     "OpenSearch domains - list error",
			contains: "listing OpenSearch domains",
			call: func() error {
				_, err := awsclient.FetchOpenSearchDomains(ctx,
					&mockOpenSearchListDomainNamesClient{err: sentinel},
					&mockOpenSearchDescribeDomainsClient{})
				return err
			},
		},
		{
			name:     "KMS keys - list error",
			contains: "listing KMS keys",
			call: func() error {
				_, err := awsclient.FetchKMSKeys(ctx,
					&mockKMSListKeysClient{err: sentinel},
					&mockKMSDescribeKeyClient{},
					&mockKMSListAliasesClient{})
				return err
			},
		},
		{
			name:     "MSK clusters",
			contains: "fetching MSK clusters",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchMSKClustersPage(ctx, &mockMSKListClustersV2Client{err: sentinel}, token)
				})
				return err
			},
		},
		{
			name:     "Backup plans",
			contains: "fetching Backup plans",
			call: func() error {
				_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
					return awsclient.FetchBackupPlansPage(ctx, &mockBackupListBackupPlansClient{err: sentinel}, token)
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			// Must wrap the original error
			if !errors.Is(err, sentinel) {
				t.Errorf("error does not wrap sentinel: %v", err)
			}

			// Must contain the descriptive context
			if !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.contains)
			}
		})
	}
}
