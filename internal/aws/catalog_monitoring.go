package aws

import (
	"strconv"
	"time"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

func colorAlarm(r domain.Resource) domain.Color {
	switch r.Fields["state"] {
	case "ALARM":
		return domain.ColorBroken
	case "INSUFFICIENT_DATA":
		return domain.ColorWarning
	case "OK":
		actionsCount, err := strconv.Atoi(r.Fields["actions_count"])
		if err != nil || actionsCount == 0 {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	}
	return domain.ColorHealthy
}

func colorLogs(r domain.Resource) domain.Color {
	if r.Fields["retention_days"] == "" {
		return domain.ColorWarning
	}
	if r.Fields["stored_bytes"] == "0 B" {
		ct := r.Fields["creation_time"]
		t, err := time.Parse("2006-01-02 15:04", ct)
		if err == nil && time.Since(t) > 90*24*time.Hour {
			return domain.ColorWarning
		}
	}
	return domain.ColorHealthy
}

func colorTrail(r domain.Resource) domain.Color {
	if r.Fields["is_logging"] == "false" {
		return domain.ColorBroken
	}
	if r.Fields["latest_delivery_error"] != "" && r.Fields["latest_delivery_error"] != "-" {
		return domain.ColorBroken
	}
	switch r.Fields["status"] {
	case "failed", "FAILED", "error", "ERROR":
		return domain.ColorBroken
	}
	if r.Fields["log_file_validation_enabled"] == "false" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorCTEvents(r domain.Resource) domain.Color {
	switch r.Fields["status"] {
	case "ct-danger":
		return domain.ColorBroken
	case "ct-attention":
		return domain.ColorWarning
	}
	return domain.ColorDim
}

var monitoringTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "CloudWatch Alarms",
		ShortName:     "alarm",
		ListTitle:     "alarms",
		Aliases:       []string{"alarm", "alarms", "cloudwatch", "cw_alarms"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "alarm_name", Title: "Alarm Name", Width: 36, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "metric_name", Title: "Metric", Width: 24, Sortable: true},
			{Key: "namespace", Title: "Namespace", Width: 24, Sortable: true},
			{Key: "threshold", Title: "Threshold", Width: 12, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "alarm_history",
			Key:            "enter",
			ContextKeys:    map[string]string{"alarm_name": "alarm_name"},
			DisplayNameKey: "alarm_name",
		}},
		Color: colorAlarm,
	},
	{
		Name:          "CloudWatch Log Groups",
		ShortName:     "logs",
		Aliases:       []string{"logs", "loggroups", "log-groups", "cwlogs", "log_groups"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "log_group_name", Title: "Log Group Name", Width: 48, Sortable: true},
			{Key: "stored_bytes", Title: "Size", Width: 14, Sortable: true},
			{Key: "retention_days", Title: "Retention", Width: 10, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 16, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "log_streams",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group_name": "Name"},
			DisplayNameKey: "log_group_name",
		}},
		Color: colorLogs,
	},
	{
		Name:          "CloudTrail Trails",
		ShortName:     "trail",
		Aliases:       []string{"trail", "cloudtrail", "trails"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "trail_name", Title: "Trail Name", Width: 28, Sortable: true},
			{Key: "s3_bucket", Title: "S3 Bucket", Width: 28, Sortable: true},
			{Key: "home_region", Title: "Home Region", Width: 16, Sortable: true},
			{Key: "multi_region", Title: "Multi-Region", Width: 14, Sortable: true},
		},
		Color: colorTrail,
	},
	{
		Name:      "CloudTrail Events",
		ShortName: "ct-events",
		Aliases:   []string{"event", "events", "ct-events", "cloudtrail-events"},
		Category:  "MONITORING",
		Columns: []domain.Column{
			{Key: "time", Title: "Time", Width: 22, Sortable: true},
			{Key: "event_name", Title: "Event Name", Width: 28, Sortable: true},
			{Key: "user", Title: "User", Width: 24, Sortable: true},
			{Key: "source", Title: "Source", Width: 28, Sortable: true},
			{Key: "resource_type", Title: "Resource Type", Width: 20, Sortable: true},
			{Key: "resource_name", Title: "Resource Name", Width: 24, Sortable: true},
			{Key: "read_only", Title: "Read Only", Width: 10, Sortable: true},
		},
		ExcludeFromIssueBadge: true,
		Color:                 colorCTEvents,
		Project:               ctevent.Project,
	},
}
