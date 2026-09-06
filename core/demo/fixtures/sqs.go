// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// SQSPublicPolicy names the one demo queue whose access policy grants send
// or receive to every principal (sqs.public-policy). Every other queue's
// Policy attribute is either absent or account-scoped.
const SQSPublicPolicy = "sqs-public-policy"

// SQSFixtures holds typed fixture data for SQS.
type SQSFixtures struct {
	// Queues maps queue URL to its attributes row.
	Queues []awsclient.SQSQueueAttributesRow
}

// NewSQSFixtures constructs SQSFixtures from the canonical demo data.
var sharedSQSFixtures = sync.OnceValue(func() *SQSFixtures {
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
					// RedrivePolicy — required for the sqs:sqs related-panel pivot
					// (checkSQSSQS forward direction: this queue's DLQ target).
					"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:data-pipeline-dlq","maxReceiveCount":5}`,
					// KmsMasterKeyId — required for the sqs:kms related-panel pivot
					// (checkSQSKMS).
					"KmsMasterKeyId": OrdersProdKMSKeyID,
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
			// S3 healthy-bucket dead-letter queue (checkS3SQS pivot).
			{
				QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/" + S3DLQueueName,
				QueueName: S3DLQueueName,
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "0",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"MessageRetentionPeriod":                "1209600",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:" + S3DLQueueName,
				},
			},
		},
	}
})

func NewSQSFixtures() *SQSFixtures {
	return sharedSQSFixtures()
}
