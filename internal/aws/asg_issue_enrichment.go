// asg_issue_enrichment.go — Wave 2 issue enrichment for the asg resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
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
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.AutoScaling == nil {
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
		total++
		name := r.ID
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return clients.AutoScaling.DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{
				AutoScalingGroupName: &name,
				MaxRecords:           aws.Int32(1),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		if len(out.Activities) == 0 {
			continue
		}
		act := out.Activities[0]
		if act.StatusCode != asgtypes.ScalingActivityStatusCodeFailed {
			continue
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
			{Label: "Status", Value: string(act.StatusCode), Tier: "!"},
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
		setWave2Finding(&result, r.ID, asgCodeScalingActivityFailed, summary, "!", "asg", rows)
	}
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result, AggregateFailures("asg-enrich: DescribeScalingActivities", failures, total)
}
