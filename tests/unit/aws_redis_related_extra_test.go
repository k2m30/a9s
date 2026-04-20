// aws_redis_related_extra_test.go covers checkRedisKMS, checkRedisSubnet, and
// checkRedisVPC — three checkers that require a live ElastiCache API call and
// were not covered by aws_redis_related_test.go.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeElastiCacheCR — implements ElastiCacheAPI (DescribeCacheClusters,
// DescribeReplicationGroups, DescribeCacheSubnetGroups, ListTagsForResource).
// Used by checkRedisKMS, checkRedisSubnet, checkRedisVPC tests.
// ---------------------------------------------------------------------------

type fakeElastiCacheCR struct {
	replicationGroupsOutput *elasticache.DescribeReplicationGroupsOutput
	replicationGroupsErr    error
	subnetGroupsOutput      *elasticache.DescribeCacheSubnetGroupsOutput
	subnetGroupsErr         error
}

func (f *fakeElastiCacheCR) DescribeCacheClusters(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return &elasticache.DescribeCacheClustersOutput{}, nil
}

func (f *fakeElastiCacheCR) DescribeReplicationGroups(_ context.Context, _ *elasticache.DescribeReplicationGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	if f.replicationGroupsErr != nil {
		return nil, f.replicationGroupsErr
	}
	if f.replicationGroupsOutput != nil {
		return f.replicationGroupsOutput, nil
	}
	return &elasticache.DescribeReplicationGroupsOutput{}, nil
}

func (f *fakeElastiCacheCR) DescribeCacheSubnetGroups(_ context.Context, _ *elasticache.DescribeCacheSubnetGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
	if f.subnetGroupsErr != nil {
		return nil, f.subnetGroupsErr
	}
	if f.subnetGroupsOutput != nil {
		return f.subnetGroupsOutput, nil
	}
	return &elasticache.DescribeCacheSubnetGroupsOutput{}, nil
}

func (f *fakeElastiCacheCR) ListTagsForResource(_ context.Context, _ *elasticache.ListTagsForResourceInput, _ ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error) {
	return &elasticache.ListTagsForResourceOutput{}, nil
}

var _ awsclient.ElastiCacheAPI = (*fakeElastiCacheCR)(nil)

// redisCacheClusterWithRGAndSubnet builds a CacheCluster RawStruct
// with the given ReplicationGroupId and CacheSubnetGroupName.
func redisCacheClusterWithRGAndSubnet(rgID, subnetGroupName string) resource.Resource {
	return resource.Resource{
		ID:   "test-redis-cluster",
		Name: "test-redis-cluster",
		Fields: map[string]string{
			"cluster_id": "test-redis-cluster",
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:       aws.String("test-redis-cluster"),
			ReplicationGroupId:   aws.String(rgID),
			CacheSubnetGroupName: aws.String(subnetGroupName),
		},
	}
}

// --- checkRedisKMS ---

// TestRelated_Redis_KMS_ARNLastSegmentExtracted verifies that checkRedisKMS extracts
// the key ID from the last "/"-delimited segment of the KmsKeyId ARN.
func TestRelated_Redis_KMS_ARNLastSegmentExtracted(t *testing.T) {
	// KmsKeyId ARN: arn:aws:kms:us-east-1:123456789012:key/mrk-aaaa-bbbb-cccc
	// Expected extracted ID: "mrk-aaaa-bbbb-cccc"
	res := redisCacheClusterWithRGAndSubnet("my-rg", "my-subnet-group")

	fake := &fakeElastiCacheCR{
		replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
			ReplicationGroups: []elasticachetypes.ReplicationGroup{
				{
					ReplicationGroupId: aws.String("my-rg"),
					KmsKeyId:           aws.String("arn:aws:kms:us-east-1:123456789012:key/mrk-aaaa-bbbb-cccc"),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ElastiCache: fake}

	checker := redisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mrk-aaaa-bbbb-cccc" {
		t.Errorf("ResourceIDs = %v, want [mrk-aaaa-bbbb-cccc]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Redis_KMS_NoKMSKey verifies that when ReplicationGroup has no KMS key
// the checker returns Count=0 (not encrypted, not unknown).
func TestRelated_Redis_KMS_NoKMSKey(t *testing.T) {
	res := redisCacheClusterWithRGAndSubnet("my-rg", "my-subnet-group")

	fake := &fakeElastiCacheCR{
		replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
			ReplicationGroups: []elasticachetypes.ReplicationGroup{
				{
					ReplicationGroupId: aws.String("my-rg"),
					KmsKeyId:           nil, // no KMS key
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ElastiCache: fake}

	checker := redisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count)
	}
}

// TestRelated_Redis_KMS_NoReplicationGroupID verifies that a CacheCluster
// with no ReplicationGroupId returns Count=-1 (cannot resolve).
func TestRelated_Redis_KMS_NoReplicationGroupID(t *testing.T) {
	res := resource.Resource{
		ID:   "standalone-cluster",
		Name: "standalone-cluster",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:     aws.String("standalone-cluster"),
			ReplicationGroupId: nil, // standalone cluster, no RG
		},
	}

	clients := &awsclient.ServiceClients{ElastiCache: &fakeElastiCacheCR{}}

	checker := redisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no ReplicationGroupId)", result.Count)
	}
}

// --- checkRedisSubnet ---

// TestRelated_Redis_Subnet_ReturnsSubnetIDs verifies that checkRedisSubnet returns
// the subnet IDs from the CacheSubnetGroup's Subnets slice.
func TestRelated_Redis_Subnet_ReturnsSubnetIDs(t *testing.T) {
	res := redisCacheClusterWithRGAndSubnet("my-rg", "my-subnet-group")

	fake := &fakeElastiCacheCR{
		subnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
			CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{
				{
					CacheSubnetGroupName: aws.String("my-subnet-group"),
					VpcId:                aws.String("vpc-aabbccdd"),
					Subnets: []elasticachetypes.Subnet{
						{SubnetIdentifier: aws.String("subnet-11111111")},
						{SubnetIdentifier: aws.String("subnet-22222222")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ElastiCache: fake}

	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs len = %d, want 2", len(result.ResourceIDs))
	}
	if result.ResourceIDs[0] != "subnet-11111111" || result.ResourceIDs[1] != "subnet-22222222" {
		t.Errorf("ResourceIDs = %v, want [subnet-11111111, subnet-22222222]", result.ResourceIDs)
	}
}

// TestRelated_Redis_Subnet_EmptySubnetList verifies that a CacheSubnetGroup with
// no subnets returns Count=0.
func TestRelated_Redis_Subnet_EmptySubnetList(t *testing.T) {
	res := redisCacheClusterWithRGAndSubnet("my-rg", "my-subnet-group")

	fake := &fakeElastiCacheCR{
		subnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
			CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{
				{
					CacheSubnetGroupName: aws.String("my-subnet-group"),
					Subnets:              []elasticachetypes.Subnet{},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ElastiCache: fake}

	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty subnet list)", result.Count)
	}
}

// TestRelated_Redis_Subnet_NoCacheSubnetGroupName verifies that a CacheCluster
// without a CacheSubnetGroupName returns Count=-1 (cannot resolve subnets).
func TestRelated_Redis_Subnet_NoCacheSubnetGroupName(t *testing.T) {
	res := resource.Resource{
		ID:   "test-cluster",
		Name: "test-cluster",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:       aws.String("test-cluster"),
			CacheSubnetGroupName: nil, // no subnet group
		},
	}

	clients := &awsclient.ServiceClients{ElastiCache: &fakeElastiCacheCR{}}

	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no CacheSubnetGroupName)", result.Count)
	}
}

// --- checkRedisVPC ---

// TestRelated_Redis_VPC_ReturnsVPCID verifies that checkRedisVPC extracts the VPC ID
// from the CacheSubnetGroup's VpcId field.
func TestRelated_Redis_VPC_ReturnsVPCID(t *testing.T) {
	res := redisCacheClusterWithRGAndSubnet("my-rg", "my-subnet-group")

	fake := &fakeElastiCacheCR{
		subnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
			CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{
				{
					CacheSubnetGroupName: aws.String("my-subnet-group"),
					VpcId:                aws.String("vpc-aabbccdd"),
					Subnets:              []elasticachetypes.Subnet{},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ElastiCache: fake}

	checker := redisCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-aabbccdd" {
		t.Errorf("ResourceIDs = %v, want [vpc-aabbccdd]", result.ResourceIDs)
	}
}

// TestRelated_Redis_VPC_EmptyVPCID verifies that when VpcId is empty the checker
// returns Count=0 (subnet group exists but no VPC ID).
func TestRelated_Redis_VPC_EmptyVPCID(t *testing.T) {
	res := redisCacheClusterWithRGAndSubnet("my-rg", "my-subnet-group")

	fake := &fakeElastiCacheCR{
		subnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
			CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{
				{
					CacheSubnetGroupName: aws.String("my-subnet-group"),
					VpcId:                aws.String(""), // empty
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ElastiCache: fake}

	checker := redisCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VpcId)", result.Count)
	}
}

// TestRelated_Redis_VPC_NoCacheSubnetGroupName verifies that a CacheCluster
// without a CacheSubnetGroupName returns Count=-1 (cannot resolve VPC).
func TestRelated_Redis_VPC_NoCacheSubnetGroupName(t *testing.T) {
	res := resource.Resource{
		ID:   "test-cluster",
		Name: "test-cluster",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:       aws.String("test-cluster"),
			CacheSubnetGroupName: nil, // no subnet group
		},
	}

	clients := &awsclient.ServiceClients{ElastiCache: &fakeElastiCacheCR{}}

	checker := redisCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no CacheSubnetGroupName)", result.Count)
	}
}
