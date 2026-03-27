package demo

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["sqs"] = sqsQueues
	demoData["sns"] = snsTopics
	demoData["sns-sub"] = snsSubscriptions
	demoData["eb-rule"] = eventBridgeRules
	demoData["kinesis"] = kinesisStreams
	demoData["msk"] = mskClusters
	demoData["sfn"] = stepFunctions

	RegisterChildDemo("sns_subscriptions", func(parentCtx map[string]string) []resource.Resource {
		return snsTopicSubscriptionFixtures(parentCtx["topic_arn"])
	})

	RegisterChildDemo("sfn_executions", func(parentCtx map[string]string) []resource.Resource {
		return sfnExecutionFixtures(parentCtx["state_machine_arn"])
	})

	RegisterChildDemo("sfn_execution_history", func(_ map[string]string) []resource.Resource {
		return sfnExecutionHistoryFixtures()
	})

	RegisterChildDemo("eb_rule_targets", func(_ map[string]string) []resource.Resource {
		return ebRuleTargetFixtures()
	})
}

// sqsQueues returns demo SQS queue fixtures.
// SQS RawStruct is a string (fmt.Sprintf of attrs map), matching the production
// fetcher behavior in internal/aws/sqs.go.
func sqsQueues() []resource.Resource {
	queues := []resource.Resource{
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

	// Generate 18 more queues to reach 22 total
	msgCounts := []string{"0", "57", "0", "1203", "0", "12", "0", "89", "0", "345", "0", "0", "7", "0", "234", "0", "0", "45"}
	notVisible := []string{"0", "3", "0", "45", "0", "1", "0", "12", "0", "23", "0", "0", "2", "0", "8", "0", "0", "3"}
	delays := []string{"0", "0", "5", "0", "0", "10", "0", "0", "0", "0", "5", "0", "0", "0", "0", "10", "0", "0"}
	for i := 0; i < 18; i++ {
		name := sqsNamePool[i]
		queueURL := fmt.Sprintf("https://sqs.us-east-1.amazonaws.com/123456789012/%s", name)
		ts := 1700000000 + i*100000
		queues = append(queues, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"queue_name":         name,
				"queue_url":          queueURL,
				"approx_messages":    msgCounts[i],
				"approx_not_visible": notVisible[i],
				"delay_seconds":      delays[i],
			},
			RawStruct: fmt.Sprintf("map[ApproximateNumberOfMessages:%s ApproximateNumberOfMessagesNotVisible:%s CreatedTimestamp:%d DelaySeconds:%s MaximumMessageSize:262144 MessageRetentionPeriod:345600 QueueArn:arn:aws:sqs:us-east-1:123456789012:%s ReceiveMessageWaitTimeSeconds:0 VisibilityTimeout:30]",
				msgCounts[i], notVisible[i], ts, delays[i], name),
		})
	}

	return queues
}

// snsTopics returns demo SNS topic fixtures.
func snsTopics() []resource.Resource {
	topics := []resource.Resource{
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

	// Generate 19 more topics to reach 22 total
	for i := 0; i < 19; i++ {
		name := snsNamePool[i]
		arn := fmt.Sprintf("arn:aws:sns:us-east-1:123456789012:%s", name)
		topics = append(topics, resource.Resource{
			ID:     arn,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"topic_arn":    arn,
				"display_name": name,
			},
			RawStruct: snstypes.Topic{
				TopicArn: aws.String(arn),
			},
		})
	}

	return topics
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

// sfnExecutionFixtures returns demo SFN execution fixtures for a given state machine ARN.
func sfnExecutionFixtures(_ string) []resource.Resource {
	start1 := time.Date(2026, 3, 22, 3, 15, 0, 0, time.UTC)
	stop1 := time.Date(2026, 3, 22, 3, 17, 47, 0, time.UTC)

	start2 := time.Date(2026, 3, 22, 2, 0, 0, 0, time.UTC)
	stop2 := time.Date(2026, 3, 22, 2, 0, 12, 0, time.UTC)

	start3 := time.Date(2026, 3, 22, 1, 30, 0, 0, time.UTC)

	start4 := time.Date(2026, 3, 21, 22, 0, 0, 0, time.UTC)
	stop4 := time.Date(2026, 3, 22, 0, 30, 0, 0, time.UTC)

	start5 := time.Date(2026, 3, 21, 18, 0, 0, 0, time.UTC)
	stop5 := time.Date(2026, 3, 21, 18, 0, 3, 0, time.UTC)

	start6 := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	stop6 := time.Date(2026, 3, 21, 12, 5, 30, 0, time.UTC)

	start7 := time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)
	stop7 := time.Date(2026, 3, 20, 8, 45, 0, 0, time.UTC)

	redriveCount := int32(1)
	redriveDate := time.Date(2026, 3, 21, 19, 0, 0, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "exec-2026-0322-0315-a1b2c3d4",
			Name:   "exec-2026-0322-0315-a1b2c3d4",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0315-a1b2c3d4",
				"name":                     "exec-2026-0322-0315-a1b2c3d4",
				"status":                   "SUCCEEDED",
				"start_date":               "2026-03-22 03:15:00",
				"stop_date":                "2026-03-22 03:17:47",
				"duration":                 "2m 47s",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "",
				"redrive_date":             "",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0315-a1b2c3d4"),
				Name:            aws.String("exec-2026-0322-0315-a1b2c3d4"),
				StartDate:       &start1,
				StopDate:        &stop1,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusSucceeded,
			},
		},
		{
			ID:     "exec-2026-0322-0200-b2c3d4e5",
			Name:   "exec-2026-0322-0200-b2c3d4e5",
			Status: "FAILED",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0200-b2c3d4e5",
				"name":                     "exec-2026-0322-0200-b2c3d4e5",
				"status":                   "FAILED",
				"start_date":               "2026-03-22 02:00:00",
				"stop_date":                "2026-03-22 02:00:12",
				"duration":                 "12s",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "",
				"redrive_date":             "",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0200-b2c3d4e5"),
				Name:            aws.String("exec-2026-0322-0200-b2c3d4e5"),
				StartDate:       &start2,
				StopDate:        &stop2,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusFailed,
			},
		},
		{
			ID:     "exec-2026-0322-0130-c3d4e5f6",
			Name:   "exec-2026-0322-0130-c3d4e5f6",
			Status: "RUNNING",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0130-c3d4e5f6",
				"name":                     "exec-2026-0322-0130-c3d4e5f6",
				"status":                   "RUNNING",
				"start_date":               "2026-03-22 01:30:00",
				"stop_date":                "",
				"duration":                 "",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "",
				"redrive_date":             "",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0130-c3d4e5f6"),
				Name:            aws.String("exec-2026-0322-0130-c3d4e5f6"),
				StartDate:       &start3,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusRunning,
			},
		},
		{
			ID:     "exec-2026-0321-2200-d4e5f6a7",
			Name:   "exec-2026-0321-2200-d4e5f6a7",
			Status: "TIMED_OUT",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-2200-d4e5f6a7",
				"name":                     "exec-2026-0321-2200-d4e5f6a7",
				"status":                   "TIMED_OUT",
				"start_date":               "2026-03-21 22:00:00",
				"stop_date":                "2026-03-22 00:30:00",
				"duration":                 "2h 30m",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "",
				"redrive_date":             "",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-2200-d4e5f6a7"),
				Name:            aws.String("exec-2026-0321-2200-d4e5f6a7"),
				StartDate:       &start4,
				StopDate:        &stop4,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusTimedOut,
			},
		},
		{
			ID:     "exec-2026-0321-1800-e5f6a7b8",
			Name:   "exec-2026-0321-1800-e5f6a7b8",
			Status: "ABORTED",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-1800-e5f6a7b8",
				"name":                     "exec-2026-0321-1800-e5f6a7b8",
				"status":                   "ABORTED",
				"start_date":               "2026-03-21 18:00:00",
				"stop_date":                "2026-03-21 18:00:03",
				"duration":                 "3s",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "1",
				"redrive_date":             "2026-03-21 19:00:00",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-1800-e5f6a7b8"),
				Name:            aws.String("exec-2026-0321-1800-e5f6a7b8"),
				StartDate:       &start5,
				StopDate:        &stop5,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusAborted,
				RedriveCount:    &redriveCount,
				RedriveDate:     &redriveDate,
			},
		},
		{
			ID:     "exec-2026-0321-1200-f6a7b8c9",
			Name:   "exec-2026-0321-1200-f6a7b8c9",
			Status: "PENDING_REDRIVE",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-1200-f6a7b8c9",
				"name":                     "exec-2026-0321-1200-f6a7b8c9",
				"status":                   "PENDING_REDRIVE",
				"start_date":               "2026-03-21 12:00:00",
				"stop_date":                "2026-03-21 12:05:30",
				"duration":                 "5m 30s",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "",
				"redrive_date":             "",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-1200-f6a7b8c9"),
				Name:            aws.String("exec-2026-0321-1200-f6a7b8c9"),
				StartDate:       &start6,
				StopDate:        &stop6,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusPendingRedrive,
			},
		},
		{
			ID:     "exec-2026-0320-0800-a7b8c9d0",
			Name:   "exec-2026-0320-0800-a7b8c9d0",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"execution_arn":            "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0320-0800-a7b8c9d0",
				"name":                     "exec-2026-0320-0800-a7b8c9d0",
				"status":                   "SUCCEEDED",
				"start_date":               "2026-03-20 08:00:00",
				"stop_date":                "2026-03-20 08:45:00",
				"duration":                 "45m 0s",
				"state_machine_arn":        "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
				"state_machine_alias_arn":  "",
				"state_machine_version_arn": "",
				"map_run_arn":              "",
				"item_count":               "",
				"redrive_count":            "",
				"redrive_date":             "",
			},
			RawStruct: sfntypes.ExecutionListItem{
				ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0320-0800-a7b8c9d0"),
				Name:            aws.String("exec-2026-0320-0800-a7b8c9d0"),
				StartDate:       &start7,
				StopDate:        &stop7,
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				Status:          sfntypes.ExecutionStatusSucceeded,
			},
		},
	}
}

// sfnExecutionHistoryFixtures returns demo SFN execution history event fixtures
// simulating a workflow that validates an order, then fails during payment processing.
func sfnExecutionHistoryFixtures() []resource.Resource {
	ts1 := time.Date(2026, 3, 22, 3, 15, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 22, 3, 15, 0, 100000000, time.UTC)
	ts3 := time.Date(2026, 3, 22, 3, 15, 0, 200000000, time.UTC)
	ts4 := time.Date(2026, 3, 22, 3, 15, 2, 0, time.UTC)
	ts5 := time.Date(2026, 3, 22, 3, 15, 2, 100000000, time.UTC)
	ts6 := time.Date(2026, 3, 22, 3, 15, 2, 200000000, time.UTC)
	ts7 := time.Date(2026, 3, 22, 3, 15, 2, 300000000, time.UTC)
	ts8 := time.Date(2026, 3, 22, 3, 15, 5, 0, time.UTC)
	ts9 := time.Date(2026, 3, 22, 3, 15, 5, 100000000, time.UTC)

	return []resource.Resource{
		{
			ID: "1", Name: "Execution Started", Status: "active",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:00", "event_type": "ExecutionStarted",
				"event_type_short": "Execution Started", "state_name": "\u2014",
				"event_detail": `{"orderId":"ORD-98765","customerId":"C-1234"}`,
				"event_id": "1", "previous_event_id": "0",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 1, Timestamp: &ts1, Type: sfntypes.HistoryEventTypeExecutionStarted,
				ExecutionStartedEventDetails: &sfntypes.ExecutionStartedEventDetails{
					Input: aws.String(`{"orderId":"ORD-98765","customerId":"C-1234"}`),
				},
			},
		},
		{
			ID: "2", Name: "Task State Entered", Status: "pending",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:00", "event_type": "TaskStateEntered",
				"event_type_short": "Task State Entered", "state_name": "ValidateOrder",
				"event_detail": `{"orderId":"ORD-98765","customerId":"C-1234"}`,
				"event_id": "2", "previous_event_id": "1",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 2, PreviousEventId: 1, Timestamp: &ts2, Type: sfntypes.HistoryEventTypeTaskStateEntered,
				StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
					Name:  aws.String("ValidateOrder"),
					Input: aws.String(`{"orderId":"ORD-98765","customerId":"C-1234"}`),
				},
			},
		},
		{
			ID: "3", Name: "Task Scheduled", Status: "pending",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:00", "event_type": "TaskScheduled",
				"event_type_short": "Task Scheduled", "state_name": "ValidateOrder",
				"event_detail": "lambda:invoke",
				"event_id": "3", "previous_event_id": "2",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 3, PreviousEventId: 2, Timestamp: &ts3, Type: sfntypes.HistoryEventTypeTaskScheduled,
				TaskScheduledEventDetails: &sfntypes.TaskScheduledEventDetails{
					Resource:     aws.String("lambda:invoke"),
					ResourceType: aws.String("lambda"),
					Region:       aws.String("us-east-1"),
					Parameters:   aws.String(`{"FunctionName":"validate-order"}`),
				},
			},
		},
		{
			ID: "4", Name: "Task Succeeded", Status: "succeeded",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:02", "event_type": "TaskSucceeded",
				"event_type_short": "Task Succeeded", "state_name": "ValidateOrder",
				"event_detail": `{"valid":true,"amount":129.99}`,
				"event_id": "4", "previous_event_id": "3",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 4, PreviousEventId: 3, Timestamp: &ts4, Type: sfntypes.HistoryEventTypeTaskSucceeded,
				TaskSucceededEventDetails: &sfntypes.TaskSucceededEventDetails{
					Resource:     aws.String("lambda:invoke"),
					ResourceType: aws.String("lambda"),
					Output:       aws.String(`{"valid":true,"amount":129.99}`),
				},
			},
		},
		{
			ID: "5", Name: "Task State Exited", Status: "succeeded",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:02", "event_type": "TaskStateExited",
				"event_type_short": "Task State Exited", "state_name": "ValidateOrder",
				"event_detail": `{"valid":true,"amount":129.99}`,
				"event_id": "5", "previous_event_id": "4",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 5, PreviousEventId: 4, Timestamp: &ts5, Type: sfntypes.HistoryEventTypeTaskStateExited,
				StateExitedEventDetails: &sfntypes.StateExitedEventDetails{
					Name:   aws.String("ValidateOrder"),
					Output: aws.String(`{"valid":true,"amount":129.99}`),
				},
			},
		},
		{
			ID: "6", Name: "Task State Entered", Status: "pending",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:02", "event_type": "TaskStateEntered",
				"event_type_short": "Task State Entered", "state_name": "ProcessPayment",
				"event_detail": `{"valid":true,"amount":129.99}`,
				"event_id": "6", "previous_event_id": "5",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 6, PreviousEventId: 5, Timestamp: &ts6, Type: sfntypes.HistoryEventTypeTaskStateEntered,
				StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
					Name:  aws.String("ProcessPayment"),
					Input: aws.String(`{"valid":true,"amount":129.99}`),
				},
			},
		},
		{
			ID: "7", Name: "Task Scheduled", Status: "pending",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:02", "event_type": "TaskScheduled",
				"event_type_short": "Task Scheduled", "state_name": "ProcessPayment",
				"event_detail": "lambda:invoke",
				"event_id": "7", "previous_event_id": "6",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 7, PreviousEventId: 6, Timestamp: &ts7, Type: sfntypes.HistoryEventTypeTaskScheduled,
				TaskScheduledEventDetails: &sfntypes.TaskScheduledEventDetails{
					Resource:     aws.String("lambda:invoke"),
					ResourceType: aws.String("lambda"),
					Region:       aws.String("us-east-1"),
					Parameters:   aws.String(`{"FunctionName":"process-payment"}`),
				},
			},
		},
		{
			ID: "8", Name: "Task Failed", Status: "failed",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:05", "event_type": "TaskFailed",
				"event_type_short": "Task Failed", "state_name": "ProcessPayment",
				"event_detail": "States.TaskFailed: Payment gateway timeout after 3 retries",
				"event_id": "8", "previous_event_id": "7",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 8, PreviousEventId: 7, Timestamp: &ts8, Type: sfntypes.HistoryEventTypeTaskFailed,
				TaskFailedEventDetails: &sfntypes.TaskFailedEventDetails{
					Resource:     aws.String("lambda:invoke"),
					ResourceType: aws.String("lambda"),
					Error:        aws.String("States.TaskFailed"),
					Cause:        aws.String("Payment gateway timeout after 3 retries"),
				},
			},
		},
		{
			ID: "9", Name: "Execution Failed", Status: "failed",
			Fields: map[string]string{
				"timestamp": "2026-03-22 03:15:05", "event_type": "ExecutionFailed",
				"event_type_short": "Execution Failed", "state_name": "\u2014",
				"event_detail": "States.TaskFailed: Payment gateway timeout after 3 retries",
				"event_id": "9", "previous_event_id": "8",
			},
			RawStruct: sfntypes.HistoryEvent{
				Id: 9, PreviousEventId: 8, Timestamp: &ts9, Type: sfntypes.HistoryEventTypeExecutionFailed,
				ExecutionFailedEventDetails: &sfntypes.ExecutionFailedEventDetails{
					Error: aws.String("States.TaskFailed"),
					Cause: aws.String("Payment gateway timeout after 3 retries"),
				},
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

// snsTopicSubscriptionFixtures returns demo SNS subscriptions for a given topic ARN.
func snsTopicSubscriptionFixtures(topicArn string) []resource.Resource {
	return []resource.Resource{
		{
			ID:   "arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-email-001",
			Name: "ops-team@acme.com",
			Fields: map[string]string{
				"protocol":            "email",
				"endpoint":            "ops-team@acme.com",
				"confirmation_status": "Confirmed",
				"owner":               "123456789012",
				"subscription_arn":    "arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-email-001",
				"topic_arn":           topicArn,
			},
			RawStruct: snstypes.Subscription{
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-email-001"),
				TopicArn:        aws.String(topicArn),
				Protocol:        aws.String("email"),
				Endpoint:        aws.String("ops-team@acme.com"),
				Owner:           aws.String("123456789012"),
			},
		},
		{
			ID:   "arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-https-002",
			Name: "https://hooks.slack.com/services/T00/B00/xxx",
			Fields: map[string]string{
				"protocol":            "https",
				"endpoint":            "https://hooks.slack.com/services/T00/B00/xxx",
				"confirmation_status": "Confirmed",
				"owner":               "123456789012",
				"subscription_arn":    "arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-https-002",
				"topic_arn":           topicArn,
			},
			RawStruct: snstypes.Subscription{
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-https-002"),
				TopicArn:        aws.String(topicArn),
				Protocol:        aws.String("https"),
				Endpoint:        aws.String("https://hooks.slack.com/services/T00/B00/xxx"),
				Owner:           aws.String("123456789012"),
			},
		},
		{
			ID:   "arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-sqs-003",
			Name: "arn:aws:sqs:us-east-1:123456789012:order-processing-queue",
			Fields: map[string]string{
				"protocol":            "sqs",
				"endpoint":            "arn:aws:sqs:us-east-1:123456789012:order-processing-queue",
				"confirmation_status": "Confirmed",
				"owner":               "123456789012",
				"subscription_arn":    "arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-sqs-003",
				"topic_arn":           topicArn,
			},
			RawStruct: snstypes.Subscription{
				SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:" + topicArn + ":sub-sqs-003"),
				TopicArn:        aws.String(topicArn),
				Protocol:        aws.String("sqs"),
				Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
				Owner:           aws.String("123456789012"),
			},
		},
		{
			ID:   "pending/lambda/arn:aws:lambda:us-east-1:123456789012:function:process-notifications",
			Name: "arn:aws:lambda:us-east-1:123456789012:function:process-notifications",
			Fields: map[string]string{
				"protocol":            "lambda",
				"endpoint":            "arn:aws:lambda:us-east-1:123456789012:function:process-notifications",
				"confirmation_status": "PendingConfirmation",
				"owner":               "123456789012",
				"subscription_arn":    "PendingConfirmation",
				"topic_arn":           topicArn,
			},
			RawStruct: snstypes.Subscription{
				SubscriptionArn: aws.String("PendingConfirmation"),
				TopicArn:        aws.String(topicArn),
				Protocol:        aws.String("lambda"),
				Endpoint:        aws.String("arn:aws:lambda:us-east-1:123456789012:function:process-notifications"),
				Owner:           aws.String("123456789012"),
			},
		},
	}
}

// ebRuleTargetFixtures returns demo EventBridge rule target fixtures
// covering Lambda, SQS, SNS, and SFN target types with different input configs.
func ebRuleTargetFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:   "lambda-processor",
			Name: "lambda-processor",
			Fields: map[string]string{
				"target_id":          "lambda-processor",
				"target_arn":         "arn:aws:lambda:us-east-1:123456789012:function:event-processor",
				"role_arn":           "arn:aws:iam::123456789012:role/EventBridgeLambdaRole",
				"resource_type_name": "Lambda: event-processor",
				"input_summary":      `{"source":"scheduled"}`,
			},
			RawStruct: eventbridgetypes.Target{
				Id:      aws.String("lambda-processor"),
				Arn:     aws.String("arn:aws:lambda:us-east-1:123456789012:function:event-processor"),
				RoleArn: aws.String("arn:aws:iam::123456789012:role/EventBridgeLambdaRole"),
				Input:   aws.String(`{"source":"scheduled"}`),
			},
		},
		{
			ID:   "sqs-dlq",
			Name: "sqs-dlq",
			Fields: map[string]string{
				"target_id":          "sqs-dlq",
				"target_arn":         "arn:aws:sqs:us-east-1:123456789012:dead-letter-queue",
				"role_arn":           "arn:aws:iam::123456789012:role/EventBridgeSQSRole",
				"resource_type_name": "SQS: dead-letter-queue",
				"input_summary":      "\u2014",
			},
			RawStruct: eventbridgetypes.Target{
				Id:      aws.String("sqs-dlq"),
				Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
				RoleArn: aws.String("arn:aws:iam::123456789012:role/EventBridgeSQSRole"),
			},
		},
		{
			ID:   "sns-notify",
			Name: "sns-notify",
			Fields: map[string]string{
				"target_id":          "sns-notify",
				"target_arn":         "arn:aws:sns:us-east-1:123456789012:alerts-topic",
				"role_arn":           "",
				"resource_type_name": "SNS: alerts-topic",
				"input_summary":      "$.detail",
			},
			RawStruct: eventbridgetypes.Target{
				Id:        aws.String("sns-notify"),
				Arn:       aws.String("arn:aws:sns:us-east-1:123456789012:alerts-topic"),
				InputPath: aws.String("$.detail"),
			},
		},
		{
			ID:   "sfn-workflow",
			Name: "sfn-workflow",
			Fields: map[string]string{
				"target_id":          "sfn-workflow",
				"target_arn":         "arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow",
				"role_arn":           "arn:aws:iam::123456789012:role/EventBridgeSFNRole",
				"resource_type_name": "SFN: order-workflow",
				"input_summary":      "InputTransformer",
			},
			RawStruct: eventbridgetypes.Target{
				Id:      aws.String("sfn-workflow"),
				Arn:     aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow"),
				RoleArn: aws.String("arn:aws:iam::123456789012:role/EventBridgeSFNRole"),
				InputTransformer: &eventbridgetypes.InputTransformer{
					InputTemplate: aws.String(`"<instance> is in state <state>"`),
				},
			},
		},
	}
}
