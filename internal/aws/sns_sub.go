package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	catalog.RegisterFieldKeys("sns-sub", []string{"topic_arn", "protocol", "endpoint", "subscription_arn"})

	catalog.RegisterFetcher("sns-sub", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		subsAPI, ok := c.SNS.(SNSListSubscriptionsAPI)
		if !ok {
			return resource.FetchResult{}, fmt.Errorf("SNS client does not support ListSubscriptions")
		}
		return FetchSNSSubscriptionsPage(ctx, subsAPI, continuationToken)
	})
}

// FetchSNSSubscriptions calls the SNS ListSubscriptions API and converts the
// response into a slice of generic Resource structs.
func FetchSNSSubscriptions(ctx context.Context, api SNSListSubscriptionsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSNSSubscriptionsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

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
			ID:     subscriptionArn,
			Name:   topicName,
			Status: "",
			Fields: map[string]string{
				"topic_arn":        topicArn,
				"protocol":         protocol,
				"endpoint":         endpoint,
				"subscription_arn": subscriptionArn,
			},
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
