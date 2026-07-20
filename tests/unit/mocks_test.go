package unit

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/smithy-go"
)

// ---------------------------------------------------------------------------
// EC2 mocks
// ---------------------------------------------------------------------------

// mockEC2Client implements awsclient.EC2DescribeInstancesAPI for testing.
type mockEC2Client struct {
	output       *ec2.DescribeInstancesOutput
	err          error
	statusOutput *ec2.DescribeInstanceStatusOutput
	statusErr    error
}

func (m *mockEC2Client) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return m.output, m.err
}

func (m *mockEC2Client) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	if m.statusOutput != nil {
		return m.statusOutput, nil
	}
	return &ec2.DescribeInstanceStatusOutput{}, nil
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

// mockElastiCacheReplicationGroupsClient implements
// awsclient.ElastiCacheDescribeReplicationGroupsAPI for testing.
type mockElastiCacheReplicationGroupsClient struct {
	output *elasticache.DescribeReplicationGroupsOutput
	err    error
}

func (m *mockElastiCacheReplicationGroupsClient) DescribeReplicationGroups(
	ctx context.Context,
	params *elasticache.DescribeReplicationGroupsInput,
	optFns ...func(*elasticache.Options),
) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return m.output, m.err
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

// mockEKSFullClient composes independently-configurable EKS mocks
// (mockEKSListClustersClient/mockEKSDescribeClusterClient here,
// mockEKSListNodegroupsClient/mockEKSDescribeNodegroupClient in
// aws_nodegroups_test.go) into one awsclient.EKSAPI value — required because
// FetchEKSClustersPage and the registered "ng" paginated fetcher each read
// every EKS operation off a single *ServiceClients.EKS field, unlike the
// removed signatures that took each operation as a separate parameter.
// newMockEKSFull defaults any nil argument to a zero-value mock (safe empty
// response) so callers only need to specify the parts they care about.
type mockEKSFullClient struct {
	*mockEKSListClustersClient
	*mockEKSDescribeClusterClient
	*mockEKSListNodegroupsClient
	*mockEKSDescribeNodegroupClient
}

func newMockEKSFull(
	list *mockEKSListClustersClient,
	describe *mockEKSDescribeClusterClient,
	listNG *mockEKSListNodegroupsClient,
	describeNG *mockEKSDescribeNodegroupClient,
) *mockEKSFullClient {
	if list == nil {
		list = &mockEKSListClustersClient{}
	}
	if describe == nil {
		describe = &mockEKSDescribeClusterClient{}
	}
	if listNG == nil {
		listNG = &mockEKSListNodegroupsClient{}
	}
	if describeNG == nil {
		describeNG = &mockEKSDescribeNodegroupClient{}
	}
	return &mockEKSFullClient{list, describe, listNG, describeNG}
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

// MockAPIError implements smithy.APIError with a caller-supplied code,
// message, and fault. Exported so tests/unit_test package files (which
// cannot share unexported identifiers with this package) can reuse it
// instead of defining their own copy — see fakes_boundary_test.go's
// boundaryAPIError and costs_selfreview_test.go's selfReviewAPIError aliases.
type MockAPIError struct {
	Code    string
	Message string
	Fault   smithy.ErrorFault
}

func (e *MockAPIError) Error() string                 { return e.Message }
func (e *MockAPIError) ErrorCode() string             { return e.Code }
func (e *MockAPIError) ErrorMessage() string          { return e.Message }
func (e *MockAPIError) ErrorFault() smithy.ErrorFault { return e.Fault }

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

// SNS mocks: the fake client for ListTopics now lives in fakes_sns_test.go
// (fakeSNSListTopics) — see that file's header for the one-fake-per-
// interface convention.

// mockSNSListSubscriptionsByTopicClient supports paginated responses.
type mockSNSListSubscriptionsByTopicClient struct {
	outputs []*sns.ListSubscriptionsByTopicOutput
	err     error
	callIdx int
}

func (m *mockSNSListSubscriptionsByTopicClient) ListSubscriptionsByTopic(ctx context.Context, params *sns.ListSubscriptionsByTopicInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &sns.ListSubscriptionsByTopicOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
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

// mockELBv2DescribeTargetHealthClient implements awsclient.ELBv2DescribeTargetHealthAPI.
type mockELBv2DescribeTargetHealthClient struct {
	output *elbv2.DescribeTargetHealthOutput
	err    error
}

func (m *mockELBv2DescribeTargetHealthClient) DescribeTargetHealth(ctx context.Context, params *elbv2.DescribeTargetHealthInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
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

// mockEC2DescribeVolumesClient implements awsclient.EC2DescribeVolumesAPI for testing.
type mockEC2DescribeVolumesClient struct {
	output *ec2.DescribeVolumesOutput
	err    error
}

func (m *mockEC2DescribeVolumesClient) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return m.output, m.err
}

// mockEC2DescribeSnapshotsClient implements awsclient.EC2DescribeSnapshotsAPI for testing.
type mockEC2DescribeSnapshotsClient struct {
	output *ec2.DescribeSnapshotsOutput
	err    error
}

func (m *mockEC2DescribeSnapshotsClient) DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return m.output, m.err
}

// mockEC2DescribeImagesClient implements awsclient.EC2DescribeImagesAPI for testing.
type mockEC2DescribeImagesClient struct {
	output *ec2.DescribeImagesOutput
	err    error
}

func (m *mockEC2DescribeImagesClient) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return m.output, m.err
}

// mockCloudTrailLookupEventsClient implements awsclient.CloudTrailLookupEventsAPI for testing.
type mockCloudTrailLookupEventsClient struct {
	output        *cloudtrail.LookupEventsOutput
	err           error
	capturedInput *cloudtrail.LookupEventsInput
}

func (m *mockCloudTrailLookupEventsClient) LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	m.capturedInput = params
	return m.output, m.err
}
