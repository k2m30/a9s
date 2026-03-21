package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["sqs"] = sqsQueues
	demoData["sns"] = snsTopics
	demoData["sns-sub"] = snsSubscriptions
	demoData["eb-rule"] = eventBridgeRules
	demoData["kinesis"] = kinesisStreams
	demoData["msk"] = mskClusters
	demoData["sfn"] = stepFunctions
}

// sqsQueues returns demo SQS queue fixtures.
// SQS RawStruct is a string (fmt.Sprintf of attrs map), matching the production
// fetcher behavior in internal/aws/sqs.go.
func sqsQueues() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "order-processing-queue",
			Name:   "order-processing-queue",
			Status: "",
			Fields: map[string]string{
				"queue_name":         "order-processing-queue",
				"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/order-processing-queue",
				"approx_messages":    "142",
				"approx_not_visible": "8",
				"delay_seconds":      "0",
			},
			RawStruct: "map[ApproximateNumberOfMessages:142 ApproximateNumberOfMessagesNotVisible:8 CreatedTimestamp:1700000000 DelaySeconds:0 MaximumMessageSize:262144 MessageRetentionPeriod:345600 QueueArn:arn:aws:sqs:us-east-1:123456789012:order-processing-queue ReceiveMessageWaitTimeSeconds:20 VisibilityTimeout:30]",
		},
		{
			ID:     "email-notification-queue",
			Name:   "email-notification-queue",
			Status: "",
			Fields: map[string]string{
				"queue_name":         "email-notification-queue",
				"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/email-notification-queue",
				"approx_messages":    "0",
				"approx_not_visible": "0",
				"delay_seconds":      "5",
			},
			RawStruct: "map[ApproximateNumberOfMessages:0 ApproximateNumberOfMessagesNotVisible:0 CreatedTimestamp:1710000000 DelaySeconds:5 MaximumMessageSize:262144 MessageRetentionPeriod:86400 QueueArn:arn:aws:sqs:us-east-1:123456789012:email-notification-queue ReceiveMessageWaitTimeSeconds:0 VisibilityTimeout:60]",
		},
		{
			ID:     "data-pipeline-dlq",
			Name:   "data-pipeline-dlq",
			Status: "",
			Fields: map[string]string{
				"queue_name":         "data-pipeline-dlq",
				"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/data-pipeline-dlq",
				"approx_messages":    "23",
				"approx_not_visible": "0",
				"delay_seconds":      "0",
			},
			RawStruct: "map[ApproximateNumberOfMessages:23 ApproximateNumberOfMessagesNotVisible:0 CreatedTimestamp:1705000000 DelaySeconds:0 MaximumMessageSize:262144 MessageRetentionPeriod:1209600 QueueArn:arn:aws:sqs:us-east-1:123456789012:data-pipeline-dlq ReceiveMessageWaitTimeSeconds:0 VisibilityTimeout:30]",
		},
		{
			ID:     "webhook-ingest-queue.fifo",
			Name:   "webhook-ingest-queue.fifo",
			Status: "",
			Fields: map[string]string{
				"queue_name":         "webhook-ingest-queue.fifo",
				"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/webhook-ingest-queue.fifo",
				"approx_messages":    "5",
				"approx_not_visible": "2",
				"delay_seconds":      "0",
			},
			RawStruct: "map[ApproximateNumberOfMessages:5 ApproximateNumberOfMessagesNotVisible:2 ContentBasedDeduplication:true CreatedTimestamp:1715000000 DelaySeconds:0 FifoQueue:true MaximumMessageSize:262144 MessageRetentionPeriod:345600 QueueArn:arn:aws:sqs:us-east-1:123456789012:webhook-ingest-queue.fifo ReceiveMessageWaitTimeSeconds:10 VisibilityTimeout:120]",
		},
	}
}

// snsTopics returns demo SNS topic fixtures.
func snsTopics() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
			Name:   "alarm-notifications",
			Status: "",
			Fields: map[string]string{
				"topic_arn":    "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
				"display_name": "alarm-notifications",
			},
			RawStruct: snstypes.Topic{
				TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			},
		},
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:order-events",
			Name:   "order-events",
			Status: "",
			Fields: map[string]string{
				"topic_arn":    "arn:aws:sns:us-east-1:123456789012:order-events",
				"display_name": "order-events",
			},
			RawStruct: snstypes.Topic{
				TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
			},
		},
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:deploy-notifications",
			Name:   "deploy-notifications",
			Status: "",
			Fields: map[string]string{
				"topic_arn":    "arn:aws:sns:us-east-1:123456789012:deploy-notifications",
				"display_name": "deploy-notifications",
			},
			RawStruct: snstypes.Topic{
				TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications"),
			},
		},
	}
}

// snsSubscriptions returns demo SNS subscription fixtures.
func snsSubscriptions() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			Name:   "alarm-notifications",
			Status: "",
			Fields: map[string]string{
				"topic_arn":        "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
				"protocol":         "email",
				"endpoint":         "oncall@acme-corp.com",
				"subscription_arn": "arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			},
			RawStruct: snstypes.Subscription{
				TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
				Protocol:        aws.String("email"),
				Endpoint:        aws.String("oncall@acme-corp.com"),
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
				Owner:           aws.String("123456789012"),
			},
		},
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901",
			Name:   "alarm-notifications",
			Status: "",
			Fields: map[string]string{
				"topic_arn":        "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
				"protocol":         "lambda",
				"endpoint":         "arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier",
				"subscription_arn": "arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901",
			},
			RawStruct: snstypes.Subscription{
				TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
				Protocol:        aws.String("lambda"),
				Endpoint:        aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications:b2c3d4e5-f6a7-8901-bcde-f12345678901"),
				Owner:           aws.String("123456789012"),
			},
		},
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
			Name:   "order-events",
			Status: "",
			Fields: map[string]string{
				"topic_arn":        "arn:aws:sns:us-east-1:123456789012:order-events",
				"protocol":         "sqs",
				"endpoint":         "arn:aws:sqs:us-east-1:123456789012:order-processing-queue",
				"subscription_arn": "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
			},
			RawStruct: snstypes.Subscription{
				TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:order-events"),
				Protocol:        aws.String("sqs"),
				Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012"),
				Owner:           aws.String("123456789012"),
			},
		},
		{
			ID:     "arn:aws:sns:us-east-1:123456789012:deploy-notifications:d4e5f6a7-b8c9-0123-def0-234567890123",
			Name:   "deploy-notifications",
			Status: "",
			Fields: map[string]string{
				"topic_arn":        "arn:aws:sns:us-east-1:123456789012:deploy-notifications",
				"protocol":         "https",
				"endpoint":         "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX",
				"subscription_arn": "arn:aws:sns:us-east-1:123456789012:deploy-notifications:d4e5f6a7-b8c9-0123-def0-234567890123",
			},
			RawStruct: snstypes.Subscription{
				TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications"),
				Protocol:        aws.String("https"),
				Endpoint:        aws.String("https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX"),
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:deploy-notifications:d4e5f6a7-b8c9-0123-def0-234567890123"),
				Owner:           aws.String("123456789012"),
			},
		},
	}
}

// eventBridgeRules returns demo EventBridge rule fixtures.
func eventBridgeRules() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "nightly-db-backup",
			Name:   "nightly-db-backup",
			Status: "ENABLED",
			Fields: map[string]string{
				"name":        "nightly-db-backup",
				"state":       "ENABLED",
				"event_bus":   "default",
				"schedule":    "cron(0 2 * * ? *)",
				"description": "Triggers nightly database backup at 2 AM UTC",
			},
			RawStruct: eventbridgetypes.Rule{
				Name:               aws.String("nightly-db-backup"),
				Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/nightly-db-backup"),
				State:              eventbridgetypes.RuleStateEnabled,
				EventBusName:       aws.String("default"),
				ScheduleExpression: aws.String("cron(0 2 * * ? *)"),
				Description:        aws.String("Triggers nightly database backup at 2 AM UTC"),
			},
		},
		{
			ID:     "ec2-state-change-handler",
			Name:   "ec2-state-change-handler",
			Status: "ENABLED",
			Fields: map[string]string{
				"name":        "ec2-state-change-handler",
				"state":       "ENABLED",
				"event_bus":   "default",
				"schedule":    "",
				"description": "Routes EC2 instance state changes to SNS",
			},
			RawStruct: eventbridgetypes.Rule{
				Name:         aws.String("ec2-state-change-handler"),
				Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/ec2-state-change-handler"),
				State:        eventbridgetypes.RuleStateEnabled,
				EventBusName: aws.String("default"),
				Description:  aws.String("Routes EC2 instance state changes to SNS"),
				EventPattern: aws.String(`{"source":["aws.ec2"],"detail-type":["EC2 Instance State-change Notification"]}`),
			},
		},
		{
			ID:     "cost-anomaly-detector",
			Name:   "cost-anomaly-detector",
			Status: "ENABLED",
			Fields: map[string]string{
				"name":        "cost-anomaly-detector",
				"state":       "ENABLED",
				"event_bus":   "default",
				"schedule":    "rate(1 hour)",
				"description": "Checks for cost anomalies every hour",
			},
			RawStruct: eventbridgetypes.Rule{
				Name:               aws.String("cost-anomaly-detector"),
				Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/cost-anomaly-detector"),
				State:              eventbridgetypes.RuleStateEnabled,
				EventBusName:       aws.String("default"),
				ScheduleExpression: aws.String("rate(1 hour)"),
				Description:        aws.String("Checks for cost anomalies every hour"),
			},
		},
		{
			ID:     "staging-cleanup-rule",
			Name:   "staging-cleanup-rule",
			Status: "DISABLED",
			Fields: map[string]string{
				"name":        "staging-cleanup-rule",
				"state":       "DISABLED",
				"event_bus":   "default",
				"schedule":    "cron(0 0 ? * SUN *)",
				"description": "Weekly staging environment cleanup (disabled)",
			},
			RawStruct: eventbridgetypes.Rule{
				Name:               aws.String("staging-cleanup-rule"),
				Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/staging-cleanup-rule"),
				State:              eventbridgetypes.RuleStateDisabled,
				EventBusName:       aws.String("default"),
				ScheduleExpression: aws.String("cron(0 0 ? * SUN *)"),
				Description:        aws.String("Weekly staging environment cleanup (disabled)"),
			},
		},
	}
}

// kinesisStreams returns demo Kinesis stream fixtures.
func kinesisStreams() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "clickstream-ingest",
			Name:   "clickstream-ingest",
			Status: "ACTIVE",
			Fields: map[string]string{
				"stream_name":   "clickstream-ingest",
				"status":        "ACTIVE",
				"stream_arn":    "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest",
				"creation_time": "2025-06-15 10:30:00",
				"stream_mode":   "ON_DEMAND",
			},
			RawStruct: kinesistypes.StreamSummary{
				StreamName:              aws.String("clickstream-ingest"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"),
				StreamStatus:            kinesistypes.StreamStatusActive,
				StreamCreationTimestamp: aws.Time(mustParseTime("2025-06-15T10:30:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeOnDemand,
				},
			},
		},
		{
			ID:     "order-events-stream",
			Name:   "order-events-stream",
			Status: "ACTIVE",
			Fields: map[string]string{
				"stream_name":   "order-events-stream",
				"status":        "ACTIVE",
				"stream_arn":    "arn:aws:kinesis:us-east-1:123456789012:stream/order-events-stream",
				"creation_time": "2025-03-01 08:00:00",
				"stream_mode":   "PROVISIONED",
			},
			RawStruct: kinesistypes.StreamSummary{
				StreamName:              aws.String("order-events-stream"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/order-events-stream"),
				StreamStatus:            kinesistypes.StreamStatusActive,
				StreamCreationTimestamp: aws.Time(mustParseTime("2025-03-01T08:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeProvisioned,
				},
			},
		},
		{
			ID:     "audit-log-stream",
			Name:   "audit-log-stream",
			Status: "CREATING",
			Fields: map[string]string{
				"stream_name":   "audit-log-stream",
				"status":        "CREATING",
				"stream_arn":    "arn:aws:kinesis:us-east-1:123456789012:stream/audit-log-stream",
				"creation_time": "2026-03-21 09:00:00",
				"stream_mode":   "ON_DEMAND",
			},
			RawStruct: kinesistypes.StreamSummary{
				StreamName:              aws.String("audit-log-stream"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/audit-log-stream"),
				StreamStatus:            kinesistypes.StreamStatusCreating,
				StreamCreationTimestamp: aws.Time(mustParseTime("2026-03-21T09:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeOnDemand,
				},
			},
		},
	}
}

// mskClusters returns demo MSK (Managed Streaming for Kafka) cluster fixtures.
func mskClusters() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-events-prod",
			Name:   "acme-events-prod",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name": "acme-events-prod",
				"cluster_type": "PROVISIONED",
				"state":        "ACTIVE",
				"version":      "K3AEGXET",
			},
			RawStruct: kafkatypes.Cluster{
				ClusterName:    aws.String("acme-events-prod"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/a1b2c3d4"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateActive,
				CurrentVersion: aws.String("K3AEGXET"),
				CreationTime:   aws.Time(mustParseTime("2025-04-10T14:00:00+00:00")),
			},
		},
		{
			ID:     "data-pipeline-kafka",
			Name:   "data-pipeline-kafka",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name": "data-pipeline-kafka",
				"cluster_type": "SERVERLESS",
				"state":        "ACTIVE",
				"version":      "K7BFGT2P",
			},
			RawStruct: kafkatypes.Cluster{
				ClusterName:    aws.String("data-pipeline-kafka"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/data-pipeline-kafka/e5f6a7b8"),
				ClusterType:    kafkatypes.ClusterTypeServerless,
				State:          kafkatypes.ClusterStateActive,
				CurrentVersion: aws.String("K7BFGT2P"),
				CreationTime:   aws.Time(mustParseTime("2025-09-20T11:30:00+00:00")),
			},
		},
		{
			ID:     "staging-events",
			Name:   "staging-events",
			Status: "CREATING",
			Fields: map[string]string{
				"cluster_name": "staging-events",
				"cluster_type": "PROVISIONED",
				"state":        "CREATING",
				"version":      "K1INITIAL",
			},
			RawStruct: kafkatypes.Cluster{
				ClusterName:    aws.String("staging-events"),
				ClusterArn:     aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/staging-events/c9d0e1f2"),
				ClusterType:    kafkatypes.ClusterTypeProvisioned,
				State:          kafkatypes.ClusterStateCreating,
				CurrentVersion: aws.String("K1INITIAL"),
				CreationTime:   aws.Time(mustParseTime("2026-03-20T16:00:00+00:00")),
			},
		},
	}
}

// stepFunctions returns demo Step Functions state machine fixtures.
func stepFunctions() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "order-fulfillment-workflow",
			Name:   "order-fulfillment-workflow",
			Status: "",
			Fields: map[string]string{
				"name":          "order-fulfillment-workflow",
				"type":          "STANDARD",
				"arn":           "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"creation_date": "2025-05-12 09:15:00",
			},
			RawStruct: sfntypes.StateMachineListItem{
				Name:            aws.String("order-fulfillment-workflow"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 5, 12, 9, 15, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "data-pipeline-orchestrator",
			Name:   "data-pipeline-orchestrator",
			Status: "",
			Fields: map[string]string{
				"name":          "data-pipeline-orchestrator",
				"type":          "STANDARD",
				"arn":           "arn:aws:states:us-east-1:123456789012:stateMachine:data-pipeline-orchestrator",
				"creation_date": "2025-08-03 14:22:00",
			},
			RawStruct: sfntypes.StateMachineListItem{
				Name:            aws.String("data-pipeline-orchestrator"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:data-pipeline-orchestrator"),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 8, 3, 14, 22, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "payment-validation",
			Name:   "payment-validation",
			Status: "",
			Fields: map[string]string{
				"name":          "payment-validation",
				"type":          "EXPRESS",
				"arn":           "arn:aws:states:us-east-1:123456789012:stateMachine:payment-validation",
				"creation_date": "2025-11-20 10:45:00",
			},
			RawStruct: sfntypes.StateMachineListItem{
				Name:            aws.String("payment-validation"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:payment-validation"),
				Type:            sfntypes.StateMachineTypeExpress,
				CreationDate:    aws.Time(time.Date(2025, 11, 20, 10, 45, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "user-onboarding-flow",
			Name:   "user-onboarding-flow",
			Status: "",
			Fields: map[string]string{
				"name":          "user-onboarding-flow",
				"type":          "STANDARD",
				"arn":           "arn:aws:states:us-east-1:123456789012:stateMachine:user-onboarding-flow",
				"creation_date": "2026-01-08 16:30:00",
			},
			RawStruct: sfntypes.StateMachineListItem{
				Name:            aws.String("user-onboarding-flow"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:user-onboarding-flow"),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2026, 1, 8, 16, 30, 0, 0, time.UTC)),
			},
		},
	}
}
