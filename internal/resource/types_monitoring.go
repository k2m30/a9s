package resource

import (
	"strconv"
	"time"
)

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
				case "ALARM":
					return ColorBroken
				case "INSUFFICIENT_DATA":
					return ColorWarning
				case "OK":
					actionsCount, err := strconv.Atoi(r.Fields["actions_count"])
					if err != nil || actionsCount == 0 {
						return ColorWarning
					}
					return ColorHealthy
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
			// Color: Warning for no retention, no KMS key (CIS CW.7), or orphan (0 bytes, >90d old).
			// Precedence: any Warning condition → ColorWarning; otherwise ColorHealthy.
			// retention_days is "" when no retention policy is set.
			// kms_key_id is "" when no KMS key is configured.
			// stored_bytes is "0 B" when the log group holds no data.
			// creation_time is stored as "2006-01-02 15:04".
			Color: func(r Resource) Color {
				if r.Fields["retention_days"] == "" {
					return ColorWarning
				}
				if r.Fields["kms_key_id"] == "" {
					return ColorWarning
				}
				if r.Fields["stored_bytes"] == "0 B" {
					ct := r.Fields["creation_time"]
					t, err := time.Parse("2006-01-02 15:04", ct)
					if err == nil && time.Since(t) > 90*24*time.Hour {
						return ColorWarning
					}
				}
				return ColorHealthy
			},
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
				// LogFileValidationEnabled=false = warning per CIS CT.2.
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
				if r.Fields["log_file_validation_enabled"] == "false" {
					return ColorWarning
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
