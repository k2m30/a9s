// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tg_issue_enrichment.go — Wave 2 issue enrichment for the tg resource type.
package aws

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// tg canonical FindingCodes.
const (
	tgCodeUnhealthyTargets    domain.FindingCode = "tg.unhealthy-targets"
	tgCodeAllTargetsUnhealthy domain.FindingCode = "tg.all-targets-unhealthy"
)

// EnrichTargetGroupHealth calls DescribeTargetHealth for each target group (1 per TG, cap ~50).
// Returns a Finding for each TG with at least one target whose TargetHealth.State
// is literally "unhealthy" ("initial", "draining", "unused", and other non-"unhealthy"
// states do not count toward the numerator). Severity is graduated: "~" when
// 0 < unhealthy < total, "!" when every reporting target is unhealthy.
// Summary: "unhealthy targets: X/Y".
// Per-TG errors are aggregated and returned as a composite error alongside partial findings (E3, E4, E5).
func EnrichTargetGroupHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.ELBv2 == nil {
		return result, nil
	}
	truncated := false
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		// DescribeTargetHealth requires the full ARN, not the bare target-group
		// name. Resource.ID is the name (set by the fetcher for display); the
		// ARN lives in Fields["target_group_arn"]. Passing r.ID would always
		// error with "target group not found" on both demo fake and real AWS.
		tgARN := r.Fields["target_group_arn"]
		if tgARN == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			return clients.ELBv2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
				TargetGroupArn: aws.String(tgARN),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			truncated = true
			return
		}
		targetCount := len(out.TargetHealthDescriptions)
		// notHealthy drives the health_summary display field (any state other
		// than "healthy" counts) — unrelated to the Finding numerator below.
		notHealthy := 0
		// literalUnhealthy is the Finding numerator: only State=="unhealthy"
		// counts. "initial", "draining", "unused", etc. are transitional or
		// intentional states, not failures, and must not trigger a finding.
		literalUnhealthy := 0
		var unhealthyRows []domain.DetailRow
		for _, t := range out.TargetHealthDescriptions {
			if t.TargetHealth == nil {
				continue
			}
			if t.TargetHealth.State != elbtypes.TargetHealthStateEnumHealthy {
				notHealthy++
			}
			if t.TargetHealth.State != elbtypes.TargetHealthStateEnumUnhealthy {
				continue
			}
			literalUnhealthy++
			value := ""
			if t.Target != nil {
				value = aws.ToString(t.Target.Id)
				if t.Target.Port != nil {
					value += fmt.Sprintf(":%d", *t.Target.Port)
				}
			}
			// humanizeTargetHealthReason (tg_health.go) strips the Target./Elb.
			// namespace prefix before humanizing — plain
			// domain.HumanizeStatusPhrase would emit "target. failed health
			// checks" for "Target.FailedHealthChecks".
			switch reason := humanizeTargetHealthReason(string(t.TargetHealth.Reason)); {
			case value == "":
				value = reason
			case reason != "":
				value += " — " + reason
			}
			if value != "" {
				unhealthyRows = append(unhealthyRows, domain.DetailRow{Label: "Unhealthy target", Value: value})
			}
		}
		healthy := targetCount - notHealthy
		healthSummary := ""
		if targetCount == 0 {
			healthSummary = "no targets"
		} else {
			healthSummary = fmt.Sprintf("%d/%d healthy", healthy, targetCount)
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"health_summary": healthSummary,
		}
		if literalUnhealthy > 0 {
			// Every target down and some targets down are different things to
			// do about, so they are different codes: one severity per code.
			allDown := literalUnhealthy == targetCount
			tier := "~"
			if allDown {
				tier = "!"
			}
			for i := range unhealthyRows {
				unhealthyRows[i].Tier = tier
			}
			if allDown {
				setWave2Finding(&result, r.ID, tgCodeAllTargetsUnhealthy, unhealthyRows, strconv.Itoa(targetCount))
			} else {
				setWave2Finding(&result, r.ID, tgCodeUnhealthyTargets, unhealthyRows, strconv.Itoa(literalUnhealthy), strconv.Itoa(targetCount))
			}
		}
	})

	SetTruncated(&result, truncated)
	return result,
		AggregateFailures("DescribeTargetHealth", failures, total)
}
