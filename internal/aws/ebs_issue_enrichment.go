// ebs_issue_enrichment.go — Wave 2 issue enrichment for the ebs resource type.
package aws

import (
	"context"

	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ebs", EnrichEBSVolumeStatus, 10)
}

// EnrichEBSVolumeStatus calls DescribeVolumeStatus (account-wide, paginated) and returns
// a Finding for every volume with non-ok status.
// Severity is "!" (broken/degraded). Walks up to EnrichmentCap pages via NextToken.
func EnrichEBSVolumeStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
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
	var allVolumeStatuses []ec2types.VolumeStatusItem
	var nextToken *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.EC2.DescribeVolumeStatus(ctx, &ec2svc.DescribeVolumeStatusInput{
			NextToken: nextToken,
		})
		pages++
		if err != nil {
			return IssueEnricherResult{TruncatedIDs: truncatedIDs}, err
		}
		allVolumeStatuses = append(allVolumeStatuses, out.VolumeStatuses...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	for _, v := range allVolumeStatuses {
		if v.VolumeId == nil {
			continue
		}
		volID := *v.VolumeId
		// Track unmatched: API returned a volume not in the input resources slice.
		if len(knownIDs) > 0 && !knownIDs[volID] {
			continue
		}
		if v.VolumeStatus == nil || v.VolumeStatus.Status == ec2types.VolumeStatusInfoStatusOk {
			continue
		}
		ioState := string(v.VolumeStatus.Status)
		rows := []resource.FindingRow{
			{Label: "I/O State", Value: ioState, Tier: "!"},
		}
		// Most recent event (if any).
		if len(v.Events) > 0 {
			ev := v.Events[0]
			eventVal := ""
			if ev.EventType != nil {
				eventVal = *ev.EventType
			}
			if ev.Description != nil && *ev.Description != "" {
				eventVal = *ev.Description
			}
			if eventVal != "" {
				rows = append(rows, resource.FindingRow{Label: "Event", Value: eventVal, Tier: "~"})
			}
		}
		// Most recent action code (if any).
		if len(v.Actions) > 0 {
			ac := v.Actions[0]
			if ac.Code != nil && *ac.Code != "" {
				rows = append(rows, resource.FindingRow{Label: "Action Code", Value: *ac.Code})
			}
		}
		findings[volID] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "volume I/O degraded",
			Rows:     rows,
		}
	}
	return IssueEnricherResult{IssueCount: len(findings), Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
