package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// SQSFake implements aws.SQSAPI against fixture data loaded at construction time.
type SQSFake struct {
	fix *fixtures.SQSFixtures
}

// NewSQS constructs an SQSFake backed by fixture data from the fixtures package.
func NewSQS() *SQSFake {
	return &SQSFake{fix: fixtures.NewSQSFixtures()}
}

func (f *SQSFake) ListQueues(_ context.Context, _ *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	urls := make([]string, 0, len(f.fix.Queues))
	for _, q := range f.fix.Queues {
		urls = append(urls, q.QueueURL)
	}
	return &sqs.ListQueuesOutput{QueueUrls: urls}, nil
}

func (f *SQSFake) GetQueueAttributes(_ context.Context, input *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	var queueURL string
	if input != nil && input.QueueUrl != nil {
		queueURL = *input.QueueUrl
	}
	for _, q := range f.fix.Queues {
		if q.QueueURL == queueURL {
			attrs := make(map[string]string, len(q.Attributes))
			for k, v := range q.Attributes {
				attrs[k] = v
			}
			return &sqs.GetQueueAttributesOutput{Attributes: attrs}, nil
		}
	}
	return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{}}, nil
}
