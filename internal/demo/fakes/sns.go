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
		if err := validateARN(*input.TopicArn); err != nil {
			return nil, err
		}
		topicARN = *input.TopicArn
	}
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: f.fix.SubscriptionsByTopic[topicARN]}, nil
}

// GetTopicAttributes returns an empty attributes map — demo mode does not
// model SNS topic attributes.
func (f *SNSFake) GetTopicAttributes(_ context.Context, _ *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	return &sns.GetTopicAttributesOutput{Attributes: map[string]string{}}, nil
}

// GetSubscriptionAttributes returns an empty attributes map — demo mode does not
// model subscription attributes.
func (f *SNSFake) GetSubscriptionAttributes(_ context.Context, _ *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	return &sns.GetSubscriptionAttributesOutput{Attributes: map[string]string{}}, nil
}

// ListTagsForResource returns an empty tag list — demo mode does not model SNS tags.
func (f *SNSFake) ListTagsForResource(_ context.Context, _ *sns.ListTagsForResourceInput, _ ...func(*sns.Options)) (*sns.ListTagsForResourceOutput, error) {
	return &sns.ListTagsForResourceOutput{}, nil
}
