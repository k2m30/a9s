package unit

// qa_pagination_services_test.go — pagination tests for service fetchers:
// rds, redis, docdb, dbi-snap, docdb-snap, efs, r53, cf, acm, apigw, cfn, cb, pipeline, ecr, codeartifact

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	catypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbstypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ecachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Mock: RDS DescribeDBInstances (paginated, uses Marker)
// ---------------------------------------------------------------------------

type mockRDSDescribeDBInstancesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*rds.DescribeDBInstancesOutput, error)
	lastInput *rds.DescribeDBInstancesInput
}

func (m *mockRDSDescribeDBInstancesAPIPaginated) DescribeDBInstances(_ context.Context, in *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchRDSInstancesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchRDSInstancesPage_FirstPage(t *testing.T) {
	mock := &mockRDSDescribeDBInstancesAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("prod-mysql"),
						Engine:               aws.String("mysql"),
						EngineVersion:        aws.String("8.0.32"),
						DBInstanceStatus:     aws.String("available"),
						DBInstanceClass:      aws.String("db.t3.medium"),
					},
				},
				Marker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "prod-mysql" {
		t.Errorf("resource ID: expected %q, got %q", "prod-mysql", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchRDSInstancesPage_Continuation(t *testing.T) {
	mock := &mockRDSDescribeDBInstancesAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("staging-postgres"),
						Engine:               aws.String("postgres"),
						EngineVersion:        aws.String("15.2"),
						DBInstanceStatus:     aws.String("available"),
					},
				},
				Marker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (Marker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchRDSInstancesPage_Empty(t *testing.T) {
	mock := &mockRDSDescribeDBInstancesAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{},
				Marker:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchRDSInstancesPage_Error(t *testing.T) {
	mock := &mockRDSDescribeDBInstancesAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBInstancesOutput, error) {
			return nil, errors.New("describe db instances failed")
		},
	}

	_, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ElastiCache DescribeReplicationGroups (paginated, uses Marker)
// ---------------------------------------------------------------------------

type mockElastiCacheDescribeReplicationGroupsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*elasticache.DescribeReplicationGroupsOutput, error)
	lastInput *elasticache.DescribeReplicationGroupsInput
}

func (m *mockElastiCacheDescribeReplicationGroupsAPIPaginated) DescribeReplicationGroups(_ context.Context, in *elasticache.DescribeReplicationGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchRedisPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchRedisPage_FirstPage(t *testing.T) {
	mock := &mockElastiCacheDescribeReplicationGroupsAPIPaginated{
		PageFunc: func(_ int) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []ecachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("my-redis"),
						Status:             aws.String("available"),
						Engine:             aws.String("redis"),
						MemberClusters:     []string{"my-redis-001"},
					},
				},
				Marker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-redis" {
		t.Errorf("resource ID: expected %q, got %q", "my-redis", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchRedisPage_Continuation(t *testing.T) {
	mock := &mockElastiCacheDescribeReplicationGroupsAPIPaginated{
		PageFunc: func(_ int) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []ecachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("staging-redis"),
						Status:             aws.String("creating"),
						Engine:             aws.String("redis"),
						MemberClusters:     []string{"staging-redis-001"},
					},
				},
				Marker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRedisPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (Marker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchRedisPage_Empty(t *testing.T) {
	mock := &mockElastiCacheDescribeReplicationGroupsAPIPaginated{
		PageFunc: func(_ int) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []ecachetypes.ReplicationGroup{},
				Marker:            nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchRedisPage_Error(t *testing.T) {
	mock := &mockElastiCacheDescribeReplicationGroupsAPIPaginated{
		PageFunc: func(_ int) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return nil, errors.New("describe replication groups failed")
		},
	}

	_, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: DocumentDB DescribeDBClusters (paginated, uses Marker)
// ---------------------------------------------------------------------------

type mockDocDBDescribeDBClustersAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*docdb.DescribeDBClustersOutput, error)
	lastInput *docdb.DescribeDBClustersInput
}

func (m *mockDocDBDescribeDBClustersAPIPaginated) DescribeDBClusters(_ context.Context, in *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchDocDBClustersPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchDocDBClustersPage_FirstPage(t *testing.T) {
	mock := &mockDocDBDescribeDBClustersAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClustersOutput, error) {
			return &docdb.DescribeDBClustersOutput{
				DBClusters: []docdbstypes.DBCluster{
					{
						DBClusterIdentifier: aws.String("prod-docdb"),
						EngineVersion:       aws.String("6.0.0"),
						Status:              aws.String("available"),
						Endpoint:            aws.String("prod-docdb.cluster-abc.us-east-1.docdb.amazonaws.com"),
					},
				},
				Marker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchDocDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "prod-docdb" {
		t.Errorf("resource ID: expected %q, got %q", "prod-docdb", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchDocDBClustersPage_Continuation(t *testing.T) {
	mock := &mockDocDBDescribeDBClustersAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClustersOutput, error) {
			return &docdb.DescribeDBClustersOutput{
				DBClusters: []docdbstypes.DBCluster{
					{
						DBClusterIdentifier: aws.String("staging-docdb"),
						EngineVersion:       aws.String("5.0.0"),
						Status:              aws.String("creating"),
					},
				},
				Marker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchDocDBClustersPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (Marker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchDocDBClustersPage_Empty(t *testing.T) {
	mock := &mockDocDBDescribeDBClustersAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClustersOutput, error) {
			return &docdb.DescribeDBClustersOutput{
				DBClusters: []docdbstypes.DBCluster{},
				Marker:     nil,
			}, nil
		},
	}

	result, err := awsclient.FetchDocDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchDocDBClustersPage_Error(t *testing.T) {
	mock := &mockDocDBDescribeDBClustersAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClustersOutput, error) {
			return nil, errors.New("describe db clusters failed")
		},
	}

	_, err := awsclient.FetchDocDBClustersPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: RDS DescribeDBSnapshots (paginated, uses Marker)
// ---------------------------------------------------------------------------

type mockRDSDescribeDBSnapshotsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*rds.DescribeDBSnapshotsOutput, error)
	lastInput *rds.DescribeDBSnapshotsInput
}

func (m *mockRDSDescribeDBSnapshotsAPIPaginated) DescribeDBSnapshots(_ context.Context, in *rds.DescribeDBSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchDBISnapshotsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchDBISnapshotsPage_FirstPage(t *testing.T) {
	mock := &mockRDSDescribeDBSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBSnapshotsOutput, error) {
			return &rds.DescribeDBSnapshotsOutput{
				DBSnapshots: []rdstypes.DBSnapshot{
					{
						DBSnapshotIdentifier: aws.String("prod-mysql-snap-2025-01-01"),
						DBInstanceIdentifier: aws.String("prod-mysql"),
						Status:               aws.String("available"),
						Engine:               aws.String("mysql"),
						SnapshotType:         aws.String("manual"),
					},
				},
				Marker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "prod-mysql-snap-2025-01-01" {
		t.Errorf("resource ID: expected %q, got %q", "prod-mysql-snap-2025-01-01", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchDBISnapshotsPage_Continuation(t *testing.T) {
	mock := &mockRDSDescribeDBSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBSnapshotsOutput, error) {
			return &rds.DescribeDBSnapshotsOutput{
				DBSnapshots: []rdstypes.DBSnapshot{
					{
						DBSnapshotIdentifier: aws.String("staging-postgres-snap-2025-02-01"),
						DBInstanceIdentifier: aws.String("staging-postgres"),
						Status:               aws.String("available"),
						Engine:               aws.String("postgres"),
						SnapshotType:         aws.String("automated"),
					},
				},
				Marker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (Marker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchDBISnapshotsPage_Empty(t *testing.T) {
	mock := &mockRDSDescribeDBSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBSnapshotsOutput, error) {
			return &rds.DescribeDBSnapshotsOutput{
				DBSnapshots: []rdstypes.DBSnapshot{},
				Marker:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchDBISnapshotsPage_Error(t *testing.T) {
	mock := &mockRDSDescribeDBSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*rds.DescribeDBSnapshotsOutput, error) {
			return nil, errors.New("describe db snapshots failed")
		},
	}

	_, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: DocumentDB DescribeDBClusterSnapshots (paginated, uses Marker)
// ---------------------------------------------------------------------------

type mockDocDBDescribeDBClusterSnapshotsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*docdb.DescribeDBClusterSnapshotsOutput, error)
	lastInput *docdb.DescribeDBClusterSnapshotsInput
}

func (m *mockDocDBDescribeDBClusterSnapshotsAPIPaginated) DescribeDBClusterSnapshots(_ context.Context, in *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchDocDBClusterSnapshotsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchDocDBClusterSnapshotsPage_FirstPage(t *testing.T) {
	mock := &mockDocDBDescribeDBClusterSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
			return &docdb.DescribeDBClusterSnapshotsOutput{
				DBClusterSnapshots: []docdbstypes.DBClusterSnapshot{
					{
						DBClusterSnapshotIdentifier: aws.String("prod-docdb-snap-2025-01-01"),
						DBClusterIdentifier:         aws.String("prod-docdb"),
						Status:                      aws.String("available"),
						Engine:                      aws.String("docdb"),
						SnapshotType:                aws.String("manual"),
					},
				},
				Marker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "prod-docdb-snap-2025-01-01" {
		t.Errorf("resource ID: expected %q, got %q", "prod-docdb-snap-2025-01-01", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchDocDBClusterSnapshotsPage_Continuation(t *testing.T) {
	mock := &mockDocDBDescribeDBClusterSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
			return &docdb.DescribeDBClusterSnapshotsOutput{
				DBClusterSnapshots: []docdbstypes.DBClusterSnapshot{
					{
						DBClusterSnapshotIdentifier: aws.String("staging-docdb-snap-2025-02-01"),
						DBClusterIdentifier:         aws.String("staging-docdb"),
						Status:                      aws.String("available"),
						SnapshotType:                aws.String("automated"),
					},
				},
				Marker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (Marker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchDocDBClusterSnapshotsPage_Empty(t *testing.T) {
	mock := &mockDocDBDescribeDBClusterSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
			return &docdb.DescribeDBClusterSnapshotsOutput{
				DBClusterSnapshots: []docdbstypes.DBClusterSnapshot{},
				Marker:             nil,
			}, nil
		},
	}

	result, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchDocDBClusterSnapshotsPage_Error(t *testing.T) {
	mock := &mockDocDBDescribeDBClusterSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
			return nil, errors.New("describe db cluster snapshots failed")
		},
	}

	_, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EFS DescribeFileSystems (paginated, uses Marker/NextMarker)
// ---------------------------------------------------------------------------

type mockEFSDescribeFileSystemsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*efs.DescribeFileSystemsOutput, error)
	lastInput *efs.DescribeFileSystemsInput
}

func (m *mockEFSDescribeFileSystemsAPIPaginated) DescribeFileSystems(_ context.Context, in *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	m.lastInput = in
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchEFSFileSystemsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchEFSFileSystemsPage_FirstPage(t *testing.T) {
	encrypted := true
	mountTargets := int32(3)
	mock := &mockEFSDescribeFileSystemsAPIPaginated{
		PageFunc: func(_ int) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{
						FileSystemId:         aws.String("fs-0abc111222333444a"),
						Name:                 aws.String("my-efs"),
						LifeCycleState:       efstypes.LifeCycleStateAvailable,
						PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
						ThroughputMode:       efstypes.ThroughputModeBursting,
						Encrypted:            &encrypted,
						NumberOfMountTargets: mountTargets,
					},
				},
				NextMarker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextMarker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "fs-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "fs-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchEFSFileSystemsPage_Continuation(t *testing.T) {
	encrypted := false
	mountTargets := int32(1)
	mock := &mockEFSDescribeFileSystemsAPIPaginated{
		PageFunc: func(_ int) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{
						FileSystemId:         aws.String("fs-0xyz999888777666b"),
						LifeCycleState:       efstypes.LifeCycleStateCreating,
						PerformanceMode:      efstypes.PerformanceModeMaxIo,
						ThroughputMode:       efstypes.ThroughputModeProvisioned,
						Encrypted:            &encrypted,
						NumberOfMountTargets: mountTargets,
					},
				},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextMarker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchEFSFileSystemsPage_Empty(t *testing.T) {
	mock := &mockEFSDescribeFileSystemsAPIPaginated{
		PageFunc: func(_ int) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{},
				NextMarker:  nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchEFSFileSystemsPage_Error(t *testing.T) {
	mock := &mockEFSDescribeFileSystemsAPIPaginated{
		PageFunc: func(_ int) (*efs.DescribeFileSystemsOutput, error) {
			return nil, errors.New("describe file systems failed")
		},
	}

	_, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Route53 ListHostedZones (paginated, uses Marker/NextMarker + IsTruncated)
// ---------------------------------------------------------------------------

type mockRoute53ListHostedZonesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*route53.ListHostedZonesOutput, error)
	lastInput *route53.ListHostedZonesInput
}

func (m *mockRoute53ListHostedZonesAPIPaginated) ListHostedZones(_ context.Context, in *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchHostedZonesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchHostedZonesPage_FirstPage(t *testing.T) {
	recordCount := int64(42)
	mock := &mockRoute53ListHostedZonesAPIPaginated{
		PageFunc: func(_ int) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{
					{
						Id:                     aws.String("/hostedzone/ABCDEF123456"),
						Name:                   aws.String("example.com."),
						ResourceRecordSetCount: &recordCount,
						Config: &r53types.HostedZoneConfig{
							PrivateZone: false,
							Comment:     aws.String("Main zone"),
						},
					},
				},
				IsTruncated: true,
				NextMarker:  aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchHostedZonesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextMarker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "/hostedzone/ABCDEF123456" {
		t.Errorf("resource ID: expected %q, got %q", "/hostedzone/ABCDEF123456", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchHostedZonesPage_Continuation(t *testing.T) {
	recordCount := int64(10)
	mock := &mockRoute53ListHostedZonesAPIPaginated{
		PageFunc: func(_ int) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{
					{
						Id:                     aws.String("/hostedzone/XYZXYZ999888"),
						Name:                   aws.String("internal.example.com."),
						ResourceRecordSetCount: &recordCount,
						Config: &r53types.HostedZoneConfig{
							PrivateZone: true,
						},
					},
				},
				IsTruncated: false,
				NextMarker:  nil,
			}, nil
		},
	}

	result, err := awsclient.FetchHostedZonesPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (IsTruncated=false)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchHostedZonesPage_Empty(t *testing.T) {
	mock := &mockRoute53ListHostedZonesAPIPaginated{
		PageFunc: func(_ int) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{},
				IsTruncated: false,
				NextMarker:  nil,
			}, nil
		},
	}

	result, err := awsclient.FetchHostedZonesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchHostedZonesPage_Error(t *testing.T) {
	mock := &mockRoute53ListHostedZonesAPIPaginated{
		PageFunc: func(_ int) (*route53.ListHostedZonesOutput, error) {
			return nil, errors.New("list hosted zones failed")
		},
	}

	_, err := awsclient.FetchHostedZonesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudFront ListDistributions (paginated, uses Marker/NextMarker + IsTruncated)
// ---------------------------------------------------------------------------

type mockCloudFrontListDistributionsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*cloudfront.ListDistributionsOutput, error)
	lastInput *cloudfront.ListDistributionsInput
}

func (m *mockCloudFrontListDistributionsAPIPaginated) ListDistributions(_ context.Context, in *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
	m.lastInput = in
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCloudFrontDistributionsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCloudFrontDistributionsPage_FirstPage(t *testing.T) {
	enabled := true
	isTruncated := true
	mock := &mockCloudFrontListDistributionsAPIPaginated{
		PageFunc: func(_ int) (*cloudfront.ListDistributionsOutput, error) {
			return &cloudfront.ListDistributionsOutput{
				DistributionList: &cftypes.DistributionList{
					Items: []cftypes.DistributionSummary{
						{
							Id:         aws.String("E1ABC111222333"),
							DomainName: aws.String("d1abc111222333.cloudfront.net"),
							Status:     aws.String("Deployed"),
							Enabled:    &enabled,
							PriceClass: cftypes.PriceClassPriceClass100,
						},
					},
					IsTruncated: &isTruncated,
					NextMarker:  aws.String("marker-page-2"),
				},
			}, nil
		},
	}

	result, err := awsclient.FetchCloudFrontDistributionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextMarker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "E1ABC111222333" {
		t.Errorf("resource ID: expected %q, got %q", "E1ABC111222333", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCloudFrontDistributionsPage_Continuation(t *testing.T) {
	enabled := false
	isTruncated := false
	mock := &mockCloudFrontListDistributionsAPIPaginated{
		PageFunc: func(_ int) (*cloudfront.ListDistributionsOutput, error) {
			return &cloudfront.ListDistributionsOutput{
				DistributionList: &cftypes.DistributionList{
					Items: []cftypes.DistributionSummary{
						{
							Id:         aws.String("E2XYZ999888777"),
							DomainName: aws.String("d2xyz999888777.cloudfront.net"),
							Status:     aws.String("InProgress"),
							Enabled:    &enabled,
							PriceClass: cftypes.PriceClassPriceClassAll,
						},
					},
					IsTruncated: &isTruncated,
					NextMarker:  nil,
				},
			}, nil
		},
	}

	result, err := awsclient.FetchCloudFrontDistributionsPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (IsTruncated=false)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchCloudFrontDistributionsPage_Empty(t *testing.T) {
	isTruncated := false
	mock := &mockCloudFrontListDistributionsAPIPaginated{
		PageFunc: func(_ int) (*cloudfront.ListDistributionsOutput, error) {
			return &cloudfront.ListDistributionsOutput{
				DistributionList: &cftypes.DistributionList{
					Items:       []cftypes.DistributionSummary{},
					IsTruncated: &isTruncated,
					NextMarker:  nil,
				},
			}, nil
		},
	}

	result, err := awsclient.FetchCloudFrontDistributionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCloudFrontDistributionsPage_Error(t *testing.T) {
	mock := &mockCloudFrontListDistributionsAPIPaginated{
		PageFunc: func(_ int) (*cloudfront.ListDistributionsOutput, error) {
			return nil, errors.New("list distributions failed")
		},
	}

	_, err := awsclient.FetchCloudFrontDistributionsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ACM ListCertificates (paginated)
// ---------------------------------------------------------------------------

type mockACMListCertificatesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*acm.ListCertificatesOutput, error)
	lastInput *acm.ListCertificatesInput
}

func (m *mockACMListCertificatesAPIPaginated) ListCertificates(_ context.Context, in *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchACMCertificatesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchACMCertificatesPage_FirstPage(t *testing.T) {
	inUse := true
	mock := &mockACMListCertificatesAPIPaginated{
		PageFunc: func(_ int) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []acmtypes.CertificateSummary{
					{
						DomainName: aws.String("example.com"),
						Status:     acmtypes.CertificateStatusIssued,
						Type:       acmtypes.CertificateTypeAmazonIssued,
						InUse:      &inUse,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchACMCertificatesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "example.com" {
		t.Errorf("resource ID: expected %q, got %q", "example.com", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchACMCertificatesPage_Continuation(t *testing.T) {
	inUse := false
	mock := &mockACMListCertificatesAPIPaginated{
		PageFunc: func(_ int) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []acmtypes.CertificateSummary{
					{
						DomainName: aws.String("*.internal.example.com"),
						Status:     acmtypes.CertificateStatusPendingValidation,
						Type:       acmtypes.CertificateTypeAmazonIssued,
						InUse:      &inUse,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchACMCertificatesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchACMCertificatesPage_Empty(t *testing.T) {
	mock := &mockACMListCertificatesAPIPaginated{
		PageFunc: func(_ int) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []acmtypes.CertificateSummary{},
				NextToken:              nil,
			}, nil
		},
	}

	result, err := awsclient.FetchACMCertificatesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchACMCertificatesPage_Error(t *testing.T) {
	mock := &mockACMListCertificatesAPIPaginated{
		PageFunc: func(_ int) (*acm.ListCertificatesOutput, error) {
			return nil, errors.New("list certificates failed")
		},
	}

	_, err := awsclient.FetchACMCertificatesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: API Gateway V2 GetApis (paginated)
// ---------------------------------------------------------------------------

type mockAPIGatewayV2GetApisAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*apigatewayv2.GetApisOutput, error)
	lastInput *apigatewayv2.GetApisInput
}

func (m *mockAPIGatewayV2GetApisAPIPaginated) GetApis(_ context.Context, in *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchAPIGatewaysPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchAPIGatewaysPage_FirstPage(t *testing.T) {
	mock := &mockAPIGatewayV2GetApisAPIPaginated{
		PageFunc: func(_ int) (*apigatewayv2.GetApisOutput, error) {
			return &apigatewayv2.GetApisOutput{
				Items: []apigwtypes.Api{
					{
						ApiId:        aws.String("abc1234567"),
						Name:         aws.String("my-http-api"),
						ProtocolType: apigwtypes.ProtocolTypeHttp,
						ApiEndpoint:  aws.String("https://abc1234567.execute-api.us-east-1.amazonaws.com"),
						Description:  aws.String("My HTTP API"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchAPIGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "abc1234567" {
		t.Errorf("resource ID: expected %q, got %q", "abc1234567", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchAPIGatewaysPage_Continuation(t *testing.T) {
	mock := &mockAPIGatewayV2GetApisAPIPaginated{
		PageFunc: func(_ int) (*apigatewayv2.GetApisOutput, error) {
			return &apigatewayv2.GetApisOutput{
				Items: []apigwtypes.Api{
					{
						ApiId:        aws.String("xyz9876543"),
						Name:         aws.String("my-websocket-api"),
						ProtocolType: apigwtypes.ProtocolTypeWebsocket,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAPIGatewaysPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchAPIGatewaysPage_Empty(t *testing.T) {
	mock := &mockAPIGatewayV2GetApisAPIPaginated{
		PageFunc: func(_ int) (*apigatewayv2.GetApisOutput, error) {
			return &apigatewayv2.GetApisOutput{
				Items:     []apigwtypes.Api{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAPIGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchAPIGatewaysPage_Error(t *testing.T) {
	mock := &mockAPIGatewayV2GetApisAPIPaginated{
		PageFunc: func(_ int) (*apigatewayv2.GetApisOutput, error) {
			return nil, errors.New("get apis failed")
		},
	}

	_, err := awsclient.FetchAPIGatewaysPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudFormation DescribeStacks (paginated)
// ---------------------------------------------------------------------------

type mockCFNDescribeStacksAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*cloudformation.DescribeStacksOutput, error)
	lastInput *cloudformation.DescribeStacksInput
}

func (m *mockCFNDescribeStacksAPIPaginated) DescribeStacks(_ context.Context, in *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCloudFormationStacksPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCloudFormationStacksPage_FirstPage(t *testing.T) {
	mock := &mockCFNDescribeStacksAPIPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cfntypes.Stack{
					{
						StackName:   aws.String("my-app-stack"),
						StackStatus: cfntypes.StackStatusCreateComplete,
						Description: aws.String("My application stack"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchCloudFormationStacksPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-app-stack" {
		t.Errorf("resource ID: expected %q, got %q", "my-app-stack", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCloudFormationStacksPage_Continuation(t *testing.T) {
	mock := &mockCFNDescribeStacksAPIPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cfntypes.Stack{
					{
						StackName:   aws.String("infra-stack"),
						StackStatus: cfntypes.StackStatusUpdateComplete,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCloudFormationStacksPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchCloudFormationStacksPage_Empty(t *testing.T) {
	mock := &mockCFNDescribeStacksAPIPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks:    []cfntypes.Stack{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCloudFormationStacksPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCloudFormationStacksPage_Error(t *testing.T) {
	mock := &mockCFNDescribeStacksAPIPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStacksOutput, error) {
			return nil, errors.New("describe stacks failed")
		},
	}

	_, err := awsclient.FetchCloudFormationStacksPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CodeBuild ListProjects + BatchGetProjects (paginated)
// ---------------------------------------------------------------------------

type mockCodeBuildListProjectsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*codebuild.ListProjectsOutput, error)
	lastInput *codebuild.ListProjectsInput
}

func (m *mockCodeBuildListProjectsAPIPaginated) ListProjects(_ context.Context, in *codebuild.ListProjectsInput, _ ...func(*codebuild.Options)) (*codebuild.ListProjectsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

type mockCodeBuildBatchGetProjectsAPIPaginated struct {
	Calls        int
	BatchGetFunc func(call int) (*codebuild.BatchGetProjectsOutput, error)
}

func (m *mockCodeBuildBatchGetProjectsAPIPaginated) BatchGetProjects(_ context.Context, _ *codebuild.BatchGetProjectsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error) {
	m.Calls++
	return m.BatchGetFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCodeBuildProjectsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCodeBuildProjectsPage_FirstPage(t *testing.T) {
	listMock := &mockCodeBuildListProjectsAPIPaginated{
		PageFunc: func(_ int) (*codebuild.ListProjectsOutput, error) {
			return &codebuild.ListProjectsOutput{
				Projects:  []string{"my-build-project"},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}
	batchMock := &mockCodeBuildBatchGetProjectsAPIPaginated{
		BatchGetFunc: func(_ int) (*codebuild.BatchGetProjectsOutput, error) {
			return &codebuild.BatchGetProjectsOutput{
				Projects: []cbtypes.Project{
					{
						Name:        aws.String("my-build-project"),
						Description: aws.String("Builds the main app"),
						Source: &cbtypes.ProjectSource{
							Type: cbtypes.SourceTypeCodecommit,
						},
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchCodeBuildProjectsPage(context.Background(), listMock, batchMock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-build-project" {
		t.Errorf("resource ID: expected %q, got %q", "my-build-project", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCodeBuildProjectsPage_Continuation(t *testing.T) {
	listMock := &mockCodeBuildListProjectsAPIPaginated{
		PageFunc: func(_ int) (*codebuild.ListProjectsOutput, error) {
			return &codebuild.ListProjectsOutput{
				Projects:  []string{"another-project"},
				NextToken: nil,
			}, nil
		},
	}
	batchMock := &mockCodeBuildBatchGetProjectsAPIPaginated{
		BatchGetFunc: func(_ int) (*codebuild.BatchGetProjectsOutput, error) {
			return &codebuild.BatchGetProjectsOutput{
				Projects: []cbtypes.Project{
					{
						Name: aws.String("another-project"),
						Source: &cbtypes.ProjectSource{
							Type: cbtypes.SourceTypeGithub,
						},
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchCodeBuildProjectsPage(context.Background(), listMock, batchMock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if listMock.lastInput == nil {
		t.Fatal("list mock was not called")
	}
	if listMock.lastInput.NextToken == nil || *listMock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", listMock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchCodeBuildProjectsPage_Empty(t *testing.T) {
	listMock := &mockCodeBuildListProjectsAPIPaginated{
		PageFunc: func(_ int) (*codebuild.ListProjectsOutput, error) {
			return &codebuild.ListProjectsOutput{
				Projects:  []string{},
				NextToken: nil,
			}, nil
		},
	}
	batchMock := &mockCodeBuildBatchGetProjectsAPIPaginated{
		BatchGetFunc: func(_ int) (*codebuild.BatchGetProjectsOutput, error) {
			return &codebuild.BatchGetProjectsOutput{Projects: []cbtypes.Project{}}, nil
		},
	}

	result, err := awsclient.FetchCodeBuildProjectsPage(context.Background(), listMock, batchMock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCodeBuildProjectsPage_Error(t *testing.T) {
	listMock := &mockCodeBuildListProjectsAPIPaginated{
		PageFunc: func(_ int) (*codebuild.ListProjectsOutput, error) {
			return nil, errors.New("list projects failed")
		},
	}
	batchMock := &mockCodeBuildBatchGetProjectsAPIPaginated{
		BatchGetFunc: func(_ int) (*codebuild.BatchGetProjectsOutput, error) {
			return &codebuild.BatchGetProjectsOutput{}, nil
		},
	}

	_, err := awsclient.FetchCodeBuildProjectsPage(context.Background(), listMock, batchMock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CodePipeline ListPipelines (paginated)
// ---------------------------------------------------------------------------

type mockCodePipelineListPipelinesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*codepipeline.ListPipelinesOutput, error)
	lastInput *codepipeline.ListPipelinesInput
}

func (m *mockCodePipelineListPipelinesAPIPaginated) ListPipelines(_ context.Context, in *codepipeline.ListPipelinesInput, _ ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCodePipelinesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCodePipelinesPage_FirstPage(t *testing.T) {
	version := int32(3)
	mock := &mockCodePipelineListPipelinesAPIPaginated{
		PageFunc: func(_ int) (*codepipeline.ListPipelinesOutput, error) {
			return &codepipeline.ListPipelinesOutput{
				Pipelines: []cptypes.PipelineSummary{
					{
						Name:         aws.String("my-deploy-pipeline"),
						PipelineType: cptypes.PipelineTypeV2,
						Version:      &version,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchCodePipelinesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-deploy-pipeline" {
		t.Errorf("resource ID: expected %q, got %q", "my-deploy-pipeline", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCodePipelinesPage_Continuation(t *testing.T) {
	version := int32(1)
	mock := &mockCodePipelineListPipelinesAPIPaginated{
		PageFunc: func(_ int) (*codepipeline.ListPipelinesOutput, error) {
			return &codepipeline.ListPipelinesOutput{
				Pipelines: []cptypes.PipelineSummary{
					{
						Name:    aws.String("infra-pipeline"),
						Version: &version,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCodePipelinesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchCodePipelinesPage_Empty(t *testing.T) {
	mock := &mockCodePipelineListPipelinesAPIPaginated{
		PageFunc: func(_ int) (*codepipeline.ListPipelinesOutput, error) {
			return &codepipeline.ListPipelinesOutput{
				Pipelines: []cptypes.PipelineSummary{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCodePipelinesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCodePipelinesPage_Error(t *testing.T) {
	mock := &mockCodePipelineListPipelinesAPIPaginated{
		PageFunc: func(_ int) (*codepipeline.ListPipelinesOutput, error) {
			return nil, errors.New("list pipelines failed")
		},
	}

	_, err := awsclient.FetchCodePipelinesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ECR DescribeRepositories (paginated)
// ---------------------------------------------------------------------------

type mockECRDescribeRepositoriesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ecr.DescribeRepositoriesOutput, error)
	lastInput *ecr.DescribeRepositoriesInput
}

func (m *mockECRDescribeRepositoriesAPIPaginated) DescribeRepositories(_ context.Context, in *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchECRRepositoriesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchECRRepositoriesPage_FirstPage(t *testing.T) {
	mock := &mockECRDescribeRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{
				Repositories: []ecrtypes.Repository{
					{
						RepositoryName:     aws.String("my-app-repo"),
						RepositoryUri:      aws.String("111111111111.dkr.ecr.us-east-1.amazonaws.com/my-app-repo"),
						ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
						ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
							ScanOnPush: true,
						},
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchECRRepositoriesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-app-repo" {
		t.Errorf("resource ID: expected %q, got %q", "my-app-repo", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchECRRepositoriesPage_Continuation(t *testing.T) {
	mock := &mockECRDescribeRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{
				Repositories: []ecrtypes.Repository{
					{
						RepositoryName:     aws.String("infra-base-images"),
						RepositoryUri:      aws.String("111111111111.dkr.ecr.us-east-1.amazonaws.com/infra-base-images"),
						ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchECRRepositoriesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchECRRepositoriesPage_Empty(t *testing.T) {
	mock := &mockECRDescribeRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{
				Repositories: []ecrtypes.Repository{},
				NextToken:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchECRRepositoriesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchECRRepositoriesPage_Error(t *testing.T) {
	mock := &mockECRDescribeRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*ecr.DescribeRepositoriesOutput, error) {
			return nil, errors.New("describe repositories failed")
		},
	}

	_, err := awsclient.FetchECRRepositoriesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CodeArtifact ListRepositories (paginated)
// ---------------------------------------------------------------------------

type mockCodeArtifactListRepositoriesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*codeartifact.ListRepositoriesOutput, error)
	lastInput *codeartifact.ListRepositoriesInput
}

func (m *mockCodeArtifactListRepositoriesAPIPaginated) ListRepositories(_ context.Context, in *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchCodeArtifactReposPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchCodeArtifactReposPage_FirstPage(t *testing.T) {
	mock := &mockCodeArtifactListRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*codeartifact.ListRepositoriesOutput, error) {
			return &codeartifact.ListRepositoriesOutput{
				Repositories: []catypes.RepositorySummary{
					{
						Name:        aws.String("my-npm-repo"),
						DomainName:  aws.String("my-domain"),
						DomainOwner: aws.String("111111111111"),
						Arn:         aws.String("arn:aws:codeartifact:us-east-1:111111111111:repository/my-domain/my-npm-repo"),
						Description: aws.String("NPM packages"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchCodeArtifactReposPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-npm-repo" {
		t.Errorf("resource ID: expected %q, got %q", "my-npm-repo", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchCodeArtifactReposPage_Continuation(t *testing.T) {
	mock := &mockCodeArtifactListRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*codeartifact.ListRepositoriesOutput, error) {
			return &codeartifact.ListRepositoriesOutput{
				Repositories: []catypes.RepositorySummary{
					{
						Name:       aws.String("my-maven-repo"),
						DomainName: aws.String("my-domain"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCodeArtifactReposPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchCodeArtifactReposPage_Empty(t *testing.T) {
	mock := &mockCodeArtifactListRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*codeartifact.ListRepositoriesOutput, error) {
			return &codeartifact.ListRepositoriesOutput{
				Repositories: []catypes.RepositorySummary{},
				NextToken:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchCodeArtifactReposPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchCodeArtifactReposPage_Error(t *testing.T) {
	mock := &mockCodeArtifactListRepositoriesAPIPaginated{
		PageFunc: func(_ int) (*codeartifact.ListRepositoriesOutput, error) {
			return nil, errors.New("list repositories failed")
		},
	}

	_, err := awsclient.FetchCodeArtifactReposPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
