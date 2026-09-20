package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type mockElastiCacheFullAPI struct {
	cacheClustersOutput     *elasticache.DescribeCacheClustersOutput
	cacheClustersErr        error
	replicationGroupsOutput *elasticache.DescribeReplicationGroupsOutput
	replicationGroupsErr    error
	cacheSubnetGroupsOutput *elasticache.DescribeCacheSubnetGroupsOutput
	cacheSubnetGroupsErr    error
	listTagsOutput          *elasticache.ListTagsForResourceOutput
	listTagsErr             error
}

func (m *mockElastiCacheFullAPI) DescribeCacheClusters(
	_ context.Context,
	_ *elasticache.DescribeCacheClustersInput,
	_ ...func(*elasticache.Options),
) (*elasticache.DescribeCacheClustersOutput, error) {
	if m.cacheClustersOutput == nil {
		return &elasticache.DescribeCacheClustersOutput{}, m.cacheClustersErr
	}
	return m.cacheClustersOutput, m.cacheClustersErr
}

func (m *mockElastiCacheFullAPI) DescribeReplicationGroups(
	_ context.Context,
	_ *elasticache.DescribeReplicationGroupsInput,
	_ ...func(*elasticache.Options),
) (*elasticache.DescribeReplicationGroupsOutput, error) {
	if m.replicationGroupsOutput == nil {
		return &elasticache.DescribeReplicationGroupsOutput{}, m.replicationGroupsErr
	}
	return m.replicationGroupsOutput, m.replicationGroupsErr
}

func (m *mockElastiCacheFullAPI) DescribeCacheSubnetGroups(
	_ context.Context,
	_ *elasticache.DescribeCacheSubnetGroupsInput,
	_ ...func(*elasticache.Options),
) (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
	if m.cacheSubnetGroupsOutput == nil {
		return &elasticache.DescribeCacheSubnetGroupsOutput{}, m.cacheSubnetGroupsErr
	}
	return m.cacheSubnetGroupsOutput, m.cacheSubnetGroupsErr
}

func (m *mockElastiCacheFullAPI) ListTagsForResource(
	_ context.Context,
	_ *elasticache.ListTagsForResourceInput,
	_ ...func(*elasticache.Options),
) (*elasticache.ListTagsForResourceOutput, error) {
	if m.listTagsOutput == nil {
		return &elasticache.ListTagsForResourceOutput{}, m.listTagsErr
	}
	return m.listTagsOutput, m.listTagsErr
}

func redisCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("redis") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("redis related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("redis related checker for %s not found", target)
	return nil
}

func redisGraphRoot() resource.Resource {
	return resource.Resource{
		ID:   "prod-redis-sessions",
		Name: "prod-redis-sessions",
		Fields: map[string]string{
			"cluster_id": "prod-redis-sessions",
			"arn":        "arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions",
		},
		RawStruct: elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-sessions"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"),
			Status:             aws.String("available"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r6g.large"),
			KmsKeyId:           aws.String("arn:aws:kms:us-east-1:123456789012:key/11111111-1111-1111-1111-111111111111"),
			MemberClusters:     []string{"prod-redis-sessions-001", "prod-redis-sessions-002", "prod-redis-sessions-003"},
			AuthTokenEnabled:   aws.Bool(true),
			LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{
				{
					LogType:         elasticachetypes.LogTypeSlowLog,
					DestinationType: elasticachetypes.DestinationTypeCloudWatchLogs,
					DestinationDetails: &elasticachetypes.DestinationDetails{
						CloudWatchLogsDetails: &elasticachetypes.CloudWatchLogsDestinationDetails{
							LogGroup: aws.String("/aws/elasticache/redis/prod-redis-sessions/slow-log"),
						},
					},
					Status: elasticachetypes.LogDeliveryConfigurationStatusActive,
				},
			},
		},
	}
}

func TestRelated_Redis_Registered(t *testing.T) {
	defs := resource.GetRelated("redis")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for redis")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	// The universal ct-events pivot plus the 9 type-specific targets
	// (docs/resources/redis.md).
	expected := map[string]expectation{
		"alarm":     {"CW Alarms", true},
		"cfn":       {"CloudFormation", true},
		"ct-events": {"CloudTrail Events", true},
		"kms":       {"KMS Key", true},
		"logs":      {"Log Groups", true},
		"secrets":   {"Secrets Manager", true},
		"sg":        {"Security Groups", true},
		"sns":       {"SNS Topics", true},
		"subnet":    {"Subnets", true},
		"vpc":       {"VPC", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("redis %q: Checker should not be nil", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("redis %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelated_Redis_Alarm(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "redis-cpu-alarm",
		Name:   "redis-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			Namespace: aws.String("AWS/ElastiCache"),
			AlarmName: aws.String("redis-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("CacheClusterId"),
					Value: aws.String("prod-redis-sessions-001"),
				},
			},
		},
	}
	otherAlarm := resource.Resource{
		ID: "unrelated-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Namespace:  aws.String("AWS/ElastiCache"),
			AlarmName:  aws.String("unrelated-alarm"),
			Dimensions: []cwtypes.Dimension{},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes, otherAlarm}},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, redisGraphRoot(), cache)

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1", result.Count())
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "redis-cpu-alarm" {
			found = true
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, expected to contain %q", result.ResourceIDs(), "redis-cpu-alarm")
	}
}

// A nil alarm cache (the "alarm" key absent) is not a proven zero, so the
// pivot resolves to UnknownRelated("alarm").
func TestRelated_Redis_Alarm_NilCache_ReturnsUnknown(t *testing.T) {
	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, redisGraphRoot(), resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil alarm cache is not a proven zero — canonical per docs/related-resources-engine.md §7)", result.State())
	}
}

func TestRelated_Redis_CFN(t *testing.T) {
	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			listTagsOutput: &elasticache.ListTagsForResourceOutput{
				TagList: []elasticachetypes.Tag{
					{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-prod-redis")},
					{Key: aws.String("Environment"), Value: aws.String("production")},
				},
			},
		},
	}
	cfnRes := resource.Resource{
		ID:     "acme-prod-redis",
		Name:   "acme-prod-redis",
		Fields: map[string]string{"stack_name": "acme-prod-redis"},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}

	checker := redisCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "acme-prod-redis" {
		t.Errorf("ResourceIDs = %v, want [acme-prod-redis]", result.ResourceIDs())
	}
}

func TestRelated_Redis_CtEvents(t *testing.T) {
	ctEventRes := resource.Resource{
		ID:   "abc123-evt-id",
		Name: "ModifyReplicationGroup",
		Fields: map[string]string{
			"resource_name": "prod-redis-sessions",
			"event_source":  "elasticache.amazonaws.com",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("abc123-evt-id"),
			EventName: aws.String("ModifyReplicationGroup"),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String("prod-redis-sessions"),
					ResourceType: aws.String("AWS::ElastiCache::ReplicationGroup"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{ctEventRes}},
	}

	checker := redisCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, redisGraphRoot(), cache)

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1", result.Count())
	}
}

func TestRelated_Redis_KMS(t *testing.T) {
	const keyID = "11111111-1111-1111-1111-111111111111"
	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("prod-redis-sessions"),
						KmsKeyId:           aws.String("arn:aws:kms:us-east-1:123456789012:key/" + keyID),
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, redisGraphRoot(), resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), keyID)
	}
}

func TestRelated_Redis_KMS_NoKey(t *testing.T) {
	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("staging-redis"),
						KmsKeyId:           nil,
					},
				},
			},
		},
	}

	src := resource.Resource{
		ID:   "staging-redis",
		Name: "staging-redis",
		Fields: map[string]string{
			"cluster_id": "staging-redis",
		},
		RawStruct: elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("staging-redis"),
			KmsKeyId:           nil,
		},
	}

	checker := redisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count())
	}
}

func TestRelated_Redis_Logs(t *testing.T) {
	const logGroupName = "/aws/elasticache/redis/prod-redis-sessions/slow-log"
	logRes := resource.Resource{
		ID:     logGroupName,
		Name:   logGroupName,
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	checker := redisCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != logGroupName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), logGroupName)
	}
}

func TestRelated_Redis_Logs_NoConfig(t *testing.T) {
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "/aws/elasticache/redis/other-group/slow-log"},
		}},
	}
	src := resource.Resource{
		ID:     "dev-redis-nologs",
		Name:   "dev-redis-nologs",
		Fields: map[string]string{},
		RawStruct: elasticachetypes.ReplicationGroup{
			ReplicationGroupId:        aws.String("dev-redis-nologs"),
			LogDeliveryConfigurations: nil,
		},
	}

	checker := redisCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no log delivery config)", result.Count())
	}
}

func TestRelated_Redis_Secrets_NameMatch(t *testing.T) {
	secretRes := resource.Resource{
		ID:     "prod-redis-sessions/auth-token",
		Name:   "prod-redis-sessions/auth-token",
		Fields: map[string]string{},
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod-redis-sessions/auth-token"),
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}

	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("prod-redis-sessions"),
						AuthTokenEnabled:   aws.Bool(true),
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (name-match: prod-redis-sessions/auth-token)", result.Count())
	}
}

func TestRelated_Redis_Secrets_TagMatch(t *testing.T) {
	secretRes := resource.Resource{
		ID:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:redis-auth-token",
		Name:   "redis-auth-token",
		Fields: map[string]string{},
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("redis-auth-token"),
			Tags: []smtypes.Tag{
				{Key: aws.String("elasticache:replication-group-id"), Value: aws.String("prod-redis-sessions")},
			},
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}

	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("prod-redis-sessions"),
						AuthTokenEnabled:   aws.Bool(true),
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (tag-match: elasticache:replication-group-id=prod-redis-sessions)", result.Count())
	}
}

func TestRelated_Redis_Secrets_NoMatch(t *testing.T) {
	secretRes := resource.Resource{
		ID:     "unrelated-secret",
		Name:   "unrelated-secret",
		Fields: map[string]string{},
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("unrelated-secret"),
			Tags: []smtypes.Tag{
				{Key: aws.String("Service"), Value: aws.String("rds")},
			},
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}

	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			replicationGroupsOutput: &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId: aws.String("prod-redis-sessions"),
						AuthTokenEnabled:   aws.Bool(true),
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, redisGraphRoot(), cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no tag/name match)", result.Count())
	}
}

func TestRelated_Redis_SG(t *testing.T) {
	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
				CacheClusters: []elasticachetypes.CacheCluster{
					{
						CacheClusterId: aws.String("prod-redis-sessions-001"),
						SecurityGroups: []elasticachetypes.SecurityGroupMembership{
							{SecurityGroupId: aws.String("sg-redis-prod-a"), Status: aws.String("active")},
						},
					},
				},
			},
		},
	}

	sgRes := resource.Resource{
		ID:     "sg-redis-prod-a",
		Name:   "sg-redis-prod-a",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"sg": resource.ResourceCacheEntry{Resources: []resource.Resource{sgRes}},
	}

	checker := redisCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "sg-redis-prod-a" {
		t.Errorf("ResourceIDs = %v, want [sg-redis-prod-a]", result.ResourceIDs())
	}
}

// sns rows are keyed by topic ARN; a row keyed by the bare topic name is no
// row the sns fetcher produces, so it is not counted.
func TestRelated_Redis_SNS(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:redis-ops-pager"
	const topicName = "redis-ops-pager"

	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
				CacheClusters: []elasticachetypes.CacheCluster{
					{
						CacheClusterId: aws.String("prod-redis-sessions-001"),
						NotificationConfiguration: &elasticachetypes.NotificationConfiguration{
							TopicArn:    aws.String(topicARN),
							TopicStatus: aws.String("active"),
						},
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "sns")
	for _, tc := range []struct {
		rowID string
		want  int
	}{{topicARN, 1}, {topicName, 0}} {
		snsRes := resource.Resource{
			ID:     tc.rowID,
			Name:   topicName,
			Fields: map[string]string{"arn": topicARN},
		}
		cache := resource.ResourceCache{
			"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{snsRes}},
		}
		result := checker(context.Background(), clients, redisGraphRoot(), cache)
		if result.Count() != tc.want {
			t.Errorf("sns row %q: Count = %d, want %d", tc.rowID, result.Count(), tc.want)
		}
	}
}

func TestRelated_Redis_Subnet(t *testing.T) {
	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
				CacheClusters: []elasticachetypes.CacheCluster{
					{
						CacheClusterId:       aws.String("prod-redis-sessions-001"),
						CacheSubnetGroupName: aws.String("prod-redis-subnet-group"),
					},
				},
			},
			cacheSubnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
				CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{
					{
						CacheSubnetGroupName: aws.String("prod-redis-subnet-group"),
						VpcId:                aws.String("vpc-prod-main"),
						Subnets: []elasticachetypes.Subnet{
							{SubnetIdentifier: aws.String("subnet-prod-a")},
							{SubnetIdentifier: aws.String("subnet-prod-b")},
						},
					},
				},
			},
		},
	}

	subnetA := resource.Resource{ID: "subnet-prod-a", Fields: map[string]string{}}
	subnetB := resource.Resource{ID: "subnet-prod-b", Fields: map[string]string{}}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{subnetA, subnetB}},
	}

	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, redisGraphRoot(), cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
}

func TestRelated_Redis_VPC(t *testing.T) {
	clients := &awsclient.ServiceClients{
		ElastiCache: &mockElastiCacheFullAPI{
			cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
				CacheClusters: []elasticachetypes.CacheCluster{
					{
						CacheClusterId:       aws.String("prod-redis-sessions-001"),
						CacheSubnetGroupName: aws.String("prod-redis-subnet-group"),
					},
				},
			},
			cacheSubnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
				CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{
					{
						CacheSubnetGroupName: aws.String("prod-redis-subnet-group"),
						VpcId:                aws.String("vpc-prod-main"),
						Subnets: []elasticachetypes.Subnet{
							{SubnetIdentifier: aws.String("subnet-prod-a")},
						},
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, redisGraphRoot(), resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpc-prod-main" {
		t.Errorf("ResourceIDs = %v, want [vpc-prod-main]", result.ResourceIDs())
	}
}

func prodRedisSessionsRG() resource.Resource {
	return resource.Resource{
		ID:   "prod-redis-sessions",
		Name: "prod-redis-sessions",
		Fields: map[string]string{
			"cluster_id": "prod-redis-sessions",
			"arn":        "arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions",
		},
		RawStruct: elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-sessions"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"),
			Status:             aws.String("available"),
		},
	}
}

// prodRedisSessionsSubRG returns a resource for "prod-redis-sessions-sessions"
// (a different RG whose name contains "prod-redis-sessions" as a substring).
func prodRedisSessionsSubRG() resource.Resource {
	return resource.Resource{
		ID:   "prod-redis-sessions-sessions",
		Name: "prod-redis-sessions-sessions",
		Fields: map[string]string{
			"cluster_id": "prod-redis-sessions-sessions",
			"arn":        "arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions-sessions",
		},
		RawStruct: elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-sessions-sessions"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions-sessions"),
			Status:             aws.String("available"),
		},
	}
}

func TestRelated_Redis_CtEvents_ExactIDMatch(t *testing.T) {
	ctEvent := resource.Resource{
		ID:   "evt-exact-id",
		Name: "ModifyReplicationGroup",
		Fields: map[string]string{
			"event_source": "elasticache.amazonaws.com",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-exact-id"),
			EventName: aws.String("ModifyReplicationGroup"),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String("prod-redis-sessions"),
					ResourceType: aws.String("AWS::ElastiCache::ReplicationGroup"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{ctEvent}},
	}

	checker := redisCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, prodRedisSessionsRG(), cache)

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1 (exact ResourceName == rgID should match)", result.Count())
	}
}

func TestRelated_Redis_CtEvents_ARNMatch(t *testing.T) {
	const rgARN = "arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"
	ctEvent := resource.Resource{
		ID:   "evt-arn-match",
		Name: "DescribeReplicationGroups",
		Fields: map[string]string{
			"event_source": "elasticache.amazonaws.com",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-arn-match"),
			EventName: aws.String("DescribeReplicationGroups"),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String(rgARN),
					ResourceType: aws.String("AWS::ElastiCache::ReplicationGroup"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{ctEvent}},
	}

	checker := redisCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, prodRedisSessionsRG(), cache)

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1 (ResourceName == RG ARN should match)", result.Count())
	}
}

// A ResourceName matches exactly: the "prod-redis-sessions" RG must not
// claim an event naming "prod-redis-sessions-sessions".
func TestRelated_Redis_CtEvents_SubstringDoesNotOvermatch(t *testing.T) {
	ctEvent := resource.Resource{
		ID:   "evt-sub-name",
		Name: "ModifyReplicationGroup",
		Fields: map[string]string{
			"event_source": "elasticache.amazonaws.com",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-sub-name"),
			EventName: aws.String("ModifyReplicationGroup"),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String("prod-redis-sessions-sessions"),
					ResourceType: aws.String("AWS::ElastiCache::ReplicationGroup"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{ctEvent}},
	}

	checker := redisCheckerByTarget(t, "ct-events")

	resultShort := checker(context.Background(), nil, prodRedisSessionsRG(), cache)
	if resultShort.Count() != 0 {
		t.Errorf("prod-redis-sessions: Count = %d, want 0 (substring overmatch — event names a different RG)", resultShort.Count())
	}

	resultLong := checker(context.Background(), nil, prodRedisSessionsSubRG(), cache)
	if resultLong.Count() < 1 {
		t.Errorf("prod-redis-sessions-sessions: Count = %d, want >= 1 (exact match)", resultLong.Count())
	}
}

// An elasticache.amazonaws.com EventSource alone does not tie an event to
// this RG.
func TestRelated_Redis_CtEvents_ElastiCacheSourceAloneDoesNotMatch(t *testing.T) {
	ctEvent := resource.Resource{
		ID:   "evt-other-rg",
		Name: "CreateReplicationGroup",
		Fields: map[string]string{
			"event_source": "elasticache.amazonaws.com",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId:     aws.String("evt-other-rg"),
			EventName:   aws.String("CreateReplicationGroup"),
			EventSource: aws.String("elasticache.amazonaws.com"),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String("completely-different-rg"),
					ResourceType: aws.String("AWS::ElastiCache::ReplicationGroup"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{ctEvent}},
	}

	checker := redisCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, prodRedisSessionsRG(), cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (EventSource alone must not match — requires ResourceName equality)", result.Count())
	}
}

// kms and vpc are field-only checkers that scan no target cache.
func TestRelated_Redis_Registration_KMSVPCNoTargetCache(t *testing.T) {
	wantNeedsCache := map[string]bool{
		"alarm":     true,
		"cfn":       true,
		"ct-events": true,
		"kms":       false,
		"logs":      true,
		"secrets":   true,
		"sg":        true,
		"sns":       true,
		"subnet":    true,
		"vpc":       false,
	}

	defs := resource.GetRelated("redis")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for redis")
	}

	byTarget := make(map[string]resource.RelatedDef, len(defs))
	for _, d := range defs {
		byTarget[d.TargetType] = d
	}

	for target, wantCache := range wantNeedsCache {
		d, found := byTarget[target]
		if !found {
			t.Errorf("expected related def for target %q not found in registration", target)
			continue
		}
		if d.NeedsTargetCache != wantCache {
			t.Errorf("redis %q: NeedsTargetCache = %v, want %v", target, d.NeedsTargetCache, wantCache)
		}
	}
}

// A truncated target cache makes the result Truncated: the count may be
// understated.

func TestRelated_Redis_Alarm_TruncatedCacheWithMatches_ReturnsTruncated(t *testing.T) {
	matchingAlarm := resource.Resource{
		ID:     "matching-alarm",
		Name:   "matching-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			Namespace: aws.String("AWS/ElastiCache"),
			AlarmName: aws.String("matching-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("CacheClusterId"),
					Value: aws.String("prod-redis-sessions-001"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingAlarm},
			IsTruncated: true,
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache must propagate Truncated flag)")
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "matching-alarm" {
			found = true
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, expected to contain %q", result.ResourceIDs(), "matching-alarm")
	}
}

func TestRelated_Redis_Alarm_TruncatedCacheNoMatches_ReturnsTruncatedResult(t *testing.T) {
	noMatchAlarm := resource.Resource{
		ID:     "unrelated-alarm",
		Name:   "unrelated-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("unrelated-alarm"),
			Dimensions: []cwtypes.Dimension{},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{noMatchAlarm},
			IsTruncated: true,
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, redisGraphRoot(), cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no dimension match)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache, no matches must still set Truncated)")
	}
}

func TestRelated_Redis_Logs_TruncatedCacheWithMatches_ReturnsTruncated(t *testing.T) {
	const logGroupName = "/aws/elasticache/redis/prod-redis-sessions/slow-log"
	logRes := resource.Resource{
		ID:     logGroupName,
		Name:   logGroupName,
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{logRes},
			IsTruncated: true,
		},
	}

	checker := redisCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, redisGraphRoot(), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache must propagate Truncated flag)")
	}
}
