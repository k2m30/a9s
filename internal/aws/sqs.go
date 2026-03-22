package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("sqs", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSQSQueues(ctx, c.SQS, c.SQS)
	})
	resource.RegisterFieldKeys("sqs", []string{"queue_name", "queue_url", "approx_messages", "approx_not_visible", "delay_seconds"})
}

// FetchSQSQueues performs a two-step fetch: ListQueues to get URLs,
// then GetQueueAttributes per queue for details.
func FetchSQSQueues(ctx context.Context, listAPI SQSListQueuesAPI, attrAPI SQSGetQueueAttributesAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListQueues(ctx, &sqs.ListQueuesInput{})
	if err != nil {
		return nil, fmt.Errorf("listing SQS queues: %w", err)
	}

	var resources []resource.Resource

	for _, queueURL := range listOutput.QueueUrls {
		attrOutput, err := attrAPI.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl: &queueURL,
			AttributeNames: []sqstypes.QueueAttributeName{
				sqstypes.QueueAttributeNameAll,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("fetching SQS queue attributes for %s: %w", queueURL, err)
		}

		attrs := attrOutput.Attributes

		// Extract queue name from URL (last segment after /)
		queueName := queueURL
		if parts := strings.Split(queueURL, "/"); len(parts) > 0 {
			queueName = parts[len(parts)-1]
		}

		approxMessages := attrs["ApproximateNumberOfMessages"]
		approxNotVisible := attrs["ApproximateNumberOfMessagesNotVisible"]
		delaySeconds := attrs["DelaySeconds"]

		r := resource.Resource{
			ID:     queueName,
			Name:   queueName,
			Status: "",
			Fields: map[string]string{
				"queue_name":         queueName,
				"queue_url":          queueURL,
				"approx_messages":    approxMessages,
				"approx_not_visible": approxNotVisible,
				"delay_seconds":      delaySeconds,
			},
			RawStruct:  fmt.Sprintf("%v", attrs),
		}

		resources = append(resources, r)
	}

	return resources, nil
}
