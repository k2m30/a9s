// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// redis_related.go contains ElastiCache Redis related-resource checker functions.
// The resource row represents a single elasticachetypes.ReplicationGroup (list API:
// DescribeReplicationGroups). Checkers that need fields only on individual member
// clusters (SG, SNS, SubnetGroup) call DescribeCacheClusters on MemberClusters[0].
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkRedisAlarms checks the alarm cache for CloudWatch alarms with a
// CacheClusterId dimension matching any member cluster of this replication group.
func checkRedisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	var memberSet map[string]struct{}
	var rgID string

	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if ok {
		memberSet = make(map[string]struct{}, len(rg.MemberClusters))
		for _, m := range rg.MemberClusters {
			memberSet[m] = struct{}{}
		}
		if rg.ReplicationGroupId != nil {
			rgID = *rg.ReplicationGroupId
		}
	} else {
		// Fall back to resource ID — may still match ReplicationGroupId dimension.
		rgID = res.ID
	}

	if rgID == "" && len(memberSet) == 0 {
		return resource.KnownRelated("alarm", nil, false)
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name == nil || d.Value == nil {
				continue
			}
			if *d.Name == "CacheClusterId" && memberSet != nil {
				if _, ok := memberSet[*d.Value]; ok {
					ids = append(ids, alarmRes.ID)
					break
				}
			}
			if *d.Name == "ReplicationGroupId" && rgID != "" && *d.Value == rgID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("alarm", nil, true)
	}
	if truncated {
		return truncatedResultRedis("alarm", ids)
	}
	return relatedResult("alarm", ids)
}

// checkRedisCFN resolves CloudFormation stack ownership via a single
// elasticache:ListTagsForResource call on the replication group ARN.
// The aws:cloudformation:stack-name tag is the only reliable IaC-ownership pivot.
func checkRedisCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok || rg.ARN == nil || *rg.ARN == "" {
		// Without the ARN we cannot call ListTagsForResource to find the CFN stack.
		// Return 0 rather than -1 so we do not signal an error condition.
		return resource.KnownRelated("cfn", nil, false)
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return resource.UnknownRelated("cfn")
	}
	arn := *rg.ARN
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.ListTagsForResourceOutput, error) {
		return c.ElastiCache.ListTagsForResource(ctx, &elasticache.ListTagsForResourceInput{
			ResourceName: &arn,
		})
	})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	stackName := ""
	for _, tag := range out.TagList {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.KnownRelated("cfn", nil, false)
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return relatedResultTrunc("cfn", nil, true)
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
		return relatedResultTrunc("cfn", nil, true)
	}
	if truncated {
		return truncatedResultRedis("cfn", ids)
	}
	return relatedResult("cfn", ids)
}

// checkRedisCtEvents scans the ct-events cache for CloudTrail events whose
// ResourceName exactly matches the replication group ID or ARN.
// Substring matching is intentionally avoided (P2-2): "prod-redis" would
// otherwise match events for "prod-redis-sessions". The EventSource fallback
// is also removed — it matched ElastiCache events for every RG on the account.
func checkRedisCtEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	var rgID, rgARN string
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if ok {
		if rg.ReplicationGroupId != nil {
			rgID = *rg.ReplicationGroupId
		}
		if rg.ARN != nil {
			rgARN = *rg.ARN
		}
	} else {
		// Fall back to resource ID — may still match ResourceName in ct-events.
		rgID = res.ID
	}
	if rgID == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}

	evList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if evList == nil {
		return relatedResultTrunc("ct-events", nil, true)
	}

	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		matched := false
		// Exact match only: ResourceName == rgID or ResourceName == rgARN.
		// This prevents "prod-redis" from matching events scoped to "prod-redis-sessions".
		for _, r := range ev.Resources {
			name := strings.TrimSpace(strings.ToLower(aws.ToString(r.ResourceName)))
			if name == strings.ToLower(rgID) || (rgARN != "" && name == strings.ToLower(rgARN)) {
				matched = true
				break
			}
		}
		if matched {
			ids = append(ids, evRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("ct-events", nil, true)
	}
	if truncated {
		return truncatedResultRedis("ct-events", ids)
	}
	return relatedResult("ct-events", ids)
}

// checkRedisKMS reads KmsKeyId directly from the ReplicationGroup RawStruct.
// No extra API call required — KmsKeyId is on the list-response struct.
func checkRedisKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok {
		// Without RawStruct we cannot read KmsKeyId — report 0 (no known key).
		return resource.KnownRelated("kms", nil, false)
	}
	if rg.KmsKeyId == nil || *rg.KmsKeyId == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := kmsKeyIDFromField(*rg.KmsKeyId, res.Type)
	return relatedResult("kms", []string{keyID})
}

// checkRedisLogs reads LogDeliveryConfigurations directly from the ReplicationGroup
// RawStruct. Entries with DestinationType == cloudwatch-logs have a
// CloudWatchLogsDetails.LogGroup that is matched against the logs cache.
func checkRedisLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok {
		return resource.KnownRelated("logs", nil, false)
	}
	var names []string
	for _, ldc := range rg.LogDeliveryConfigurations {
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
		return resource.KnownRelated("logs", nil, false)
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return relatedResultTrunc("logs", nil, true)
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
		return relatedResultTrunc("logs", nil, true)
	}
	if truncated {
		return truncatedResultRedis("logs", ids)
	}
	return relatedResult("logs", ids)
}

// checkRedisSecrets scans the loaded secrets cache for secrets whose name
// matches "<rgID>/auth-token" OR that carry the tag
// "elasticache:replication-group-id=<rgID>". Best-effort; may return zero
// when no tag/naming convention is followed (allowed per spec §2).
func checkRedisSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	var rgID string
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if ok {
		if rg.ReplicationGroupId != nil {
			rgID = *rg.ReplicationGroupId
		}
	} else {
		// Fall back to resource ID for naming-convention match.
		rgID = res.ID
	}
	if rgID == "" {
		return resource.KnownRelated("secrets", nil, false)
	}

	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return relatedResultTrunc("secrets", nil, true)
	}

	namingConvention := rgID + "/auth-token"
	var ids []string
	for _, secRes := range secretList {
		// Name-based match.
		if secRes.ID == namingConvention || secRes.Name == namingConvention {
			ids = append(ids, secRes.ID)
			continue
		}
		// Tag-based match via RawStruct.
		entry, ok := assertStruct[smtypes.SecretListEntry](secRes.RawStruct)
		if !ok {
			continue
		}
		for _, tag := range entry.Tags {
			if tag.Key != nil && *tag.Key == "elasticache:replication-group-id" &&
				tag.Value != nil && *tag.Value == rgID {
				ids = append(ids, secRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("secrets", nil, true)
	}
	if truncated {
		return truncatedResultRedis("secrets", ids)
	}
	return relatedResult("secrets", ids)
}

// checkRedisSG resolves the security groups for the replication group by calling
// DescribeCacheClusters on MemberClusters[0] and reading SecurityGroups[].
// All members share the same SG set, so one call is sufficient.
func checkRedisSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cc := redisMemberCluster(ctx, clients, res)
	if cc == nil {
		// redisMemberCluster collapses "no members", "wrong client type", and
		// "DescribeCacheClusters failed" into nil — we cannot tell which, so
		// this is unresolved, not a proven zero (mirrors dbc_related.go's
		// checkDbcSubnet/checkDbcVPC for the same two-hop shape).
		return resource.UnknownRelated("sg")
	}
	sgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sg")
	if err != nil {
		return resource.ErrorRelated("sg", err)
	}

	var sgIDs []string
	for _, sg := range cc.SecurityGroups {
		if sg.SecurityGroupId != nil && *sg.SecurityGroupId != "" {
			sgIDs = append(sgIDs, *sg.SecurityGroupId)
		}
	}
	if len(sgIDs) == 0 {
		return resource.KnownRelated("sg", nil, false)
	}

	wantedSet := make(map[string]struct{}, len(sgIDs))
	for _, id := range sgIDs {
		wantedSet[id] = struct{}{}
	}
	var ids []string
	for _, sgRes := range sgList {
		if _, ok := wantedSet[sgRes.ID]; ok {
			ids = append(ids, sgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("sg", nil, true)
	}
	if truncated {
		return truncatedResultRedis("sg", ids)
	}
	return relatedResult("sg", ids)
}

// checkRedisSNS extracts the SNS topic ARN from the member cluster's
// NotificationConfiguration.TopicArn and matches it against the sns cache.
// Uses the same DescribeCacheClusters call as checkRedisSG.
func checkRedisSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cc := redisMemberCluster(ctx, clients, res)
	if cc == nil {
		// See checkRedisSG: redisMemberCluster's nil is unresolved, not proven.
		return resource.UnknownRelated("sns")
	}
	if cc.NotificationConfiguration == nil || cc.NotificationConfiguration.TopicArn == nil || *cc.NotificationConfiguration.TopicArn == "" {
		return resource.KnownRelated("sns", nil, false)
	}
	topicARN := *cc.NotificationConfiguration.TopicArn

	snsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns")
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	if snsList == nil {
		return relatedResultTrunc("sns", nil, true)
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
		return relatedResultTrunc("sns", nil, true)
	}
	if truncated {
		return truncatedResultRedis("sns", ids)
	}
	return relatedResult("sns", ids)
}

// checkRedisSubnet resolves the subnets for the replication group by calling
// DescribeCacheClusters on MemberClusters[0] to get CacheSubnetGroupName, then
// calling DescribeCacheSubnetGroups to read the individual subnet IDs.
func checkRedisSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sng := redisSubnetGroup(ctx, clients, res)
	if sng == nil {
		// redisSubnetGroup collapses "no member cluster", "no subnet group
		// name", and "DescribeCacheSubnetGroups failed" into nil — we cannot
		// tell which, so this is unresolved, not a proven zero (mirrors
		// dbc_related.go's checkDbcSubnet for the same two-hop shape).
		return resource.UnknownRelated("subnet")
	}

	subnetList, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.ErrorRelated("subnet", err)
	}

	var subnetIDs []string
	for _, s := range sng.Subnets {
		if s.SubnetIdentifier != nil && *s.SubnetIdentifier != "" {
			subnetIDs = append(subnetIDs, *s.SubnetIdentifier)
		}
	}
	if len(subnetIDs) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}

	wantedSet := make(map[string]struct{}, len(subnetIDs))
	for _, id := range subnetIDs {
		wantedSet[id] = struct{}{}
	}
	var ids []string
	for _, subRes := range subnetList {
		if _, ok := wantedSet[subRes.ID]; ok {
			ids = append(ids, subRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("subnet", nil, true)
	}
	if truncated {
		return truncatedResultRedis("subnet", ids)
	}
	return relatedResult("subnet", ids)
}

// checkRedisVPC resolves the VPC for the replication group via the same
// DescribeCacheSubnetGroups call used by checkRedisSubnet.
func checkRedisVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng := redisSubnetGroup(ctx, clients, res)
	if sng == nil {
		// See checkRedisSubnet: redisSubnetGroup's nil is unresolved, not proven.
		return resource.UnknownRelated("vpc")
	}
	if sng.VpcId == nil || *sng.VpcId == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{*sng.VpcId})
}

// redisMemberCluster calls DescribeCacheClusters on MemberClusters[0] of the
// replication group. Returns nil when the RG has no members or the call fails.
// SG, SNS, and SubnetGroup data all live on the member cluster struct.
func redisMemberCluster(ctx context.Context, clients any, res resource.Resource) *elasticachetypes.CacheCluster {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok || len(rg.MemberClusters) == 0 {
		return nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return nil
	}
	memberID := rg.MemberClusters[0]
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.DescribeCacheClustersOutput, error) {
		return c.ElastiCache.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
			CacheClusterId: &memberID,
		})
	})
	if err != nil || out == nil || len(out.CacheClusters) == 0 {
		return nil
	}
	cc := out.CacheClusters[0]
	return &cc
}

// redisSubnetGroup performs the two-step resolution:
// 1. DescribeCacheClusters(MemberClusters[0]) → CacheSubnetGroupName
// 2. DescribeCacheSubnetGroups(name) → CacheSubnetGroup
func redisSubnetGroup(ctx context.Context, clients any, res resource.Resource) *elasticachetypes.CacheSubnetGroup {
	cc := redisMemberCluster(ctx, clients, res)
	if cc == nil || cc.CacheSubnetGroupName == nil || *cc.CacheSubnetGroupName == "" {
		return nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return nil
	}
	name := *cc.CacheSubnetGroupName
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
		return c.ElastiCache.DescribeCacheSubnetGroups(ctx, &elasticache.DescribeCacheSubnetGroupsInput{
			CacheSubnetGroupName: &name,
		})
	})
	if err != nil || out == nil || len(out.CacheSubnetGroups) == 0 {
		return nil
	}
	sng := out.CacheSubnetGroups[0]
	return &sng
}

// truncatedResultRedis returns a RelatedCheckResult with Truncated=true when the
// target cache is truncated and matches were found. Later pages may contain
// additional matches, so the displayed count is a lower bound — rendered as "(N+)".
func truncatedResultRedis(target string, ids []string) resource.RelatedCheckResult {
	return resource.KnownRelated(target, ids, true)
}
