// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// asg_issue_enrichment.go — Wave 2 issue enrichment for the asg resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// asg canonical FindingCodes.
const (
	asgCodeScalingActivityFailed domain.FindingCode = "asg.scaling-activity-failed"
)

// EnrichASGScalingActivities calls DescribeScalingActivities(MaxRecords=1) for each ASG
// (cap EnrichmentCap) and returns a Finding when the latest activity StatusCode == Failed.
// Severity is "!" (broken/degraded). Summary: "latest scaling activity failed: <statusMessage>".
func EnrichASGScalingActivities(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.AutoScaling == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		name := r.ID
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return clients.AutoScaling.DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{
				AutoScalingGroupName: &name,
				MaxRecords:           aws.Int32(1),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			return
		}
		if len(out.Activities) == 0 {
			return
		}
		act := out.Activities[0]
		if act.StatusCode != asgtypes.ScalingActivityStatusCodeFailed {
			return
		}
		statusMsg := ""
		if act.StatusMessage != nil {
			statusMsg = *act.StatusMessage
		}
		summary := "latest scaling activity failed"
		if statusMsg != "" {
			summary = fmt.Sprintf("latest scaling activity failed: %s", statusMsg)
		}
		rows := []domain.DetailRow{
			{Label: "Status", Value: domain.HumanizeStatusPhrase(string(act.StatusCode)), Tier: "!"},
		}
		if statusMsg != "" {
			rows = append(rows, domain.DetailRow{Label: "Message", Value: statusMsg, Tier: "!"})
		}
		if act.Cause != nil && *act.Cause != "" {
			rows = append(rows, domain.DetailRow{Label: "Cause", Value: *act.Cause})
		}
		if act.StartTime != nil {
			rows = append(rows, domain.DetailRow{Label: "Started", Value: act.StartTime.Format("2006-01-02")})
		}
		setWave2Finding(&result, r.ID, asgCodeScalingActivityFailed, summary, "!", "asg", rows, "")
	})
	sort.Strings(failures)
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result, AggregateFailures("asg-enrich: DescribeScalingActivities", failures, total)
}
