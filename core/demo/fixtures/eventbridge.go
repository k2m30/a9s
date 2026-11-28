// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

// EventBridgeFixtures holds typed fixture data for EventBridge.
type EventBridgeFixtures struct {
	Rules []eventbridgetypes.Rule
	// EventBuses are the account's buses; ListRules answers for one of them
	// at a time.
	EventBuses []eventbridgetypes.EventBus
	// TargetsByRule maps rule name to its targets.
	TargetsByRule map[string][]eventbridgetypes.Target
}

const prodEBRoleARN = "arn:aws:iam::123456789012:role/prod-ci-deploy-role"

// EBRuleTargetKinds is the rule whose targets are the kinds that carry their
// own parameters: an ECS task, a Batch job, an API destination, an AppSync
// mutation and a SageMaker pipeline. Each target has a dead-letter queue.
const EBRuleTargetKinds = "acme-integration-fanout"

// EBRuleManaged is a rule an AWS service created on the account's behalf:
// Auto Scaling's rule for capacity rebalancing.
const EBRuleManaged = "acme-web-prod-asg-managed-rule"

// ebTargetDLQ is the dead-letter queue every EBRuleTargetKinds target sends a
// failed delivery to.
var ebTargetDLQ = &eventbridgetypes.DeadLetterConfig{Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:scheduled-tasks-dlq")}

// EBRuleForOneClustersService names both a service group and a cluster, so it
// belongs to the acme-batch log-aggregator and not to the acme-staging
// service of that name.
const EBRuleForOneClustersService = "ecs-batch-log-aggregator-task-state-change"

// EBRuleECRPrefix is the rule whose pattern matches ECR repositories by a
// prefix of their name: acme/batch-processor and acme/batch-etl (ecr.go).
const EBRuleECRPrefix = "ecr-batch-push-audit"

// EBCustomBus is the account's bus beside the default one, and
// EBRuleOnCustomBus is the rule that sits on it: a rule name is unique on its
// bus, so a rule is only listed by the bus it was created on.
const (
	EBCustomBus       = "acme-orders-bus"
	EBRuleOnCustomBus = "order-placed-fanout"
)

// NewEventBridgeFixtures constructs EventBridgeFixtures from the canonical demo data.
var sharedEventBridgeFixtures = shared(func() *EventBridgeFixtures {
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
		// ECS task-state rule for the acme-services cluster — required for
		// the ecs-svc→eb-rule related-panel pivot: its pattern names the
		// cluster in detail.clusterArn, so it matches every service's tasks
		// there.
		{
			Name:         aws.String("ecs-acme-services-task-state-change"),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/ecs-acme-services-task-state-change"),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.ecs"],"detail-type":["ECS Task State Change"],"detail":{"clusterArn":["arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"]}}`),
			Description:  aws.String("Routes ECS task state changes for the acme-services cluster to SNS"),
		},
		// A rule scoped to one cluster's service: it names the service group
		// and the cluster, and only the acme-batch log-aggregator is the
		// service it fires for.
		{
			Name:         aws.String(EBRuleForOneClustersService),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + EBRuleForOneClustersService),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.ecs"],"detail-type":["ECS Task State Change"],"detail":{"group":["service:log-aggregator"],"clusterArn":["arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch"]}}`),
			Description:  aws.String("Routes the acme-batch log-aggregator's task state changes to SNS"),
		},
		// S3 healthy-bucket rule — required for the s3→eb-rule related-panel
		// pivot: its pattern names the bucket in detail.bucket.name.
		{
			Name:         aws.String("a9s-demo-s3-events-rule"),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/a9s-demo-s3-events-rule"),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.s3"],"detail-type":["Object Created"],"detail":{"bucket":{"name":["` + HealthyBucketName + `"]}}}`),
			Description:  aws.String("Routes S3 object-created events from a9s-demo-healthy (" + HealthyBucketARN + ") to Lambda"),
		},
		// ECR image-scan-complete rule for acme/api-service — required for
		// ecr:eb-rule related-panel pivot: its pattern names the repository
		// in detail.repository-name.
		{
			Name:         aws.String("ecr-api-service-scan-complete"),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/ecr-api-service-scan-complete"),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.ecr"],"detail-type":["ECR Image Scan"],"detail":{"repository-name":["acme/api-service"]}}`),
			Description:  aws.String("Routes ECR image scan completion events for acme/api-service to SNS"),
		},
		// EBRuleECRPrefix filters the repository name with a prefix, so it
		// is counted on every repository whose name starts acme/batch-.
		{
			Name:         aws.String(EBRuleECRPrefix),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + EBRuleECRPrefix),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.ecr"],"detail-type":["ECR Image Action"],"detail":{"action-type":["PUSH"],"result":["SUCCESS"],"repository-name":[{"prefix":"acme/batch-"}]}}`),
			Description:  aws.String("Routes successful pushes to the batch repositories to SNS"),
		},
		// order-fulfillment-workflow schedule rule — required for sfn:eb-rule
		// related-panel pivot. checkSFNEbRule calls ListRuleNamesByTarget
		// with the state machine ARN as TargetArn (see EventBridgeFake).
		{
			Name:               aws.String("nightly-order-fulfillment-trigger"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/nightly-order-fulfillment-trigger"),
			State:              eventbridgetypes.RuleStateEnabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("cron(0 3 * * ? *)"),
			Description:        aws.String("Triggers order-fulfillment-workflow nightly at 3 AM UTC"),
			RoleArn:            aws.String(prodEBRoleARN),
		},
		// acme-api-deploy-trigger — required for the pipeline:eb-rule
		// related-panel pivot (checkPipelineEbRule calls
		// events:ListRuleNamesByTarget with the pipeline's constructed ARN as
		// TargetArn; see EventBridgeFake.ListRuleNamesByTarget).
		{
			Name:         aws.String("acme-api-deploy-trigger"),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/acme-api-deploy-trigger"),
			State:        eventbridgetypes.RuleStateEnabled,
			EventBusName: aws.String("default"),
			EventPattern: aws.String(`{"source":["aws.codecommit"],"detail-type":["CodeCommit Repository State Change"],"detail":{"referenceName":["main"]}}`),
			Description:  aws.String("Triggers acme-api-deploy on main branch push"),
		},
		// scheduled-report-generator — ENABLED with targets that each carry
		// a DeadLetterConfig, so EnrichEventBridgeRuleTargets raises no
		// eb-rule.target-issue finding.
		{
			Name:               aws.String("scheduled-report-generator"),
			Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/scheduled-report-generator"),
			State:              eventbridgetypes.RuleStateEnabled,
			EventBusName:       aws.String("default"),
			ScheduleExpression: aws.String("cron(0 7 * * ? *)"),
			Description:        aws.String("Generates and emails the daily ops report"),
		},
	}

	rules = append(rules, eventbridgetypes.Rule{
		Name:         aws.String(EBRuleTargetKinds),
		Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + EBRuleTargetKinds),
		State:        eventbridgetypes.RuleStateEnabled,
		EventBusName: aws.String("default"),
		EventPattern: aws.String(`{"source":["acme.integrations"],"detail-type":["PartnerSyncRequested"]}`),
		Description:  aws.String("Starts the partner sync jobs when a sync is requested"),
	}, eventbridgetypes.Rule{
		Name:         aws.String(EBRuleManaged),
		Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + EBRuleManaged),
		State:        eventbridgetypes.RuleStateEnabled,
		EventBusName: aws.String("default"),
		EventPattern: aws.String(`{"source":["aws.ec2"],"detail-type":["EC2 Instance Rebalance Recommendation","EC2 Spot Instance Interruption Warning"]}`),
		Description:  aws.String("This rule is used to route Instance notifications to EC2 Auto Scaling"),
		ManagedBy:    aws.String("autoscaling.amazonaws.com"),
	}, eventbridgetypes.Rule{
		Name:         aws.String(EBRuleOnCustomBus),
		Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + EBCustomBus + "/" + EBRuleOnCustomBus),
		State:        eventbridgetypes.RuleStateEnabled,
		EventBusName: aws.String(EBCustomBus),
		EventPattern: aws.String(`{"source":["acme.orders"],"detail-type":["OrderPlaced"]}`),
		Description:  aws.String("Fans an accepted order out to fulfilment"),
	})

	targetsByRule := map[string][]eventbridgetypes.Target{
		EBRuleOnCustomBus: {
			{
				Id:  aws.String("OrderFulfilmentQueue"),
				Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
				DeadLetterConfig: &eventbridgetypes.DeadLetterConfig{
					Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:scheduled-tasks-dlq"),
				},
			},
			{
				Id:               aws.String("WebhookIngestQueue"),
				Arn:              aws.String("arn:aws:sqs:us-east-1:123456789012:webhook-ingest-queue.fifo"),
				SqsParameters:    &eventbridgetypes.SqsParameters{MessageGroupId: aws.String("orders")},
				DeadLetterConfig: ebTargetDLQ,
			},
		},
		EBRuleManaged: {
			{
				Id:               aws.String("autoscaling"),
				Arn:              aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:11111111-1111-1111-1111-111111111111:autoScalingGroupName/acme-web-prod-asg"),
				DeadLetterConfig: ebTargetDLQ,
			},
		},
		EBRuleTargetKinds: {
			{
				Id:      aws.String("PartnerSyncTask"),
				Arn:     aws.String(ecsClusterArnServices),
				RoleArn: aws.String(prodEBRoleARN),
				EcsParameters: &eventbridgetypes.EcsParameters{
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
					LaunchType:        eventbridgetypes.LaunchTypeFargate,
					TaskCount:         aws.Int32(1),
				},
				DeadLetterConfig: ebTargetDLQ,
			},
			{
				Id:      aws.String("PartnerReportJob"),
				Arn:     aws.String("arn:aws:batch:us-east-1:123456789012:job-queue/acme-reports"),
				RoleArn: aws.String(prodEBRoleARN),
				BatchParameters: &eventbridgetypes.BatchParameters{
					JobDefinition: aws.String("acme-report-builder:4"),
					JobName:       aws.String("partner-sync-report"),
				},
				DeadLetterConfig: ebTargetDLQ,
			},
			{
				Id:      aws.String("PartnerWebhook"),
				Arn:     aws.String("arn:aws:events:us-east-1:123456789012:api-destination/partner-webhook/6f1e2d3c-4b5a-4968-8776-a5b4c3d2e1f0"),
				RoleArn: aws.String(prodEBRoleARN),
				HttpParameters: &eventbridgetypes.HttpParameters{
					HeaderParameters: map[string]string{"X-Acme-Source": "eventbridge"},
				},
				DeadLetterConfig: ebTargetDLQ,
			},
			{
				Id:      aws.String("PartnerStatusMutation"),
				Arn:     aws.String("arn:aws:appsync:us-east-1:123456789012:endpoints/graphql-api/a1b2c3d4e5f6g7h8i9j0klmnop"),
				RoleArn: aws.String(prodEBRoleARN),
				AppSyncParameters: &eventbridgetypes.AppSyncParameters{
					GraphQLOperation: aws.String(`mutation PartnerSynced($id: ID!) { partnerSynced(id: $id) { id } }`),
				},
				DeadLetterConfig: ebTargetDLQ,
			},
			{
				Id:      aws.String("PartnerScoringPipeline"),
				Arn:     aws.String("arn:aws:sagemaker:us-east-1:123456789012:pipeline/partner-scoring"),
				RoleArn: aws.String(prodEBRoleARN),
				SageMakerPipelineParameters: &eventbridgetypes.SageMakerPipelineParameters{
					PipelineParameterList: []eventbridgetypes.SageMakerPipelineParameter{{Name: aws.String("PartnerId"), Value: aws.String("all")}},
				},
				DeadLetterConfig: ebTargetDLQ,
			},
		},
		"nightly-db-backup": {
			{
				Id:          aws.String("LambdaBackupFunction"),
				Arn:         aws.String("arn:aws:lambda:us-east-1:123456789012:function:db-backup-trigger"),
				Input:       aws.String(`{"backupType":"full","retainDays":35}`),
				RetryPolicy: &eventbridgetypes.RetryPolicy{MaximumRetryAttempts: aws.Int32(3), MaximumEventAgeInSeconds: aws.Int32(3600)},
			},
			// KinesisAuditStream — required for eb-rule:kinesis related-panel
			// pivot (checkEbRuleKinesis). Streams a copy of every backup event.
			{
				Id:                aws.String("KinesisAuditStream"),
				Arn:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/audit-log-stream"),
				RoleArn:           aws.String(prodEBRoleARN),
				KinesisParameters: &eventbridgetypes.KinesisParameters{PartitionKeyPath: aws.String("$.detail.backup-id")},
			},
			// CloudWatchLogsTarget — required for eb-rule:logs related-panel
			// pivot (checkEbRuleLogs).
			{
				Id:        aws.String("CloudWatchLogsTarget"),
				Arn:       aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/app/legacy/orphan-old:*"),
				InputPath: aws.String("$.detail"),
			},
			// SQSBackupCompletionQueue — required for sqs:eb-rule related-panel
			// pivot (checkSQSEbRule calls events:ListRuleNamesByTarget with the
			// queue's ARN as TargetArn).
			{
				Id:  aws.String("SQSBackupCompletionQueue"),
				Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
			},
		},
		"ec2-state-change-handler": {
			{
				Id:  aws.String("SNSAlertTopic"),
				Arn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
				InputTransformer: &eventbridgetypes.InputTransformer{
					InputPathsMap: map[string]string{"instance": "$.detail.instance-id", "state": "$.detail.state"},
					InputTemplate: aws.String(`"Instance <instance> is now <state>"`),
				},
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
		"ecs-acme-services-task-state-change": {
			{
				Id:  aws.String("SNSECSAlertTopic"),
				Arn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			},
		},
		EBRuleForOneClustersService: {
			{
				Id:  aws.String("SNSBatchTaskStateTopic"),
				Arn: aws.String("arn:aws:sns:us-east-1:123456789012:alarm-notifications"),
			},
		},
		// S3 healthy-bucket rule targets.
		"a9s-demo-s3-events-rule": {
			{
				Id:  aws.String("S3NotifierLambda"),
				Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + S3NotifierLambdaName),
			},
		},
		"ecr-api-service-scan-complete": {
			{
				Id:  aws.String("SNSECRScanAlertTopic"),
				Arn: aws.String(relatedAlarmSNSARN),
			},
		},
		EBRuleECRPrefix: {
			{
				Id:  aws.String("SNSECRPushTopic"),
				Arn: aws.String(relatedAlarmSNSARN),
			},
		},
		"nightly-order-fulfillment-trigger": {
			{
				Id:  aws.String("SFNOrderFulfillmentWorkflow"),
				Arn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
			},
		},
		"acme-api-deploy-trigger": {
			{
				Id:  aws.String("CodePipelineAcmeApiDeploy"),
				Arn: aws.String("arn:aws:codepipeline:us-east-1:123456789012:acme-api-deploy"),
			},
		},
		// scheduled-report-generator's targets carry a DeadLetterConfig —
		// the DLQ ARN reuses the queue already referenced by
		// eb-rule-disabled-with-targets's SQSDeadLetterQueue target.
		"scheduled-report-generator": {
			{
				Id:  aws.String("LambdaReportGenerator"),
				Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:daily-report-generator"),
				DeadLetterConfig: &eventbridgetypes.DeadLetterConfig{
					Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:scheduled-tasks-dlq"),
				},
			},
			{
				Id:      aws.String("RefreshReportRollups"),
				Arn:     aws.String("arn:aws:redshift:us-east-1:123456789012:cluster:" + AcmeWarehouseID),
				RoleArn: aws.String(prodEBRoleARN),
				RedshiftDataParameters: &eventbridgetypes.RedshiftDataParameters{
					Database: aws.String("analytics"),
					DbUser:   aws.String("etl"),
					Sql:      aws.String("CALL refresh_daily_rollups()"),
				},
				DeadLetterConfig: ebTargetDLQ,
			},
		},
	}

	return &EventBridgeFixtures{
		Rules: rules,
		EventBuses: []eventbridgetypes.EventBus{
			{
				Name: aws.String("default"),
				Arn:  aws.String("arn:aws:events:us-east-1:123456789012:event-bus/default"),
			},
			{
				Name: aws.String(EBCustomBus),
				Arn:  aws.String("arn:aws:events:us-east-1:123456789012:event-bus/" + EBCustomBus),
			},
		},
		TargetsByRule: targetsByRule,
	}
})

func NewEventBridgeFixtures() *EventBridgeFixtures {
	return sharedEventBridgeFixtures()
}

func init() {
	Register(Pin{ShortName: "eb-rule", Rows: 17, Issues: 0})
}
