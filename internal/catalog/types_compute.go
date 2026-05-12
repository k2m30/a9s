package catalog

import (
	"strconv"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// deprecatedLambdaRuntimes is the set of Lambda runtime identifiers that AWS
// has end-of-lifed per docs/attention-signals.md.
var deprecatedLambdaRuntimes = map[string]struct{}{ //nolint:gochecknoglobals // static catalog: intentional package-level var
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

func colorEC2(r domain.Resource) domain.Color {
	for i := range r.Findings {
		if r.Findings[i].Source == "wave1" {
			return colorFromSeverity(r.Findings[i].Severity)
		}
	}
	sys := r.Fields["system_status"]
	inst := r.Fields["instance_status"]
	if sys == "impaired" || inst == "impaired" {
		return domain.ColorBroken
	}
	if sys == "initializing" || inst == "initializing" {
		return domain.ColorWarning
	}
	state := r.Fields["state"]
	if state == "" {
		state = r.Status
	}
	switch state {
	case "running", "":
		return domain.ColorHealthy
	case "pending", "shutting-down", "stopping":
		return domain.ColorWarning
	case "stopped":
		if strings.HasPrefix(r.Fields["state_reason_code"], "Server.") {
			return domain.ColorBroken
		}
		return domain.ColorWarning
	case "terminated":
		return domain.ColorDim
	}
	return colorFallback(state)
}

func colorECSSvc(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["status"] {
	case "INACTIVE":
		return domain.ColorBroken
	case "DRAINING":
		return domain.ColorWarning
	}
	running := r.Fields["running_count"]
	desired := r.Fields["desired_count"]
	if desired == "0" || desired == "" {
		return domain.ColorHealthy
	}
	if running == "0" {
		return domain.ColorBroken
	}
	if running != desired {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorECSCluster(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		return domain.ColorHealthy
	case "PROVISIONING", "DEPROVISIONING":
		return domain.ColorWarning
	case "FAILED", "INACTIVE":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func colorECSTask(r domain.Resource) domain.Color {
	if r.Fields["health_status"] == "UNHEALTHY" {
		return domain.ColorBroken
	}
	if r.Fields["last_status"] == "STOPPED" && r.Fields["stop_code"] != "" && r.Fields["stop_code"] != "UserInitiated" {
		return domain.ColorBroken
	}
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["last_status"] {
	case "RUNNING":
		return domain.ColorHealthy
	case "PROVISIONING", "PENDING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING":
		return domain.ColorWarning
	case "STOPPED":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorLambda(r domain.Resource) domain.Color {
	if r.Fields["last_update_status"] == "Failed" {
		return domain.ColorBroken
	}
	if _, ok := deprecatedLambdaRuntimes[r.Fields["runtime"]]; ok {
		return domain.ColorBroken
	}
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	state := r.Fields["state"]
	if state == "" {
		state = r.Status
	}
	switch state {
	case "Failed":
		return domain.ColorBroken
	case "Pending":
		return domain.ColorWarning
	}
	if r.Fields["dlq_target_arn"] == "" {
		return domain.ColorWarning
	}
	if state == "Inactive" {
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorASG(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	status := r.Fields["status"]
	if status == "Delete in progress" {
		return domain.ColorWarning
	}
	inService := r.Fields["in_service_count"]
	minSz := r.Fields["min_size"]
	if inService != "" && minSz != "" {
		inSvc, err1 := strconv.Atoi(inService)
		minSzInt, err2 := strconv.Atoi(minSz)
		if err1 == nil && err2 == nil && inSvc < minSzInt {
			return domain.ColorBroken
		}
	}
	if unhealthy := r.Fields["instances_unhealthy_count"]; unhealthy != "" {
		if n, err := strconv.Atoi(unhealthy); err == nil && n > 0 {
			return domain.ColorWarning
		}
	}
	if sp := r.Fields["suspended_processes"]; sp != "" {
		if strings.Contains(sp, "Launch") || strings.Contains(sp, "Terminate") || strings.Contains(sp, "HealthCheck") {
			return domain.ColorWarning
		}
	}
	return domain.ColorHealthy
}

func colorEB(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	var healthColor domain.Color
	healthSet := true
	switch r.Fields["health"] {
	case "Red":
		healthColor = domain.ColorBroken
	case "Yellow":
		healthColor = domain.ColorWarning
	case "Grey":
		healthColor = domain.ColorWarning
	case "Green":
		healthColor = domain.ColorHealthy
	default:
		healthSet = false
		healthColor = domain.ColorHealthy
	}
	if r.Fields["status"] == "Terminated" && healthColor != domain.ColorBroken {
		return domain.ColorDim
	}
	if healthSet {
		return healthColor
	}
	switch r.Fields["status"] {
	case "Ready":
		return domain.ColorHealthy
	case "Launching", "Updating":
		return domain.ColorWarning
	case "Terminating":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorEBS(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	var base domain.Color
	switch r.Fields["state"] {
	case "in-use":
		base = domain.ColorHealthy
	case "available":
		base = domain.ColorHealthy
		if r.Fields["attached_to"] == "" {
			if t, err := time.Parse("2006-01-02 15:04", r.Fields["created"]); err == nil {
				if time.Since(t) > 7*24*time.Hour {
					base = domain.ColorWarning
				}
			}
		}
	case "creating", "deleting":
		base = domain.ColorWarning
	case "error":
		base = domain.ColorBroken
	default:
		base = domain.ColorHealthy
	}
	if base == domain.ColorBroken {
		return domain.ColorBroken
	}
	if r.Fields["encrypted"] == "false" && base == domain.ColorHealthy {
		base = domain.ColorWarning
	}
	return base
}

func colorEBSSnap(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	var base domain.Color
	switch r.Fields["state"] {
	case "completed":
		base = domain.ColorHealthy
	case "pending":
		base = domain.ColorWarning
	case "error", "recoverable", "recovering":
		base = domain.ColorBroken
	default:
		base = domain.ColorHealthy
	}
	if base == domain.ColorBroken {
		return base
	}
	if r.Fields["encrypted"] == "false" {
		return domain.ColorWarning
	}
	if started, err := time.Parse(time.RFC3339, r.Fields["started"]); err == nil {
		if time.Since(started) > 365*24*time.Hour {
			desc := r.Fields["description"]
			if strings.HasPrefix(desc, "Created by CreateImage") ||
				strings.Contains(strings.ToLower(desc), "automated") {
				return domain.ColorWarning
			}
		}
	}
	if strings.HasPrefix(r.Fields["volume_id"], "vol-") && r.Fields["volume_orphan"] == "true" {
		return domain.ColorWarning
	}
	return base
}

func colorAMI(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	var stateColor domain.Color
	switch r.Fields["state"] {
	case "available":
		stateColor = domain.ColorHealthy
	case "pending", "transient":
		stateColor = domain.ColorWarning
	case "failed", "error", "invalid":
		stateColor = domain.ColorBroken
	case "deregistered", "disabled":
		stateColor = domain.ColorDim
	default:
		stateColor = domain.ColorHealthy
	}
	if stateColor == domain.ColorBroken {
		return domain.ColorBroken
	}
	if depStr := r.Fields["deprecation_time"]; depStr != "" {
		if depTime, err := time.Parse(time.RFC3339, depStr); err == nil {
			if time.Now().After(depTime) {
				if stateColor != domain.ColorDim {
					return domain.ColorWarning
				}
			}
		}
	}
	return stateColor
}

// augmentEC2StatusChecks injects a Status Checks section after the State block.
func augmentEC2StatusChecks(r domain.Resource, sections []domain.Section) []domain.Section {
	state := r.Fields["state"]
	if state != "running" {
		return sections
	}
	sysStatus := r.Fields["system_status"]
	instStatus := r.Fields["instance_status"]
	if sysStatus == "" && instStatus == "" {
		return sections
	}
	if sysStatus == "ok" && instStatus == "ok" {
		return sections
	}

	sysVal := sysStatus
	if sysVal == "" {
		sysVal = "—"
	}
	instVal := instStatus
	if instVal == "" {
		instVal = "—"
	}

	statusSection := domain.Section{
		Title: "Status Checks",
		Items: []domain.Item{
			{
				Kind:        domain.ItemSubfield,
				Label:       "System",
				Value:       sysVal,
				Tier:        ec2StatusCheckTier(sysStatus),
				IndentLevel: 1,
			},
			{
				Kind:        domain.ItemSubfield,
				Label:       "Instance",
				Value:       instVal,
				Tier:        ec2StatusCheckTier(instStatus),
				IndentLevel: 1,
			},
		},
	}

	for i, sec := range sections {
		for j, item := range sec.Items {
			if item.Kind != domain.ItemHeader || item.Label != "State" {
				continue
			}
			endOfState := j + 1
			for endOfState < len(sec.Items) &&
				(sec.Items[endOfState].Kind == domain.ItemSubfield ||
					sec.Items[endOfState].Kind == domain.ItemSpacer) {
				endOfState++
			}
			leading := domain.Section{
				Title: sec.Title,
				Items: sec.Items[:endOfState],
			}
			var tail *domain.Section
			if endOfState < len(sec.Items) {
				tail = &domain.Section{
					Title: sec.Title,
					Items: sec.Items[endOfState:],
				}
			}
			result := make([]domain.Section, 0, len(sections)+2)
			result = append(result, sections[:i]...)
			result = append(result, leading)
			result = append(result, statusSection)
			if tail != nil {
				result = append(result, *tail)
			}
			result = append(result, sections[i+1:]...)
			return result
		}
	}
	return append(sections, statusSection)
}

func ec2StatusCheckTier(status string) string {
	switch status {
	case "ok":
		return "ok"
	case "impaired":
		return "impaired"
	case "initializing":
		return "initializing"
	default:
		return ""
	}
}

// computeTypes is the declarative catalog for all COMPUTE category resource types.
var computeTypes = []ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "EC2 Instances",
		ShortName:     "ec2",
		Aliases:       []string{"ec2", "instances"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "lifecycle", Title: "Lifecycle", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
			{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
			{Key: "launch_time", Title: "Launch Time", Width: 22, Sortable: true},
		},
		CellDecorators: map[string]func(domain.Resource, string) string{
			"state": func(r domain.Resource, v string) string {
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
		Color:   colorEC2,
		Augment: augmentEC2StatusChecks,
	},
	{
		Name:          "ECS Services",
		ShortName:     "ecs-svc",
		Aliases:       []string{"ecs-svc", "ecs-services"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "service_name", Title: "Service Name", Width: 32, Sortable: true},
			{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "desired_count", Title: "Desired", Width: 9, Sortable: true},
			{Key: "running_count", Title: "Running", Width: 9, Sortable: true},
			{Key: "launch_type", Title: "Launch Type", Width: 12, Sortable: true},
		},
		Children: []domain.ChildViewDef{
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
		Color: colorECSSvc,
	},
	{
		Name:          "ECS Clusters",
		ShortName:     "ecs",
		Aliases:       []string{"ecs", "ecs-clusters"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 32, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "running_tasks", Title: "Running", Width: 9, Sortable: true},
			{Key: "pending_tasks", Title: "Pending", Width: 9, Sortable: true},
			{Key: "services_count", Title: "Services", Width: 10, Sortable: true},
		},
		Color: colorECSCluster,
	},
	{
		Name:          "ECS Tasks",
		ShortName:     "ecs-task",
		Aliases:       []string{"ecs-task", "ecs-tasks", "tasks"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "task_id", Title: "Task ID", Width: 38, Sortable: true},
			{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "task_definition", Title: "Task Definition", Width: 30, Sortable: true},
			{Key: "launch_type", Title: "Launch", Width: 10, Sortable: true},
			{Key: "cpu", Title: "CPU", Width: 6, Sortable: true},
			{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
		},
		Color: colorECSTask,
	},
	{
		Name:          "Lambda Functions",
		ShortName:     "lambda",
		Aliases:       []string{"lambda", "functions"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:Fields.arn",
		Columns: []domain.Column{
			{Key: "function_name", Title: "Function Name", Width: 36, Sortable: true},
			{Key: "runtime", Title: "Runtime", Width: 16, Sortable: true},
			{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
			{Key: "timeout", Title: "Timeout", Width: 8, Sortable: true},
			{Key: "handler", Title: "Handler", Width: 30, Sortable: false},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{
			{
				ChildType:      "lambda_invocations",
				Key:            "enter",
				ContextKeys:    map[string]string{"function_name": "function_name", "log_group": "log_group"},
				DisplayNameKey: "function_name",
			},
		},
		Color: colorLambda,
	},
	{
		Name:          "Auto Scaling Groups",
		ShortName:     "asg",
		Aliases:       []string{"asg", "autoscaling", "auto-scaling"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "asg_name", Title: "ASG Name", Width: 36, Sortable: true},
			{Key: "min_size", Title: "Min", Width: 6, Sortable: true},
			{Key: "max_size", Title: "Max", Width: 6, Sortable: true},
			{Key: "desired", Title: "Desired", Width: 8, Sortable: true},
			{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
		},
		Children: []domain.ChildViewDef{
			{ChildType: "asg_activities", Key: "enter", ContextKeys: map[string]string{"asg_name": "asg_name"}, DisplayNameKey: "asg_name"},
		},
		Color: colorASG,
	},
	{
		Name:          "Elastic Beanstalk",
		ShortName:     "eb",
		Aliases:       []string{"eb", "beanstalk", "elastic-beanstalk"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "environment_name", Title: "Environment", Width: 28, Sortable: true},
			{Key: "application_name", Title: "Application", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "health", Title: "Health", Width: 10, Sortable: true},
			{Key: "version_label", Title: "Version", Width: 16, Sortable: true},
		},
		Color: colorEB,
	},
	{
		Name:          "EBS Volumes",
		ShortName:     "ebs",
		Aliases:       []string{"ebs", "volumes", "ebs-vol"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
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
		Color: colorEBS,
	},
	{
		Name:          "EBS Snapshots",
		ShortName:     "ebs-snap",
		Aliases:       []string{"ebs-snap", "snapshots", "snap"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
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
		Color: colorEBSSnap,
	},
	{
		Name:          "AMIs",
		ShortName:     "ami",
		Aliases:       []string{"ami", "amis", "images"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 32, Sortable: true},
			{Key: "image_id", Title: "Image ID", Width: 22, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "architecture", Title: "Arch", Width: 10, Sortable: true},
			{Key: "platform", Title: "Platform", Width: 16, Sortable: true},
			{Key: "root_device_type", Title: "Root Device", Width: 14, Sortable: true},
			{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "public", Title: "Public", Width: 8, Sortable: true},
		},
		StubCreator: func(id string) domain.Resource {
			return domain.Resource{
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
		Color: colorAMI,
	},
}
