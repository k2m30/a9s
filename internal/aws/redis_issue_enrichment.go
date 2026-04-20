// redis_issue_enrichment.go — Wave 2 issue enrichment for the redis resource type.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("redis", EnrichRedisReplicationGroup, 100)
	resource.RegisterIssueEnricherFieldKeys("redis", []string{"automatic_failover", "multi_az"})
}

// EnrichRedisReplicationGroup calls DescribeReplicationGroups (paginated) and writes
// automatic_failover and multi_az field updates for each cache cluster ID in the resource list.
//
// No findings are raised — this enricher is field-update only.
// Skip when clients.ElastiCache == nil.
func EnrichRedisReplicationGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.ElastiCache == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	// Build set of resource IDs (CacheClusterIds) we know about so we don't
	// emit FieldUpdates for clusters not in the current list.
	known := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		known[r.ID] = struct{}{}
	}
	var marker *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.ElastiCache.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{
			Marker: marker,
		})
		pages++
		if err != nil {
			// ElastiCache errors on this call are non-fatal — return an empty result
			// rather than propagating (enricher is best-effort for field updates).
			return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
		}
		for _, rg := range out.ReplicationGroups {
			af := strings.ToLower(string(rg.AutomaticFailover))
			multi := strings.ToLower(string(rg.MultiAZ))
			for _, member := range rg.MemberClusters {
				if _, ok := known[member]; !ok {
					continue
				}
				fieldUpdates[member] = map[string]string{
					"automatic_failover": af,
					"multi_az":           multi,
				}
			}
		}
		if out.Marker == nil {
			break
		}
		marker = out.Marker
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
