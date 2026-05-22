package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// messagingChildTypes is the declarative child-type catalog for the MESSAGING
// category. Appended to allChildTypes() in install.go alongside the other
// per-category child slices (containers, …) introduced by AS-808.
//
// AS-812 PR #402 round 2 (CTO arbitration 2026-05-22T01:46Z): eb_rule_targets
// migrates here from eb_rule_targets.go's init() body. Identity / Columns /
// CopyField preserved from the removed RegisterChildType call; FieldKeys and
// ChildFetcher carried over from the removed RegisterFieldKeys /
// RegisterPaginatedChild calls.
var messagingChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "EB Rule Targets",
		ShortName: "eb_rule_targets",
		Columns:   resource.EbRuleTargetColumns(),
		CopyField: "target_arn",
		FieldKeys: []string{"target_id", "target_arn", "role_arn", "resource_type_name", "input_summary"},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEventBridgeRuleTargets(ctx, c.EventBridge, parentCtx, continuationToken)
		},
	},
}

func colorSQS(_ domain.Resource) domain.Color { return domain.ColorHealthy }
func colorSNS(_ domain.Resource) domain.Color { return domain.ColorHealthy }
func colorSFN(_ domain.Resource) domain.Color { return domain.ColorHealthy }

func colorSNSSub(r domain.Resource) domain.Color {
	switch r.Fields["subscription_arn"] {
	case "PendingConfirmation":
		return domain.ColorWarning
	case "Deleted":
		return domain.ColorDim
	default:
		return domain.ColorHealthy
	}
}

func colorEBRule(r domain.Resource) domain.Color {
	switch strings.ToUpper(r.Fields["state"]) {
	case "ENABLED", "ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS":
		return domain.ColorHealthy
	case "DISABLED":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorKinesis(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["stream_status"] {
	case "ACTIVE":
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return domain.ColorWarning
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorMSK(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "ACTIVE":
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "MAINTENANCE", "REBOOTING_BROKER", "HEALING":
		return domain.ColorWarning
	case "FAILED":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func colorSES(r domain.Resource) domain.Color {
	phrase := stripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "verification failed", "verify: temp failure", "verification not started",
		"account SHUTDOWN", "account PROBATION":
		return domain.ColorBroken
	case "pending verification", "sending disabled":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var messagingTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "SQS Queues",
		ShortName:     "sqs",
		Aliases:       []string{"sqs", "queues"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:Fields.arn",
		Columns: []domain.Column{
			{Key: "queue_name", Title: "Queue Name", Width: 36, Sortable: true},
			{Key: "approx_messages", Title: "Messages", Width: 10, Sortable: true},
			{Key: "approx_not_visible", Title: "In Flight", Width: 10, Sortable: true},
			{Key: "delay_seconds", Title: "Delay", Width: 8, Sortable: true},
			{Key: "queue_url", Title: "Queue URL", Width: 50, Sortable: false},
		},
		Color: colorSQS,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			listAPI, ok := c.SQS.(SQSListQueuesAPI)
			if !ok {
				return resource.FetchResult{}, fmt.Errorf("SQS client does not support ListQueues")
			}
			return FetchSQSQueuesPage(ctx, listAPI, c.SQS, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichSQSAttributes, Priority: 100},
		FieldKeys: []string{"queue_name", "queue_url", "arn", "approx_messages", "approx_not_visible", "delay_seconds"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSQSAlarm, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSQSLambda, NeedsTargetCache: false},
			{TargetType: "sqs", DisplayName: "Dead Letter Queues", Checker: checkSQSSQS, NeedsTargetCache: true},
			{TargetType: "sns-sub", DisplayName: "SNS Subscriptions", Checker: checkSQSSNSSub, NeedsTargetCache: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSQSSNS, NeedsTargetCache: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSQSEbRule, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSQSKMS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sqs")},
		},
	},
	{
		Name:          "SNS Topics",
		ShortName:     "sns",
		Aliases:       []string{"sns", "topics"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "display_name", Title: "Topic Name", Width: 40, Sortable: true},
			{Key: "topic_arn", Title: "Topic ARN", Width: 60, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "sns_subscriptions",
			Key:            "enter",
			ContextKeys:    map[string]string{"topic_arn": "ID"},
			DisplayNameKey: "display_name",
		}},
		Color: colorSNS,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			topicsAPI, ok := c.SNS.(SNSListTopicsAPI)
			if !ok {
				return resource.FetchResult{}, fmt.Errorf("SNS client does not support ListTopics")
			}
			return FetchSNSTopicsPage(ctx, topicsAPI, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichSNSSubscriptions, Priority: 100},
		FieldKeys: []string{"topic_arn", "display_name"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSNSAlarm, NeedsTargetCache: false},
			{TargetType: "sns-sub", DisplayName: "Subscriptions", Checker: checkSNSSub, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSNSKMS, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkSNSRole, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sns")},
		},
	},
	{
		Name:          "SNS Subscriptions",
		ShortName:     "sns-sub",
		Aliases:       []string{"sns-sub", "sns-subscriptions", "subscriptions"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "topic_arn", Title: "Topic ARN", Width: 48, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			{Key: "subscription_arn", Title: "Subscription ARN", Width: 60, Sortable: false},
		},
		Color: colorSNSSub,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			subsAPI, ok := c.SNS.(SNSListSubscriptionsAPI)
			if !ok {
				return resource.FetchResult{}, fmt.Errorf("SNS client does not support ListSubscriptions")
			}
			return FetchSNSSubscriptionsPage(ctx, subsAPI, continuationToken)
		},
		FieldKeys: []string{"topic_arn", "protocol", "endpoint", "subscription_arn"},
		Related: []domain.RelatedDef{
			{TargetType: "sns", DisplayName: "SNS Topic", Checker: checkSNSSubTopic, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Function", Checker: checkSNSSubLambda, NeedsTargetCache: true},
			{TargetType: "sqs", DisplayName: "SQS Queue", Checker: checkSNSSubSQS, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sns-sub")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "TopicArn", TargetType: "sns"},
		},
	},
	{
		// Elastic Beanstalk migrates from catalog_compute.go → catalog_messaging.go
		// per AS-795 §3 spec row (messaging category lists eb) + CTO arbitration
		// on AS-812 PR #402 round 2 (2026-05-22T01:46Z). Category is now
		// MESSAGING to keep the main menu's category-grouping contiguous
		// (TestQA_MainMenu_CategoryOrderMatchesSpec). Identity / Columns / Color
		// preserved from the removed catalog_compute.go entry.
		Name:          "Elastic Beanstalk",
		ShortName:     "eb",
		Aliases:       []string{"eb", "beanstalk", "elastic-beanstalk"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "environment_name", Title: "Environment", Width: 28, Sortable: true},
			{Key: "application_name", Title: "Application", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "health", Title: "Health", Width: 10, Sortable: true},
			{Key: "version_label", Title: "Version", Width: 16, Sortable: true},
		},
		Color: colorEB,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEBEnvironmentsPage(ctx, c.ElasticBeanstalk, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichEBEnvironmentHealth, Priority: 100},
		FieldKeys: []string{"environment_name", "application_name", "status", "health", "version_label"},
		Related: []domain.RelatedDef{
			{TargetType: "cfn", DisplayName: "CloudFormation Stack", Checker: checkEbCFN, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEbLogs, NeedsTargetCache: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEbASG, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEbEC2, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEbAlarm, NeedsTargetCache: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkEbELB, NeedsTargetCache: false},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkEbTG, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEbSG, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEbRole, NeedsTargetCache: false},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkEbS3, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eb")},
		},
	},
	{
		Name:          "EventBridge Rules",
		ShortName:     "eb-rule",
		Aliases:       []string{"eb-rule", "eventbridge", "events"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Rule Name", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 10, Sortable: true},
			{Key: "event_bus", Title: "Event Bus", Width: 18, Sortable: true},
			{Key: "schedule", Title: "Schedule", Width: 24, Sortable: false},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "eb_rule_targets",
			Key:            "enter",
			ContextKeys:    map[string]string{"rule_name": "ID", "event_bus": "event_bus"},
			DisplayNameKey: "rule_name",
		}},
		Color: colorEBRule,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEventBridgeRulesPage(ctx, c.EventBridge, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichEventBridgeRuleTargets, Priority: 100},
		FieldKeys: []string{"name", "state", "event_bus", "schedule", "description", "event_pattern"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEbRuleRole, NeedsTargetCache: false},
			{TargetType: "kinesis", DisplayName: "Kinesis (targets)", Checker: checkEbRuleKinesis},
			{TargetType: "lambda", DisplayName: "Lambda (targets)", Checker: checkEbRuleLambda},
			{TargetType: "logs", DisplayName: "Log Groups (targets)", Checker: checkEbRuleLogs},
			{TargetType: "sfn", DisplayName: "Step Functions (targets)", Checker: checkEbRuleSFN},
			{TargetType: "sns", DisplayName: "SNS (targets)", Checker: checkEbRuleSNS},
			{TargetType: "sqs", DisplayName: "SQS (targets)", Checker: checkEbRuleSQS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eb-rule")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "RoleArn", TargetType: "role"},
		},
	},
	{
		Name:          "Kinesis Streams",
		ShortName:     "kinesis",
		Aliases:       []string{"kinesis", "streams"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "stream_name", Title: "Stream Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "stream_mode", Title: "Mode", Width: 14, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 22, Sortable: true},
		},
		Color: colorKinesis,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchKinesisStreamsPage(ctx, c.Kinesis, continuationToken)
		},
		FieldKeys: []string{"stream_name", "status", "stream_mode", "creation_time"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkKinesisAlarms, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkKinesisLambda, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkKinesisCFN},
			{TargetType: "ddb", DisplayName: "DynamoDB Streams", Checker: checkKinesisDDB, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkKinesisKMS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("kinesis")},
		},
	},
	{
		Name:          "MSK Clusters",
		ShortName:     "msk",
		Aliases:       []string{"msk", "kafka"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
			{Key: "cluster_type", Title: "Type", Width: 14, Sortable: true},
			{Key: "state", Title: "State", Width: 14, Sortable: true},
			{Key: "version", Title: "Version", Width: 14, Sortable: true},
		},
		Color: colorMSK,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchMSKClustersPage(ctx, c.MSK, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichMSKCluster, Priority: 100},
		FieldKeys: []string{"cluster_name", "cluster_type", "state", "version"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMSKAlarms, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkMSKSG, NeedsTargetCache: false},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkMSKKMS},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkMSKLambda, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkMSKCFN, NeedsTargetCache: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkMSKSubnet},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkMSKVPC, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkMSKLogs},
			{TargetType: "s3", DisplayName: "S3 (broker logs)", Checker: checkMSKS3},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkMSKSecrets},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("msk")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId", TargetType: "kms"},
		},
	},
	{
		Name:          "Step Functions",
		ShortName:     "sfn",
		Aliases:       []string{"sfn", "stepfunctions", "state-machines"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
			{Key: "type", Title: "Type", Width: 10, Sortable: true},
			{Key: "arn", Title: "ARN", Width: 60, Sortable: false},
			{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "sfn_executions",
			Key:            "enter",
			ContextKeys:    map[string]string{"state_machine_arn": "arn", "state_machine_name": "Name"},
			DisplayNameKey: "state_machine_name",
			DrillCondition: func(r domain.Resource) bool {
				return r.Fields["type"] != "EXPRESS"
			},
			DrillBlockMessage: "Execution history is not available for Express state machines",
		}},
		Color: colorSFN,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchStepFunctionsPage(ctx, c.SFN, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichStepFunctionsStatus, Priority: 10},
		FieldKeys: []string{"name", "type", "arn", "creation_date"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSFNAlarm, NeedsTargetCache: false},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSFNLogs, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkSFNRole, NeedsTargetCache: false},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSFNEbRule, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSFNKMS, NeedsTargetCache: false},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSFNLambda, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sfn")},
		},
		// RoleArn is declared navigable even though sfntypes.StateMachineListItem (the
		// list RawStruct) lacks it — the navigable-field registration is an intent
		// contract: "if the raw struct exposes RoleArn, treat it as a role navigation".
		// It resolves only when enriched detail (DescribeStateMachine) is present.
		Navigable: []domain.NavigableField{
			{FieldPath: "RoleArn", TargetType: "role"},
		},
	},
	{
		Name:          "SES Identities",
		ShortName:     "ses",
		Aliases:       []string{"ses", "email", "ses-identities"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "identity_name", Title: "Identity", Width: 36, Sortable: true},
			{Key: "identity_type", Title: "Type", Width: 16, Sortable: true},
			{Key: "status", Title: "Status", Width: 36, Sortable: true},
		},
		Color: colorSES,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchSESIdentitiesPage(ctx, c.SESv2, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichSESAccount, Priority: 100},
		FieldKeys: []string{"identity_name", "identity_type", "verification_status", "sending_enabled", "status"},
		Related: []domain.RelatedDef{
			{TargetType: "r53", DisplayName: "Route 53 (DNS)", Checker: checkSESR53, NeedsTargetCache: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSESEbRule, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSESLambda, NeedsTargetCache: false},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkSESS3, NeedsTargetCache: false},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSESSns, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ses")},
		},
	},
}
