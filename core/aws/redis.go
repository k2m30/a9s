// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

		findings, attentionDetails := computeRedisFindings(status, multiAZ, autoFailover, rg)
		statusPhrase := domain.StatusPhrase(findings)

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
			RawStruct:        rg,
			AttentionDetails: attentionDetails,
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

// computeRedisFindings derives the ordered findings slice for a ReplicationGroup
// plus the supporting AttentionDetail rows keyed by the finding that owns them.
// Lifecycle findings come first (they own the status column); the
// security-posture rows are evaluated independently and appended after them.
func computeRedisFindings(status string, multiAZ bool, autoFailover bool, rg elasticachetypes.ReplicationGroup) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	nodeGroups := rg.NodeGroups
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
	findings := make([]domain.Finding, 0, len(broken)+len(warnings))
	findings = append(findings, broken...)
	findings = append(findings, warnings...)

	if isTeardownStatus(status) {
		// A group on its way out has no posture worth reporting.
		return findings, nil
	}
	posture, details := redisPostureFindings(rg)
	return append(findings, posture...), details
}

// redisPostureFindings evaluates the four encryption/authentication/backup
// posture rows against the ReplicationGroup. Every predicate treats a nil
// pointer as "off": ElastiCache omits these fields exactly when the feature
// was never enabled, so unknown and disabled are the same state here.
func redisPostureFindings(rg elasticachetypes.ReplicationGroup) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	var findings []domain.Finding
	details := map[domain.FindingCode]domain.AttentionDetail{}

	add := func(code domain.FindingCode, phrase, detail string, sev domain.Severity, rows []domain.DetailRow) {
		findings = append(findings, domain.Finding{
			Code: code, Phrase: phrase, Detail: detail, Severity: sev, Source: "wave1",
		})
		details[code] = domain.AttentionDetail{Rows: rows}
	}

	if !aws.ToBool(rg.AtRestEncryptionEnabled) {
		add(CodeRedisAtRestOff, "encryption at rest off", redisAtRestOffDetail, domain.SevWarn, nil)
	}
	transitOn := aws.ToBool(rg.TransitEncryptionEnabled)
	if !transitOn {
		add(CodeRedisTransitOff, "encryption in transit off", redisTransitOffDetail, domain.SevWarn, nil)
	}
	// AWS only accepts an AUTH token on a group that also encrypts in
	// transit, so a group without in-transit encryption is already reported
	// by the row above — reporting a missing AUTH token there too would name
	// the same misconfiguration twice.
	if transitOn && !aws.ToBool(rg.AuthTokenEnabled) {
		add(CodeRedisNoAuth, "no authentication token", redisNoAuthDetail, domain.SevBroken,
			[]domain.DetailRow{{Label: "Authentication token", Value: "none", Tier: "!"}})
	}
	if rg.SnapshotRetentionLimit == nil || *rg.SnapshotRetentionLimit == 0 {
		add(CodeRedisNoBackup, "automatic backups off", redisNoBackupDetail, domain.SevWarn,
			[]domain.DetailRow{{Label: "Snapshot retention", Value: "0 days", Tier: "~"}})
	}

	if len(details) == 0 {
		return findings, nil
	}
	return findings, details
}
