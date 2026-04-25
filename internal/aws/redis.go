package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("redis", []string{"cluster_id", "node_type", "status", "nodes", "endpoint", "arn"})

	resource.RegisterPaginated("redis", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRedisPage(ctx, c.ElastiCache, continuationToken)
	})
}

// computeShardIssues returns one §4 phrase per non-available NodeGroup on a
// multi-shard (cluster-mode-enabled) replication group, ordered alphabetically
// by phrase so rule-7 precedence is stable. Returns nil when the RG has ≤1
// NodeGroup — single-shard RGs use the RG-level phrase instead.
func computeShardIssues(nodeGroups []elasticachetypes.NodeGroup) []string {
	if len(nodeGroups) <= 1 {
		return nil
	}
	var out []string
	for _, ng := range nodeGroups {
		ngStatus := strings.ToLower(aws.ToString(ng.Status))
		if ngStatus == "" || ngStatus == "available" {
			continue
		}
		ngID := aws.ToString(ng.NodeGroupId)
		if ngID == "" {
			continue
		}
		out = append(out, fmt.Sprintf("shard %s: %s", ngID, ngStatus))
	}
	sort.Strings(out)
	return out
}

// rgTransientPhrase maps a transient RG-level status to its §4 list phrase.
func rgTransientPhrase(state string) string {
	switch state {
	case "modifying":
		return "modifying \u2014 config change"
	case "snapshotting":
		return "snapshotting \u2014 backup running"
	}
	return ""
}

// FetchRedis calls the ElastiCache DescribeReplicationGroups API and converts
// all pages into a slice of generic Resource structs.
func FetchRedis(ctx context.Context, api ElastiCacheDescribeReplicationGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRedisPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchRedisPage fetches a single page of ElastiCache ReplicationGroups and maps
// each to a resource.Resource. RawStruct is set to the full ReplicationGroup struct.
func FetchRedisPage(ctx context.Context, api ElastiCacheDescribeReplicationGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &elasticache.DescribeReplicationGroupsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticache.DescribeReplicationGroupsOutput, error) {
		return api.DescribeReplicationGroups(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Redis replication groups: %w", err)
	}

	var resources []resource.Resource

	for _, rg := range output.ReplicationGroups {
		// Engine filter (spec §3.1, P2-1): skip any RG whose engine is not "redis".
		// DescribeReplicationGroups returns all ElastiCache engines (redis, valkey,
		// memcached) through the same API; this fetcher is redis-only.
		if rg.Engine == nil || !strings.EqualFold(aws.ToString(rg.Engine), "redis") {
			continue
		}

		rgID := ""
		if rg.ReplicationGroupId != nil {
			rgID = *rg.ReplicationGroupId
		}

		nodeType := ""
		if rg.CacheNodeType != nil {
			nodeType = *rg.CacheNodeType
		}

		nodes := fmt.Sprintf("%d", len(rg.MemberClusters))

		endpoint := ""
		if rg.ConfigurationEndpoint != nil && rg.ConfigurationEndpoint.Address != nil {
			endpoint = *rg.ConfigurationEndpoint.Address
		}

		arn := ""
		if rg.ARN != nil {
			arn = *rg.ARN
		}

		status := ""
		if rg.Status != nil {
			status = strings.ToLower(*rg.Status)
		}
		multiAZ := rg.MultiAZ == elasticachetypes.MultiAZStatusEnabled
		autoFailover := rg.AutomaticFailover == elasticachetypes.AutomaticFailoverStatusEnabled

		issues := computeRedisIssues(status, multiAZ, autoFailover, rg.NodeGroups)
		statusPhrase := redisStatusPhrase(issues)

		r := resource.Resource{
			ID:   rgID,
			Name: rgID,
			Fields: map[string]string{
				"cluster_id": rgID,
				"node_type":  nodeType,
				"nodes":      nodes,
				"endpoint":   endpoint,
				"status":     statusPhrase,
				"arn":        arn,
			},
			Issues:    issues,
			RawStruct: rg,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil {
		nextToken = *output.Marker
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// computeRedisIssues derives the ordered §4 issues slice for a ReplicationGroup.
// Wave 1 signals only — spec §3.2 has no Wave 2 signals for redis.
//
// Precedence: Broken first, then Warnings alphabetically (per impl-plan §5).
// The em-dash character (—) is used literally in all phrases.
// For multi-shard RGs (len(nodeGroups) > 1) in modifying/snapshotting state,
// per-shard phrases are emitted instead of the RG-level phrase.
func computeRedisIssues(status string, multiAZ bool, autoFailover bool, nodeGroups []elasticachetypes.NodeGroup) []string {
	var broken []string
	var warnings []string

	switch status {
	case "available":
		// Healthy — no state issue.
	case "creating":
		warnings = append(warnings, "creating \u2014 new group")
	case "deleting":
		warnings = append(warnings, "deleting \u2014 teardown")
	case "create-failed":
		broken = append(broken, "create failed \u2014 see events")
	case "modifying", "snapshotting":
		shardIssues := computeShardIssues(nodeGroups)
		if len(shardIssues) > 0 {
			// Multi-shard with at least one non-available shard: use shard-level phrases.
			warnings = append(warnings, shardIssues...)
		} else {
			// Single-shard OR all shards available (transient RG-level state):
			// fall back to the RG-level phrase.
			if p := rgTransientPhrase(status); p != "" {
				warnings = append(warnings, p)
			}
		}
	}

	if multiAZ && !autoFailover {
		warnings = append(warnings, "multi-AZ without auto-failover")
	}

	// Sort warnings alphabetically (per impl-plan §5 precedence).
	sort.Strings(warnings)

	// Broken first, then sorted warnings.
	return append(broken, warnings...)
}

// redisStatusPhrase converts an issues slice to the S4 status column string.
// Empty slice → "" (Healthy silence per spec §4).
// Single issue → the phrase verbatim.
// Multiple issues → top phrase + " (+N)" suffix per universal rule 7.
func redisStatusPhrase(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	if len(issues) == 1 {
		return issues[0]
	}
	return fmt.Sprintf("%s (+%d)", issues[0], len(issues)-1)
}
