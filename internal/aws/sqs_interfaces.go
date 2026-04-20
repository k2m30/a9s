package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSListQueuesAPI defines the interface for the SQS ListQueues operation.
type SQSListQueuesAPI interface {
	ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
}

// SQSGetQueueAttributesAPI defines the interface for the SQS GetQueueAttributes operation.
type SQSGetQueueAttributesAPI interface {
	GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
}

// SQSAPI is the aggregate interface covering SQS operations used by a9s enrichers.
// Fetchers that need ListQueues perform a runtime type assertion to SQSListQueuesAPI.
// *sqs.Client structurally satisfies this interface.
type SQSAPI interface {
	SQSGetQueueAttributesAPI
}
