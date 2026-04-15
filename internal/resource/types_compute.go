package resource

import (
	"strconv"
	"strings"
	"time"
)

// deprecatedLambdaRuntimes is the set of Lambda runtime identifiers that AWS
// has end-of-lifed per docs/attention-signals.md.
var deprecatedLambdaRuntimes = map[string]struct{}{
	"nodejs":          {},
	"nodejs4.3":       {},
	"nodejs6.10":      {},
	"nodejs8.10":      {},
	"nodejs10.x":      {},
	"nodejs12.x":      {},
	"nodejs14.x":      {},
	"python2.7":       {},
	"python3.6":       {},
	"python3.7":       {},
	"ruby2.5":         {},
	"ruby2.7":         {},
	"dotnetcore1.0":   {},
	"dotnetcore2.0":   {},
	"dotnetcore2.1":   {},
	"dotnetcore3.1":   {},
	"java8":           {},
	"go1.x":           {},
}

func computeResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "EC2 Instances",
			ShortName:     "ec2",
			Aliases:       []string{"ec2", "instances"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "lifecycle", Title: "Lifecycle", Width: 12, Sortable: true},
				{Key: "type", Title: "Type", Width: 14, Sortable: true},
				{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
				{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
				{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
				{Key: "launch_time", Title: "Launch Time", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				sys := r.Fields["system_status"]
				inst := r.Fields["instance_status"]
				if sys == "impaired" || inst == "impaired" {
					return ColorBroken
				}
				if sys == "initializing" || inst == "initializing" {
					return ColorWarning
				}
				// Prefer Fields["state"] (set by real fetcher); fall back to r.Status
				// for test doubles and synthetic resources that only set Status.
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
					// Server.* reason code means AWS forced the stop (capacity issue) → Broken.
					if strings.HasPrefix(r.Fields["state_reason_code"], "Server.") {
						return ColorBroken
					}
					// User-initiated or default stop → Warning (intentional, not broken).
					return ColorWarning
				case "terminated":
					return ColorDim
				}
				// Delegate unknown states to the shared fallback classifier so generic
				// status strings (e.g. "failed", "error", "creating") are handled correctly.
				return fallbackColor(state)
			},
			CellDecorators: map[string]func(Resource, string) string{
				"state": func(r Resource, v string) string {
					// Only decorate a running instance — the prefix signals that a
					// background status check is degrading a nominally-up instance.
					if v != "running" {
						return v
					}
					sys := r.Fields["system_status"]
					inst := r.Fields["instance_status"]
					if sys == "impaired" || inst == "impaired" {
						return "! " + v
					}
					if sys == "initializing" || inst == "initializing" {
						return "~ " + v
					}
					return v
				},
			},
		},
		{
			Name:          "ECS Services",
			ShortName:     "ecs-svc",
			Aliases:       []string{"ecs-svc", "ecs-services"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "service_name", Title: "Service Name", Width: 32, Sortable: true},
				{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "desired_count", Title: "Desired", Width: 9, Sortable: true},
				{Key: "running_count", Title: "Running", Width: 9, Sortable: true},
				{Key: "launch_type", Title: "Launch Type", Width: 12, Sortable: true},
			},
			Color: func(r Resource) Color {
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
			},
			Children: []ChildViewDef{
				{
					ChildType:      "ecs_tasks",
					Key:            "enter",
					ContextKeys:    map[string]string{"cluster": "cluster", "service_name": "service_name"},
					DisplayNameKey: "service_name",
				},
				{
					ChildType:      "ecs_svc_events",
					Key:            "e",
					ContextKeys:    map[string]string{"cluster": "cluster", "service_name": "service_name"},
					DisplayNameKey: "service_name",
				},
				{
					ChildType:      "ecs_svc_logs",
					Key:            "L",
					ContextKeys:    map[string]string{"cluster": "cluster", "service_name": "service_name", "task_definition": "task_definition"},
					DisplayNameKey: "service_name",
				},
			},
		},
		{
			Name:          "ECS Clusters",
			ShortName:     "ecs",
			Aliases:       []string{"ecs", "ecs-clusters"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "cluster_name", Title: "Cluster Name", Width: 32, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "running_tasks", Title: "Running", Width: 9, Sortable: true},
				{Key: "pending_tasks", Title: "Pending", Width: 9, Sortable: true},
				{Key: "services_count", Title: "Services", Width: 10, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["status"] {
				case "ACTIVE":
					return ColorHealthy
				case "PROVISIONING", "DEPROVISIONING":
					return ColorWarning
				case "FAILED", "INACTIVE":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:          "ECS Tasks",
			ShortName:     "ecs-task",
			Aliases:       []string{"ecs-task", "ecs-tasks", "tasks"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "task_id", Title: "Task ID", Width: 38, Sortable: true},
				{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "task_definition", Title: "Task Definition", Width: 30, Sortable: true},
				{Key: "launch_type", Title: "Launch", Width: 10, Sortable: true},
				{Key: "cpu", Title: "CPU", Width: 6, Sortable: true},
				{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
			},
			Color: func(r Resource) Color {
				// health_status == UNHEALTHY overrides everything (Broken wins).
				if r.Fields["health_status"] == "UNHEALTHY" {
					return ColorBroken
				}
				switch r.Fields["last_status"] {
				case "RUNNING":
					return ColorHealthy
				case "PROVISIONING", "PENDING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING":
					return ColorWarning
				case "STOPPED":
					sc := r.Fields["stop_code"]
					if sc != "" && sc != "UserInitiated" {
						return ColorBroken
					}
					return ColorDim
				}
				return ColorHealthy
			},
		},
		{
			Name:          "Lambda Functions",
			ShortName:     "lambda",
			Aliases:       []string{"lambda", "functions"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:Fields.arn",
			Columns: []Column{
				{Key: "function_name", Title: "Function Name", Width: 36, Sortable: true},
				{Key: "runtime", Title: "Runtime", Width: 16, Sortable: true},
				{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
				{Key: "timeout", Title: "Timeout", Width: 8, Sortable: true},
				{Key: "handler", Title: "Handler", Width: 30, Sortable: false},
				{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Compute state-based color first.
				var stateColor Color
				switch r.Fields["state"] {
				case "Active":
					stateColor = ColorHealthy
				case "Pending":
					stateColor = ColorWarning
				case "Inactive":
					stateColor = ColorDim
				case "Failed":
					stateColor = ColorBroken
				default:
					stateColor = ColorHealthy
				}
				// Override signals — Broken wins over Warning wins over Dim.
				// last_update_status=Failed → Broken.
				if r.Fields["last_update_status"] == "Failed" {
					return ColorBroken
				}
				// Deprecated runtime → Broken.
				if _, ok := deprecatedLambdaRuntimes[r.Fields["runtime"]]; ok {
					return ColorBroken
				}
				// State-based Broken is already set; return it before Warning upgrades.
				if stateColor == ColorBroken {
					return ColorBroken
				}
				// No dead-letter queue → Warning (unless already Broken).
				if r.Fields["dlq_target_arn"] == "" {
					return ColorWarning
				}
				return stateColor
			},
			Children: []ChildViewDef{
				{
					ChildType:      "lambda_invocations",
					Key:            "enter",
					ContextKeys:    map[string]string{"function_name": "function_name", "log_group": "log_group"},
					DisplayNameKey: "function_name",
				},
			},
		},
		{
			Name:          "Auto Scaling Groups",
			ShortName:     "asg",
			Aliases:       []string{"asg", "autoscaling", "auto-scaling"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "asg_name", Title: "ASG Name", Width: 36, Sortable: true},
				{Key: "min_size", Title: "Min", Width: 6, Sortable: true},
				{Key: "max_size", Title: "Max", Width: 6, Sortable: true},
				{Key: "desired", Title: "Desired", Width: 8, Sortable: true},
				{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
			},
			Color: func(r Resource) Color {
				// status="" → Healthy; "Delete in progress" → Warning (transitional, noteworthy).
				status := r.Fields["status"]
				if status == "Delete in progress" {
					return ColorWarning
				}

				// TODO(enrichment): instances_unhealthy_count, in_service_count, and
				// suspended_processes are not yet populated by the ASG fetcher
				// (autoscaling.go registers only asg_name, min_size, max_size, desired,
				// instances, status). The checks below are wired but will never fire
				// until the fetcher is extended to emit those fields.

				// InService count below minimum → Broken.
				inService := r.Fields["in_service_count"]
				minSz := r.Fields["min_size"]
				if inService != "" && minSz != "" {
					inSvc, err1 := strconv.Atoi(inService)
					minSzInt, err2 := strconv.Atoi(minSz)
					if err1 == nil && err2 == nil && inSvc < minSzInt {
						return ColorBroken
					}
				}

				// Any unhealthy instances → Warning.
				if unhealthy := r.Fields["instances_unhealthy_count"]; unhealthy != "" {
					if n, err := strconv.Atoi(unhealthy); err == nil && n > 0 {
						return ColorWarning
					}
				}

				// Critical suspended processes → Warning.
				if sp := r.Fields["suspended_processes"]; sp != "" {
					if strings.Contains(sp, "Launch") || strings.Contains(sp, "Terminate") || strings.Contains(sp, "HealthCheck") {
						return ColorWarning
					}
				}

				return ColorHealthy
			},
			Children: []ChildViewDef{
				{ChildType: "asg_activities", Key: "enter", ContextKeys: map[string]string{"asg_name": "asg_name"}, DisplayNameKey: "asg_name"},
			},
		},
		{
			Name:          "Elastic Beanstalk",
			ShortName:     "eb",
			Aliases:       []string{"eb", "beanstalk", "elastic-beanstalk"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "environment_name", Title: "Environment", Width: 28, Sortable: true},
				{Key: "application_name", Title: "Application", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "health", Title: "Health", Width: 10, Sortable: true},
				{Key: "version_label", Title: "Version", Width: 16, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Environment health takes precedence over status when available.
				switch r.Fields["health"] {
				case "Red":
					return ColorBroken
				case "Yellow":
					return ColorWarning
				case "Grey":
					return ColorDim
				case "Green":
					return ColorHealthy
				}
				// Fall back to status when health is not set.
				switch r.Fields["status"] {
				case "Ready":
					return ColorHealthy
				case "Launching", "Updating":
					return ColorWarning
				case "Terminating", "Terminated":
					return ColorDim
				}
				return ColorHealthy
			},
		},
		{
			Name:          "EBS Volumes",
			ShortName:     "ebs",
			Aliases:       []string{"ebs", "volumes", "ebs-vol"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 24, Sortable: true},
				{Key: "volume_id", Title: "Volume ID", Width: 22, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "size", Title: "Size (GiB)", Width: 10, Sortable: true},
				{Key: "type", Title: "Type", Width: 8, Sortable: true},
				{Key: "iops", Title: "IOPS", Width: 8, Sortable: true},
				{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
				{Key: "attached_to", Title: "Attached To", Width: 20, Sortable: true},
				{Key: "az", Title: "AZ", Width: 16, Sortable: true},
				{Key: "created", Title: "Created", Width: 18, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Base color from volume state.
				var base Color
				switch r.Fields["state"] {
				case "in-use":
					base = ColorHealthy
				case "available":
					base = ColorHealthy
					// Orphan check: unattached and older than 7 days.
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
				// Do not downgrade Broken.
				if base == ColorBroken {
					return ColorBroken
				}
				// Unencrypted → upgrade to Warning (CIS EC2.7).
				if r.Fields["encrypted"] == "false" && base == ColorHealthy {
					base = ColorWarning
				}
				return base
			},
		},
		{
			Name:          "EBS Snapshots",
			ShortName:     "ebs-snap",
			Aliases:       []string{"ebs-snap", "snapshots", "snap"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 24, Sortable: true},
				{Key: "snapshot_id", Title: "Snapshot ID", Width: 24, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "volume_id", Title: "Volume ID", Width: 22, Sortable: true},
				{Key: "size", Title: "Size (GiB)", Width: 10, Sortable: true},
				{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: true},
				{Key: "started", Title: "Started", Width: 18, Sortable: true},
				{Key: "progress", Title: "Progress", Width: 10, Sortable: false},
			},
			Color: func(r Resource) Color {
				// Base color from snapshot state.
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
				// Do not downgrade Broken.
				if base == ColorBroken {
					return base
				}
				// CIS EC2.1 — unencrypted snapshot.
				if r.Fields["encrypted"] == "false" {
					return ColorWarning
				}
				// Long-lived automated snapshot (> 365 days).
				if started, err := time.Parse(time.RFC3339, r.Fields["started"]); err == nil {
					if time.Since(started) > 365*24*time.Hour {
						desc := r.Fields["description"]
						if strings.HasPrefix(desc, "Created by CreateImage") ||
							strings.Contains(strings.ToLower(desc), "automated") {
							return ColorWarning
						}
					}
				}
				// Orphaned snapshot — source volume is gone.
				if strings.HasPrefix(r.Fields["volume_id"], "vol-") && r.Fields["volume_orphan"] == "true" {
					return ColorWarning
				}
				return base
			},
		},
		{
			Name:          "AMIs",
			ShortName:     "ami",
			Aliases:       []string{"ami", "amis", "images"},
			Category:      "COMPUTE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 32, Sortable: true},
				{Key: "image_id", Title: "Image ID", Width: 22, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "architecture", Title: "Arch", Width: 10, Sortable: true},
				{Key: "platform", Title: "Platform", Width: 16, Sortable: true},
				{Key: "root_device_type", Title: "Root Device", Width: 14, Sortable: true},
				{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "public", Title: "Public", Width: 8, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Compute state-based color first.
				var stateColor Color
				switch r.Fields["state"] {
				case "available":
					stateColor = ColorHealthy
				case "pending", "transient":
					stateColor = ColorWarning
				case "failed", "error", "invalid":
					stateColor = ColorBroken
				case "deregistered", "disabled":
					// Admin terminal states — dim, not broken.
					stateColor = ColorDim
				default:
					stateColor = ColorHealthy
				}
				// Broken wins over everything.
				if stateColor == ColorBroken {
					return ColorBroken
				}
				// Deprecated AMI (deprecation_time is in the past) → Warning.
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
			},
			StubCreator: func(id string) Resource {
				return Resource{
					ID:     id,
					Name:   id,
					Status: "-",
					Fields: map[string]string{
						"image_id": id,
						"ImageId":  id,
						"name":     id,
						"Name":     id,
					},
				}
			},
		},
	}
}
