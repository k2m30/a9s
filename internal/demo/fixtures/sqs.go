package fixtures

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// SQSFixtures holds typed fixture data for SQS.
type SQSFixtures struct {
	// Queues maps queue URL to its attributes row.
	Queues []awsclient.SQSQueueAttributesRow
}

// NewSQSFixtures constructs SQSFixtures from the canonical demo data.
func NewSQSFixtures() *SQSFixtures {
	return &SQSFixtures{
		Queues: []awsclient.SQSQueueAttributesRow{
			{
				QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/order-processing-queue",
				QueueName: "order-processing-queue",
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "142",
					"ApproximateNumberOfMessagesNotVisible": "8",
					"VisibilityTimeout":                     "30",
					"MessageRetentionPeriod":                "345600",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:order-processing-queue",
				},
			},
			{
				QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/email-notification-queue",
				QueueName: "email-notification-queue",
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "0",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"DelaySeconds":                          "5",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:email-notification-queue",
				},
			},
			{
				QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/data-pipeline-dlq",
				QueueName: "data-pipeline-dlq",
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "23",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"MessageRetentionPeriod":                "1209600",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:data-pipeline-dlq",
				},
			},
			{
				QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/webhook-ingest-queue.fifo",
				QueueName: "webhook-ingest-queue.fifo",
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "5",
					"ApproximateNumberOfMessagesNotVisible": "2",
					"FifoQueue":                             "true",
					"ContentBasedDeduplication":             "true",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:webhook-ingest-queue.fifo",
				},
			},
		},
	}
}
