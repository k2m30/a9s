// redis_related.go contains ElastiCache Redis related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("redis", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedisAlarms, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedisCFN, NeedsTargetCache: false},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedisSG, NeedsTargetCache: false},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedisVPC},
	})

	// elasticachetypes.ReplicationGroup: SecurityGroups[].SecurityGroupId, KmsKeyId
	resource.RegisterNavigableFields("redis", []resource.NavigableField{
		{FieldPath: "SecurityGroups.SecurityGroupId", TargetType: "sg"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})
}

// checkRedisCFN returns Count: 0 because ElastiCache replication group tags are
// not included in the DescribeReplicationGroups list response — the CFN
// relationship cannot be determined from cache alone.
func checkRedisCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkRedisAlarms checks the cache for CloudWatch alarms with CacheClusterId dimension matching this cluster's ID.
func checkRedisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := redisRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "CacheClusterId" && d.Value != nil && *d.Value == clusterID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkRedisSG returns the security groups associated with this Redis cache cluster (Pattern F).
// It reads SecurityGroups[].SecurityGroupId from the CacheCluster RawStruct.
func checkRedisSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[elasticachetypes.CacheCluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, sg := range cluster.SecurityGroups {
		if sg.SecurityGroupId != nil && *sg.SecurityGroupId != "" {
			ids = append(ids, *sg.SecurityGroupId)
		}
	}
	return relatedResult("sg", ids)
}

// redisRelatedResources returns the resource list for target from cache or by fetching the first page.
func redisRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkRedisVPC — ElastiCache replication group has no direct VPC field
// on the list response. VPC is on the cache subnet group. Stub for now.
func checkRedisVPC(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
}
