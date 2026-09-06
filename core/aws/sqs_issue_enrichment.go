// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sqs_issue_enrichment.go — Wave 2 issue enrichment for the sqs resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sqs canonical FindingCodes.
const (
	sqsCodeMissingDLQ   domain.FindingCode = "sqs.missing-dlq"
	sqsCodePublicPolicy domain.FindingCode = "sqs.public-policy"
)

// EnrichSQSAttributes calls GetQueueAttributes per queue (cap EnrichmentCap)
// to surface missing DLQ and missing KMS encryption as Wave 2 findings.
// Per-queue errors set Truncated=true + TruncatedIDs[id]=true for the affected
// queue (per-row `?` marker) AND aggregate into the returned composite error
// via AggregateFailures so the operator sees the failure in the error log (!).
// Partial findings are returned alongside the composite error on partial fail.
func EnrichSQSAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SQS == nil {
		return result, nil
	}
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		queueURL := r.Fields["queue_url"]
		if queueURL == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sqssvc.GetQueueAttributesOutput, error) {
			return clients.SQS.GetQueueAttributes(ctx, &sqssvc.GetQueueAttributesInput{
				QueueUrl: aws.String(queueURL),
				AttributeNames: []sqstypes.QueueAttributeName{
					sqstypes.QueueAttributeNameRedrivePolicy,
					sqstypes.QueueAttributeNameVisibilityTimeout,
					sqstypes.QueueAttributeNameKmsMasterKeyId,
				},
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			result.TruncatedIDs[r.ID] = true
			return
		}
		_, hasDLQ := out.Attributes["RedrivePolicy"]
		dlqVal := "no"
		if hasDLQ {
			dlqVal = "yes"
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"dlq": dlqVal,
		}
		var rows []domain.DetailRow
		if !hasDLQ {
			rows = append(rows, domain.DetailRow{
				Label: "DLQ",
				Value: "no DLQ configured",
				Tier:  "~",
			})
		}
		if _, ok := out.Attributes["KmsMasterKeyId"]; !ok {
			rows = append(rows, domain.DetailRow{
				Label: "Encryption",
				Value: "no KMS encryption configured",
				Tier:  "~",
			})
		}
		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, r.ID, sqsCodeMissingDLQ, rows[0].Value, "~", "sqs", rows)
	})
	sort.Strings(failures)
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result,
		AggregateFailures("sqs-enrich: GetQueueAttributes", failures, total)
}
