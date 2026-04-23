package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSFixtures holds typed fixture data for SNS.
type SNSFixtures struct {
	Topics        []snstypes.Topic
	Subscriptions []snstypes.Subscription
	// SubscriptionsByTopic maps topic ARN to its subscriptions.
	SubscriptionsByTopic map[string][]snstypes.Subscription
}

// NewSNSFixtures constructs SNSFixtures from the canonical demo data.
func NewSNSFixtures() *SNSFixtures {
	topics := []snstypes.Topic{
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications")},
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events")},
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications")},
		// S3 healthy-bucket event notifications topic (checkS3SNS pivot).
		{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:" + S3EventsTopicName)},
		// Redis prod ops pager topic — required for redis→sns related-panel pivot.
		// The prod-redis-sessions member cluster NotificationConfiguration.TopicArn
		// points here so checkRedisSNS resolves a non-zero count for the demo showroom.
		{TopicArn: aws.String(ProdRedisSNSTopicARN)},
	}

	subscriptions := []snstypes.Subscription{
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			Protocol:        aws.String("email"),
			Endpoint:        aws.String("oncall@acme-corp.com"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
			Owner:           aws.String("123456789012"),
		},
		// Issue: SubscriptionArn=="PendingConfirmation" → Warning (never confirmed)
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
			Protocol:        aws.String("email"),
			Endpoint:        aws.String("pending-recipient@partner.example.com"),
			SubscriptionArn: aws.String("PendingConfirmation"),
			Owner:           aws.String("123456789012"),
		},
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			Protocol:        aws.String("lambda"),
			Endpoint:        aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901"),
			Owner:           aws.String("123456789012"),
		},
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
			Protocol:        aws.String("sqs"),
			Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012"),
			Owner:           aws.String("123456789012"),
		},
		{
			TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications"),
			Protocol:        aws.String("https"),
			Endpoint:        aws.String("https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX"),
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications:d4e5f6a7-b8c9-0123-def0-234567890123"),
			Owner:           aws.String("123456789012"),
		},
	}

	subsByTopic := map[string][]snstypes.Subscription{
		"arn:aws:sns:us-east-1:123456789012:alarm-notifications":  subscriptions[:2],
		"arn:aws:sns:us-east-1:123456789012:order-events":         subscriptions[2:4],
		"arn:aws:sns:us-east-1:123456789012:deploy-notifications": subscriptions[4:],
	}

	return &SNSFixtures{
		Topics:               topics,
		Subscriptions:        subscriptions,
		SubscriptionsByTopic: subsByTopic,
	}
}
