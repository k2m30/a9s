package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("sns", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSNSTopics(ctx, c.SNS)
	})
	resource.RegisterFieldKeys("sns", []string{"topic_arn", "display_name"})
}

// FetchSNSTopics calls the SNS ListTopics API and converts the
// response into a slice of generic Resource structs.
func FetchSNSTopics(ctx context.Context, api SNSListTopicsAPI) ([]resource.Resource, error) {
	output, err := api.ListTopics(ctx, &sns.ListTopicsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching SNS topics: %w", err)
	}

	var resources []resource.Resource

	for _, topic := range output.Topics {
		topicArn := ""
		if topic.TopicArn != nil {
			topicArn = *topic.TopicArn
		}

		// Extract display name from ARN (last segment after :)
		displayName := topicArn
		if parts := strings.Split(topicArn, ":"); len(parts) > 0 {
			displayName = parts[len(parts)-1]
		}

		r := resource.Resource{
			ID:     topicArn,
			Name:   displayName,
			Status: "",
			Fields: map[string]string{
				"topic_arn":    topicArn,
				"display_name": displayName,
			},
			RawStruct:  topic,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
