package unit

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// ---------------------------------------------------------------------------
// CloudFormation mocks
// ---------------------------------------------------------------------------

type mockCFNDescribeStacksClient struct {
	output *cloudformation.DescribeStacksOutput
	err    error
}

func (m *mockCFNDescribeStacksClient) DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return m.output, m.err
}

// mockCFNDescribeStackEventsClient implements awsclient.CFNDescribeStackEventsAPI.
// It supports pagination via the outputs slice with callIdx counter.
// For backward compatibility, if outputs is nil it falls back to the single output field.
type mockCFNDescribeStackEventsClient struct {
	output  *cloudformation.DescribeStackEventsOutput
	outputs []*cloudformation.DescribeStackEventsOutput
	err     error
	callIdx int
}

func (m *mockCFNDescribeStackEventsClient) DescribeStackEvents(ctx context.Context, params *cloudformation.DescribeStackEventsInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.outputs != nil {
		if m.callIdx >= len(m.outputs) {
			return &cloudformation.DescribeStackEventsOutput{}, nil
		}
		out := m.outputs[m.callIdx]
		m.callIdx++
		return out, nil
	}
	return m.output, nil
}

// mockCFNListStackResourcesClient implements awsclient.CFNListStackResourcesAPI.
// It supports pagination via the outputs slice with callIdx counter.
// For backward compatibility, if outputs is nil it falls back to the single output field.
type mockCFNListStackResourcesClient struct {
	output  *cloudformation.ListStackResourcesOutput
	outputs []*cloudformation.ListStackResourcesOutput
	err     error
	callIdx int
}

func (m *mockCFNListStackResourcesClient) ListStackResources(ctx context.Context, params *cloudformation.ListStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.outputs != nil {
		if m.callIdx >= len(m.outputs) {
			return &cloudformation.ListStackResourcesOutput{}, nil
		}
		out := m.outputs[m.callIdx]
		m.callIdx++
		return out, nil
	}
	return m.output, nil
}

// ---------------------------------------------------------------------------
// IAM mocks
// ---------------------------------------------------------------------------

type mockIAMListRolesClient struct {
	output *iam.ListRolesOutput
	err    error
}

func (m *mockIAMListRolesClient) ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CloudWatch Logs mocks
// ---------------------------------------------------------------------------

type mockCWLogsDescribeLogGroupsClient struct {
	output *cloudwatchlogs.DescribeLogGroupsOutput
	err    error
}

func (m *mockCWLogsDescribeLogGroupsClient) DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// SSM mocks
// ---------------------------------------------------------------------------

type mockSSMDescribeParametersClient struct {
	output *ssm.DescribeParametersOutput
	err    error
}

func (m *mockSSMDescribeParametersClient) DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// DynamoDB mocks
// ---------------------------------------------------------------------------

type mockDDBListTablesClient struct {
	output *dynamodb.ListTablesOutput
	err    error
}

func (m *mockDDBListTablesClient) ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return m.output, m.err
}

type mockDDBDescribeTableClient struct {
	outputs map[string]*dynamodb.DescribeTableOutput
	err     error
}

func (m *mockDDBDescribeTableClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.TableName]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("table %q not found", *params.TableName)
}

// ---------------------------------------------------------------------------
// Elastic IP mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeAddressesClient struct {
	output *ec2.DescribeAddressesOutput
	err    error
}

func (m *mockEC2DescribeAddressesClient) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ACM mocks
// ---------------------------------------------------------------------------

type mockACMListCertificatesClient struct {
	output *acm.ListCertificatesOutput
	err    error
}

func (m *mockACMListCertificatesClient) ListCertificates(ctx context.Context, params *acm.ListCertificatesInput, optFns ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Auto Scaling mocks
// ---------------------------------------------------------------------------

type mockASGDescribeAutoScalingGroupsClient struct {
	output *autoscaling.DescribeAutoScalingGroupsOutput
	err    error
}

func (m *mockASGDescribeAutoScalingGroupsClient) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ECS Tasks mocks
// ---------------------------------------------------------------------------

type mockECSListTasksClient struct {
	outputs map[string]*ecs.ListTasksOutput // keyed by cluster ARN
	err     error
}

func (m *mockECSListTasksClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.Cluster]; ok {
		return out, nil
	}
	return &ecs.ListTasksOutput{}, nil
}

type mockECSDescribeTasksClient struct {
	output *ecs.DescribeTasksOutput
	err    error
}

func (m *mockECSDescribeTasksClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// IAM Policies mocks
// ---------------------------------------------------------------------------

type mockIAMListPoliciesClient struct {
	output *iam.ListPoliciesOutput
	err    error
}

func (m *mockIAMListPoliciesClient) ListPolicies(ctx context.Context, params *iam.ListPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// RDS Snapshots mocks
// ---------------------------------------------------------------------------

type mockRDSDescribeDBSnapshotsClient struct {
	output *rds.DescribeDBSnapshotsOutput
	err    error
}

func (m *mockRDSDescribeDBSnapshotsClient) DescribeDBSnapshots(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Transit Gateways mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeTransitGatewaysClient struct {
	output *ec2.DescribeTransitGatewaysOutput
	err    error
}

func (m *mockEC2DescribeTransitGatewaysClient) DescribeTransitGateways(ctx context.Context, params *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// VPC Endpoints mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeVpcEndpointsClient struct {
	output *ec2.DescribeVpcEndpointsOutput
	err    error
}

func (m *mockEC2DescribeVpcEndpointsClient) DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Network Interfaces (ENI) mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeNetworkInterfacesClient struct {
	output *ec2.DescribeNetworkInterfacesOutput
	err    error
}

func (m *mockEC2DescribeNetworkInterfacesClient) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// SNS Subscriptions mocks
// ---------------------------------------------------------------------------

type mockSNSListSubscriptionsClient struct {
	output *sns.ListSubscriptionsOutput
	err    error
}

func (m *mockSNSListSubscriptionsClient) ListSubscriptions(ctx context.Context, params *sns.ListSubscriptionsInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// IAM Users mocks
// ---------------------------------------------------------------------------

type mockIAMListUsersClient struct {
	output *iam.ListUsersOutput
	err    error
}

func (m *mockIAMListUsersClient) ListUsers(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// IAM Groups mocks
// ---------------------------------------------------------------------------

type mockIAMListGroupsClient struct {
	output *iam.ListGroupsOutput
	err    error
}

func (m *mockIAMListGroupsClient) ListGroups(ctx context.Context, params *iam.ListGroupsInput, optFns ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// DocDB Snapshots mocks
// ---------------------------------------------------------------------------

type mockDocDBDescribeSnapshotsClient struct {
	output *docdb.DescribeDBClusterSnapshotsOutput
	err    error
}

func (m *mockDocDBDescribeSnapshotsClient) DescribeDBClusterSnapshots(ctx context.Context, params *docdb.DescribeDBClusterSnapshotsInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CloudFront mocks
// ---------------------------------------------------------------------------

type mockCloudFrontClient struct {
	output *cloudfront.ListDistributionsOutput
	err    error
}

func (m *mockCloudFrontClient) ListDistributions(ctx context.Context, params *cloudfront.ListDistributionsInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Route 53 mocks
// ---------------------------------------------------------------------------

type mockRoute53Client struct {
	output *route53.ListHostedZonesOutput
	err    error
}

func (m *mockRoute53Client) ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return m.output, m.err
}

// mockRoute53RecordSetsClient implements awsclient.Route53ListResourceRecordSetsAPI for testing.
// It supports pagination by returning successive outputs from the outputs slice.
type mockRoute53RecordSetsClient struct {
	outputs []*route53.ListResourceRecordSetsOutput
	err     error
	callIdx int
}

func (m *mockRoute53RecordSetsClient) ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &route53.ListResourceRecordSetsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// ---------------------------------------------------------------------------
// API Gateway V2 mocks
// ---------------------------------------------------------------------------

type mockAPIGatewayV2Client struct {
	output *apigatewayv2.GetApisOutput
	err    error
}

func (m *mockAPIGatewayV2Client) GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ECR mocks
// ---------------------------------------------------------------------------

type mockECRClient struct {
	output *ecr.DescribeRepositoriesOutput
	err    error
}

func (m *mockECRClient) DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// EFS mocks
// ---------------------------------------------------------------------------

type mockEFSClient struct {
	output *efs.DescribeFileSystemsOutput
	err    error
}

func (m *mockEFSClient) DescribeFileSystems(ctx context.Context, params *efs.DescribeFileSystemsInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// EventBridge mocks
// ---------------------------------------------------------------------------

type mockEventBridgeClient struct {
	output *eventbridge.ListRulesOutput
	err    error
}

func (m *mockEventBridgeClient) ListRules(ctx context.Context, params *eventbridge.ListRulesInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Step Functions (SFN) mocks
// ---------------------------------------------------------------------------

type mockSFNClient struct {
	output *sfn.ListStateMachinesOutput
	err    error
}

func (m *mockSFNClient) ListStateMachines(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CodePipeline mocks
// ---------------------------------------------------------------------------

type mockCodePipelineClient struct {
	output *codepipeline.ListPipelinesOutput
	err    error
}

func (m *mockCodePipelineClient) ListPipelines(ctx context.Context, params *codepipeline.ListPipelinesInput, optFns ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Kinesis mocks
// ---------------------------------------------------------------------------

type mockKinesisClient struct {
	output *kinesis.ListStreamsOutput
	err    error
}

func (m *mockKinesisClient) ListStreams(ctx context.Context, params *kinesis.ListStreamsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// WAFv2 mocks
// ---------------------------------------------------------------------------

type mockWAFv2Client struct {
	output *wafv2.ListWebACLsOutput
	err    error
}

func (m *mockWAFv2Client) ListWebACLs(ctx context.Context, params *wafv2.ListWebACLsInput, optFns ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	return m.output, m.err
}

type mockWAFv2CaptureClient struct {
	output        *wafv2.ListWebACLsOutput
	capturedInput *wafv2.ListWebACLsInput
}

func (m *mockWAFv2CaptureClient) ListWebACLs(ctx context.Context, params *wafv2.ListWebACLsInput, optFns ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	m.capturedInput = params
	return m.output, nil
}

// ---------------------------------------------------------------------------
// Glue mocks
// ---------------------------------------------------------------------------

type mockGlueClient struct {
	output *glue.GetJobsOutput
	err    error
}

func (m *mockGlueClient) GetJobs(ctx context.Context, params *glue.GetJobsInput, optFns ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Elastic Beanstalk mocks
// ---------------------------------------------------------------------------

type mockEBClient struct {
	output *elasticbeanstalk.DescribeEnvironmentsOutput
	err    error
}

func (m *mockEBClient) DescribeEnvironments(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// SES v2 mocks
// ---------------------------------------------------------------------------

type mockSESv2Client struct {
	output *sesv2.ListEmailIdentitiesOutput
	err    error
}

func (m *mockSESv2Client) ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, optFns ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Redshift mocks
// ---------------------------------------------------------------------------

type mockRedshiftClient struct {
	output *redshift.DescribeClustersOutput
	err    error
}

func (m *mockRedshiftClient) DescribeClusters(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CloudTrail mocks
// ---------------------------------------------------------------------------

type mockCloudTrailClient struct {
	output *cloudtrail.DescribeTrailsOutput
	err    error
}

func (m *mockCloudTrailClient) DescribeTrails(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Athena mocks
// ---------------------------------------------------------------------------

type mockAthenaClient struct {
	output *athena.ListWorkGroupsOutput
	err    error
}

func (m *mockAthenaClient) ListWorkGroups(ctx context.Context, params *athena.ListWorkGroupsInput, optFns ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CodeArtifact mocks
// ---------------------------------------------------------------------------

type mockCodeArtifactClient struct {
	output *codeartifact.ListRepositoriesOutput
	err    error
}

func (m *mockCodeArtifactClient) ListRepositories(ctx context.Context, params *codeartifact.ListRepositoriesInput, optFns ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CodeBuild mocks
// ---------------------------------------------------------------------------

type mockCodeBuildListProjectsClient struct {
	output *codebuild.ListProjectsOutput
	err    error
}

func (m *mockCodeBuildListProjectsClient) ListProjects(ctx context.Context, params *codebuild.ListProjectsInput, optFns ...func(*codebuild.Options)) (*codebuild.ListProjectsOutput, error) {
	return m.output, m.err
}

type mockCodeBuildBatchGetProjectsClient struct {
	output *codebuild.BatchGetProjectsOutput
	err    error
}

func (m *mockCodeBuildBatchGetProjectsClient) BatchGetProjects(ctx context.Context, params *codebuild.BatchGetProjectsInput, optFns ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// OpenSearch mocks
// ---------------------------------------------------------------------------

type mockOpenSearchListDomainNamesClient struct {
	output *opensearch.ListDomainNamesOutput
	err    error
}

func (m *mockOpenSearchListDomainNamesClient) ListDomainNames(ctx context.Context, params *opensearch.ListDomainNamesInput, optFns ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error) {
	return m.output, m.err
}

type mockOpenSearchDescribeDomainsClient struct {
	output *opensearch.DescribeDomainsOutput
	err    error
}

func (m *mockOpenSearchDescribeDomainsClient) DescribeDomains(ctx context.Context, params *opensearch.DescribeDomainsInput, optFns ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// KMS mocks
// ---------------------------------------------------------------------------

type mockKMSListKeysClient struct {
	output *kms.ListKeysOutput
	err    error
}

func (m *mockKMSListKeysClient) ListKeys(ctx context.Context, params *kms.ListKeysInput, optFns ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return m.output, m.err
}

type mockKMSDescribeKeyClient struct {
	outputs map[string]*kms.DescribeKeyOutput
	err     error
}

func (m *mockKMSDescribeKeyClient) DescribeKey(ctx context.Context, params *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.KeyId]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("key %q not found", *params.KeyId)
}

type mockKMSListAliasesClient struct {
	output *kms.ListAliasesOutput
	err    error
}

func (m *mockKMSListAliasesClient) ListAliases(ctx context.Context, params *kms.ListAliasesInput, optFns ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// MSK (Kafka) mocks
// ---------------------------------------------------------------------------

type mockMSKListClustersV2Client struct {
	output *kafka.ListClustersV2Output
	err    error
}

func (m *mockMSKListClustersV2Client) ListClustersV2(ctx context.Context, params *kafka.ListClustersV2Input, optFns ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Backup mocks
// ---------------------------------------------------------------------------

type mockBackupListBackupPlansClient struct {
	output *backup.ListBackupPlansOutput
	err    error
}

func (m *mockBackupListBackupPlansClient) ListBackupPlans(ctx context.Context, params *backup.ListBackupPlansInput, optFns ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CloudWatch Logs Streams mocks (child of Log Groups)
// ---------------------------------------------------------------------------

// mockCWLogsDescribeLogStreamsClient implements awsclient.CWLogsDescribeLogStreamsAPI.
type mockCWLogsDescribeLogStreamsClient struct {
	outputs []*cloudwatchlogs.DescribeLogStreamsOutput
	err     error
	callIdx int
}

func (m *mockCWLogsDescribeLogStreamsClient) DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &cloudwatchlogs.DescribeLogStreamsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// ---------------------------------------------------------------------------
// CloudWatch Logs Events mocks (child of Log Streams)
// ---------------------------------------------------------------------------

// mockCWLogsGetLogEventsClient implements awsclient.CWLogsGetLogEventsAPI.
type mockCWLogsGetLogEventsClient struct {
	output    *cloudwatchlogs.GetLogEventsOutput
	err       error
	lastInput *cloudwatchlogs.GetLogEventsInput
}

func (m *mockCWLogsGetLogEventsClient) GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	m.lastInput = params
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Lambda Invocations mocks (child of Lambda, cross-service via CW Logs)
// ---------------------------------------------------------------------------

// mockCWLogsFilterLogEventsClient implements awsclient.CWLogsFilterLogEventsAPI.
// Supports multiple outputs for pagination via NextToken.
type mockCWLogsFilterLogEventsClient struct {
	outputs   []*cloudwatchlogs.FilterLogEventsOutput
	err       error
	callIdx   int
	lastInput *cloudwatchlogs.FilterLogEventsInput
}

func (m *mockCWLogsFilterLogEventsClient) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	m.lastInput = params
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &cloudwatchlogs.FilterLogEventsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// ---------------------------------------------------------------------------
// Lambda GetFunction mocks (for Lambda Code viewer)
// ---------------------------------------------------------------------------

// mockLambdaGetFunctionClient implements awsclient.LambdaGetFunctionAPI.
type mockLambdaGetFunctionClient struct {
	output *lambda.GetFunctionOutput
	err    error
}

func (m *mockLambdaGetFunctionClient) GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ECS DescribeTaskDefinition mocks (for ECS Service Logs child view)
// ---------------------------------------------------------------------------

// mockECSDescribeTaskDefinitionClient implements awsclient.ECSDescribeTaskDefinitionAPI.
type mockECSDescribeTaskDefinitionClient struct {
	output *ecs.DescribeTaskDefinitionOutput
	err    error
}

func (m *mockECSDescribeTaskDefinitionClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ASG Scaling Activities mocks (child of Auto Scaling Groups)
// ---------------------------------------------------------------------------

// mockASGDescribeScalingActivitiesClient implements awsclient.ASGDescribeScalingActivitiesAPI.
// It supports pagination via the outputs slice with callIdx counter.
// For backward compatibility, if outputs is nil it falls back to the single output field.
type mockASGDescribeScalingActivitiesClient struct {
	output  *autoscaling.DescribeScalingActivitiesOutput
	outputs []*autoscaling.DescribeScalingActivitiesOutput
	err     error
	callIdx int
}

func (m *mockASGDescribeScalingActivitiesClient) DescribeScalingActivities(ctx context.Context, params *autoscaling.DescribeScalingActivitiesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.outputs != nil {
		if m.callIdx >= len(m.outputs) {
			return &autoscaling.DescribeScalingActivitiesOutput{}, nil
		}
		out := m.outputs[m.callIdx]
		m.callIdx++
		return out, nil
	}
	return m.output, nil
}

// ---------------------------------------------------------------------------
// CloudWatch Alarm History mocks (child of CloudWatch Alarms)
// ---------------------------------------------------------------------------

// mockCloudWatchDescribeAlarmHistoryClient implements awsclient.CloudWatchDescribeAlarmHistoryAPI.
// It supports pagination via the outputs slice with callIdx counter.
// For backward compatibility, if outputs is nil it falls back to the single output field.
type mockCloudWatchDescribeAlarmHistoryClient struct {
	output  *cloudwatch.DescribeAlarmHistoryOutput
	outputs []*cloudwatch.DescribeAlarmHistoryOutput
	err     error
	callIdx int
}

func (m *mockCloudWatchDescribeAlarmHistoryClient) DescribeAlarmHistory(ctx context.Context, params *cloudwatch.DescribeAlarmHistoryInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.outputs != nil {
		if m.callIdx >= len(m.outputs) {
			return &cloudwatch.DescribeAlarmHistoryOutput{}, nil
		}
		out := m.outputs[m.callIdx]
		m.callIdx++
		return out, nil
	}
	return m.output, nil
}

// ---------------------------------------------------------------------------
// ELBv2 Describe Listeners mocks (child of Load Balancers)
// ---------------------------------------------------------------------------

// mockELBv2DescribeListenersClient implements awsclient.ELBv2DescribeListenersAPI.
// It supports pagination via the outputs slice with callIdx counter.
// For backward compatibility, if outputs is nil it falls back to the single output field.
type mockELBv2DescribeListenersClient struct {
	output  *elbv2.DescribeListenersOutput
	outputs []*elbv2.DescribeListenersOutput
	err     error
	callIdx int
}

func (m *mockELBv2DescribeListenersClient) DescribeListeners(ctx context.Context, params *elbv2.DescribeListenersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.outputs != nil {
		if m.callIdx >= len(m.outputs) {
			return &elbv2.DescribeListenersOutput{}, nil
		}
		out := m.outputs[m.callIdx]
		m.callIdx++
		return out, nil
	}
	return m.output, nil
}

// Ensure unused imports are used
var _ = time.Now
var _ = aws.String
