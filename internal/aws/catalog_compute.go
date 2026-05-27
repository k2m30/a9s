package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
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
var computeTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEC2InstancesPage(ctx, c.EC2, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichEC2InstanceStatus, Priority: 100},
		FieldKeys: []string{
			"instance_id", "name", "state", "type", "private_ip", "public_ip",
			"launch_time", "lifecycle", "image_id", "vpc_id",
			"system_status", "instance_status", "state_reason_code",
		},
		FieldAliases: map[string]string{
			"instance_id":  "InstanceId",
			"type":         "InstanceType",
			"state":        "State",
			"lifecycle":    "InstanceLifecycle",
			"image_id":     "ImageId",
			"key_name":     "KeyName",
			"vpc_id":       "VpcId",
			"subnet_id":    "SubnetId",
			"private_ip":   "PrivateIpAddress",
			"private_dns":  "PrivateDnsName",
			"public_ip":    "PublicIpAddress",
			"iam_profile":  "IamInstanceProfile",
			"architecture": "Architecture",
			"platform":     "Platform",
			"launch_time":  "LaunchTime",
		},
		Related: []domain.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkEC2TargetGroups, NeedsTargetCache: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEC2ASG, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEC2Alarms, NeedsTargetCache: true},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkEC2NodeGroups, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEC2CFN, NeedsTargetCache: true},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkEC2EIP, NeedsTargetCache: true},
			{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkEC2EBS},
			{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEC2EBSSnap, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkEC2CloudTrailEvents, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEC2SG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkEC2VPC},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEC2Role},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkEC2AMI},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEC2ENI},
			{TargetType: "subnet", DisplayName: "Subnet", Checker: checkEC2Subnet},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkEC2KMS, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEC2Logs, NeedsTargetCache: true},
			{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkEC2SSM},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkEC2Backup, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetId", TargetType: "subnet"},
			{FieldPath: "ImageId", TargetType: "ami"},
			{FieldPath: "BlockDeviceMappings.Ebs.VolumeId", TargetType: "ebs"},
			{FieldPath: "SecurityGroups.GroupId", TargetType: "sg"},
			{FieldPath: "NetworkInterfaces.NetworkInterfaceId", TargetType: "eni"},
			{FieldPath: "IamInstanceProfile.Arn", TargetType: "role"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchECSServicesPage(ctx, c.ECS, c.ECS, c.ECS, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichECSServices, Priority: 100},
		FieldKeys: []string{
			"service_name", "cluster", "status", "desired_count",
			"running_count", "launch_type", "task_definition",
		},
		Related: []domain.RelatedDef{
			{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkECSSvcCluster},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkECSSvcTargetGroups},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSSvcAlarms, NeedsTargetCache: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkECSSvcELB, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSSvcLogs, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkECSSvcSG},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkECSSvcRole},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECSSvcCFN, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkECSSvcCTEvents, NeedsTargetCache: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkECSSvcEbRule, NeedsTargetCache: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkECSSvcECR},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkECSSvcTasks, NeedsTargetCache: true},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkECSSvcSecrets},
			{TargetType: "sfn", DisplayName: "Step Functions", Checker: checkECSSvcSFN, NeedsTargetCache: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkECSSvcSubnet},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkECSSvcVPC, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "ClusterArn", TargetType: "ecs"},
			{FieldPath: "RoleArn", TargetType: "role"},
			{FieldPath: "NetworkConfiguration.AwsvpcConfiguration.Subnets", TargetType: "subnet"},
			{FieldPath: "NetworkConfiguration.AwsvpcConfiguration.SecurityGroups", TargetType: "sg"},
			{FieldPath: "LoadBalancers.TargetGroupArn", TargetType: "tg"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchECSClustersPage(ctx, c.ECS, c.ECS, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichECSClusters, Priority: 100},
		FieldKeys: []string{"cluster_name", "status", "running_tasks", "pending_tasks", "services_count"},
		Related: []domain.RelatedDef{
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkECSServices, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSAlarms, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECSCFN, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkECSKMS},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkECSASG, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkECSEC2, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkECSCTEvents, NeedsTargetCache: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkECSTasks, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSLogs, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Configuration.ExecuteCommandConfiguration.KmsKeyId", TargetType: "kms"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return fetchECSTasksPageWithJoin(ctx, c.ECS, c.ECS, c.ECS, c.ECS, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichECSTasks, Priority: 100},
		FieldKeys: []string{"task_id", "cluster", "last_status", "stop_code", "health_status", "task_definition", "launch_type", "cpu", "memory", "status", "efs_file_system_ids"},
		Related: []domain.RelatedDef{
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkECSTaskService},
			{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkECSTaskCluster},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSTaskLogs, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkECSTaskRole},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSTaskAlarm, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkECSTaskCTEvents, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkECSTaskEC2},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkECSTaskECR},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkECSTaskENI},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkECSTaskSecrets},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkECSTaskSG},
			{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkECSTaskSSM},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkECSTaskSubnet},
		},
		// ecstypes.Task: ClusterArn (parent cluster for this task execution)
		Navigable: []domain.NavigableField{
			{FieldPath: "ClusterArn", TargetType: "ecs"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchLambdaFunctionsPage(ctx, c.Lambda, continuationToken)
		},
		FieldKeys: []string{
			"function_name", "runtime", "state", "memory", "timeout", "handler",
			"last_modified", "code_size", "log_group", "package_type",
			"event_source_arn", "arn",
		},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkLambdaRole, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkLambdaAlarms, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkLambdaLogs, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkLambdaSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkLambdaVPC},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkLambdaKMS},
			{TargetType: "sqs", DisplayName: "SQS Queues", Checker: checkLambdaSQS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkLambdaCFN, NeedsTargetCache: false},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkLambdaEBRule, NeedsTargetCache: false},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkLambdaSubnet},
			{TargetType: "efs", DisplayName: "EFS File Systems", Checker: checkLambdaEFS},
			{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkLambdaAPIGW, NeedsTargetCache: true},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkLambdaCF, NeedsTargetCache: true},
			{TargetType: "ddb", DisplayName: "DynamoDB Tables", Checker: checkLambdaDDB},
			{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkLambdaKinesis},
			{TargetType: "msk", DisplayName: "MSK Clusters", Checker: checkLambdaMSK},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkLambdaCTEvents, NeedsTargetCache: true},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkLambdaTG, NeedsTargetCache: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkLambdaSNS, NeedsTargetCache: true},
			{TargetType: "sns-sub", DisplayName: "SNS Subscriptions", Checker: checkLambdaSNSSub, NeedsTargetCache: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkLambdaS3, NeedsTargetCache: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkLambdaECR},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkLambdaENI, NeedsTargetCache: true},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkLambdaSecrets, NeedsTargetCache: true},
			{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkLambdaSSM, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Role", TargetType: "role"},
			{FieldPath: "KMSKeyArn", TargetType: "kms"},
			{FieldPath: "VpcConfig.VpcId", TargetType: "vpc"},
			{FieldPath: "VpcConfig.SubnetIds", TargetType: "subnet"},
			{FieldPath: "VpcConfig.SecurityGroupIds", TargetType: "sg"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchAutoScalingGroupsPage(ctx, c.AutoScaling, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichASGScalingActivities, Priority: 100},
		FieldKeys: []string{
			"asg_name", "min_size", "max_size", "desired", "instances", "status",
			"instances_unhealthy_count", "in_service_count", "suspended_processes",
		},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkASGEC2},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkASGTG},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkASGSubnets},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkASGAlarm, NeedsTargetCache: true},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkASGNG, NeedsTargetCache: true},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkASGAMI, NeedsTargetCache: false},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkASGELB, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkASGRole, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkASGSG, NeedsTargetCache: false},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkASGSNS, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkASGVPC, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("asg")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "TargetGroupARNs", TargetType: "tg"},
			{FieldPath: "VPCZoneIdentifier", TargetType: "subnet"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEBSVolumesPage(ctx, c.EC2, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichEBSVolumeStatus, Priority: 10},
		FieldKeys: []string{"volume_id", "name", "state", "size", "type", "iops", "encrypted", "attached_to", "az", "created"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instance", Checker: checkEBSEC2, NeedsTargetCache: false},
			{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEBSSnap, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEBSKMS, NeedsTargetCache: false},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkEBSAlarm, NeedsTargetCache: true},
			{TargetType: "backup", DisplayName: "Backup", Checker: checkEBSBackup},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkEBSCFN, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ebs")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Attachments.InstanceId", TargetType: "ec2"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEBSSnapshotsPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"snapshot_id", "name", "state", "volume_id", "size", "encrypted", "description", "started", "progress"},
		FetchByIDs: func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return nil, fmt.Errorf("AWS clients not initialized")
			}
			return FetchEBSSnapshotsByIDs(ctx, c.EC2, ids)
		},
		Related: []domain.RelatedDef{
			{TargetType: "ami", DisplayName: "AMIs", Checker: checkEBSSnapAMI, NeedsTargetCache: true},
			{TargetType: "ebs", DisplayName: "EBS Volume", Checker: checkEBSSnapEBS, NeedsTargetCache: false},
			{TargetType: "ec2", DisplayName: "EC2 Instance", Checker: checkEBSSnapEC2, NeedsTargetCache: false},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEBSSnapKMS, NeedsTargetCache: false},
			{TargetType: "backup", DisplayName: "Backup", Checker: checkEBSSnapBackup},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ebs-snap")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VolumeId", TargetType: "ebs"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
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
				ID:   id,
				Name: id,
				Fields: map[string]string{
					"image_id": id,
					"ImageId":  id,
					"name":     id,
					"Name":     id,
				},
			}
		},
		Color: colorAMI,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchAMIsPage(ctx, c.EC2, continuationToken)
		},
		FetchByIDs: func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return nil, fmt.Errorf("AWS clients not initialized")
			}
			return FetchAMIsByIDs(ctx, c.EC2, ids)
		},
		FieldKeys: []string{
			"image_id", "name", "state", "architecture", "platform",
			"root_device_type", "creation_date", "public", "deprecated",
		},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkAMIEC2, NeedsTargetCache: true},
			{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkAMIEBSSnaps, NeedsTargetCache: false},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkAMIASG, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkAMICFN, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkAMIKMS},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkAMING, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ami")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "BlockDeviceMappings.Ebs.SnapshotId", TargetType: "ebs-snap"},
		},
	},
}

var computeChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Lambda Invocations",
		ShortName: "lambda_invocations",
		Columns:   resource.LambdaInvocationColumns(),
		FieldKeys: []string{
			"request_id", "timestamp", "status", "duration_ms",
			"billed_duration_ms", "memory_size_mb", "memory_used_mb",
			"memory_used", "init_duration_ms", "cold_start", "xray_trace_id",
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "lambda_invocation_logs",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group": "@parent.log_group", "request_id": "request_id"},
			DisplayNameKey: "request_id",
		}},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchLambdaInvocations(ctx, c.CloudWatchLogs, parentCtx["function_name"], parentCtx["log_group"], continuationToken)
		},
	},
	{
		Name:      "Lambda Invocation Logs",
		ShortName: "lambda_invocation_logs",
		Columns:   resource.LambdaInvocationLogColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{"timestamp", "message"},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchLambdaInvocationLogs(ctx, c.CloudWatchLogs, parentCtx["log_group"], parentCtx["request_id"], continuationToken)
		},
	},
	{
		Name:      "Scaling Activities",
		ShortName: "asg_activities",
		Columns:   resource.AsgActivityColumns(),
		FieldKeys: []string{"start_time", "status_code", "description", "cause"},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchAsgActivities(ctx, c.AutoScaling, parentCtx, continuationToken)
		},
	},
}
