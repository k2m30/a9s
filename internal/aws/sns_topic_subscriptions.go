package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	// Child view: SNS Topic Subscriptions
	resource.RegisterFieldKeys("sns_subscriptions", []string{
		"protocol", "endpoint", "confirmation_status", "owner", "subscription_arn", "topic_arn",
	})

	resource.RegisterPaginatedChild("sns_subscriptions", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSNSTopicSubscriptions(ctx, c.SNS, parentCtx["topic_arn"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "SNS Subscriptions",
		ShortName: "sns_subscriptions",
		Columns:   resource.SnsSubscriptionColumns(),
		CopyField: "endpoint",
	})
}

// FetchSNSTopicSubscriptions calls the SNS ListSubscriptionsByTopic API and
// converts the response into a FetchResult with pagination support. Each call
// returns up to 200 results. When the cap is reached and more pages exist,
// FetchResult.Pagination.IsTruncated is set to true with a NextToken for
// continuation.
func FetchSNSTopicSubscriptions(ctx context.Context, api SNSListSubscriptionsByTopicAPI, topicArn string, continuationToken string) (resource.FetchResult, error) {
	const maxSubscriptions = 200

	var resources []resource.Resource
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	for {
		output, err := api.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
			TopicArn:  &topicArn,
			NextToken: nextToken,
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("fetching SNS topic subscriptions: %w", err)
		}

		for _, sub := range output.Subscriptions {
			resources = append(resources, convertSNSSubscription(sub))
		}

		if len(resources) >= maxSubscriptions {
			apiNextToken := ""
			if output.NextToken != nil {
				apiNextToken = *output.NextToken
			}
			return resource.FetchResult{
				Resources: resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: apiNextToken != "",
					NextToken:   apiNextToken,
					PageSize:    len(resources),
				},
			}, nil
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}

// convertSNSSubscription converts an SNS Subscription into a generic Resource.
func convertSNSSubscription(sub snstypes.Subscription) resource.Resource {
	protocol := ""
	if sub.Protocol != nil {
		protocol = *sub.Protocol
	}

	endpoint := ""
	if sub.Endpoint != nil {
		endpoint = *sub.Endpoint
	}

	owner := ""
	if sub.Owner != nil {
		owner = *sub.Owner
	}

	subscriptionArn := ""
	if sub.SubscriptionArn != nil {
		subscriptionArn = *sub.SubscriptionArn
	}

	topicArn := ""
	if sub.TopicArn != nil {
		topicArn = *sub.TopicArn
	}

	// Determine confirmation status and ID
	confirmationStatus := "Confirmed"
	id := subscriptionArn
	if subscriptionArn == "PendingConfirmation" {
		confirmationStatus = "PendingConfirmation"
		id = fmt.Sprintf("pending/%s/%s", protocol, endpoint)
	}

	return resource.Resource{
		ID:     id,
		Name:   endpoint,
		Status: "",
		Fields: map[string]string{
			"protocol":            protocol,
			"endpoint":            endpoint,
			"confirmation_status": confirmationStatus,
			"owner":               owner,
			"subscription_arn":    subscriptionArn,
			"topic_arn":           topicArn,
		},
		RawStruct: sub,
	}
}
