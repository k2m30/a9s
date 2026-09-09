// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// messagingChildTypes is the declarative child-type catalog for the MESSAGING
// category, appended to allChildTypes() in install.go alongside the other
// per-category child slices.
var messagingChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "EB Rule Targets",
		ShortName: "eb_rule_targets",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			rule := r.Fields["rule_name"]
			if rule == "" {
				return ""
			}
			bus := r.Fields["event_bus"]
			if bus == "" {
				bus = "default"
			}
			return consolelink.Regional(region, "events/home?region="+region+"#/eventbus/"+url.PathEscape(bus)+"/rules/"+url.PathEscape(rule))
		},
		Columns:   resource.EbRuleTargetColumns(),
		CopyField: "target_arn",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{"target_id", "target_arn", "role_arn", "resource_type_name", "input_summary", "rule_name", "event_bus"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEventBridgeRuleTargets(ctx, c.EventBridge, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeEBRuleTargetNoDLQ, Phrase: "no DLQ configured", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:      "SFN Executions",
		ShortName: "sfn_executions",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["execution_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "states/home?region="+region+"#/executions/details/"+arn)
		},
		Columns:   resource.SFNExecutionColumns(),
		CopyField: "execution_arn",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"execution_arn", "name", "status", "start_date", "stop_date",
			"duration", "state_machine_arn", "state_machine_alias_arn",
			"state_machine_version_arn", "map_run_arn", "item_count",
			"redrive_count", "redrive_date",
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "sfn_execution_history",
			Key:            "enter",
			ContextKeys:    map[string]string{"execution_arn": "execution_arn", "execution_name": "Name"},
			DisplayNameKey: "execution_name",
		}},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchSFNExecutions(ctx, c.SFN, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeSFNExecutionFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSFNExecutionTimedOut, Phrase: "timed out", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSFNExecutionAborted, Phrase: "aborted", Severity: domain.SevBroken, Source: "wave1"},
		},
	},
	{
		Name:      "SFN Execution History",
		ShortName: "sfn_execution_history",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["execution_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "states/home?region="+region+"#/executions/details/"+arn)
		},
		Columns:   resource.SFNExecutionHistoryColumns(),
		CopyField: "event_detail",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"timestamp", "event_type", "event_type_short",
			"state_name", "event_detail", "event_id", "previous_event_id", "execution_arn",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchSFNExecutionHistory(ctx, c.SFN, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeSFNHistoryEventFailed, Phrase: "task failed", Severity: domain.SevBroken, Source: "wave1"},
		},
	},
	{
		Name:      "SNS Subscriptions",
		ShortName: "sns_subscriptions",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["subscription_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "sns/v3/home?region="+region+"#/subscription/"+arn)
		},
		Columns:   resource.SnsSubscriptionColumns(),
		CopyField: "endpoint",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"protocol", "endpoint", "confirmation_status", "owner", "subscription_arn", "topic_arn",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchSNSTopicSubscriptions(ctx, c.SNS, parentCtx["topic_arn"], continuationToken)
		}),
		// Same codes the sns-sub top-level type registers (sns_sub.go) — a
		// subscription is pending/deleted regardless of whether it was
		// listed via ListSubscriptions or ListSubscriptionsByTopic.
		Findings: []catalog.FindingDef{
			{Code: CodeSNSSubPendingConfirmation, Phrase: "endpoint has not confirmed the subscription", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSNSSubDeleted, Phrase: "endpoint deleted", Severity: domain.SevDim, Source: "wave1"},
			{Code: CodeSNSSubPlainHTTP, Phrase: "delivers over plain HTTP", Severity: domain.SevWarn, Source: "wave1", Detail: "The subscription delivers over plain HTTP, so every message crosses the network in the clear and anyone on the path can read or alter it before the endpoint sees it. Point the subscription at an HTTPS endpoint."},
		},
	},
}

func colorSQS(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorSNS(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorSFN(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorSNSSub(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	findings, _ := snsSubFindings(r.Fields["subscription_arn"], r.Fields["protocol"], r.Fields["endpoint"])
	return colorFromFindings(findings)
}

func colorEBRule(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(ebRuleStateFindings(r.Fields["state"]))
}

func colorKinesis(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(computeKinesisFindings(kinesistypes.StreamStatus(r.Fields["stream_status"])))
}

func colorMSK(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(computeMSKFindings(kafkatypes.ClusterState(r.Fields["state"])))
}

func colorSES(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

var messagingTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "SQS Queues",
		ShortName:     "sqs",
		Aliases:       []string{"sqs", "queues"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:Fields.arn",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "sqs/v3/home?region="+region+"#/queues/"+arn)
		},
		Columns: []domain.Column{
			{Key: "queue_name", Title: "Queue Name", Width: 36, Sortable: true},
			{Title: "Status", Width: 12},
			{Key: "approx_messages", Title: "Messages", Width: 10, Sortable: true},
			{Key: "approx_not_visible", Title: "In Flight", Width: 10, Sortable: true},
			{Key: "dlq", Title: "DLQ", Width: 5},
			{Key: "delay_seconds", Title: "Delay", Width: 8, Sortable: true},
			{Key: "queue_url", Title: "Queue URL", Width: 50},
		},
		Color: colorSQS,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			listAPI, ok := c.SQS.(SQSListQueuesAPI)
			if !ok {
				return resource.FetchResult{}, fmt.Errorf("SQS client does not support ListQueues")
			}
			return FetchSQSQueuesPage(ctx, listAPI, c.SQS, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichSQSAttributes, Priority: 100},
		FieldKeys:              []string{"queue_name", "queue_url", "arn", "approx_messages", "approx_not_visible", "delay_seconds", "kms_key_id"},
		IssueEnricherFieldKeys: []string{"dlq"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSQSAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSQSLambda, NeedsTargetCache: false},
			{TargetType: "sqs", DisplayName: "Dead Letter Queues", Checker: checkSQSSQS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sns-sub", DisplayName: "SNS Subscriptions", Checker: checkSQSSNSSub, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSQSSNS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSQSEbRule, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSQSKMS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sqs")},
		},
		Findings: []catalog.FindingDef{
			{Code: sqsCodeMissingDLQ, Phrase: "no DLQ configured", Severity: domain.SevWarn, Source: "wave2", Detail: "Messages this queue's consumers keep failing on are retried until they expire and are then thrown away, so a poison message is lost with no record of it. Set a redrive policy pointing at a dead-letter queue."},
			{Code: sqsCodeNoKMS, Phrase: "not encrypted with KMS", Severity: domain.SevWarn, Source: "wave2", Detail: "Messages sit unencrypted in the queue, so anyone who reaches the backing storage reads their contents. Set a KMS key on the queue so AWS encrypts each message at rest."},
			{Code: sqsCodePublicPolicy, Phrase: "queue policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The queue's access policy grants send or receive to every AWS principal, so anyone can drain the messages or flood the workers reading them. Scope the policy's Principal to the accounts and roles that actually use the queue."},
		},
	},
	{
		Name:          "SNS Topics",
		ShortName:     "sns",
		Aliases:       []string{"sns", "topics"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "sns/v3/home?region="+region+"#/topic/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "display_name", Title: "Topic Name", Width: 40, Sortable: true},
			{Title: "Status", Width: 12},
			{Key: "subs_count", Title: "Subs", Width: 6},
			{Key: "topic_arn", Title: "Topic ARN", Path: "TopicArn", Width: 60, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "sns_subscriptions",
			Key:            "enter",
			ContextKeys:    map[string]string{"topic_arn": "ID"},
			DisplayNameKey: "display_name",
		}},
		Color: colorSNS,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			topicsAPI, ok := c.SNS.(SNSListTopicsAPI)
			if !ok {
				return resource.FetchResult{}, fmt.Errorf("SNS client does not support ListTopics")
			}
			return FetchSNSTopicsPage(ctx, topicsAPI, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichSNSSubscriptions, Priority: 100},
		FieldKeys:              []string{"topic_arn", "display_name"},
		IssueEnricherFieldKeys: []string{"subs_count"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSNSAlarm, NeedsTargetCache: false, Truncated: true},
			{TargetType: "sns-sub", DisplayName: "Subscriptions", Checker: checkSNSSub, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSNSKMS, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkSNSRole, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sns")},
		},
		DetailEnrich: enrichSns,
		Findings: []catalog.FindingDef{
			{Code: snsCodeNoSubscribers, Phrase: "topic has no subscribers", Severity: domain.SevWarn, Source: "wave2", Detail: "Nothing is subscribed to this topic, so every message published to it is discarded on arrival. Either subscribe the endpoint that was meant to receive them, or delete the topic and whatever still publishes to it."},
			{Code: snsCodeAllPending, Phrase: "all pending confirmation", Severity: domain.SevWarn, Source: "wave2", Detail: "Every subscription on this topic is still waiting for its endpoint to confirm, so no message is being delivered to anyone. Confirm the subscriptions from their endpoints, or remove the ones that were never wanted."},
			{Code: snsCodePublicPolicy, Phrase: "topic policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The topic's access policy grants publish or subscribe to every AWS principal, so anyone can read what this topic broadcasts or inject messages its subscribers will trust. Scope the policy's Principal to the accounts and roles that actually use the topic."},
			{Code: snsCodeNoKMS, Phrase: "not encrypted with KMS", Severity: domain.SevWarn, Source: "wave2", Detail: "Messages sit unencrypted in the topic, so anyone who reaches the backing storage or a raw log of it reads their contents. Set a KMS key on the topic so AWS encrypts each message at rest."},
		},
	},
	{
		Name:          "SNS Subscriptions",
		ShortName:     "sns-sub",
		Aliases:       []string{"sns-sub", "sns-subscriptions", "subscriptions"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "sns/v3/home?region="+region+"#/subscription/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "topic_arn", Title: "Topic ARN", Path: "TopicArn", Width: 48, Sortable: true},
			{Title: "Status", Width: 12},
			{Key: "protocol", Title: "Protocol", Path: "Protocol", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint", Width: 48},
			{Key: "confirmed", Title: "Confirmed", Width: 12},
			{Key: "subscription_arn", Title: "Subscription ARN", Width: 60},
		},
		Color: colorSNSSub,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			subsAPI, ok := c.SNS.(SNSListSubscriptionsAPI)
			if !ok {
				return resource.FetchResult{}, fmt.Errorf("SNS client does not support ListSubscriptions")
			}
			return FetchSNSSubscriptionsPage(ctx, subsAPI, continuationToken)
		}),
		FieldKeys: []string{"topic_arn", "protocol", "endpoint", "subscription_arn", "confirmed"},
		Related: []domain.RelatedDef{
			{TargetType: "sns", DisplayName: "SNS Topic", Checker: checkSNSSubTopic, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Function", Checker: checkSNSSubLambda, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sqs", DisplayName: "SQS Queue", Checker: checkSNSSubSQS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sns-sub")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "TopicArn", TargetType: "sns"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeSNSSubPendingConfirmation, Phrase: "endpoint has not confirmed the subscription", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSNSSubDeleted, Phrase: "endpoint deleted", Severity: domain.SevDim, Source: "wave1"},
			{Code: CodeSNSSubPlainHTTP, Phrase: "delivers over plain HTTP", Severity: domain.SevWarn, Source: "wave1", Detail: "The subscription delivers over plain HTTP, so every message crosses the network in the clear and anyone on the path can read or alter it before the endpoint sees it. Point the subscription at an HTTPS endpoint."},
		},
	},
	{
		// Elastic Beanstalk lives in the MESSAGING category (not compute) so the
		// main menu's category grouping stays contiguous
		// (TestQA_MainMenu_CategoryOrderMatchesSpec).
		Name:          "Elastic Beanstalk",
		ShortName:     "eb",
		Aliases:       []string{"eb", "beanstalk", "elastic-beanstalk"},
		Category:      "MESSAGING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			// environment_arn shape: arn:aws:elasticbeanstalk:region:account:environment/{app}/{env}
			_, rest, ok := strings.Cut(r.Fields["environment_arn"], ":environment/")
			if !ok {
				return ""
			}
			appEnv := strings.SplitN(rest, "/", 2)
			if len(appEnv) != 2 || appEnv[0] == "" || appEnv[1] == "" {
				return ""
			}
			return consolelink.Regional(region, "elasticbeanstalk/home?region="+region+
				"#/environment/dashboard?applicationName="+url.QueryEscape(appEnv[0])+
				"&environmentName="+url.QueryEscape(appEnv[1]))
		},
		Columns: []domain.Column{
			{Key: "environment_name", Title: "Environment", Path: "EnvironmentName", Width: 28, Sortable: true},
			{Key: "application_name", Title: "Application", Path: "ApplicationName", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Path: "Status", Width: 12, Sortable: true},
			{Key: "health", Title: "Health", Path: "Health", Width: 10, Sortable: true},
			{Key: "version_label", Title: "Version", Path: "VersionLabel", Width: 16, Sortable: true},
		},
		Color: colorEB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchEBEnvironmentsPage(ctx, c.ElasticBeanstalk, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichEBEnvironmentHealth, Priority: 100},
		FieldKeys: []string{"environment_name", "application_name", "status", "health", "version_label", "environment_arn"},
		Related: []domain.RelatedDef{
			{TargetType: "cfn", DisplayName: "CloudFormation Stack", Checker: checkEbCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEbLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEbASG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEbEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEbAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkEbELB, NeedsTargetCache: false},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkEbTG, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEbSG, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEbRole, NeedsTargetCache: false},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkEbS3, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eb")},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEBHealthRed, Phrase: "health: red", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeEBHealthYellow, Phrase: "health: yellow", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEBHealthGrey, Phrase: "health: grey", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEBTerminated, Phrase: "terminated", Severity: domain.SevDim, Source: "wave1"},
			{Code: CodeEBLaunching, Phrase: "launching", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEBTerminating, Phrase: "terminating", Severity: domain.SevDim, Source: "wave1"},
			{Code: ebCodeEnvironmentCauses, Phrase: "environment reports health causes", Severity: domain.SevWarn, Source: "wave2"},
			{Code: ebCodeManagedUpdatesOff, Phrase: "managed platform updates off", Severity: domain.SevWarn, Source: "wave2", Detail: "The environment never takes platform patches on its own, so it stays on whatever version it was launched with until someone updates it by hand. Turn managed platform updates on and pick a weekly maintenance window."},
			{Code: ebCodeEnhancedHealthOff, Phrase: "enhanced health reporting off", Severity: domain.SevWarn, Source: "wave2", Detail: "Health is reported from basic checks only, so the environment cannot tell you which instance or which request is failing, or why. Switch health reporting to enhanced."},
			{Code: ebCodeCWLogsOff, Phrase: "log streaming to CloudWatch off", Severity: domain.SevWarn, Source: "wave2", Detail: "Instance logs stay on the instances and disappear when those instances are replaced, so there is nothing left to read after a failure. Turn on log streaming to CloudWatch Logs."},
		},
	},
	{
		Name:           "EventBridge Rules",
		ShortName:      "eb-rule",
		HumanizeFields: []string{"state"},
		Aliases:        []string{"eb-rule", "eventbridge"},
		Category:       "MESSAGING",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			bus := r.Fields["event_bus"]
			if bus == "" {
				bus = "default"
			}
			return consolelink.Regional(region, "events/home?region="+region+"#/eventbus/"+url.PathEscape(bus)+"/rules/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Rule Name", Path: "Name", Width: 28, Sortable: true},
			{Title: "Status", Path: "State", Width: 10},
			{Key: "target_count", Title: "Targets", Width: 8},
			{Key: "event_bus", Title: "Event Bus", Path: "EventBusName", Width: 18, Sortable: true},
			{Key: "schedule", Title: "Schedule", Path: "ScheduleExpression", Width: 24},
			{Key: "description", Title: "Description", Path: "Description", Width: 30},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "eb_rule_targets",
			Key:            "enter",
			ContextKeys:    map[string]string{"rule_name": "ID", "event_bus": "event_bus"},
			DisplayNameKey: "rule_name",
		}},
		Color: colorEBRule,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchEventBridgeRulesPage(ctx, c.EventBridge, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichEventBridgeRuleTargets, Priority: 100},
		FieldKeys:              []string{"name", "state", "event_bus", "schedule", "description", "event_pattern"},
		IssueEnricherFieldKeys: []string{"target_count"},
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
		Findings: []catalog.FindingDef{
			{Code: CodeEBRuleDisabled, Phrase: "disabled", Severity: domain.SevDim, Source: "wave1"},
			{Code: ebRuleCodeNoTargets, Phrase: "enabled rule has no targets", Severity: domain.SevBroken, Source: "wave2", Detail: "This rule is enabled and its pattern still matches events, but it has no target to deliver them to, so every match is silently discarded. Attach the target it was created for, or disable the rule."},
			{Code: ebRuleCodeTargetIssue, Phrase: "target drift or no dead-letter config", Severity: domain.SevWarn, Source: "wave2", Detail: "This rule's targets are not configured the way the rule implies: a disabled rule still carries targets, or a target has no dead-letter queue, so a delivery that fails leaves no trace. Remove the stale targets, or attach a dead-letter queue to the ones that matter."},
		},
	},
	{
		Name:           "Kinesis Streams",
		ShortName:      "kinesis",
		HumanizeFields: []string{"stream_mode", "StreamModeDetails.StreamMode", "stream_status"},
		Aliases:        []string{"kinesis", "streams"},
		Category:       "MESSAGING",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "kinesis/home?region="+region+"#/streams/details/"+url.PathEscape(r.ID)+"/monitoring")
		},
		Columns: []domain.Column{
			{Key: "stream_name", Title: "Stream Name", Path: "StreamName", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Path: "StreamStatus", Width: 12, Sortable: true},
			{Key: "stream_mode", Title: "Mode", Path: "StreamModeDetails.StreamMode", Width: 14, Sortable: true},
			{Key: "creation_time", Title: "Created", Path: "StreamCreationTimestamp", Width: 22, Sortable: true},
		},
		Color: colorKinesis,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchKinesisStreamsPage(ctx, c.Kinesis, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichKinesisStreamSummary, Priority: 100},
		FieldKeys: []string{"stream_name", "status", "stream_mode", "creation_time"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkKinesisAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkKinesisLambda, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkKinesisCFN, Truncated: true},
			{TargetType: "ddb", DisplayName: "DynamoDB Streams", Checker: checkKinesisDDB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkKinesisKMS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("kinesis")},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeKinesisCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeKinesisUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeKinesisDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: kinesisCodeUnencrypted, Phrase: "not encrypted at rest", Severity: domain.SevWarn, Source: "wave2", Detail: "Records sit unencrypted at rest, so anyone who reaches the backing storage reads whatever the stream carries. Turn on server-side encryption and point the stream at a KMS key."},
			{Code: kinesisCodeMinRetention, Phrase: "24h retention", Severity: domain.SevWarn, Source: "wave2", Detail: "The stream keeps only the default 24 hours of records, so a consumer that falls behind for a day, or an outage longer than one, loses data with no way to replay it. Raise the retention period to cover the longest replay you expect to need."},
		},
	},
	{
		Name:           "MSK Clusters",
		ShortName:      "msk",
		HumanizeFields: []string{"cluster_type", "state"},
		Aliases:        []string{"msk", "kafka"},
		Category:       "MESSAGING",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["cluster_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "msk/home?region="+region+"#/cluster/"+url.QueryEscape(arn)+"/view?tabId=details")
		},
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Path: "ClusterName", Width: 28, Sortable: true},
			{Key: "cluster_type", Title: "Type", Path: "ClusterType", Width: 14, Sortable: true},
			{Key: "state", Title: "Status", Path: "State", Width: 14, Sortable: true},
			{Key: "version", Title: "Version", Path: "CurrentVersion", Width: 14, Sortable: true},
		},
		Color: colorMSK,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchMSKClustersPage(ctx, c.MSK, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichMSKCluster, Priority: 100},
		FieldKeys: []string{"cluster_name", "cluster_type", "state", "version", "cluster_arn"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMSKAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkMSKSG, NeedsTargetCache: false},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkMSKKMS},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkMSKLambda, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkMSKCFN, NeedsTargetCache: true, Truncated: true},
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
		Findings: []catalog.FindingDef{
			{Code: CodeMSKCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeMSKUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeMSKMaintenance, Phrase: "maintenance", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeMSKRebootingBroker, Phrase: "rebooting broker", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeMSKHealing, Phrase: "healing", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeMSKDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeMSKFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: mskCodeBrokerOutdated, Phrase: "broker software outdated", Severity: domain.SevWarn, Source: "wave2"},
			{Code: mskCodeEncryptionNotTLS, Phrase: "encryption in transit not enforced", Severity: domain.SevWarn, Source: "wave2"},
			{Code: mskCodePublicAccess, Phrase: "brokers reachable from the internet", Severity: domain.SevBroken, Source: "wave2", Detail: "Kafka brokers are published to the internet with their own public addresses, so the cluster is reachable from anywhere its security groups allow rather than only from inside the VPC. Turn public access off and reach the brokers from within the VPC or over a peered network."},
			{Code: mskCodeUnauthenticated, Phrase: "unauthenticated access allowed", Severity: domain.SevBroken, Source: "wave2", Detail: "The cluster accepts Kafka clients that present no credentials at all, so anyone who can reach a broker can read and write every topic. Turn unauthenticated access off and require one of the cluster's authentication methods."},
		},
	},
	{
		Name:           "Step Functions",
		ShortName:      "sfn",
		HumanizeFields: []string{"type"},
		Aliases:        []string{"sfn", "stepfunctions", "state-machines"},
		Category:       "MESSAGING",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "states/home?region="+region+"#/statemachines/view/"+arn)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Path: "Name", Width: 36, Sortable: true},
			{Key: "type", Title: "Type", Path: "Type", Width: 10, Sortable: true},
			{Title: "Status", Width: 12},
			{Key: "last_run", Title: "Last Run", Width: 18},
			{Key: "arn", Title: "ARN", Path: "StateMachineArn", Width: 60},
			{Key: "creation_date", Title: "Created", Path: "CreationDate", Width: 22, Sortable: true},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchStepFunctionsPage(ctx, c.SFN, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichStepFunctionsStatus, Priority: 10},
		FieldKeys:              []string{"name", "type", "arn", "creation_date"},
		IssueEnricherFieldKeys: []string{"last_run"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSFNAlarm, NeedsTargetCache: false, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSFNLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkSFNRole, NeedsTargetCache: false},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSFNEbRule, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSFNKMS, NeedsTargetCache: false},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSFNLambda, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sfn")},
		},
		DetailEnrich: enrichSfn,
		// RoleArn is declared navigable even though sfntypes.StateMachineListItem (the
		// list RawStruct) lacks it — the navigable-field registration is an intent
		// contract: "if the raw struct exposes RoleArn, treat it as a role navigation".
		// It resolves only when enriched detail (DescribeStateMachine) is present.
		Navigable: []domain.NavigableField{
			{FieldPath: "RoleArn", TargetType: "role"},
		},
		Findings: []catalog.FindingDef{
			{Code: sfnCodeLatestExecutionFailed, Phrase: "latest execution <STATUS>", Severity: domain.SevBroken, Source: "wave2"},
			{Code: sfnCodeLoggingOff, Phrase: "execution logging off", Severity: domain.SevWarn, Source: "wave2", Detail: "The state machine records nothing about its executions, so a failed run leaves no trace of which state failed or what it was handed. Turn on execution logging to a CloudWatch log group."},
			{Code: sfnCodeNoCMK, Phrase: "not encrypted with a customer key", Severity: domain.SevWarn, Source: "wave2", Detail: "Execution history and state data are encrypted with an AWS-owned key you cannot audit, rotate, or revoke. Point the state machine at a customer managed KMS key."},
			{Code: sfnCodeDefinitionSecret, Phrase: "credential in state machine definition", Severity: domain.SevBroken, Source: "wave2", Detail: "A credential is written into the state machine's definition, so it is readable by anyone who can call states:DescribeStateMachine and it travels with every export of the workflow. Move the value to Secrets Manager and reference it at run time, then rotate it."},
		},
	},
	{
		Name:           "SES Identities",
		ShortName:      "ses",
		HumanizeFields: []string{"verification_status"},
		Aliases:        []string{"ses", "email", "ses-identities"},
		Category:       "MESSAGING",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ses/home?region="+region+"#/identities/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "identity_name", Title: "Identity", Path: "IdentityName", Width: 36, Sortable: true},
			{Key: "identity_type", Title: "Type", Width: 16, Sortable: true},
			{Key: "status", Title: "Status", Width: 36, Sortable: true},
		},
		Color: colorSES,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchSESIdentitiesPage(ctx, c.SESv2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichSESAccount, Priority: 100},
		FieldKeys: []string{"identity_name", "identity_type", "verification_status", "sending_enabled", "status"},
		Related: []domain.RelatedDef{
			{TargetType: "r53", DisplayName: "Route 53 (DNS)", Checker: checkSESR53, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSESEbRule, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSESLambda, NeedsTargetCache: false},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkSESS3, NeedsTargetCache: false},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSESSns, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ses")},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeSESVerificationFailed, Phrase: "verification failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSESVerificationTempFail, Phrase: "verify: temp failure", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSESVerificationNotStarted, Phrase: "verification not started", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSESVerificationPending, Phrase: "pending verification", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSESSendingDisabled, Phrase: "sending disabled", Severity: domain.SevWarn, Source: "wave1"},
			{Code: sesCodeShutdown, Phrase: "sending paused by AWS (shutdown)", Severity: domain.SevBroken, Source: "wave2"},
			{Code: sesCodeProbation, Phrase: "account under review (probation)", Severity: domain.SevBroken, Source: "wave2"},
			{Code: sesCodeQuota, Phrase: "quota 80%+ used", Severity: domain.SevWarn, Source: "wave2"},
			{Code: sesCodeDKIMOff, Phrase: "DKIM not enabled", Severity: domain.SevWarn, Source: "wave2", Detail: "Outbound mail from this domain is not signed, so receivers cannot tell genuine mail from a forgery and are more likely to reject it or file it as spam. Enable DKIM signing for the identity and publish the records AWS gives you."},
		},
	},
}
