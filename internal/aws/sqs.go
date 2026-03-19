package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("sqs", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSQSQueues(ctx, c.SQS, c.SQS)
	})
}

// FetchSQSQueues performs a two-step fetch: ListQueues to get URLs,
// then GetQueueAttributes per queue for details.
func FetchSQSQueues(ctx context.Context, listAPI SQSListQueuesAPI, attrAPI SQSGetQueueAttributesAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListQueues(ctx, &sqs.ListQueuesInput{})
	if err != nil {
		return nil, err
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
			return nil, err
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

		detail := map[string]string{
			"Queue Name":              queueName,
			"Queue URL":               queueURL,
			"Approximate Messages":    approxMessages,
			"Messages Not Visible":    approxNotVisible,
			"Delay Seconds":           delaySeconds,
		}

		if v, ok := attrs["QueueArn"]; ok {
			detail["ARN"] = v
		}
		if v, ok := attrs["VisibilityTimeout"]; ok {
			detail["Visibility Timeout"] = v
		}
		if v, ok := attrs["MaximumMessageSize"]; ok {
			detail["Max Message Size"] = v
		}
		if v, ok := attrs["MessageRetentionPeriod"]; ok {
			detail["Retention Period"] = v
		}
		if v, ok := attrs["CreatedTimestamp"]; ok {
			detail["Created"] = v
		}
		if v, ok := attrs["RedrivePolicy"]; ok {
			detail["Redrive Policy"] = v
		}

		rawJSON := ""
		raw := map[string]interface{}{
			"QueueUrl":   queueURL,
			"Attributes": attrs,
		}
		if jsonBytes, err := json.MarshalIndent(raw, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  fmt.Sprintf("%v", attrs),
		}

		resources = append(resources, r)
	}

	return resources, nil
}
