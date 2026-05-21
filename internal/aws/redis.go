package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// computeShardIssues returns one phrase per non-available NodeGroup on a
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

// rgTransientPhrase maps a transient RG-level status to its list phrase.
func rgTransientPhrase(state string) string {
	switch state {
	case "modifying":
		return "modifying — config change"
	case "snapshotting":
		return "snapshotting — backup running"
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
		// Engine filter: skip any RG whose engine is not "redis".
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

		findings := computeRedisFindings(status, multiAZ, autoFailover, rg.NodeGroups)
		statusPhrase := phraseFromFindings(findings)

		r := resource.Resource{
			ID:       rgID,
			Name:     rgID,
			Findings: findings,
			Fields: map[string]string{
				"cluster_id": rgID,
				"node_type":  nodeType,
				"nodes":      nodes,
				"endpoint":   endpoint,
				"status":     statusPhrase,
				"arn":        arn,
			},
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

// computeRedisFindings derives the ordered findings slice for a ReplicationGroup.
func computeRedisFindings(status string, multiAZ bool, autoFailover bool, nodeGroups []elasticachetypes.NodeGroup) []domain.Finding {
	var broken []domain.Finding
	var warnings []domain.Finding

	switch status {
	case "available":
		// healthy
	case "creating":
		warnings = append(warnings, domain.Finding{Code: CodeRedisCreating, Phrase: "creating — new group", Severity: domain.SevWarn, Source: "wave1"})
	case "deleting":
		warnings = append(warnings, domain.Finding{Code: CodeRedisDeleting, Phrase: "deleting — teardown", Severity: domain.SevWarn, Source: "wave1"})
	case "create-failed":
		broken = append(broken, domain.Finding{Code: CodeRedisCreateFailed, Phrase: "create failed — see events", Severity: domain.SevBroken, Source: "wave1"})
	case "modifying", "snapshotting":
		shardIssues := computeShardIssues(nodeGroups)
		if len(shardIssues) > 0 {
			for _, si := range shardIssues {
				warnings = append(warnings, domain.Finding{Code: CodeRedisShardIssue, Phrase: si, Severity: domain.SevWarn, Source: "wave1"})
			}
		} else {
			p := rgTransientPhrase(status)
			if p != "" {
				code := CodeRedisModifying
				if status == "snapshotting" {
					code = CodeRedisSnapshotting
				}
				warnings = append(warnings, domain.Finding{Code: code, Phrase: p, Severity: domain.SevWarn, Source: "wave1"})
			}
		}
	}

	if multiAZ && !autoFailover {
		warnings = append(warnings, domain.Finding{Code: CodeRedisMultiAZWithoutAutoFailover, Phrase: "multi-AZ without auto-failover", Severity: domain.SevWarn, Source: "wave1"})
	}

	sort.Slice(warnings, func(i, j int) bool { return warnings[i].Phrase < warnings[j].Phrase })
	return append(broken, warnings...)
}
