// tg_issue_enrichment.go — Wave 2 issue enrichment for the tg resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tg canonical FindingCodes.
const (
	tgCodeUnhealthyTargets domain.FindingCode = "tg.unhealthy-targets"
)

// EnrichTargetGroupHealth calls DescribeTargetHealth for each target group (1 per TG, cap ~50).
// Returns a Finding for each TG with at least one unhealthy target.
// Severity is "!" (broken/degraded). Summary: "unhealthy targets: X/Y".
// Per-TG errors are aggregated and returned as a composite error alongside partial findings (E3, E4, E5).
func EnrichTargetGroupHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.ELBv2 == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.ID == "" {
			continue
		}
		// DescribeTargetHealth requires the full ARN, not the bare target-group
		// name. Resource.ID is the name (set by the fetcher for display); the
		// ARN lives in Fields["target_group_arn"]. Passing r.ID would always
		// error with "target group not found" on both demo fake and real AWS.
		tgARN := r.Fields["target_group_arn"]
		if tgARN == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			return clients.ELBv2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
				TargetGroupArn: aws.String(tgARN),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		targetCount := len(out.TargetHealthDescriptions)
		unhealthy := 0
		var firstReason string
		for _, t := range out.TargetHealthDescriptions {
			if t.TargetHealth != nil && t.TargetHealth.State != elbtypes.TargetHealthStateEnumHealthy {
				unhealthy++
				if firstReason == "" && t.TargetHealth.Reason != "" {
					firstReason = string(t.TargetHealth.Reason)
				}
			}
		}
		healthy := targetCount - unhealthy
		healthSummary := ""
		if targetCount == 0 {
			healthSummary = "ORPHAN"
		} else {
			healthSummary = fmt.Sprintf("%d/%d healthy", healthy, targetCount)
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"health_summary": healthSummary,
		}
		if unhealthy > 0 {
			rows := []domain.DetailRow{
				{Label: "Unhealthy Targets", Value: fmt.Sprintf("%d/%d", unhealthy, targetCount), Tier: "!"},
			}
			if firstReason != "" {
				rows = append(rows, domain.DetailRow{Label: "Reason", Value: firstReason, Tier: "~"})
			}
			setWave2Finding(&result, r.ID, tgCodeUnhealthyTargets, fmt.Sprintf("unhealthy targets: %d/%d", unhealthy, targetCount), "!", "tg", rows)
		}
	}
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result,
		AggregateFailures("tg-enrich: DescribeTargetHealth", failures, total)
}
