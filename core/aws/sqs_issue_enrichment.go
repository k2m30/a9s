// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sqs_issue_enrichment.go — Wave 2 issue enrichment for the sqs resource type.
package aws

import (
	"context"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sqs canonical FindingCodes.
const (
	sqsCodeMissingDLQ   domain.FindingCode = "sqs.missing-dlq"
	sqsCodeNoKMS        domain.FindingCode = "sqs.no-kms"
	sqsCodePublicPolicy domain.FindingCode = "sqs.public-policy"
)

// S5 operator sentences for the queue posture codes above.
// EnrichSQSAttributes calls GetQueueAttributes per queue (cap EnrichmentCap)
// to surface missing DLQ and missing KMS encryption as Wave 2 findings.
// Per-queue errors set Truncated=true + TruncatedIDs[id]=true for the affected
// queue (per-row `?` marker) AND aggregate into the returned composite error
// via AggregateFailures so the operator sees the failure in the error log (!).
// Partial findings are returned alongside the composite error on partial fail.
func EnrichSQSAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SQS == nil {
		return result, nil
	}
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
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
					sqstypes.QueueAttributeNameSqsManagedSseEnabled,
					sqstypes.QueueAttributeNamePolicy,
				},
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
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
		// Each condition is its own code with its own phrase: a queue
		// missing only encryption must not report the dead-letter code.
		// Neither carries a supporting row — the phrase is the whole fact,
		// and a row repeating it is the detail block saying it twice.
		if !hasDLQ {
			setWave2Finding(&result, r.ID, sqsCodeMissingDLQ, nil)
		}
		// SSE-SQS is encryption. An empty KmsMasterKeyId says only that no
		// customer key is in use; with SqsManagedSseEnabled the queue is
		// encrypted at rest with an AWS-owned key, and it is on by default for
		// queues created in the console.
		if out.Attributes["KmsMasterKeyId"] == "" &&
			!strings.EqualFold(out.Attributes["SqsManagedSseEnabled"], "true") {
			setWave2Finding(&result, r.ID, sqsCodeNoKMS, nil)
		}
		if doc, parseErr := iampolicy.Parse(out.Attributes["Policy"]); parseErr == nil {
			if ex := iampolicy.Evaluate(doc, ownAccount); ex.Public {
				setWave2Finding(&result, r.ID, sqsCodePublicPolicy, publicPolicyRows(ex))
			}
		}
	})

	return result,
		AggregateFailures("GetQueueAttributes", failures, total)
}
