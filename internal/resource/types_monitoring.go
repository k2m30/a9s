package resource

func monitoringResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "CloudWatch Alarms",
			ShortName:     "alarm",
			ListTitle:     "alarms",
			Aliases:       []string{"alarm", "alarms", "cloudwatch", "cw_alarms"},
			Category:      "MONITORING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "alarm_name", Title: "Alarm Name", Width: 36, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "metric_name", Title: "Metric", Width: 24, Sortable: true},
				{Key: "namespace", Title: "Namespace", Width: 24, Sortable: true},
				{Key: "threshold", Title: "Threshold", Width: 12, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["state"] {
				case "OK":
					return ColorHealthy
				case "ALARM":
					return ColorBroken
				case "INSUFFICIENT_DATA":
					return ColorWarning
				}
				return ColorHealthy
			},
			Children: []ChildViewDef{{
				ChildType:      "alarm_history",
				Key:            "enter",
				ContextKeys:    map[string]string{"alarm_name": "alarm_name"},
				DisplayNameKey: "alarm_name",
			}},
		},
		{
			Name:          "CloudWatch Log Groups",
			ShortName:     "logs",
			Aliases:       []string{"logs", "loggroups", "log-groups", "cwlogs", "log_groups"},
			Category:      "MONITORING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "log_group_name", Title: "Log Group Name", Width: 48, Sortable: true},
				{Key: "stored_bytes", Title: "Size", Width: 14, Sortable: true},
				{Key: "retention_days", Title: "Retention", Width: 10, Sortable: true},
				{Key: "creation_time", Title: "Created", Width: 16, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			Children: []ChildViewDef{{
				ChildType:      "log_streams",
				Key:            "enter",
				ContextKeys:    map[string]string{"log_group_name": "Name"},
				DisplayNameKey: "log_group_name",
			}},
		},
		{
			Name:          "CloudTrail Trails",
			ShortName:     "trail",
			Aliases:       []string{"trail", "cloudtrail", "trails"},
			Category:      "MONITORING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "trail_name", Title: "Trail Name", Width: 28, Sortable: true},
				{Key: "s3_bucket", Title: "S3 Bucket", Width: 28, Sortable: true},
				{Key: "home_region", Title: "Home Region", Width: 16, Sortable: true},
				{Key: "multi_region", Title: "Multi-Region", Width: 14, Sortable: true},
			},
			Color: func(r Resource) Color {
				// GetTrailStatus: IsLogging=false = trail not capturing events (broken).
				// LatestDeliveryError = S3 delivery failing (broken).
				if r.Fields["is_logging"] == "false" {
					return ColorBroken
				}
				if r.Fields["latest_delivery_error"] != "" && r.Fields["latest_delivery_error"] != "-" {
					return ColorBroken
				}
				switch r.Fields["status"] {
				case "failed", "FAILED", "error", "ERROR":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:      "CloudTrail Events",
			ShortName: "ct-events",
			Aliases:   []string{"event", "events", "ct-events", "cloudtrail-events"},
			Category:  "MONITORING",
			Columns: []Column{
				{Key: "time", Title: "Time", Width: 22, Sortable: true},
				{Key: "event_name", Title: "Event Name", Width: 28, Sortable: true},
				{Key: "user", Title: "User", Width: 24, Sortable: true},
				{Key: "source", Title: "Source", Width: 28, Sortable: true},
				{Key: "resource_type", Title: "Resource Type", Width: 20, Sortable: true},
				{Key: "resource_name", Title: "Resource Name", Width: 24, Sortable: true},
				{Key: "read_only", Title: "Read Only", Width: 10, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Status {
				case "ct-danger":
					return ColorBroken
				case "ct-attention":
					return ColorWarning
				}
				return ColorDim
			},
			ExcludeFromIssueBadge: true,
		},
	}
}
