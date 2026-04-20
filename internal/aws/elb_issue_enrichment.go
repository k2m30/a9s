// elb_issue_enrichment.go — Wave 2 issue enrichment for the elb resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("elb", EnrichELBAttributes, 100)
}

// EnrichELBAttributes calls DescribeLoadBalancerAttributes for each load
// balancer (1 per LB, cap 50) and returns an informational "~" finding for
// each LB missing deletion protection or access logging.
// The worst finding per LB is promoted to "!" if both attributes are missing;
// otherwise "~" is used. IssueCount counts findings with Severity "!".
func EnrichELBAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.ELBv2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.ID == "" {
			continue
		}
		out, err := clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
			LoadBalancerArn: aws.String(r.ID),
		})
		if err != nil {
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		var rows []resource.FindingRow
		for _, attr := range out.Attributes {
			if attr.Key == nil || attr.Value == nil {
				continue
			}
			switch *attr.Key {
			case "deletion_protection.enabled":
				if *attr.Value == "false" {
					rows = append(rows, resource.FindingRow{Label: "Deletion Protection", Value: "disabled", Tier: "~"})
				}
			case "access_logs.s3.enabled":
				if *attr.Value == "false" {
					rows = append(rows, resource.FindingRow{Label: "Access Logs", Value: "disabled", Tier: "~"})
				}
			}
		}
		if len(rows) == 0 {
			continue
		}
		// Severity is "~" for each individual finding; promote to "!" only
		// when both misconfiguration flags are present simultaneously.
		severity := "~"
		if len(rows) >= 2 {
			severity = "!"
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: severity,
			Summary:  rows[0].Label + ": " + rows[0].Value,
			Rows:     rows,
		}
	}
	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return IssueEnricherResult{IssueCount: issueCount, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
