package fixtures

import (
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
func NewEventBridgeFixtures() *EventBridgeFixtures {
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
	}

	return &EventBridgeFixtures{
		Rules:         rules,
		TargetsByRule: targetsByRule,
	}
}
