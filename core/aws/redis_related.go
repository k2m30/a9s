// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// redis_related.go contains ElastiCache Redis related-resource checker functions.
// The resource row represents a single elasticachetypes.ReplicationGroup (list API:
// DescribeReplicationGroups). Checkers that need fields only on individual member
// clusters (SG, SNS, SubnetGroup) call DescribeCacheClusters on MemberClusters[0].
package aws

import (
	"context"
	"errors"
	"fmt"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// errRedisNoGroupDetail is the answer when the row holds something that is not
// a replication group, or the ElastiCache client is unusable. Distinct from
// errRawStructMissing: there the row was never read, here what we have cannot
// be used.
var errRedisNoGroupDetail = errors.New("the replication group could not be read")

// checkRedisAlarms reports the CloudWatch alarms on this replication group:
// ElastiCache publishes per-node metrics, so an alarm on any member cluster
// is an alarm on the group.
func checkRedisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "redis", res)
}

// checkRedisCFN resolves CloudFormation stack ownership via a single
// elasticache:ListTagsForResource call on the replication group ARN.
// The aws:cloudformation:stack-name tag is the only reliable IaC-ownership pivot.
func checkRedisCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok || rg.ARN == nil || *rg.ARN == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("cfn")
		}
		// A group that carries no ARN cannot be passed to ListTagsForResource,
		// so no stack can own it.
		return resource.ProvenZero("cfn", "rg.ARN")
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
		return resource.ProvenZero("cfn", "stackName")
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
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
	return relatedResultTrunc("cfn", ids, false)
}

// checkRedisKMS reads KmsKeyId directly from the ReplicationGroup RawStruct.
// No extra API call required — KmsKeyId is on the list-response struct.
func checkRedisKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok {
		// Without RawStruct we cannot read KmsKeyId — report 0 (no known key).
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.KnownRelated("kms", nil, false)
	}
	if rg.KmsKeyId == nil || *rg.KmsKeyId == "" {
		return resource.ProvenZero("kms", "rg.KmsKeyId")
	}
	keyID := kmsRefFromField(*rg.KmsKeyId, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkRedisLogs reads LogDeliveryConfigurations directly from the ReplicationGroup
// RawStruct. Entries with DestinationType == cloudwatch-logs have a
// CloudWatchLogsDetails.LogGroup that is matched against the logs cache.
func checkRedisLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("logs")
		}
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
		return resource.ProvenZero("logs", "names")
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
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
	return relatedResultTrunc("logs", ids, false)
}

// checkRedisSecrets scans the loaded secrets cache for secrets whose name
// matches "<rgID>/auth-token" OR that carry the tag
// "elasticache:replication-group-id=<rgID>". Best-effort; may return zero
// when no tag/naming convention is followed.
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
		return resource.ProvenZero("secrets", "rgID")
	}

	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return resource.UnknownRelated("secrets")
	}

	namingConvention := rgID + "/auth-token"
	var ids []string
	for _, secRes := range secretList {
		if secRes.ID == namingConvention || secRes.Name == namingConvention {
			ids = append(ids, secRes.ID)
			continue
		}
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
	return relatedResultTrunc("secrets", ids, false)
}

// checkRedisSG resolves the security groups for the replication group by calling
// DescribeCacheClusters on MemberClusters[0] and reading SecurityGroups[].
// All members share the same SG set, so one call is sufficient.
func checkRedisSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cc, err := redisMemberCluster(ctx, clients, res)
	if err != nil {
		return relatedFromErr("sg", err)
	}
	if cc == nil {
		return resource.ProvenZero("sg", "cc")
	}
	sgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sg")
	if err != nil {
		return resource.ErrorRelated("sg", err)
	}
	if sgList == nil {
		return resource.UnknownRelated("sg")
	}

	var sgIDs []string
	for _, sg := range cc.SecurityGroups {
		if sg.SecurityGroupId != nil && *sg.SecurityGroupId != "" {
			sgIDs = append(sgIDs, *sg.SecurityGroupId)
		}
	}
	if len(sgIDs) == 0 {
		return resource.ProvenZero("sg", "sgIDs")
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
	return relatedResultTrunc("sg", ids, false)
}

// checkRedisSNS extracts the SNS topic ARN from the member cluster's
// NotificationConfiguration.TopicArn and matches it against the sns cache.
// Uses the same DescribeCacheClusters call as checkRedisSG.
func checkRedisSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cc, err := redisMemberCluster(ctx, clients, res)
	if err != nil {
		return relatedFromErr("sns", err)
	}
	if cc == nil {
		return resource.ProvenZero("sns", "cc")
	}
	if cc.NotificationConfiguration == nil || cc.NotificationConfiguration.TopicArn == nil || *cc.NotificationConfiguration.TopicArn == "" {
		return resource.ProvenZero("sns", "cc.NotificationConfiguration.TopicArn")
	}
	topicARN := *cc.NotificationConfiguration.TopicArn

	snsList, _, err := relatedResourcesFor(ctx, clients, cache, "sns")
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	if snsList == nil {
		return resource.UnknownRelated("sns")
	}

	ids, lowerBound := listedRefs("sns", []string{topicARN}, refContext(clients, cache, "sns"), snsList)
	return relatedResultTrunc("sns", ids, lowerBound)
}

// checkRedisSubnet resolves the subnets for the replication group by calling
// DescribeCacheClusters on MemberClusters[0] to get CacheSubnetGroupName, then
// calling DescribeCacheSubnetGroups to read the individual subnet IDs.
func checkRedisSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sng, err := redisSubnetGroup(ctx, clients, res)
	if err != nil {
		return relatedFromErr("subnet", err)
	}
	if sng == nil {
		return resource.ProvenZero("subnet", "sng")
	}

	subnetList, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.ErrorRelated("subnet", err)
	}
	if subnetList == nil {
		return resource.UnknownRelated("subnet")
	}

	var subnetIDs []string
	for _, s := range sng.Subnets {
		if s.SubnetIdentifier != nil && *s.SubnetIdentifier != "" {
			subnetIDs = append(subnetIDs, *s.SubnetIdentifier)
		}
	}
	if len(subnetIDs) == 0 {
		return resource.ProvenZero("subnet", "subnetIDs")
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
	return relatedResultTrunc("subnet", ids, false)
}

// checkRedisVPC resolves the VPC for the replication group via the same
// DescribeCacheSubnetGroups call used by checkRedisSubnet.
func checkRedisVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng, err := redisSubnetGroup(ctx, clients, res)
	if err != nil {
		return relatedFromErr("vpc", err)
	}
	if sng == nil {
		return resource.ProvenZero("vpc", "sng")
	}
	if sng.VpcId == nil || *sng.VpcId == "" {
		return resource.ProvenZero("vpc", "sng.VpcId")
	}
	return relatedResultTrunc("vpc", []string{*sng.VpcId}, false)
}

// redisMemberCluster calls DescribeCacheClusters on MemberClusters[0] of the
// replication group. SG, SNS, and SubnetGroup data all live on the member
// cluster struct. The three outcomes are distinct: a cluster, a nil cluster
// with a nil error (the group has no member to read — a fact about the group),
// or an error (we could not look).
func redisMemberCluster(ctx context.Context, clients any, res resource.Resource) (*elasticachetypes.CacheCluster, error) {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return nil, errRawStructMissing
		}
		return nil, errRedisNoGroupDetail
	}
	if len(rg.MemberClusters) == 0 {
		return nil, nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ElastiCache == nil {
		return nil, errRedisNoGroupDetail
	}
	memberID := rg.MemberClusters[0]
	clusters, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elasticachetypes.CacheCluster, *string, error) {
		out, err := c.ElastiCache.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
			CacheClusterId: &memberID,
			Marker:         marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.CacheClusters, out.Marker, nil
	})
	if err != nil {
		return nil, fmt.Errorf("describing member cluster %s: %w", memberID, err)
	}
	if len(clusters) == 0 {
		return nil, nil
	}
	return &clusters[0], nil
}

// redisSubnetGroup performs the two-step resolution:
// 1. DescribeCacheClusters(MemberClusters[0]) → CacheSubnetGroupName
// 2. DescribeCacheSubnetGroups(name) → CacheSubnetGroup
//
// A nil group with a nil error means there is none to reach: no member cluster,
// or a member that names no subnet group (a cluster outside a VPC).
func redisSubnetGroup(ctx context.Context, clients any, res resource.Resource) (*elasticachetypes.CacheSubnetGroup, error) {
	cc, err := redisMemberCluster(ctx, clients, res)
	if err != nil {
		return nil, err
	}
	if cc == nil || cc.CacheSubnetGroupName == nil || *cc.CacheSubnetGroupName == "" {
		return nil, nil
	}
	// A member cluster came back, so redisMemberCluster already proved the
	// ElastiCache client is usable; the checked assertion only recovers the
	// typed pointer.
	c, cok := clients.(*ServiceClients)
	if !cok {
		return nil, errRedisNoGroupDetail
	}
	name := *cc.CacheSubnetGroupName
	groups, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]elasticachetypes.CacheSubnetGroup, *string, error) {
		out, callErr := c.ElastiCache.DescribeCacheSubnetGroups(ctx, &elasticache.DescribeCacheSubnetGroupsInput{
			CacheSubnetGroupName: &name,
			Marker:               marker,
		})
		if callErr != nil {
			return nil, nil, callErr
		}
		return out.CacheSubnetGroups, out.Marker, nil
	})
	if err != nil {
		return nil, fmt.Errorf("describing subnet group %s: %w", name, err)
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return &groups[0], nil
}

// truncatedResultRedis returns a RelatedCheckResult with Truncated=true when the
// target cache is truncated and matches were found. Later pages may contain
// additional matches, so the displayed count is a lower bound — rendered as "(N+)".
func truncatedResultRedis(target string, ids []string) resource.RelatedCheckResult {
	return resource.KnownRelated(target, ids, true)
}
