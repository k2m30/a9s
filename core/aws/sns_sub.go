package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// CodeSNSSubPendingConfirmation is the canonical FindingCode for a
// subscription that has not yet confirmed its endpoint.
const CodeSNSSubPendingConfirmation domain.FindingCode = "sns-sub.state.pending-confirmation"

// CodeSNSSubDeleted is the canonical FindingCode for a subscription whose
// endpoint has been deleted.
const CodeSNSSubDeleted domain.FindingCode = "sns-sub.state.deleted"

// FetchSNSSubscriptionsPage fetches a single page of SNS subscriptions.
func FetchSNSSubscriptionsPage(ctx context.Context, api SNSListSubscriptionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sns.ListSubscriptionsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListSubscriptions(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SNS subscriptions: %w", err)
	}

	var resources []resource.Resource

	for _, sub := range output.Subscriptions {
		subscriptionArn := ""
		if sub.SubscriptionArn != nil {
			subscriptionArn = *sub.SubscriptionArn
		}

		topicArn := ""
		if sub.TopicArn != nil {
			topicArn = *sub.TopicArn
		}

		protocol := ""
		if sub.Protocol != nil {
			protocol = *sub.Protocol
		}

		endpoint := ""
		if sub.Endpoint != nil {
			endpoint = *sub.Endpoint
		}

		// Extract topic name from TopicArn (last segment after ":")
		topicName := topicArn
		if parts := strings.Split(topicArn, ":"); len(parts) > 0 {
			topicName = parts[len(parts)-1]
		}

		r := resource.Resource{
			ID:   subscriptionArn,
			Name: topicName,
			Fields: map[string]string{
				"topic_arn":        topicArn,
				"protocol":         protocol,
				"endpoint":         endpoint,
				"subscription_arn": subscriptionArn,
			},
			Findings:  snsSubStateFindings(subscriptionArn),
			RawStruct: sub,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}
	// SNS ListSubscriptions may return a NextToken even with 0 results (known API quirk).
	if len(resources) == 0 {
		isTruncated = false
		nextToken = ""
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// snsSubStateFindings mirrors colorSNSSub's own precedence: AWS returns the
// literal strings "PendingConfirmation" / "Deleted" AS the SubscriptionArn
// value for subscriptions in those states.
func snsSubStateFindings(subscriptionArn string) []domain.Finding {
	switch subscriptionArn {
	case "PendingConfirmation":
		return []domain.Finding{{
			Code: CodeSNSSubPendingConfirmation, Phrase: "endpoint has not confirmed the subscription",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case "Deleted":
		return []domain.Finding{{
			Code: CodeSNSSubDeleted, Phrase: "endpoint deleted",
			Severity: domain.SevDim, Source: "wave1",
		}}
	}
	return nil
}
