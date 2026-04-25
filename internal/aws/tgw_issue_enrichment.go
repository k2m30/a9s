// tgw_issue_enrichment.go — Wave 2 issue enrichment for the tgw resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("tgw", EnrichTGWAttachments, 100)
	resource.RegisterIssueEnricherFieldKeys("tgw", []string{"att_status"})
}

// EnrichTGWAttachments calls DescribeTransitGatewayAttachments per TGW (cap EnrichmentCap,
// per-TGW pagination up to PerParentPageCap pages) and returns a Finding for any TGW with
// attachments in a failed or transitional state.
// Severity "!" for failed/failing; severity "~" for modifying/pendingAcceptance/rollingBack.
// When multiple issues exist on the same TGW, the worst severity ("!") takes precedence.
// Per-TGW errors are aggregated and returned as a composite error alongside partial findings (E3, E4, E5).
func EnrichTGWAttachments(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.EC2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		tgwID := r.ID
		if tgwID == "" {
			continue
		}
		total++
		// Paginate attachments per TGW using NextToken.
		var allAttachments []ec2types.TransitGatewayAttachment
		var attNextToken *string
		attPages := 0
		attTruncated := false
		for {
			if attPages >= PerParentPageCap {
				attTruncated = true
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2svc.DescribeTransitGatewayAttachmentsOutput, error) {
				return clients.EC2.DescribeTransitGatewayAttachments(ctx, &ec2svc.DescribeTransitGatewayAttachmentsInput{
					Filters: []ec2types.Filter{
						{Name: aws.String("transit-gateway-id"), Values: []string{tgwID}},
					},
					NextToken: attNextToken,
				})
			})
			attPages++
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			allAttachments = append(allAttachments, out.TransitGatewayAttachments...)
			if out.NextToken == nil {
				break
			}
			attNextToken = out.NextToken
		}
		if attTruncated {
			continue
		}
		// Collect worst finding across all attachments for this TGW.
		// "!" severity beats "~" severity.
		var worst *resource.EnrichmentFinding
		issueCount := 0
		for _, att := range allAttachments {
			attID := ""
			if att.TransitGatewayAttachmentId != nil {
				attID = *att.TransitGatewayAttachmentId
			}
			state := string(att.State)
			var candidate *resource.EnrichmentFinding
			switch state {
			case "failed", "failing":
				issueCount++
				candidate = &resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("attachment %s failed", attID),
					Rows: []resource.FindingRow{
						{Label: "Attachment", Value: attID, Tier: "!"},
						{Label: "State", Value: state, Tier: "!"},
					},
				}
			case "modifying", "pendingAcceptance", "rollingBack":
				issueCount++
				candidate = &resource.EnrichmentFinding{
					Severity: "~",
					Summary:  fmt.Sprintf("attachment %s %s", attID, state),
					Rows: []resource.FindingRow{
						{Label: "Attachment", Value: attID, Tier: "~"},
						{Label: "State", Value: state, Tier: "~"},
					},
				}
			}
			if candidate == nil {
				continue
			}
			if worst == nil || (worst.Severity != "!" && candidate.Severity == "!") {
				worst = candidate
			}
		}
		attStatusVal := ""
		if issueCount > 0 {
			attStatusVal = fmt.Sprintf("%d issues", issueCount)
		}
		fieldUpdates[tgwID] = map[string]string{
			"att_status": attStatusVal,
		}
		if worst != nil {
			findings[tgwID] = *worst
		}
	}
	return IssueEnricherResult{IssueCount: len(findings), Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates},
		AggregateFailures("tgw-enrich: DescribeTransitGatewayAttachments", failures, total)
}
