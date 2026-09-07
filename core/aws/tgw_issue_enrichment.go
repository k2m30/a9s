// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tgw_issue_enrichment.go — Wave 2 issue enrichment for the tgw resource type.
package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// tgw canonical FindingCodes.
const (
	tgwCodeAttachmentFailed       domain.FindingCode = "tgw.attachment-failed"
	tgwCodeAttachmentTransitional domain.FindingCode = "tgw.attachment-transitional"
)

// EnrichTGWAttachments calls DescribeTransitGatewayAttachments per TGW (cap EnrichmentCap,
// per-TGW pagination up to PerParentPageCap pages) and returns a Finding for any TGW with
// attachments in a failed or transitional state.
// Severity "!" for failed/failing; severity "~" for modifying/pendingAcceptance/rollingBack.
// When multiple issues exist on the same TGW, the worst severity ("!") takes precedence.
// Per-TGW errors are aggregated and returned as a composite error alongside partial findings (E3, E4, E5).
func EnrichTGWAttachments(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.EC2 == nil {
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
		tgwID := r.ID
		if tgwID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		// Paginate attachments per TGW using NextToken.
		var allAttachments []ec2types.TransitGatewayAttachment
		var attNextToken *string
		attPages := 0
		attTruncated := false
		fetchErr := false
		var lastErr error
		for {
			if attPages >= PerParentPageCap {
				attTruncated = true
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
				fetchErr = true
				lastErr = err
				break
			}
			allAttachments = append(allAttachments, out.TransitGatewayAttachments...)
			if out.NextToken == nil {
				break
			}
			attNextToken = out.NextToken
		}

		if attTruncated || fetchErr {
			mu.Lock()
			defer mu.Unlock()
			truncated = true
			if fetchErr {
				MarkSkipped(&result, r.ID, &failures, lastErr)
			} else {
				// A page cap, not a failed call: there is no error to record.
				result.TruncatedIDs[r.ID] = true
			}
			return
		}
		// One finding per condition, every attachment in that condition a row:
		// a gateway with three failed attachments has three to fix, and naming
		// only the worst of them leaves the other two unsayable.
		attRows := map[domain.FindingCode][]domain.DetailRow{}
		issueCount := 0
		for _, att := range allAttachments {
			attID := ""
			if att.TransitGatewayAttachmentId != nil {
				attID = *att.TransitGatewayAttachmentId
			}
			state := string(att.State)
			humanState := domain.HumanizeStatusPhrase(state)
			code := domain.FindingCode("")
			tier := ""
			switch state {
			case "failed", "failing":
				code, tier = tgwCodeAttachmentFailed, "!"
			case "modifying", "pendingAcceptance", "rollingBack":
				code, tier = tgwCodeAttachmentTransitional, "~"
			default:
				continue
			}
			issueCount++
			attRows[code] = append(attRows[code],
				domain.DetailRow{Label: "Attachment", Value: attID, Tier: tier},
				domain.DetailRow{Label: "State", Value: humanState, Tier: tier})
		}
		attStatusVal := ""
		if issueCount > 0 {
			attStatusVal = fmt.Sprintf("%d issues", issueCount)
		}

		mu.Lock()
		defer mu.Unlock()
		result.FieldUpdates[tgwID] = map[string]string{
			"att_status": attStatusVal,
		}
		for _, c := range []struct {
			code domain.FindingCode
			tier string
		}{{tgwCodeAttachmentFailed, "!"}, {tgwCodeAttachmentTransitional, "~"}} {
			if rows := attRows[c.code]; len(rows) > 0 {
				setWave2Finding(&result, tgwID, c.code, catalog.Phrase(c.code), c.tier, "tgw", rows)
			}
		}
	})

	SetTruncated(&result, truncated)
	return result,
		AggregateFailures("DescribeTransitGatewayAttachments", failures, total)
}
