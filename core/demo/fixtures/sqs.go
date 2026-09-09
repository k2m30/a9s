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

// sqsPublicPolicyQueueURL is the same queue addressed the way
// GetQueueAttributes wants it; the row itself is keyed by queue name.
const sqsPublicPolicyQueueURL = "https://sqs.us-east-1.amazonaws.com/123456789012/" + SQSPublicPolicy

// SQSManagedSSE names the queue encrypted with the encryption SQS manages
// itself: SqsManagedSseEnabled is true and there is no customer key, which is
// what a queue created in the console looks like. It must carry NO encryption
// finding — an empty KmsMasterKeyId there means no customer key, not no
// encryption. Every other queue either names a key or has neither, and the
// ones with neither are the finding's failing rows.
const SQSManagedSSE = "sqs-managed-sse"

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
			// SQSPublicPolicy: the only queue whose access policy names a
			// wildcard principal. Encrypted and with a redrive policy, so
			// it trips that one row alone.
			{
				QueueURL:  sqsPublicPolicyQueueURL,
				QueueName: SQSPublicPolicy,
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "3",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:" + SQSPublicPolicy,
					"RedrivePolicy":                         `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:data-pipeline-dlq","maxReceiveCount":5}`,
					"KmsMasterKeyId":                        OrdersProdKMSKeyID,
					"Policy":                                `{"Version":"2012-10-17","Statement":[{"Sid":"AllowEveryone","Effect":"Allow","Principal":"*","Action":["sqs:ReceiveMessage","sqs:SendMessage"],"Resource":"arn:aws:sqs:us-east-1:123456789012:` + SQSPublicPolicy + `"}]}`,
				},
			},
			// SQSManagedSSE: encrypted by SQS with an AWS-owned key. A
			// redrive policy keeps it off the missing-DLQ row, so the only
			// thing this queue demonstrates is that managed encryption is
			// encryption.
			{
				QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/" + SQSManagedSSE,
				QueueName: SQSManagedSSE,
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "11",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:" + SQSManagedSSE,
					"RedrivePolicy":                         `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:data-pipeline-dlq","maxReceiveCount":5}`,
					"SqsManagedSseEnabled":                  "true",
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

func init() {
	Register(Pin{ShortName: "sqs", Rows: 7, Issues: 0, CoverageGaps: []string{"dim"}})
}
