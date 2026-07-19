// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// elb_issue_enrichment.go — Wave 2 issue enrichment for the elb resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// elb canonical FindingCodes.
const (
	elbCodeMisconfigured domain.FindingCode = "elb.misconfigured"
)

// EnrichELBAttributes calls DescribeLoadBalancerAttributes for each load
// balancer (1 per LB, cap 50) and returns a "~" (SevWarn) finding for each LB
// missing deletion protection or access logging — matching the single
// elbCodeMisconfigured FindingDef declared at SevWarn in
// catalog_networking.go. Both flags missing at once is the AWS
// create-load-balancer default and must not escalate to SevBroken; doing so
// previously painted every freshly-created, unhardened LB red.
//
// Per-LB API failures aggregate into a composite error returned alongside
// the partial findings (E1–E6 contract). LoadBalancerArn is read from
// r.Fields["load_balancer_arn"] — the elb fetcher emits ID = bare LB name
// and stores the ARN in Fields. Each call is wrapped in RetryOnThrottle.
func EnrichELBAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ELBv2 == nil {
		return result, nil
	}
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		// DescribeLoadBalancerAttributes requires the LB ARN. The elb fetcher
		// (elb.go) sets ID = bare name and stores the ARN in
		// Fields["load_balancer_arn"]. Passing r.ID errors with ValidationError.
		lbARN := r.Fields["load_balancer_arn"]
		if lbARN == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput, error) {
			return clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
				LoadBalancerArn: aws.String(lbARN),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			result.TruncatedIDs[r.ID] = true
			return
		}
		var rows []domain.DetailRow
		var phrases []string
		for _, attr := range out.Attributes {
			if attr.Key == nil || attr.Value == nil {
				continue
			}
			switch *attr.Key {
			case "deletion_protection.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Deletion Protection", Value: "disabled", Tier: "~"})
					phrases = append(phrases, "deletion protection disabled")
				}
			case "access_logs.s3.enabled":
				if *attr.Value == "false" {
					rows = append(rows, domain.DetailRow{Label: "Access Logs", Value: "disabled", Tier: "~"})
					phrases = append(phrases, "access logs disabled")
				}
			}
		}
		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, r.ID, elbCodeMisconfigured, phrases[0], "~", "elb", rows, "")
	})
	sort.Strings(failures)
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, AggregateFailures("elb-enrich: DescribeLoadBalancerAttributes", failures, total)
}
