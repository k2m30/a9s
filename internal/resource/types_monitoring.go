package resource

func monitoringResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:      "CloudWatch Alarms",
			ShortName: "alarm",
			Aliases:   []string{"alarm", "alarms", "cloudwatch"},
			Category:  "MONITORING",
			Columns: []Column{
				{Key: "alarm_name", Title: "Alarm Name", Width: 36, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "metric_name", Title: "Metric", Width: 24, Sortable: true},
				{Key: "namespace", Title: "Namespace", Width: 24, Sortable: true},
				{Key: "threshold", Title: "Threshold", Width: 12, Sortable: true},
			},
		},
		{
			Name:      "CloudWatch Log Groups",
			ShortName: "logs",
			Aliases:   []string{"logs", "loggroups", "log-groups", "cwlogs"},
			Category:  "MONITORING",
			Columns: []Column{
				{Key: "log_group_name", Title: "Log Group Name", Width: 48, Sortable: true},
				{Key: "stored_bytes", Title: "Size", Width: 14, Sortable: true},
				{Key: "retention_days", Title: "Retention", Width: 10, Sortable: true},
				{Key: "creation_time", Title: "Created", Width: 16, Sortable: true},
			},
			Children: []ChildViewDef{{
				ChildType:      "log_streams",
				Key:            "enter",
				ContextKeys:    map[string]string{"log_group_name": "Name"},
				DisplayNameKey: "log_group_name",
			}},
		},
		{
			Name:      "CloudTrail Trails",
			ShortName: "trail",
			Aliases:   []string{"trail", "cloudtrail", "trails"},
			Category:  "MONITORING",
			Columns: []Column{
				{Key: "trail_name", Title: "Trail Name", Width: 28, Sortable: true},
				{Key: "s3_bucket", Title: "S3 Bucket", Width: 28, Sortable: true},
				{Key: "home_region", Title: "Home Region", Width: 16, Sortable: true},
				{Key: "multi_region", Title: "Multi-Region", Width: 14, Sortable: true},
			},
		},
	}
}
