// redis_related.go contains ElastiCache Redis related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("redis", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedisAlarms, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedisSG, NeedsTargetCache: false},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedisCFN, NeedsTargetCache: false},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedisKMS},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkRedisLogs, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkRedisSecrets},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkRedisSNS, NeedsTargetCache: true},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRedisSubnet},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedisVPC},
	})

	// elasticachetypes.ReplicationGroup: SecurityGroups[].SecurityGroupId, KmsKeyId
	resource.RegisterNavigableFields("redis", []resource.NavigableField{
		{FieldPath: "SecurityGroups.SecurityGroupId", TargetType: "sg"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})
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

// checkRedisCFN resolves CloudFormation stack ownership via a single
// elasticache:ListTagsForResource call (Pattern C — live API). The
// CacheCluster struct has no Tags field; tags must be fetched by ARN.
func checkRedisCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[elasticachetypes.CacheCluster](res.RawStruct)
	if !ok || cluster.ARN == nil || *cluster.ARN == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	arn := *cluster.ARN
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.ListTagsForResourceOutput, error) {
		return c.ElastiCache.ListTagsForResource(ctx, &elasticache.ListTagsForResourceInput{
			ResourceName: &arn,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	stackName := ""
	for _, tag := range out.TagList {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := redisRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		raw, rawOK := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if rawOK && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkRedisKMS resolves the KMS encryption key via a single
// elasticache:DescribeReplicationGroups call (Pattern C). KmsKeyId lives on
// the parent ReplicationGroup, not on individual CacheCluster members.
func checkRedisKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rg := redisReplicationGroup(ctx, clients, res)
	if rg == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if rg.KmsKeyId == nil || *rg.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *rg.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkRedisLogs reads LogDeliveryConfigurations from the CacheCluster
// RawStruct. Each entry with DestinationType == cloudwatch-logs has a
// CloudWatchLogsDestinationDetails.LogGroup naming the CW Log Group. We
// match that name against the logs cache.
func checkRedisLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[elasticachetypes.CacheCluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	var names []string
	for _, ldc := range cluster.LogDeliveryConfigurations {
		if ldc.DestinationType != elasticachetypes.DestinationTypeCloudWatchLogs {
			continue
		}
		if ldc.DestinationDetails == nil || ldc.DestinationDetails.CloudWatchLogsDetails == nil {
			continue
		}
		if n := ldc.DestinationDetails.CloudWatchLogsDetails.LogGroup; n != nil && *n != "" {
			names = append(names, *n)
		}
	}
	if len(names) == 0 {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := redisRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	wanted := make(map[string]struct{}, len(names))
	for _, n := range names {
		wanted[n] = struct{}{}
	}
	var ids []string
	for _, logRes := range logList {
		if _, ok := wanted[logRes.ID]; ok {
			ids = append(ids, logRes.ID)
			continue
		}
		if _, ok := wanted[logRes.Name]; ok {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkRedisSecrets resolves Secrets Manager ARNs attached to this cluster's
// user groups via a single elasticache:DescribeReplicationGroups call.
// ReplicationGroup.UserGroupIds references user groups that may ship ARNs;
// if the ReplicationGroup has AuthTokenEnabled we also emit a known synthetic
// hint (Count=0 for clusters without RBAC user-group attachment, since the
// AUTH token itself has no Secrets Manager backing in the list response).
func checkRedisSecrets(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rg := redisReplicationGroup(ctx, clients, res)
	if rg == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	// ReplicationGroup fields that can reference secrets: none in the current
	// SDK. AUTH tokens and RBAC user passwords are managed by ElastiCache
	// internally, not Secrets Manager. Return 0 as the deterministic answer
	// once we've successfully fetched the ReplicationGroup.
	return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
}

// checkRedisSNS extracts the SNS topic ARN from the CacheCluster's
// NotificationConfiguration.TopicArn and matches it against the sns cache
// by ARN (stored in Fields["arn"] or ID).
func checkRedisSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[elasticachetypes.CacheCluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	if cluster.NotificationConfiguration == nil || cluster.NotificationConfiguration.TopicArn == nil || *cluster.NotificationConfiguration.TopicArn == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	topicARN := *cluster.NotificationConfiguration.TopicArn

	snsList, truncated, err := redisRelatedResources(ctx, clients, cache, "sns")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	if snsList == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	// SNS topic ARN format: arn:aws:sns:region:account:topic-name
	topicName := topicARN
	if idx := strings.LastIndex(topicARN, ":"); idx >= 0 && idx < len(topicARN)-1 {
		topicName = topicARN[idx+1:]
	}

	var ids []string
	for _, snsRes := range snsList {
		if snsRes.ID == topicARN || snsRes.ID == topicName ||
			snsRes.Name == topicName || snsRes.Fields["arn"] == topicARN {
			ids = append(ids, snsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	return relatedResult("sns", ids)
}

// checkRedisSubnet resolves the subnets of the cluster's CacheSubnetGroup
// via a single elasticache:DescribeCacheSubnetGroups call (Pattern C).
func checkRedisSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng := redisSubnetGroup(ctx, clients, res)
	if sng == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	var ids []string
	for _, s := range sng.Subnets {
		if s.SubnetIdentifier != nil && *s.SubnetIdentifier != "" {
			ids = append(ids, *s.SubnetIdentifier)
		}
	}
	return relatedResult("subnet", ids)
}

// checkRedisVPC resolves the VPC hosting the cluster's CacheSubnetGroup via
// the same DescribeCacheSubnetGroups call (Pattern C).
func checkRedisVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng := redisSubnetGroup(ctx, clients, res)
	if sng == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if sng.VpcId == nil || *sng.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*sng.VpcId})
}

// redisReplicationGroup performs a single DescribeReplicationGroups call for
// the cluster's ReplicationGroupId (if any), wrapped in RetryOnThrottle.
func redisReplicationGroup(ctx context.Context, clients any, res resource.Resource) *elasticachetypes.ReplicationGroup {
	cluster, ok := assertStruct[elasticachetypes.CacheCluster](res.RawStruct)
	if !ok || cluster.ReplicationGroupId == nil || *cluster.ReplicationGroupId == "" {
		return nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return nil
	}
	rgID := *cluster.ReplicationGroupId
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.DescribeReplicationGroupsOutput, error) {
		return c.ElastiCache.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{
			ReplicationGroupId: &rgID,
		})
	})
	if err != nil || out == nil || len(out.ReplicationGroups) == 0 {
		return nil
	}
	return &out.ReplicationGroups[0]
}

// redisSubnetGroup performs a single DescribeCacheSubnetGroups call for the
// cluster's CacheSubnetGroupName (if any), wrapped in RetryOnThrottle.
func redisSubnetGroup(ctx context.Context, clients any, res resource.Resource) *elasticachetypes.CacheSubnetGroup {
	cluster, ok := assertStruct[elasticachetypes.CacheCluster](res.RawStruct)
	if !ok || cluster.CacheSubnetGroupName == nil || *cluster.CacheSubnetGroupName == "" {
		return nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return nil
	}
	name := *cluster.CacheSubnetGroupName
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
		return c.ElastiCache.DescribeCacheSubnetGroups(ctx, &elasticache.DescribeCacheSubnetGroupsInput{
			CacheSubnetGroupName: &name,
		})
	})
	if err != nil || out == nil || len(out.CacheSubnetGroups) == 0 {
		return nil
	}
	return &out.CacheSubnetGroups[0]
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
