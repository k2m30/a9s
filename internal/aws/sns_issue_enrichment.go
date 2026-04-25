// sns_issue_enrichment.go — Wave 2 issue enrichment for the sns resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("sns", EnrichSNSSubscriptions, 100)
	resource.RegisterIssueEnricherFieldKeys("sns", []string{"subs_count"})
}

// EnrichSNSSubscriptions calls ListSubscriptionsByTopic per topic (cap EnrichmentCap)
// to surface orphan topics and topics with all-pending-confirmation subscribers.
// Per-topic errors are treated as truncated (skip silently).
func EnrichSNSSubscriptions(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.SNS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
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
				truncatedIDs[r.ID] = true
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
		fieldUpdates[r.ID] = map[string]string{
			"subs_count": resource.FormatExact(len(subs)),
		}
		if len(subs) == 0 {
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "topic has no subscribers",
				Rows: []resource.FindingRow{
					{Label: "Subscribers", Value: "topic has no subscribers", Tier: "~"},
				},
			}
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
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "all pending confirmation",
				Rows: []resource.FindingRow{
					{Label: "Subscribers", Value: "all pending confirmation", Tier: "~"},
				},
			}
		}
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
