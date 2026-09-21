// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tgw_issue_enrichment.go — Wave 2 issue enrichment for the tgw resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// tgw canonical FindingCodes.
const (
	tgwCodeAttachmentFailed       domain.FindingCode = "tgw.attachment-failed"
	tgwCodeAttachmentTransitional domain.FindingCode = "tgw.attachment-transitional"
)

// tgwPendingAcceptanceGrace is how long a cross-account attachment may sit
// unaccepted before it reads as stuck rather than in progress.
const tgwPendingAcceptanceGrace = 24 * time.Hour

// EnrichTGWAttachments calls DescribeTransitGatewayAttachments per TGW (cap EnrichmentCap,
// per-TGW pagination up to PerParentPageCap pages) and returns a Finding for any TGW with
// attachments in a failed or transitional state.
// Severity "!" for failed/failing/rejected/rejecting; severity "~" for
// modifying/rollingBack and for pendingAcceptance older than tgwPendingAcceptanceGrace.
// When multiple issues exist on the same TGW, the worst severity ("!") takes precedence.
// Per-TGW errors are aggregated and returned as a composite error alongside partial findings.
func EnrichTGWAttachments(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.EC2 == nil {
		return result, nil
	}
	truncated := false
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		tgwID := r.ID
		if tgwID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		allAttachments, complete, lastErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.TransitGatewayAttachment, *string, error) {
			out, err := clients.EC2.DescribeTransitGatewayAttachments(ctx, &ec2svc.DescribeTransitGatewayAttachmentsInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("transit-gateway-id"), Values: []string{tgwID}},
				},
				NextToken: token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.TransitGatewayAttachments, out.NextToken, nil
		})

		if !complete {
			mu.Lock()
			defer mu.Unlock()
			truncated = true
			if lastErr != nil {
				MarkSkipped(&result, r.ID, &failures, lastErr)
			} else {
				markUninspected(&result, r.ID, CheckCap)
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
			switch state {
			case "failed", "failing", "rejected", "rejecting":
				code = tgwCodeAttachmentFailed
			case "pendingAcceptance":
				// The owning account has a request waiting, which is the
				// normal first minutes of every cross-account attachment.
				// Only one left sitting is something to chase.
				if att.CreationTime == nil || time.Since(*att.CreationTime) <= tgwPendingAcceptanceGrace {
					continue
				}
				code = tgwCodeAttachmentTransitional
			case "modifying", "rollingBack":
				code = tgwCodeAttachmentTransitional
			default:
				continue
			}
			tier := tierOf(code)
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
				setWave2Finding(&result, tgwID, c.code, rows)
			}
		}
	})

	SetTruncated(&result, truncated)
	return result, errors.Join(loopErr, AggregateFailures("DescribeTransitGatewayAttachments", failures, total))
}
