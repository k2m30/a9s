// sns_issue_enrichment.go — Wave 2 issue enrichment for the sns resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sns canonical FindingCodes.
const (
	snsCodeNoSubscribers    domain.FindingCode = "sns.no-subscribers"
	snsCodeAllPending       domain.FindingCode = "sns.all-pending-confirmation"
)

// EnrichSNSSubscriptions calls ListSubscriptionsByTopic per topic (cap EnrichmentCap)
// to surface orphan topics and topics with all-pending-confirmation subscribers.
// Per-topic errors are treated as truncated (skip silently).
func EnrichSNSSubscriptions(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SNS == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		topicARN := r.ID
		if topicARN == "" {
			continue
		}
		// Walk all pages so subs_count is exact for topics with >100 subscribers.
		var subs []snstypes.Subscription
		var nextToken *string
		pagedErr := false
		for {
			out, err := clients.SNS.ListSubscriptionsByTopic(ctx, &snssvc.ListSubscriptionsByTopicInput{
				TopicArn:  aws.String(topicARN),
				NextToken: nextToken,
			})
			if err != nil {
				truncated = true
				result.TruncatedIDs[r.ID] = true
				pagedErr = true
				break
			}
			subs = append(subs, out.Subscriptions...)
			if out.NextToken == nil || *out.NextToken == "" {
				break
			}
			nextToken = out.NextToken
		}
		if pagedErr {
			continue
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"subs_count": resource.FormatExact(len(subs)),
		}
		if len(subs) == 0 {
			setWave2Finding(&result, r.ID, snsCodeNoSubscribers, "topic has no subscribers", "~", "sns", []domain.DetailRow{
				{Label: "Subscribers", Value: "topic has no subscribers", Tier: "~"},
			})
			continue
		}
		allPending := true
		for _, sub := range subs {
			arn := ""
			if sub.SubscriptionArn != nil {
				arn = *sub.SubscriptionArn
			}
			if arn != "PendingConfirmation" {
				allPending = false
				break
			}
		}
		if allPending {
			setWave2Finding(&result, r.ID, snsCodeAllPending, "all pending confirmation", "~", "sns", []domain.DetailRow{
				{Label: "Subscribers", Value: "all pending confirmation", Tier: "~"},
			})
		}
	}
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
