package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
				_, err := awsclient.FetchEC2Instances(ctx, &mockEC2Client{err: sentinel})
				return err
			},
		},
		{
			name:     "S3 buckets",
			contains: "fetching S3 buckets",
			call: func() error {
				_, err := awsclient.FetchS3Buckets(ctx, &mockS3ListBucketsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "S3 objects",
			contains: "fetching S3 objects",
			call: func() error {
				_, err := awsclient.FetchS3Objects(ctx, &mockS3ListObjectsV2Client{err: sentinel}, "test-bucket", "")
				return err
			},
		},
		{
			name:     "RDS instances",
			contains: "fetching RDS instances",
			call: func() error {
				_, err := awsclient.FetchRDSInstances(ctx, &mockRDSClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Redis clusters",
			contains: "fetching Redis clusters",
			call: func() error {
				_, err := awsclient.FetchRedisClusters(ctx, &mockElastiCacheClient{err: sentinel})
				return err
			},
		},
		{
			name:     "DocDB clusters",
			contains: "fetching DocumentDB clusters",
			call: func() error {
				_, err := awsclient.FetchDocDBClusters(ctx, &mockDocDBClient{err: sentinel})
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
				_, err := awsclient.FetchSecrets(ctx, &mockSecretsManagerClient{err: sentinel})
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
				_, err := awsclient.FetchVPCs(ctx, &mockEC2DescribeVpcsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Security groups",
			contains: "fetching security groups",
			call: func() error {
				_, err := awsclient.FetchSecurityGroups(ctx, &mockEC2DescribeSecurityGroupsClient{err: sentinel})
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
				_, err := awsclient.FetchSubnets(ctx, &mockEC2DescribeSubnetsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Route tables",
			contains: "fetching route tables",
			call: func() error {
				_, err := awsclient.FetchRouteTables(ctx, &mockEC2DescribeRouteTablesClient{err: sentinel})
				return err
			},
		},
		{
			name:     "NAT gateways",
			contains: "fetching NAT gateways",
			call: func() error {
				_, err := awsclient.FetchNatGateways(ctx, &mockEC2DescribeNatGatewaysClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Internet gateways",
			contains: "fetching internet gateways",
			call: func() error {
				_, err := awsclient.FetchInternetGateways(ctx, &mockEC2DescribeInternetGatewaysClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Lambda functions",
			contains: "fetching Lambda functions",
			call: func() error {
				_, err := awsclient.FetchLambdaFunctions(ctx, &mockLambdaListFunctionsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "CloudWatch alarms",
			contains: "fetching CloudWatch alarms",
			call: func() error {
				_, err := awsclient.FetchCloudWatchAlarms(ctx, &mockCloudWatchDescribeAlarmsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "SNS topics",
			contains: "fetching SNS topics",
			call: func() error {
				_, err := awsclient.FetchSNSTopics(ctx, &mockSNSListTopicsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "SQS queues - list error",
			contains: "listing SQS queues",
			call: func() error {
				_, err := awsclient.FetchSQSQueues(ctx,
					&mockSQSListQueuesClient{err: sentinel},
					&mockSQSGetQueueAttributesClient{})
				return err
			},
		},
		{
			name:     "Load balancers",
			contains: "fetching load balancers",
			call: func() error {
				_, err := awsclient.FetchLoadBalancers(ctx, &mockELBv2DescribeLoadBalancersClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Target groups",
			contains: "fetching target groups",
			call: func() error {
				_, err := awsclient.FetchTargetGroups(ctx, &mockELBv2DescribeTargetGroupsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "ECS clusters - list error",
			contains: "listing ECS clusters",
			call: func() error {
				_, err := awsclient.FetchECSClusters(ctx,
					&mockECSListClustersClient{err: sentinel},
					&mockECSDescribeClustersClient{})
				return err
			},
		},
		{
			name:     "ECS services - list clusters error",
			contains: "listing ECS clusters",
			call: func() error {
				_, err := awsclient.FetchECSServices(ctx,
					&mockECSListClustersClient{err: sentinel},
					&mockECSListServicesClient{},
					&mockECSDescribeServicesClient{})
				return err
			},
		},
		{
			name:     "CloudFormation stacks",
			contains: "fetching CloudFormation stacks",
			call: func() error {
				_, err := awsclient.FetchCloudFormationStacks(ctx, &mockCFNDescribeStacksClient{err: sentinel})
				return err
			},
		},
		{
			name:     "IAM roles",
			contains: "fetching IAM roles",
			call: func() error {
				_, err := awsclient.FetchIAMRoles(ctx, &mockIAMListRolesClient{err: sentinel})
				return err
			},
		},
		{
			name:     "CloudWatch log groups",
			contains: "fetching CloudWatch log groups",
			call: func() error {
				_, err := awsclient.FetchCloudWatchLogGroups(ctx, &mockCWLogsDescribeLogGroupsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "SSM parameters",
			contains: "fetching SSM parameters",
			call: func() error {
				_, err := awsclient.FetchSSMParameters(ctx, &mockSSMDescribeParametersClient{err: sentinel})
				return err
			},
		},
		{
			name:     "DynamoDB tables - list error",
			contains: "listing DynamoDB tables",
			call: func() error {
				_, err := awsclient.FetchDynamoDBTables(ctx,
					&mockDDBListTablesClient{err: sentinel},
					&mockDDBDescribeTableClient{})
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
				_, err := awsclient.FetchACMCertificates(ctx, &mockACMListCertificatesClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Auto Scaling groups",
			contains: "fetching Auto Scaling groups",
			call: func() error {
				_, err := awsclient.FetchAutoScalingGroups(ctx, &mockASGDescribeAutoScalingGroupsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "IAM users",
			contains: "fetching IAM users",
			call: func() error {
				_, err := awsclient.FetchIAMUsers(ctx, &mockIAMListUsersClient{err: sentinel})
				return err
			},
		},
		{
			name:     "IAM groups",
			contains: "fetching IAM groups",
			call: func() error {
				_, err := awsclient.FetchIAMGroups(ctx, &mockIAMListGroupsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "RDS snapshots",
			contains: "fetching RDS snapshots",
			call: func() error {
				_, err := awsclient.FetchRDSSnapshots(ctx, &mockRDSDescribeDBSnapshotsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Transit gateways",
			contains: "fetching transit gateways",
			call: func() error {
				_, err := awsclient.FetchTransitGateways(ctx, &mockEC2DescribeTransitGatewaysClient{err: sentinel})
				return err
			},
		},
		{
			name:     "VPC endpoints",
			contains: "fetching VPC endpoints",
			call: func() error {
				_, err := awsclient.FetchVPCEndpoints(ctx, &mockEC2DescribeVpcEndpointsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Network interfaces",
			contains: "fetching network interfaces",
			call: func() error {
				_, err := awsclient.FetchNetworkInterfaces(ctx, &mockEC2DescribeNetworkInterfacesClient{err: sentinel})
				return err
			},
		},
		{
			name:     "SNS subscriptions",
			contains: "fetching SNS subscriptions",
			call: func() error {
				_, err := awsclient.FetchSNSSubscriptions(ctx, &mockSNSListSubscriptionsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "DocDB cluster snapshots",
			contains: "fetching DocumentDB cluster snapshots",
			call: func() error {
				_, err := awsclient.FetchDocDBClusterSnapshots(ctx, &mockDocDBDescribeSnapshotsClient{err: sentinel})
				return err
			},
		},
		{
			name:     "ECS tasks - list clusters error",
			contains: "listing ECS clusters",
			call: func() error {
				_, err := awsclient.FetchECSTasks(ctx,
					&mockECSListClustersClient{err: sentinel},
					&mockECSListTasksClient{},
					&mockECSDescribeTasksClient{})
				return err
			},
		},
		{
			name:     "IAM policies",
			contains: "fetching IAM policies",
			call: func() error {
				_, err := awsclient.FetchIAMPolicies(ctx, &mockIAMListPoliciesClient{err: sentinel})
				return err
			},
		},
		{
			name:     "CloudFront distributions",
			contains: "fetching CloudFront distributions",
			call: func() error {
				_, err := awsclient.FetchCloudFrontDistributions(ctx, &mockCloudFrontClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Route53 hosted zones",
			contains: "fetching Route53 hosted zones",
			call: func() error {
				_, err := awsclient.FetchHostedZones(ctx, &mockRoute53Client{err: sentinel})
				return err
			},
		},
		{
			name:     "API Gateways",
			contains: "fetching API gateways",
			call: func() error {
				_, err := awsclient.FetchAPIGateways(ctx, &mockAPIGatewayV2Client{err: sentinel})
				return err
			},
		},
		{
			name:     "ECR repositories",
			contains: "fetching ECR repositories",
			call: func() error {
				_, err := awsclient.FetchECRRepositories(ctx, &mockECRClient{err: sentinel})
				return err
			},
		},
		{
			name:     "EFS file systems",
			contains: "fetching EFS file systems",
			call: func() error {
				_, err := awsclient.FetchEFSFileSystems(ctx, &mockEFSClient{err: sentinel})
				return err
			},
		},
		{
			name:     "EventBridge rules",
			contains: "fetching EventBridge rules",
			call: func() error {
				_, err := awsclient.FetchEventBridgeRules(ctx, &mockEventBridgeClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Step Functions",
			contains: "fetching Step Functions",
			call: func() error {
				_, err := awsclient.FetchStepFunctions(ctx, &mockSFNClient{err: sentinel})
				return err
			},
		},
		{
			name:     "CodePipeline pipelines",
			contains: "fetching CodePipeline pipelines",
			call: func() error {
				_, err := awsclient.FetchCodePipelines(ctx, &mockCodePipelineClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Kinesis streams",
			contains: "fetching Kinesis streams",
			call: func() error {
				_, err := awsclient.FetchKinesisStreams(ctx, &mockKinesisClient{err: sentinel})
				return err
			},
		},
		{
			name:     "WAF web ACLs",
			contains: "fetching WAF web ACLs",
			call: func() error {
				_, err := awsclient.FetchWAFWebACLs(ctx, &mockWAFv2Client{err: sentinel})
				return err
			},
		},
		{
			name:     "Glue jobs",
			contains: "fetching Glue jobs",
			call: func() error {
				_, err := awsclient.FetchGlueJobs(ctx, &mockGlueClient{err: sentinel})
				return err
			},
		},
		{
			name:     "Elastic Beanstalk environments",
			contains: "fetching Elastic Beanstalk environments",
			call: func() error {
				_, err := awsclient.FetchEBEnvironments(ctx, &mockEBClient{err: sentinel})
				return err
			},
		},
		{
			name:     "SES identities",
			contains: "fetching SES identities",
			call: func() error {
				_, err := awsclient.FetchSESIdentities(ctx, &mockSESv2Client{err: sentinel})
				return err
			},
		},
		{
			name:     "Redshift clusters",
			contains: "fetching Redshift clusters",
			call: func() error {
				_, err := awsclient.FetchRedshiftClusters(ctx, &mockRedshiftClient{err: sentinel})
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
				_, err := awsclient.FetchAthenaWorkgroups(ctx, &mockAthenaClient{err: sentinel})
				return err
			},
		},
		{
			name:     "CodeArtifact repositories",
			contains: "fetching CodeArtifact repositories",
			call: func() error {
				_, err := awsclient.FetchCodeArtifactRepos(ctx, &mockCodeArtifactClient{err: sentinel})
				return err
			},
		},
		{
			name:     "CodeBuild projects - list error",
			contains: "listing CodeBuild projects",
			call: func() error {
				_, err := awsclient.FetchCodeBuildProjects(ctx,
					&mockCodeBuildListProjectsClient{err: sentinel},
					&mockCodeBuildBatchGetProjectsClient{})
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
				_, err := awsclient.FetchMSKClusters(ctx, &mockMSKListClustersV2Client{err: sentinel})
				return err
			},
		},
		{
			name:     "Backup plans",
			contains: "fetching Backup plans",
			call: func() error {
				_, err := awsclient.FetchBackupPlans(ctx, &mockBackupListBackupPlansClient{err: sentinel})
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
