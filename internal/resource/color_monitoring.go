package resource

import (
	"strconv"
	"time"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

func init() {
	colorRegistry["alarm"] = colorAlarm
	colorRegistry["logs"] = colorLogs
	colorRegistry["trail"] = colorTrail
	colorRegistry["ct-events"] = colorCTEvents
	projectRegistry["ct-events"] = ctevent.Project
}

func colorAlarm(r Resource) Color {
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
}

func colorLogs(r Resource) Color {
	if r.Fields["retention_days"] == "" {
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
}

func colorTrail(r Resource) Color {
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
}

func colorCTEvents(r Resource) Color {
	switch r.Fields["status"] {
	case "ct-danger":
		return ColorBroken
	case "ct-attention":
		return ColorWarning
	}
	return ColorDim
}
