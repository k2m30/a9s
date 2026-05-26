// elb_issue_enrichment.go — Wave 2 issue enrichment for the elb resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// elb canonical FindingCodes.
const (
	elbCodeMisconfigured domain.FindingCode = "elb.misconfigured"
)

// EnrichELBAttributes calls DescribeLoadBalancerAttributes for each load
// balancer (1 per LB, cap 50) and returns an informational "~" finding for
// each LB missing deletion protection or access logging.
// The worst finding per LB is promoted to "!" if both attributes are missing;
// otherwise "~" is used. IssueCount counts findings with Severity "!".
//
// Per-LB API failures aggregate into a composite error returned alongside
// the partial findings (E1–E6 contract). LoadBalancerArn is read from
// r.Fields["load_balancer_arn"] — the elb fetcher emits ID = bare LB name
// and stores the ARN in Fields. Each call is wrapped in RetryOnThrottle.
func EnrichELBAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
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
		// DescribeLoadBalancerAttributes requires the LB ARN. The elb fetcher
		// (elb.go) sets ID = bare name and stores the ARN in
		// Fields["load_balancer_arn"]. Passing r.ID errors with ValidationError.
		lbARN := r.Fields["load_balancer_arn"]
		if lbARN == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput, error) {
			return clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
				LoadBalancerArn: aws.String(lbARN),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		var rows []domain.DetailRow
		for _, attr := range out.Attributes {
			if attr.Key == nil || attr.Value == nil {
				continue
			}
			switch *attr.Key {
			case "deletion_protection.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Deletion Protection", Value: "disabled", Tier: "~"})
				}
			case "access_logs.s3.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Access Logs", Value: "disabled", Tier: "~"})
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
		setWave2Finding(&result, r.ID, elbCodeMisconfigured, rows[0].Label+": "+rows[0].Value, severity, "elb", rows)
	}
	issueCount := 0
	for _, f := range result.Findings {
		if f.Severity == domain.SevBroken {
			issueCount++
		}
	}
	result.IssueCount = issueCount
	result.Truncated = truncated
	return result, AggregateFailures("elb-enrich: DescribeLoadBalancerAttributes", failures, total)
}
