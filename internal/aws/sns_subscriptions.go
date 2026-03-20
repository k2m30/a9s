package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("sns-sub", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSNSSubscriptions(ctx, c.SNS)
	})
	resource.RegisterFieldKeys("sns-sub", []string{"topic_arn", "protocol", "endpoint", "subscription_arn"})
}

// FetchSNSSubscriptions calls the SNS ListSubscriptions API and converts the
// response into a slice of generic Resource structs.
func FetchSNSSubscriptions(ctx context.Context, api SNSListSubscriptionsAPI) ([]resource.Resource, error) {
	output, err := api.ListSubscriptions(ctx, &sns.ListSubscriptionsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching SNS subscriptions: %w", err)
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
			RawStruct:  sub,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
