package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// SNSFake implements aws.SNSAPI against fixture data loaded at construction time.
type SNSFake struct {
	fix *fixtures.SNSFixtures
}

// NewSNS constructs an SNSFake backed by fixture data from the fixtures package.
func NewSNS() *SNSFake {
	return &SNSFake{fix: fixtures.NewSNSFixtures()}
}

func (f *SNSFake) ListTopics(_ context.Context, _ *sns.ListTopicsInput, _ ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
	return &sns.ListTopicsOutput{Topics: f.fix.Topics}, nil
}

func (f *SNSFake) ListSubscriptions(_ context.Context, _ *sns.ListSubscriptionsInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error) {
	return &sns.ListSubscriptionsOutput{Subscriptions: f.fix.Subscriptions}, nil
}

func (f *SNSFake) ListSubscriptionsByTopic(_ context.Context, input *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	var topicARN string
	if input != nil && input.TopicArn != nil {
		topicARN = *input.TopicArn
	}
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: f.fix.SubscriptionsByTopic[topicARN]}, nil
}
