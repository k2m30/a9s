// tgw_issue_enrichment.go — Wave 2 issue enrichment for the tgw resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tgw canonical FindingCodes.
const (
	tgwCodeAttachmentFailed      domain.FindingCode = "tgw.attachment-failed"
	tgwCodeAttachmentTransitional domain.FindingCode = "tgw.attachment-transitional"
)

// worstTGWFinding holds the temporary state for tracking the worst attachment finding
// across all attachments for a single TGW during the enrichment loop.
type worstTGWFinding struct {
	code    domain.FindingCode
	summary string
	glyph   string
	rows    []domain.DetailRow
}

// EnrichTGWAttachments calls DescribeTransitGatewayAttachments per TGW (cap EnrichmentCap,
// per-TGW pagination up to PerParentPageCap pages) and returns a Finding for any TGW with
// attachments in a failed or transitional state.
// Severity "!" for failed/failing; severity "~" for modifying/pendingAcceptance/rollingBack.
// When multiple issues exist on the same TGW, the worst severity ("!") takes precedence.
// Per-TGW errors are aggregated and returned as a composite error alongside partial findings (E3, E4, E5).
func EnrichTGWAttachments(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.EC2 == nil {
		return result, nil
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
				result.TruncatedIDs[r.ID] = true
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
				result.TruncatedIDs[r.ID] = true
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
		var worst *worstTGWFinding
		issueCount := 0
		for _, att := range allAttachments {
			attID := ""
			if att.TransitGatewayAttachmentId != nil {
				attID = *att.TransitGatewayAttachmentId
			}
			state := string(att.State)
			var candidate *worstTGWFinding
			switch state {
			case "failed", "failing":
				issueCount++
				candidate = &worstTGWFinding{
					code:    tgwCodeAttachmentFailed,
					summary: fmt.Sprintf("attachment %s failed", attID),
					glyph:   "!",
					rows: []domain.DetailRow{
						{Label: "Attachment", Value: attID, Tier: "!"},
						{Label: "State", Value: state, Tier: "!"},
					},
				}
			case "modifying", "pendingAcceptance", "rollingBack":
				issueCount++
				candidate = &worstTGWFinding{
					code:    tgwCodeAttachmentTransitional,
					summary: fmt.Sprintf("attachment %s %s", attID, state),
					glyph:   "~",
					rows: []domain.DetailRow{
						{Label: "Attachment", Value: attID, Tier: "~"},
						{Label: "State", Value: state, Tier: "~"},
					},
				}
			}
			if candidate == nil {
				continue
			}
			if worst == nil || (worst.glyph != "!" && candidate.glyph == "!") {
				worst = candidate
			}
		}
		attStatusVal := ""
		if issueCount > 0 {
			attStatusVal = fmt.Sprintf("%d issues", issueCount)
		}
		result.FieldUpdates[tgwID] = map[string]string{
			"att_status": attStatusVal,
		}
		if worst != nil {
			setWave2Finding(&result, tgwID, worst.code, worst.summary, worst.glyph, "tgw", worst.rows)
		}
	}
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result,
		AggregateFailures("tgw-enrich: DescribeTransitGatewayAttachments", failures, total)
}
