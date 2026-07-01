package fixtures

import (
	"sync"
	"github.com/aws/aws-sdk-go-v2/aws"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

// EventBridgeFixtures holds typed fixture data for EventBridge.
type EventBridgeFixtures struct {
	Rules []eventbridgetypes.Rule
	// TargetsByRule maps rule name to its targets.
	TargetsByRule map[string][]eventbridgetypes.Target
}

const prodEBRoleARN = "arn:aws:iam::123456789012:role/prod-ci-deploy-role"

// NewEventBridgeFixtures constructs EventBridgeFixtures from the canonical demo data.
var sharedEventBridgeFixtures = sync.OnceValue(func() *EventBridgeFixtures {
	rules := []eventbridgetypes.Rule{
		{
			Name:               aws.String("nightly-db-backup"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/nightly-db-backup"),
			State:              eventbridgetypes.RuleStateEnabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("cron(0 2 * * ? *)"),
			Description:        aws.String("Triggers nightly database backup at 2 AM UTC"),
			RoleArn:            aws.String(prodEBRoleARN),
		},
		{
			Name:         aws.String("ec2-state-change-handler"),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/ec2-state-change-handler"),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			Description:  aws.String("Routes EC2 instance state changes to SNS"),
			EventPattern: aws.String(`{"source":["aws.ec2"],"detail-type":["EC2 Instance State-change Notification"]}`),
		},
		{
			Name:               aws.String("cost-anomaly-detector"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/cost-anomaly-detector"),
			State:              eventbridgetypes.RuleStateEnabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("rate(1 hour)"),
			Description:        aws.String("Checks for cost anomalies every hour"),
		},
		{
			Name:               aws.String("staging-cleanup-rule"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/staging-cleanup-rule"),
			State:              eventbridgetypes.RuleStateDisabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("cron(0 0 ? * SUN *)"),
			Description:        aws.String("Weekly staging environment cleanup (disabled)"),
		},
		// Issue: ENABLED rule with no targets → Broken (rule fires but goes nowhere)
		{
			Name:               aws.String("eb-rule-no-targets"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/eb-rule-no-targets"),
			State:              eventbridgetypes.RuleStateEnabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("rate(5 minutes)"),
			Description:        aws.String("Enabled rule with no targets configured — events are silently dropped"),
		},
		// Issue: DISABLED rule with targets → Warning (targets are configured but rule won't fire)
		{
			Name:               aws.String("eb-rule-disabled-with-targets"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/eb-rule-disabled-with-targets"),
			State:              eventbridgetypes.RuleStateDisabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("cron(0 6 * * ? *)"),
			Description:        aws.String("Disabled rule — targets are configured but this rule will not trigger"),
			RoleArn:            aws.String(prodEBRoleARN),
		},
		// S3 healthy-bucket event bridge rule (checkS3EBRule pivot).
		// checkS3EBRule reads ruleRes.Fields["target_arns"] (emitted by the
		// eventbridge fetcher); this rule is pre-set so the demo related graph renders.
		{
			Name:         aws.String("a9s-demo-s3-events-rule"),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/a9s-demo-s3-events-rule"),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.s3"],"detail-type":["Object Created"],"detail":{"bucket":{"name":["` + HealthyBucketName + `"]}}}`),
			Description:  aws.String("Routes S3 object-created events from a9s-demo-healthy (" + HealthyBucketARN + ") to Lambda"),
		},
	}

	targetsByRule := map[string][]eventbridgetypes.Target{
		"nightly-db-backup": {
			{
				Id:  aws.String("LambdaBackupFunction"),
				Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:db-backup-trigger"),
			},
		},
		"ec2-state-change-handler": {
			{
				Id:  aws.String("SNSAlertTopic"),
				Arn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			},
		},
		// eb-rule-no-targets: intentionally empty — the rule itself has no targets
		"eb-rule-no-targets": {},
		// eb-rule-disabled-with-targets: has targets but rule is disabled
		"eb-rule-disabled-with-targets": {
			{
				Id:  aws.String("SQSDeadLetterQueue"),
				Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:scheduled-tasks-dlq"),
			},
		},
		// S3 healthy-bucket rule targets.
		"a9s-demo-s3-events-rule": {
			{
				Id:  aws.String("S3NotifierLambda"),
				Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + S3NotifierLambdaName),
			},
		},
	}

	return &EventBridgeFixtures{
		Rules:         rules,
		TargetsByRule: targetsByRule,
	}
})

func NewEventBridgeFixtures() *EventBridgeFixtures {
	return sharedEventBridgeFixtures()
}
