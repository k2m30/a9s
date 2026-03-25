package resource

func messagingResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:      "SQS Queues",
			ShortName: "sqs",
			Aliases:   []string{"sqs", "queues"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "queue_name", Title: "Queue Name", Width: 36, Sortable: true},
				{Key: "approx_messages", Title: "Messages", Width: 10, Sortable: true},
				{Key: "approx_not_visible", Title: "In Flight", Width: 10, Sortable: true},
				{Key: "delay_seconds", Title: "Delay", Width: 8, Sortable: true},
				{Key: "queue_url", Title: "Queue URL", Width: 50, Sortable: false},
			},
		},
		{
			Name:      "SNS Topics",
			ShortName: "sns",
			Aliases:   []string{"sns", "topics"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "display_name", Title: "Topic Name", Width: 40, Sortable: true},
				{Key: "topic_arn", Title: "Topic ARN", Width: 60, Sortable: true},
			},
		},
		{
			Name:      "SNS Subscriptions",
			ShortName: "sns-sub",
			Aliases:   []string{"sns-sub", "sns-subscriptions", "subscriptions"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "topic_arn", Title: "Topic ARN", Width: 48, Sortable: true},
				{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
				{Key: "subscription_arn", Title: "Subscription ARN", Width: 60, Sortable: false},
			},
		},
		{
			Name:      "EventBridge Rules",
			ShortName: "eb-rule",
			Aliases:   []string{"eb-rule", "eventbridge", "events"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "name", Title: "Rule Name", Width: 28, Sortable: true},
				{Key: "state", Title: "State", Width: 10, Sortable: true},
				{Key: "event_bus", Title: "Event Bus", Width: 18, Sortable: true},
				{Key: "schedule", Title: "Schedule", Width: 24, Sortable: false},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
		},
		{
			Name:      "Kinesis Streams",
			ShortName: "kinesis",
			Aliases:   []string{"kinesis", "streams"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "stream_name", Title: "Stream Name", Width: 36, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "stream_mode", Title: "Mode", Width: 14, Sortable: true},
				{Key: "creation_time", Title: "Created", Width: 22, Sortable: true},
			},
		},
		{
			Name:      "MSK Clusters",
			ShortName: "msk",
			Aliases:   []string{"msk", "kafka"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
				{Key: "cluster_type", Title: "Type", Width: 14, Sortable: true},
				{Key: "state", Title: "State", Width: 14, Sortable: true},
				{Key: "version", Title: "Version", Width: 14, Sortable: true},
			},
		},
		{
			Name:      "Step Functions",
			ShortName: "sfn",
			Aliases:   []string{"sfn", "stepfunctions", "state-machines"},
			Category:  "MESSAGING",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 36, Sortable: true},
				{Key: "type", Title: "Type", Width: 10, Sortable: true},
				{Key: "arn", Title: "ARN", Width: 60, Sortable: false},
				{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
			},
			Children: []ChildViewDef{{
				ChildType:      "sfn_executions",
				Key:            "enter",
				ContextKeys:    map[string]string{"state_machine_arn": "arn", "state_machine_name": "Name"},
				DisplayNameKey: "state_machine_name",
				DrillCondition: func(r Resource) bool {
					return r.Fields["type"] != "EXPRESS"
				},
				DrillBlockMessage: "Execution history is not available for Express state machines",
			}},
		},
	}
}
