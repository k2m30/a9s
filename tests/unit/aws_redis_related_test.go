package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

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

// --- Navigable Fields ---

func TestNavigableFields_Redis_SecurityGroup(t *testing.T) {
	nav := resource.IsFieldNavigable("redis", "SecurityGroups.SecurityGroupId")
	if nav == nil {
		t.Fatal("expected SecurityGroups.SecurityGroupId to be navigable for redis")
	}
	if nav.TargetType != "sg" {
		t.Errorf("expected TargetType=sg, got %q", nav.TargetType)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, CacheClusterId dimension) ---

func TestRelated_Redis_Alarms_Found(t *testing.T) {
	const clusterID = "acme-prod-sessions"

	alarmRes := resource.Resource{
		ID: "redis-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("redis-cpu-high"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterID,
		Name: clusterID,
		Fields: map[string]string{
			"cluster_id": clusterID,
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String(clusterID),
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "redis-cpu-high" {
		t.Errorf("ResourceIDs = %v, want [redis-cpu-high]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Redis_Alarms_NotFound(t *testing.T) {
	const clusterID = "acme-prod-sessions"

	alarmRes := resource.Resource{
		ID: "other-cluster-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-cluster-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("CacheClusterId"), Value: aws.String("different-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterID,
		Name: clusterID,
		Fields: map[string]string{
			"cluster_id": clusterID,
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String(clusterID),
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Redis_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		Fields: map[string]string{
			"cluster_id": "acme-prod-sessions",
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- redis→cfn: unknown (Count: -1) because DescribeCacheClusters does not
// return tags — ListTagsForResource is required to determine CFN ownership and
// the fetcher does not make that per-cluster call. ---

// TestRelated_Redis_CFN_UnknownWithoutTags verifies the checker returns Count=-1
// (unknown) regardless of whether there are CFN stacks in cache, because the
// CacheCluster RawStruct has no Tags field to read aws:cloudformation:stack-name
// from. Real behavior: we cannot deterministically decide relatedness.
func TestRelated_Redis_CFN_UnknownWithoutTags(t *testing.T) {
	checker := redisCheckerByTarget(t, "cfn")

	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
		},
	}

	// With a populated cfn cache we still cannot answer (no tags available).
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "some-stack", Name: "some-stack", Fields: map[string]string{"stack_name": "some-stack"}},
		}},
	}
	result := checker(context.Background(), nil, source, cache)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — tags not fetched)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}

	// With an empty cache the answer is still unknown (not zero).
	result2 := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result2.Count != -1 {
		t.Errorf("empty cache: Count = %d, want -1", result2.Count)
	}
}

// --- Security Group checker (Pattern F — reads SecurityGroups from CacheCluster RawStruct) ---

func TestRelated_Redis_SG_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String("sg-abc123")},
				{SecurityGroupId: aws.String("sg-def456")},
			},
		},
	}

	checker := redisCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["sg-abc123"] || !found["sg-def456"] {
		t.Errorf("ResourceIDs = %v, want [sg-abc123, sg-def456]", result.ResourceIDs)
	}
}

func TestRelated_Redis_SG_EmptyList(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{},
		},
	}

	checker := redisCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no security groups)", result.Count)
	}
}

func TestRelated_Redis_SG_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-prod-sessions",
		RawStruct: "not-a-cache-cluster",
	}

	checker := redisCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- Log Groups checker (Pattern F+C — LogDeliveryConfigurations from RawStruct, cache lookup) ---

func TestRelated_Redis_Logs_MatchByLogGroup(t *testing.T) {
	const logGroupName = "/elasticache/acme-sessions"
	logRes := resource.Resource{ID: logGroupName, Name: logGroupName}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
			LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{
				{
					DestinationType: elasticachetypes.DestinationTypeCloudWatchLogs,
					DestinationDetails: &elasticachetypes.DestinationDetails{
						CloudWatchLogsDetails: &elasticachetypes.CloudWatchLogsDestinationDetails{
							LogGroup: aws.String(logGroupName),
						},
					},
				},
			},
		},
	}

	checker := redisCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != logGroupName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, logGroupName)
	}
}

func TestRelated_Redis_Logs_NoLogDeliveryConfig(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:            aws.String("acme-prod-sessions"),
			LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{},
		},
	}

	checker := redisCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no log delivery configurations)", result.Count)
	}
}

func TestRelated_Redis_Logs_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-prod-sessions",
		RawStruct: "not-a-cache-cluster",
	}

	checker := redisCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for invalid RawStruct (no log delivery)", result.Count)
	}
}

// --- SNS checker (Pattern F+C — NotificationConfiguration.TopicArn, cache lookup) ---

func TestRelated_Redis_SNS_MatchByTopicName(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:elasticache-events"
	const topicName = "elasticache-events"
	snsRes := resource.Resource{ID: topicName, Name: topicName}
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{snsRes}},
	}
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
			NotificationConfiguration: &elasticachetypes.NotificationConfiguration{
				TopicArn: aws.String(topicARN),
			},
		},
	}

	checker := redisCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != topicName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, topicName)
	}
}

func TestRelated_Redis_SNS_NoNotificationConfig(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:            aws.String("acme-prod-sessions"),
			NotificationConfiguration: nil,
		},
	}

	checker := redisCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no notification config)", result.Count)
	}
}

func TestRelated_Redis_SNS_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-prod-sessions",
		RawStruct: "not-a-cache-cluster",
	}

	checker := redisCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for invalid RawStruct (sns checker returns 0)", result.Count)
	}
}

// --- Secrets checker (Pattern C — returns deterministic 0 once RG fetched, -1 without clients) ---

func TestRelated_Redis_Secrets_NilClients(t *testing.T) {
	// checkRedisSecrets calls redisReplicationGroup which needs a ServiceClients.
	// Without clients it returns -1.
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:     aws.String("acme-prod-sessions"),
			ReplicationGroupId: aws.String("acme-prod-rg"),
		},
	}

	checker := redisCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients, cannot fetch replication group)", result.Count)
	}
}

func TestRelated_Redis_Secrets_NoReplicationGroupID(t *testing.T) {
	// Without a ReplicationGroupId, redisReplicationGroup returns nil → Count -1.
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId:     aws.String("acme-prod-sessions"),
			ReplicationGroupId: nil,
		},
	}

	checker := redisCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no replication group ID)", result.Count)
	}
}
