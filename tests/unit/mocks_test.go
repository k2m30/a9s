package unit

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
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

// mockS3ListBucketsAPI implements awsclient.S3ListBucketsAPI with exported fields.
type mockS3ListBucketsAPI struct {
	Output *s3.ListBucketsOutput
	Err    error
}

func (m *mockS3ListBucketsAPI) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.Output, m.Err
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
