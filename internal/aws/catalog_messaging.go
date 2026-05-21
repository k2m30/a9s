package aws

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

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
	},
}
