// sqs_issue_enrichment.go — Wave 2 issue enrichment for the sqs resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// EnrichSQSAttributes calls GetQueueAttributes per queue (cap EnrichmentCap)
// to surface missing DLQ and missing KMS encryption as Wave 2 findings.
// Per-queue errors set Truncated=true + TruncatedIDs[id]=true for the affected
// queue (per-row `?` marker) AND aggregate into the returned composite error
// via AggregateFailures so the operator sees the failure in the error log (!).
// Partial findings are returned alongside the composite error on partial fail.
func EnrichSQSAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.SQS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
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
			truncatedIDs[r.ID] = true
			continue
		}
		_, hasDLQ := out.Attributes["RedrivePolicy"]
		dlqVal := "no"
		if hasDLQ {
			dlqVal = "yes"
		}
		fieldUpdates[r.ID] = map[string]string{
			"dlq": dlqVal,
		}
		var rows []resource.FindingRow
		if !hasDLQ {
			rows = append(rows, resource.FindingRow{
				Label: "DLQ",
				Value: "no DLQ configured",
				Tier:  "~",
			})
		}
		if _, ok := out.Attributes["KmsMasterKeyId"]; !ok {
			rows = append(rows, resource.FindingRow{
				Label: "Encryption",
				Value: "no KMS encryption configured",
				Tier:  "~",
			})
		}
		if len(rows) == 0 {
			continue
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates},
		AggregateFailures("sqs-enrich: GetQueueAttributes", failures, total)
}
