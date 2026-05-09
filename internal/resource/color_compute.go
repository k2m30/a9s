// color_compute.go holds the Color classification functions for the COMPUTE
// category resource types. These are extracted from types_compute.go (deleted
// in PR-04b) and registered via colorRegistry so that adaptFromCatalog can
// wire them back in during the migration window (PR-04b through PR-04n).
package resource

import (
	"strconv"
	"strings"
	"time"
)

// deprecatedLambdaRuntimes is the set of Lambda runtime identifiers that AWS
// has end-of-lifed per docs/attention-signals.md.
var deprecatedLambdaRuntimes = map[string]struct{}{
	"nodejs":        {},
	"nodejs4.3":     {},
	"nodejs6.10":    {},
	"nodejs8.10":    {},
	"nodejs10.x":    {},
	"nodejs12.x":    {},
	"nodejs14.x":    {},
	"python2.7":     {},
	"python3.6":     {},
	"python3.7":     {},
	"ruby2.5":       {},
	"ruby2.7":       {},
	"dotnetcore1.0": {},
	"dotnetcore2.0": {},
	"dotnetcore2.1": {},
	"dotnetcore3.1": {},
	"java8":         {},
	"go1.x":         {},
}

func init() {
	colorRegistry["ec2"] = colorEC2
	colorRegistry["ecs-svc"] = colorECSSvc
	colorRegistry["ecs"] = colorECSCluster
	colorRegistry["ecs-task"] = colorECSTask
	colorRegistry["lambda"] = colorLambda
	colorRegistry["asg"] = colorASG
	colorRegistry["eb"] = colorEB
	colorRegistry["ebs"] = colorEBS
	colorRegistry["ebs-snap"] = colorEBSSnap
	colorRegistry["ami"] = colorAMI

	augmentRegistry["ec2"] = augmentEC2StatusChecks
}

func colorEC2(r Resource) Color {
	for i := range r.Findings {
		if r.Findings[i].Source == "wave1" {
			return ColorFromSeverity(r.Findings[i].Severity)
		}
	}
	sys := r.Fields["system_status"]
	inst := r.Fields["instance_status"]
	if sys == "impaired" || inst == "impaired" {
		return ColorBroken
	}
	if sys == "initializing" || inst == "initializing" {
		return ColorWarning
	}
	state := r.Fields["state"]
	if state == "" {
		state = r.Status
	}
	switch state {
	case "running", "":
		return ColorHealthy
	case "pending", "shutting-down", "stopping":
		return ColorWarning
	case "stopped":
		if strings.HasPrefix(r.Fields["state_reason_code"], "Server.") {
			return ColorBroken
		}
		return ColorWarning
	case "terminated":
		return ColorDim
	}
	return fallbackColor(state)
}

func colorECSSvc(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	switch r.Fields["status"] {
	case "INACTIVE":
		return ColorBroken
	case "DRAINING":
		return ColorWarning
	}
	running := r.Fields["running_count"]
	desired := r.Fields["desired_count"]
	if desired == "0" || desired == "" {
		return ColorHealthy
	}
	if running == "0" {
		return ColorBroken
	}
	if running != desired {
		return ColorWarning
	}
	return ColorHealthy
}

func colorECSCluster(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		return ColorHealthy
	case "PROVISIONING", "DEPROVISIONING":
		return ColorWarning
	case "FAILED", "INACTIVE":
		return ColorBroken
	}
	return ColorHealthy
}

func colorECSTask(r Resource) Color {
	if r.Fields["health_status"] == "UNHEALTHY" {
		return ColorBroken
	}
	if r.Fields["last_status"] == "STOPPED" && r.Fields["stop_code"] != "" && r.Fields["stop_code"] != "UserInitiated" {
		return ColorBroken
	}
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	switch r.Fields["last_status"] {
	case "RUNNING":
		return ColorHealthy
	case "PROVISIONING", "PENDING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING":
		return ColorWarning
	case "STOPPED":
		return ColorDim
	}
	return ColorHealthy
}

func colorLambda(r Resource) Color {
	if r.Fields["last_update_status"] == "Failed" {
		return ColorBroken
	}
	if _, ok := deprecatedLambdaRuntimes[r.Fields["runtime"]]; ok {
		return ColorBroken
	}
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	state := r.Fields["state"]
	if state == "" {
		state = r.Status
	}
	switch state {
	case "Failed":
		return ColorBroken
	case "Pending":
		return ColorWarning
	}
	if r.Fields["dlq_target_arn"] == "" {
		return ColorWarning
	}
	if state == "Inactive" {
		return ColorDim
	}
	return ColorHealthy
}

func colorASG(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	status := r.Fields["status"]
	if status == "Delete in progress" {
		return ColorWarning
	}
	inService := r.Fields["in_service_count"]
	minSz := r.Fields["min_size"]
	if inService != "" && minSz != "" {
		inSvc, err1 := strconv.Atoi(inService)
		minSzInt, err2 := strconv.Atoi(minSz)
		if err1 == nil && err2 == nil && inSvc < minSzInt {
			return ColorBroken
		}
	}
	if unhealthy := r.Fields["instances_unhealthy_count"]; unhealthy != "" {
		if n, err := strconv.Atoi(unhealthy); err == nil && n > 0 {
			return ColorWarning
		}
	}
	if sp := r.Fields["suspended_processes"]; sp != "" {
		if strings.Contains(sp, "Launch") || strings.Contains(sp, "Terminate") || strings.Contains(sp, "HealthCheck") {
			return ColorWarning
		}
	}
	return ColorHealthy
}

func colorEB(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	var healthColor Color
	healthSet := true
	switch r.Fields["health"] {
	case "Red":
		healthColor = ColorBroken
	case "Yellow":
		healthColor = ColorWarning
	case "Grey":
		healthColor = ColorWarning
	case "Green":
		healthColor = ColorHealthy
	default:
		healthSet = false
		healthColor = ColorHealthy
	}
	if r.Fields["status"] == "Terminated" && healthColor != ColorBroken {
		return ColorDim
	}
	if healthSet {
		return healthColor
	}
	switch r.Fields["status"] {
	case "Ready":
		return ColorHealthy
	case "Launching", "Updating":
		return ColorWarning
	case "Terminating":
		return ColorDim
	}
	return ColorHealthy
}

func colorEBS(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	var base Color
	switch r.Fields["state"] {
	case "in-use":
		base = ColorHealthy
	case "available":
		base = ColorHealthy
		if r.Fields["attached_to"] == "" {
			if t, err := time.Parse("2006-01-02 15:04", r.Fields["created"]); err == nil {
				if time.Since(t) > 7*24*time.Hour {
					base = ColorWarning
				}
			}
		}
	case "creating", "deleting":
		base = ColorWarning
	case "error":
		base = ColorBroken
	default:
		base = ColorHealthy
	}
	if base == ColorBroken {
		return ColorBroken
	}
	if r.Fields["encrypted"] == "false" && base == ColorHealthy {
		base = ColorWarning
	}
	return base
}

func colorEBSSnap(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	var base Color
	switch r.Fields["state"] {
	case "completed":
		base = ColorHealthy
	case "pending":
		base = ColorWarning
	case "error", "recoverable", "recovering":
		base = ColorBroken
	default:
		base = ColorHealthy
	}
	if base == ColorBroken {
		return base
	}
	if r.Fields["encrypted"] == "false" {
		return ColorWarning
	}
	if started, err := time.Parse(time.RFC3339, r.Fields["started"]); err == nil {
		if time.Since(started) > 365*24*time.Hour {
			desc := r.Fields["description"]
			if strings.HasPrefix(desc, "Created by CreateImage") ||
				strings.Contains(strings.ToLower(desc), "automated") {
				return ColorWarning
			}
		}
	}
	if strings.HasPrefix(r.Fields["volume_id"], "vol-") && r.Fields["volume_orphan"] == "true" {
		return ColorWarning
	}
	return base
}

func colorAMI(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	var stateColor Color
	switch r.Fields["state"] {
	case "available":
		stateColor = ColorHealthy
	case "pending", "transient":
		stateColor = ColorWarning
	case "failed", "error", "invalid":
		stateColor = ColorBroken
	case "deregistered", "disabled":
		stateColor = ColorDim
	default:
		stateColor = ColorHealthy
	}
	if stateColor == ColorBroken {
		return ColorBroken
	}
	if depStr := r.Fields["deprecation_time"]; depStr != "" {
		if depTime, err := time.Parse(time.RFC3339, depStr); err == nil {
			if time.Now().After(depTime) {
				if stateColor != ColorDim {
					return ColorWarning
				}
			}
		}
	}
	return stateColor
}
