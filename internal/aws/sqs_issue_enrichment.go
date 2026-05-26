// sqs_issue_enrichment.go — Wave 2 issue enrichment for the sqs resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sqs canonical FindingCodes.
const (
	sqsCodeMissingDLQ domain.FindingCode = "sqs.missing-dlq"
)

// EnrichSQSAttributes calls GetQueueAttributes per queue (cap EnrichmentCap)
// to surface missing DLQ and missing KMS encryption as Wave 2 findings.
// Per-queue errors set Truncated=true + TruncatedIDs[id]=true for the affected
// queue (per-row `?` marker) AND aggregate into the returned composite error
// via AggregateFailures so the operator sees the failure in the error log (!).
// Partial findings are returned alongside the composite error on partial fail.
func EnrichSQSAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SQS == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		queueURL := r.Fields["queue_url"]
		if queueURL == "" {
			continue
		}
		total++
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
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
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
			continue
		}
		setWave2Finding(&result, r.ID, sqsCodeMissingDLQ, rows[0].Value, "~", "sqs", rows)
	}
	result.IssueCount = 0
	result.Truncated = truncated
	return result,
		AggregateFailures("sqs-enrich: GetQueueAttributes", failures, total)
}
