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
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
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
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/smithy-go"
)

// ---------------------------------------------------------------------------
// S3 mocks
// ---------------------------------------------------------------------------

// mockS3ListBucketsClient implements awsclient.S3ListBucketsAPI for testing.
type mockS3ListBucketsClient struct {
	output *s3.ListBucketsOutput
	err    error
}

func (m *mockS3ListBucketsClient) ListBuckets(
	ctx context.Context,
	params *s3.ListBucketsInput,
	optFns ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	return m.output, m.err
}

// mockS3ListObjectsV2Client implements awsclient.S3ListObjectsV2API for testing.
type mockS3ListObjectsV2Client struct {
	output *s3.ListObjectsV2Output
	err    error
}

func (m *mockS3ListObjectsV2Client) ListObjectsV2(
	ctx context.Context,
	params *s3.ListObjectsV2Input,
	optFns ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	return m.output, m.err
}

// mockPaginatedS3ListBucketsClient returns multiple pages of S3 buckets.
type mockPaginatedS3ListBucketsClient struct {
	pages []*s3.ListBucketsOutput
	calls int
}

func (m *mockPaginatedS3ListBucketsClient) ListBuckets(
	ctx context.Context,
	params *s3.ListBucketsInput,
	optFns ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	idx := m.calls
	if idx >= len(m.pages) {
		return &s3.ListBucketsOutput{}, nil
	}
	m.calls++
	return m.pages[idx], nil
}

// mockFastListBucketsClient generates a configurable number of buckets in a single call.
type mockFastListBucketsClient struct {
	count int
}

func (m *mockFastListBucketsClient) ListBuckets(
	ctx context.Context,
	params *s3.ListBucketsInput,
	optFns ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	buckets := make([]s3types.Bucket, m.count)
	for i := range buckets {
		name := fmt.Sprintf("bucket-%03d", i)
		created := time.Now()
		buckets[i] = s3types.Bucket{
			Name:         aws.String(name),
			CreationDate: &created,
		}
	}
	return &s3.ListBucketsOutput{Buckets: buckets}, nil
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

// ---------------------------------------------------------------------------
// EC2 mocks
// ---------------------------------------------------------------------------

// mockEC2Client implements awsclient.EC2DescribeInstancesAPI for testing.
type mockEC2Client struct {
	output *ec2.DescribeInstancesOutput
	err    error
}

func (m *mockEC2Client) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// RDS mocks
// ---------------------------------------------------------------------------

// mockRDSClient implements awsclient.RDSDescribeDBInstancesAPI for testing.
type mockRDSClient struct {
	output *rds.DescribeDBInstancesOutput
	err    error
}

func (m *mockRDSClient) DescribeDBInstances(
	ctx context.Context,
	params *rds.DescribeDBInstancesInput,
	optFns ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ElastiCache (Redis) mocks
// ---------------------------------------------------------------------------

// mockElastiCacheClient implements awsclient.ElastiCacheDescribeCacheClustersAPI for testing.
type mockElastiCacheClient struct {
	output *elasticache.DescribeCacheClustersOutput
	err    error
}

func (m *mockElastiCacheClient) DescribeCacheClusters(
	ctx context.Context,
	params *elasticache.DescribeCacheClustersInput,
	optFns ...func(*elasticache.Options),
) (*elasticache.DescribeCacheClustersOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// DocumentDB mocks
// ---------------------------------------------------------------------------

// mockDocDBClient implements awsclient.DocDBDescribeDBClustersAPI for testing.
type mockDocDBClient struct {
	output *docdb.DescribeDBClustersOutput
	err    error
}

func (m *mockDocDBClient) DescribeDBClusters(
	ctx context.Context,
	params *docdb.DescribeDBClustersInput,
	optFns ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	return m.output, m.err
}

// mockDocDBFilterCapture captures the input to verify filters are passed.
type mockDocDBFilterCapture struct {
	output        *docdb.DescribeDBClustersOutput
	capturedInput *docdb.DescribeDBClustersInput
}

func (m *mockDocDBFilterCapture) DescribeDBClusters(
	ctx context.Context,
	params *docdb.DescribeDBClustersInput,
	optFns ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	m.capturedInput = params
	return m.output, nil
}

// ---------------------------------------------------------------------------
// EKS mocks
// ---------------------------------------------------------------------------

// mockEKSListClustersClient implements awsclient.EKSListClustersAPI for testing.
type mockEKSListClustersClient struct {
	output *eks.ListClustersOutput
	err    error
}

func (m *mockEKSListClustersClient) ListClusters(
	ctx context.Context,
	params *eks.ListClustersInput,
	optFns ...func(*eks.Options),
) (*eks.ListClustersOutput, error) {
	return m.output, m.err
}

// mockEKSDescribeClusterClient implements awsclient.EKSDescribeClusterAPI for testing.
type mockEKSDescribeClusterClient struct {
	outputs map[string]*eks.DescribeClusterOutput
	err     error
}

func (m *mockEKSDescribeClusterClient) DescribeCluster(
	ctx context.Context,
	params *eks.DescribeClusterInput,
	optFns ...func(*eks.Options),
) (*eks.DescribeClusterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.Name]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("cluster %q not found", *params.Name)
}

// ---------------------------------------------------------------------------
// Secrets Manager mocks
// ---------------------------------------------------------------------------

// mockSecretsManagerClient implements awsclient.SecretsManagerListSecretsAPI for testing.
type mockSecretsManagerClient struct {
	output *secretsmanager.ListSecretsOutput
	err    error
}

func (m *mockSecretsManagerClient) ListSecrets(
	ctx context.Context,
	params *secretsmanager.ListSecretsInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.ListSecretsOutput, error) {
	return m.output, m.err
}

// mockSecretsManagerGetSecretValueClient implements awsclient.SecretsManagerGetSecretValueAPI.
type mockSecretsManagerGetSecretValueClient struct {
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (m *mockSecretsManagerGetSecretValueClient) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// VPC mocks
// ---------------------------------------------------------------------------

// mockEC2DescribeVpcsClient implements awsclient.EC2DescribeVpcsAPI for testing.
type mockEC2DescribeVpcsClient struct {
	output *ec2.DescribeVpcsOutput
	err    error
}

func (m *mockEC2DescribeVpcsClient) DescribeVpcs(
	ctx context.Context,
	params *ec2.DescribeVpcsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Security Groups mocks
// ---------------------------------------------------------------------------

// mockEC2DescribeSecurityGroupsClient implements awsclient.EC2DescribeSecurityGroupsAPI for testing.
type mockEC2DescribeSecurityGroupsClient struct {
	output *ec2.DescribeSecurityGroupsOutput
	err    error
}

func (m *mockEC2DescribeSecurityGroupsClient) DescribeSecurityGroups(
	ctx context.Context,
	params *ec2.DescribeSecurityGroupsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSecurityGroupsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// EKS Node Groups mocks
// ---------------------------------------------------------------------------

// mockEKSListNodegroupsClient implements awsclient.EKSListNodegroupsAPI for testing.
type mockEKSListNodegroupsClient struct {
	outputs map[string]*eks.ListNodegroupsOutput // keyed by cluster name
	err     error
}

func (m *mockEKSListNodegroupsClient) ListNodegroups(
	ctx context.Context,
	params *eks.ListNodegroupsInput,
	optFns ...func(*eks.Options),
) (*eks.ListNodegroupsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.ClusterName]; ok {
		return out, nil
	}
	return &eks.ListNodegroupsOutput{}, nil
}

// mockEKSDescribeNodegroupClient implements awsclient.EKSDescribeNodegroupAPI for testing.
type mockEKSDescribeNodegroupClient struct {
	outputs map[string]*eks.DescribeNodegroupOutput // keyed by "cluster/nodegroup"
	err     error
}

func (m *mockEKSDescribeNodegroupClient) DescribeNodegroup(
	ctx context.Context,
	params *eks.DescribeNodegroupInput,
	optFns ...func(*eks.Options),
) (*eks.DescribeNodegroupOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := *params.ClusterName + "/" + *params.NodegroupName
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("nodegroup %q not found", key)
}

// ---------------------------------------------------------------------------
// AWS error mocks
// ---------------------------------------------------------------------------

type mockAPIError struct {
	code    string
	message string
	fault   smithy.ErrorFault
}

func (e *mockAPIError) Error() string                 { return e.message }
func (e *mockAPIError) ErrorCode() string              { return e.code }
func (e *mockAPIError) ErrorMessage() string           { return e.message }
func (e *mockAPIError) ErrorFault() smithy.ErrorFault  { return e.fault }

// ---------------------------------------------------------------------------
// S3 object pagination mock
// ---------------------------------------------------------------------------

// mockPaginatedS3ListObjectsV2Client returns multiple pages of S3 objects.
type mockPaginatedS3ListObjectsV2Client struct {
	pages []*s3.ListObjectsV2Output
	calls int
}

func (m *mockPaginatedS3ListObjectsV2Client) ListObjectsV2(
	ctx context.Context,
	params *s3.ListObjectsV2Input,
	optFns ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	idx := m.calls
	if idx >= len(m.pages) {
		return &s3.ListObjectsV2Output{}, nil
	}
	m.calls++
	return m.pages[idx], nil
}

// ---------------------------------------------------------------------------
// Subnet mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeSubnetsClient struct {
	output *ec2.DescribeSubnetsOutput
	err    error
}

func (m *mockEC2DescribeSubnetsClient) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Route Tables mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeRouteTablesClient struct {
	output *ec2.DescribeRouteTablesOutput
	err    error
}

func (m *mockEC2DescribeRouteTablesClient) DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// NAT Gateways mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeNatGatewaysClient struct {
	output *ec2.DescribeNatGatewaysOutput
	err    error
}

func (m *mockEC2DescribeNatGatewaysClient) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Internet Gateways mocks
// ---------------------------------------------------------------------------

type mockEC2DescribeInternetGatewaysClient struct {
	output *ec2.DescribeInternetGatewaysOutput
	err    error
}

func (m *mockEC2DescribeInternetGatewaysClient) DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Lambda mocks
// ---------------------------------------------------------------------------

type mockLambdaListFunctionsClient struct {
	output *lambda.ListFunctionsOutput
	err    error
}

func (m *mockLambdaListFunctionsClient) ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CloudWatch Alarms mocks
// ---------------------------------------------------------------------------

type mockCloudWatchDescribeAlarmsClient struct {
	output *cloudwatch.DescribeAlarmsOutput
	err    error
}

func (m *mockCloudWatchDescribeAlarmsClient) DescribeAlarms(ctx context.Context, params *cloudwatch.DescribeAlarmsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// SNS mocks
// ---------------------------------------------------------------------------

type mockSNSListTopicsClient struct {
	output *sns.ListTopicsOutput
	err    error
}

func (m *mockSNSListTopicsClient) ListTopics(ctx context.Context, params *sns.ListTopicsInput, optFns ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// SQS mocks
// ---------------------------------------------------------------------------

type mockSQSListQueuesClient struct {
	output *sqs.ListQueuesOutput
	err    error
}

func (m *mockSQSListQueuesClient) ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	return m.output, m.err
}

type mockSQSGetQueueAttributesClient struct {
	outputs map[string]*sqs.GetQueueAttributesOutput
	err     error
}

func (m *mockSQSGetQueueAttributesClient) GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.QueueUrl]; ok {
		return out, nil
	}
	return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{}}, nil
}

// ---------------------------------------------------------------------------
// ELBv2 mocks
// ---------------------------------------------------------------------------

type mockELBv2DescribeLoadBalancersClient struct {
	output *elbv2.DescribeLoadBalancersOutput
	err    error
}

func (m *mockELBv2DescribeLoadBalancersClient) DescribeLoadBalancers(ctx context.Context, params *elbv2.DescribeLoadBalancersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return m.output, m.err
}

type mockELBv2DescribeTargetGroupsClient struct {
	output *elbv2.DescribeTargetGroupsOutput
	err    error
}

func (m *mockELBv2DescribeTargetGroupsClient) DescribeTargetGroups(ctx context.Context, params *elbv2.DescribeTargetGroupsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// ECS mocks
// ---------------------------------------------------------------------------

type mockECSListClustersClient struct {
	output *ecs.ListClustersOutput
	err    error
}

func (m *mockECSListClustersClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return m.output, m.err
}

type mockECSDescribeClustersClient struct {
	output *ecs.DescribeClustersOutput
	err    error
}

func (m *mockECSDescribeClustersClient) DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return m.output, m.err
}

type mockECSListServicesClient struct {
	outputs map[string]*ecs.ListServicesOutput
	err     error
}

func (m *mockECSListServicesClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.Cluster]; ok {
		return out, nil
	}
	return &ecs.ListServicesOutput{}, nil
}

type mockECSDescribeServicesClient struct {
	output *ecs.DescribeServicesOutput
	err    error
}

func (m *mockECSDescribeServicesClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return m.output, m.err
}

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

// Ensure unused imports are used
var _ = time.Now
var _ = aws.String
