// r53_issue_enrichment.go — Wave 2 issue enrichment for the r53 resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53svc "github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("r53", EnrichRoute53Zone, 100)
}

// EnrichRoute53Zone calls GetHostedZone per zone (cap EnrichmentCap) and raises a finding
// for private zones that have no VPC associations (orphaned private zone).
//
// Findings:
//   - HostedZone.Config.PrivateZone == true AND VPCs[] empty → "~" finding
//     "private zone with no VPC associations (orphan)"
//
// Skip if clients.Route53 == nil. Per-zone errors → Truncated.
func EnrichRoute53Zone(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.Route53 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		zoneID := r.Fields["zone_id"]
		if zoneID == "" {
			zoneID = r.ID
		}
		if zoneID == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*r53svc.GetHostedZoneOutput, error) {
			return clients.Route53.GetHostedZone(ctx, &r53svc.GetHostedZoneInput{
				Id: aws.String(zoneID),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.HostedZone == nil {
			continue
		}
		// Only raise a finding for private zones — public zones cannot have VPC associations.
		if out.HostedZone.Config == nil || !out.HostedZone.Config.PrivateZone {
			continue
		}
		if len(out.VPCs) > 0 {
			continue
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  "private zone with no VPC associations (orphan)",
			Rows: []resource.FindingRow{
				{Label: "Zone ID", Value: zoneID, Tier: "~"},
				{Label: "Issue", Value: "private zone with no VPC associations (orphan)", Tier: "~"},
			},
		}
	}
	// All Route53 findings are severity "~" (informational).
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings},
		AggregateFailures("r53-enrich: GetHostedZone", failures, total)
}
