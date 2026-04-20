// ec2_issue_enrichment.go — Wave 2 issue enrichment for the ec2 resource type.
package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ec2", EnrichEC2InstanceStatus, 100)
}

// EnrichEC2InstanceStatus calls DescribeInstanceStatus(IncludeAllInstances=true) (account-wide,
// paginated) and returns a Finding for every instance whose system or instance status is not "ok".
// Scheduled events with NotBeforeDeadline within the next 7 days also produce a Finding.
// Severity "!" for status != ok; "~" for scheduled events. IssueCount counts "!" findings only.
// Pagination uses NextToken; walks up to EnrichmentCap pages.
func EnrichEC2InstanceStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.EC2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	// Build a set of known resource IDs so we can detect unmatched API returns.
	knownIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			knownIDs[r.ID] = true
		}
	}
	var allInstanceStatuses []ec2types.InstanceStatus
	var nextToken *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2svc.DescribeInstanceStatusInput{
			IncludeAllInstances: aws.Bool(true),
			NextToken:           nextToken,
		})
		pages++
		if err != nil {
			return IssueEnricherResult{TruncatedIDs: truncatedIDs}, err
		}
		allInstanceStatuses = append(allInstanceStatuses, out.InstanceStatuses...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	now := time.Now()
	cutoff := now.Add(7 * 24 * time.Hour)

	for _, is := range allInstanceStatuses {
		if is.InstanceId == nil {
			continue
		}
		id := *is.InstanceId
		// Track unmatched: API returned an instance not in the input resources slice.
		if len(knownIDs) > 0 && !knownIDs[id] {
			continue
		}

		// Collect rows for this instance.
		var rows []resource.FindingRow
		severity := "~" // start informational; upgrade to "!" on real impairment

		// Check instance status.
		if is.InstanceStatus != nil && is.InstanceStatus.Status != ec2types.SummaryStatusOk {
			statusStr := string(is.InstanceStatus.Status)
			rows = append(rows, resource.FindingRow{Label: "Instance Status", Value: statusStr, Tier: "!"})
			severity = "!"
		}

		// Check system status.
		if is.SystemStatus != nil && is.SystemStatus.Status != ec2types.SummaryStatusOk {
			statusStr := string(is.SystemStatus.Status)
			rows = append(rows, resource.FindingRow{Label: "System Status", Value: statusStr, Tier: "!"})
			severity = "!"
		}

		// Check scheduled events within 7 days.
		// NotBeforeDeadline is the hard deadline (forced retirement/reboot).
		// NotBefore is the earliest scheduled start — also within 7d is actionable.
		for _, ev := range is.Events {
			var eventDate *time.Time
			if ev.NotBeforeDeadline != nil && ev.NotBeforeDeadline.Before(cutoff) {
				eventDate = ev.NotBeforeDeadline
			} else if ev.NotBefore != nil && ev.NotBefore.Before(cutoff) {
				eventDate = ev.NotBefore
			}
			if eventDate == nil {
				continue
			}
			code := string(ev.Code)
			dateStr := eventDate.Format("2006-01-02")
			rows = append(rows, resource.FindingRow{
				Label: "Scheduled Event",
				Value: fmt.Sprintf("%s at %s", code, dateStr),
				Tier:  "~",
			})
		}

		if len(rows) == 0 {
			continue
		}

		// Build summary: prefer the first "!" row's value, fall back to "~".
		summary := ""
		for _, row := range rows {
			if row.Tier == "!" {
				summary = fmt.Sprintf("%s: %s", strings.ToLower(row.Label), row.Value)
				break
			}
		}
		if summary == "" && len(rows) > 0 {
			summary = fmt.Sprintf("scheduled event: %s", rows[0].Value)
		}

		findings[id] = resource.EnrichmentFinding{
			Severity: severity,
			Summary:  summary,
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
