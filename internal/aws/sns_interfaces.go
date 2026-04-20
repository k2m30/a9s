package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSGetTopicAttributesAPI defines the interface for the SNS GetTopicAttributes
// operation. Used by sns→kms (KmsMasterKeyId) and sns→role (Policy document).
type SNSGetTopicAttributesAPI interface {
	GetTopicAttributes(ctx context.Context, params *sns.GetTopicAttributesInput, optFns ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error)
}

// SNSListTopicsAPI defines the interface for the SNS ListTopics operation.
type SNSListTopicsAPI interface {
	ListTopics(ctx context.Context, params *sns.ListTopicsInput, optFns ...func(*sns.Options)) (*sns.ListTopicsOutput, error)
}

// SNSListSubscriptionsAPI defines the interface for the SNS ListSubscriptions operation.
type SNSListSubscriptionsAPI interface {
	ListSubscriptions(ctx context.Context, params *sns.ListSubscriptionsInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error)
}

// SNSListSubscriptionsByTopicAPI defines the interface for the SNS ListSubscriptionsByTopic operation.
type SNSListSubscriptionsByTopicAPI interface {
	ListSubscriptionsByTopic(ctx context.Context, params *sns.ListSubscriptionsByTopicInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error)
}

// SNSListTagsForResourceAPI for sns→cfn (Tags -> aws:cloudformation:stack-name).
type SNSListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *sns.ListTagsForResourceInput, optFns ...func(*sns.Options)) (*sns.ListTagsForResourceOutput, error)
}

// SNSGetSubscriptionAttributesAPI for sns-sub→kms and sns-sub→policy.
type SNSGetSubscriptionAttributesAPI interface {
	GetSubscriptionAttributes(ctx context.Context, params *sns.GetSubscriptionAttributesInput, optFns ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error)
}

// SNSAPI is the aggregate interface covering SNS operations used by a9s
// enrichers (GetTopicAttributes, ListSubscriptionsByTopic).
//
// Operations NOT in this aggregate that fetchers/enrichers may need:
//   - ListTopics (paginated)         — used by SNS top-level fetcher
//   - ListSubscriptions (paginated)  — used by sns-sub fetcher
//
// Fetchers that need those operations type-assert clients.SNS to
// SNSListTopicsAPI / SNSListSubscriptionsAPI at the call site.
//
// *sns.Client structurally satisfies all of the above.
type SNSAPI interface {
	SNSListSubscriptionsByTopicAPI
	SNSGetTopicAttributesAPI
	SNSListTagsForResourceAPI
	SNSGetSubscriptionAttributesAPI
}
