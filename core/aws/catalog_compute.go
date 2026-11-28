// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"net/url"
	"reflect"
	"strconv"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ecsTaskStopCodeFailedDef is the stop-code finding, declared once. The
// ecs-task and ecs_tasks catalog entries are the same rows reached by two
// routes, so the definition they share is one literal.
var ecsTaskStopCodeFailedDef = catalog.FindingDef{Code: CodeECSTaskStopCodeFailed, Phrase: "stopped: <reason>", Severity: domain.SevBroken, Source: "wave1", Detail: "The task stopped because something went wrong rather than because the scheduler or the platform stopped it, so whatever it was serving stopped with it. The stop reason names the cause: an essential container exiting, a task that never started, or the host it was placed on going unhealthy are the common ones.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal, catalog.LifecycleTransitional, catalog.LifecycleFailedLive, catalog.LifecyclePaused, catalog.LifecycleTearingDown, catalog.LifecycleEnded, catalog.LifecyclePendingDeletion}}

// lambdaRuntimeDeprecation maps a Lambda runtime identifier to its AWS
// deprecation date: the "Deprecated runtimes" table and the dates the
// "Supported runtimes" table announces, from
// https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html as
// fetched on 2026-09-23. Runtimes AWS lists as "Not scheduled" are absent.
var lambdaRuntimeDeprecation = map[string]time.Time{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	"provided.al2":   lambdaRuntimeDate(2026, 7, 31),
	"nodejs20.x":     lambdaRuntimeDate(2026, 4, 30),
	"ruby3.2":        lambdaRuntimeDate(2026, 3, 31),
	"python3.9":      lambdaRuntimeDate(2025, 12, 15),
	"nodejs18.x":     lambdaRuntimeDate(2025, 9, 1),
	"dotnet6":        lambdaRuntimeDate(2024, 12, 20),
	"python3.8":      lambdaRuntimeDate(2024, 10, 14),
	"nodejs16.x":     lambdaRuntimeDate(2024, 6, 12),
	"dotnet7":        lambdaRuntimeDate(2024, 5, 14),
	"java8":          lambdaRuntimeDate(2024, 1, 8),
	"go1.x":          lambdaRuntimeDate(2024, 1, 8),
	"provided":       lambdaRuntimeDate(2024, 1, 8),
	"ruby2.7":        lambdaRuntimeDate(2023, 12, 7),
	"nodejs14.x":     lambdaRuntimeDate(2023, 12, 4),
	"python3.7":      lambdaRuntimeDate(2023, 12, 4),
	"dotnetcore3.1":  lambdaRuntimeDate(2023, 4, 3),
	"nodejs12.x":     lambdaRuntimeDate(2023, 3, 31),
	"python3.6":      lambdaRuntimeDate(2022, 7, 18),
	"dotnet5.0":      lambdaRuntimeDate(2022, 5, 10),
	"dotnetcore2.1":  lambdaRuntimeDate(2022, 1, 5),
	"nodejs10.x":     lambdaRuntimeDate(2021, 7, 30),
	"ruby2.5":        lambdaRuntimeDate(2021, 7, 30),
	"python2.7":      lambdaRuntimeDate(2021, 7, 15),
	"nodejs8.10":     lambdaRuntimeDate(2020, 3, 6),
	"nodejs4.3":      lambdaRuntimeDate(2020, 3, 5),
	"nodejs4.3-edge": lambdaRuntimeDate(2020, 3, 5),
	"nodejs6.10":     lambdaRuntimeDate(2019, 8, 12),
	"dotnetcore1.0":  lambdaRuntimeDate(2019, 6, 27),
	"dotnetcore2.0":  lambdaRuntimeDate(2019, 5, 30),
	"nodejs":         lambdaRuntimeDate(2016, 8, 30),
	// Supported runtimes, deprecation scheduled.
	"nodejs24.x":      lambdaRuntimeDate(2028, 4, 30),
	"nodejs22.x":      lambdaRuntimeDate(2027, 4, 30),
	"python3.14":      lambdaRuntimeDate(2029, 6, 30),
	"python3.13":      lambdaRuntimeDate(2029, 6, 30),
	"python3.12":      lambdaRuntimeDate(2028, 10, 31),
	"python3.11":      lambdaRuntimeDate(2027, 6, 30),
	"python3.10":      lambdaRuntimeDate(2026, 10, 31),
	"java25":          lambdaRuntimeDate(2029, 6, 30),
	"java21":          lambdaRuntimeDate(2029, 6, 30),
	"java17":          lambdaRuntimeDate(2027, 6, 30),
	"java11":          lambdaRuntimeDate(2027, 6, 30),
	"java8.al2":       lambdaRuntimeDate(2027, 6, 30),
	"dotnet10":        lambdaRuntimeDate(2028, 11, 14),
	"dotnet9":         lambdaRuntimeDate(2026, 11, 10),
	"dotnet8":         lambdaRuntimeDate(2026, 11, 10),
	"ruby4.0":         lambdaRuntimeDate(2029, 3, 31),
	"ruby3.4":         lambdaRuntimeDate(2028, 3, 31),
	"ruby3.3":         lambdaRuntimeDate(2027, 3, 31),
	"provided.al2023": lambdaRuntimeDate(2029, 6, 30),
}

func lambdaRuntimeDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// isDeprecatedLambdaRuntime reports whether AWS's deprecation date for
// runtime is on or before now.
func isDeprecatedLambdaRuntime(runtime string, now time.Time) bool {
	date, ok := lambdaRuntimeDeprecation[runtime]
	return ok && !now.Before(date)
}

func colorEC2(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorLambda(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorAMI(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

// augmentEC2StatusChecks injects a Status Checks section after the State block.
func augmentEC2StatusChecks(r domain.Resource, sections []domain.Section) []domain.Section {
	if LifecycleClassOf("ec2", r.Fields["state"]) != catalog.LifecycleNormal {
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

// CostExplorerServiceNameEC2 is the exact Cost Explorer SERVICE dimension
// value EC2's billed usage is reported under. Exported so the demo fixture
// dataset (core/demo/fixtures/costs.go's CostsResourceRowsByService) can
// key off the same symbol as the catalog entry below, instead of a
// duplicated string literal that could silently drift.
const CostExplorerServiceNameEC2 = "Amazon Elastic Compute Cloud - Compute"

// computeTypes is the declarative catalog for all COMPUTE category resource types.
var computeTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:       "EC2 Instances",
		ShortName:  "ec2",
		RowTypes:   []reflect.Type{reflect.TypeFor[ec2types.Instance]()},
		ReplayKeys: map[string]string{"State.Name": "state"},
		RefToID:    ec2RefToID,
		// UserData is base64 on DescribeInstanceAttribute and is decoded onto
		// InstanceEnriched when the detail opens.
		ComputedDetailPaths:     []string{"UserData"},
		Aliases:                 []string{"ec2", "instances"},
		Category:                "COMPUTE",
		CloudTrailKey:           "ResourceName:ID",
		CostExplorerServiceName: CostExplorerServiceNameEC2,
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#InstanceDetails:instanceId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "instance_status", Title: "Health", Width: 12},
			{Key: "lifecycle", Title: "Lifecycle", Width: 12},
			{Key: "type", Title: "Type", Width: 14},
			{Key: "private_ip", Title: "Private IP", Width: 16},
			{Key: "public_ip", Title: "Public IP", Width: 16},
			{Key: "instance_id", Title: "Instance ID", Width: 20},
			{Key: "launch_time", Title: "Launch Time", Width: 22},
		},
		Color:          colorEC2,
		StateEnum:      enumValues[ec2types.InstanceStateName](),
		NormalStates:   []string{string(ec2types.InstanceStateNameRunning)},
		UnknownState:   CodeEC2UnrecognisedState,
		Augment:        augmentEC2StatusChecks,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.EC2} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchEC2InstancesPage(ctx, c.EC2, continuationToken)
		}),
		FetchByIDs: fetchByIDsWithClients(func(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
			return FetchEC2InstancesByIDs(ctx, c.EC2, ids)
		}),
		Wave2: IssueEnricher{Fn: EnrichEC2InstanceStatus, Priority: 100, Reads: []string{"sg", "rtb"}},
		FieldKeys: []string{
			"instance_id", "name", "state", "type", "private_ip", "public_ip", "ipv6_address",
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
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkEC2TargetGroups, NeedsTargetCache: true, Truncated: true, Distinct: "the instance target groups in its VPC (TargetGroup.VpcId) whose DescribeTargetHealth lists this instance"},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEC2ASG, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEC2Alarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkEC2NodeGroups, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEC2CFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkEC2EIP, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkEC2EBS, Mirror: true},
			{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEC2EBSSnap, NeedsTargetCache: true, Truncated: true, Distinct: "snapshots of the volumes this instance has attached, by Snapshot.VolumeId, and of its AMI, by BlockDeviceMappings[].Ebs.SnapshotId"},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ec2")},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEC2SG, Mirror: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkEC2VPC, Mirror: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEC2Role, Truncated: true, Distinct: "the roles this instance's profile carries, from GetInstanceProfile"},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkEC2AMI, Mirror: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEC2ENI, Mirror: true},
			{TargetType: "subnet", DisplayName: "Subnet", Checker: checkEC2Subnet, Mirror: true},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkEC2KMS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEC2Logs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkEC2Backup, NeedsTargetCache: true, Truncated: true},
		},
		DetailEnrich: enrichEc2,
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetId", TargetType: "subnet"},
			{FieldPath: "ImageId", TargetType: "ami"},
			{FieldPath: "BlockDeviceMappings.Ebs.VolumeId", TargetType: "ebs"},
			{FieldPath: "SecurityGroups.GroupId", TargetType: "sg"},
			{FieldPath: "NetworkInterfaces.NetworkInterfaceId", TargetType: "eni"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEC2StatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance is still booting, so it is not serving yet and its status checks have not run. Give it a minute; if it stays here, check the launch's status reason and whether the instance type has capacity in that Availability Zone.", States: []string{"pending"}, Class: catalog.LifecycleTransitional},
			{Code: CodeEC2StateShuttingDown, Phrase: "shutting down", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance is being terminated and the call cannot be taken back. Anything on its instance-store volumes is gone; an EBS volume survives only if it was set not to delete on termination. If this was not intended, find the snapshots and retained volumes and launch a replacement from them.", States: []string{"shutting-down"}, Class: catalog.LifecycleTearingDown},
			{Code: CodeEC2StateStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance is on its way down and is no longer accepting traffic, so anything routed to it is failing now. Wait for it to reach a stopped state before starting it again or detaching its volumes.", States: []string{"stopping"}, Class: catalog.LifecycleTransitional},
			{Code: CodeEC2StateStopped, Phrase: "stopped", Severity: domain.SevWarn, Source: "wave1", Detail: "Nothing this instance hosts is answering, while its EBS volumes and any Elastic IP attached to it keep costing money. Start it if it should be serving, or terminate it and release its volumes if it is genuinely finished with.", States: []string{"stopped"}, Class: catalog.LifecyclePaused, SeverityDeviates: true},
			{Code: CodeEC2StateStoppedServer, Phrase: "stopped by AWS", Severity: domain.SevBroken, Source: "wave1", Detail: "AWS stopped this instance itself rather than an operator doing it, which points at degraded underlying hardware or a billing or compliance action on the account. Start it again so it comes up on different hardware, and check the instance's status events and your account notifications for the reason.", AppliesIn: []catalog.LifecycleClass{catalog.LifecyclePaused}},
			unrecognisedStateDef(CodeEC2UnrecognisedState, "<state>", "wave1", "EC2 reports an instance state a9s does not recognise, so a9s cannot tell whether the instance is serving. Check the instance in the AWS console before relying on it."),
			{Code: CodeEC2StateTerminated, Phrase: "terminated", Severity: domain.SevDim, Source: "wave1", States: []string{"terminated"}, Class: catalog.LifecycleEnded},
			{Code: ec2CodeInstanceStatusImpaired, Phrase: "impaired: system checks failing", Severity: domain.SevBroken, Source: "wave2", Detail: "AWS's own checks of the host or the instance are failing, which means the problem is below your software: the hypervisor, the network path, or the instance's ability to boot. Stop and start the instance so it moves to different hardware, and read the system log first if you need the cause on record."},
			{Code: ec2CodeInstanceStatusInitializing, Phrase: "initializing: checks in progress", Severity: domain.SevWarn, Source: "wave2", Detail: "The instance is up but its checks have not passed yet, so a load balancer will not send it traffic and an alarm on it has nothing to judge. Give it a few minutes; a check still initializing well past boot usually means the instance is not finishing its startup."},
			{Code: ec2CodeInstanceStatusInsufficient, Phrase: "status unknown: checks not reporting", Severity: domain.SevWarn, Source: "wave2", Detail: "AWS cannot reach the instance to check it, so its health is unknown rather than good, and a monitor reading this as healthy is reading nothing. Wait for the next check, and treat a run of these as a possible host problem worth a stop and start."},
			{Code: ec2CodeScheduledEvent, Phrase: "scheduled event", Severity: domain.SevWarn, Source: "wave2", Detail: "AWS has scheduled work on this instance and will carry it out at the stated deadline whether or not anyone is ready. Read the event's type and window and take the action that matches it: a reboot or maintenance event can often be rescheduled, and a retirement wants a stop and start so the instance moves to healthy hardware. Stop and start only an instance backed by EBS, and copy anything on its instance-store volumes off first, because stopping discards them."},
			{Code: CodeEC2IMDSv1Allowed, Phrase: "IMDSv1 allowed", Severity: domain.SevWarn, Source: "wave1", Detail: "Instance metadata answers requests without a session token, so an SSRF bug on this host can read the attached IAM role's credentials. Require session tokens for instance metadata."},
			{Code: CodeEC2PublicIP, Phrase: "public address", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance holds a routable public address, so every port its security groups leave open is reachable from the internet. Put it behind a NAT gateway or load balancer unless it must be addressed directly."},
			{Code: ec2CodeInternetExposed, Phrase: "<port(s) LIST> reachable from the internet", Severity: domain.SevBroken, Source: "wave2", Detail: "Sensitive ports on this instance answer from any address on the internet, so the services behind them are exposed to untargeted scanning. Narrow the security group's ingress rules to known CIDRs or reach the host through a bastion.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal}},
			{Code: ec2CodeInternetExposedAll, Phrase: "every port reachable from the internet", Severity: domain.SevBroken, Source: "wave2", Detail: "A security group on this instance admits every protocol and port from any address on the internet, so nothing the host listens on is shielded. Replace the all-protocols rule with the ports the service actually needs, from the addresses that need them.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal}},
			{Code: ec2CodeUserDataSecret, Phrase: "credential in user data", Severity: domain.SevBroken, Source: "wave2", Detail: "A credential is stored in this instance's user data, which every principal holding ec2:DescribeInstanceAttribute can read. Move the value into Secrets Manager or Systems Manager Parameter Store and rotate it."},
		},
	},
	{
		Name:           "ECS Services",
		ShortName:      "ecs-svc",
		RowTypes:       []reflect.Type{reflect.TypeFor[ecstypes.Service]()},
		RefToID:        ecsSvcRefToID,
		HumanizeFields: []string{"launch_type", "status", "SchedulingStrategy"},
		Aliases:        []string{"ecs-svc", "ecs-services"},
		Category:       "COMPUTE",
		CloudTrailKey:  "ResourceName:Fields.service_name",
		CloudTrailQualifier: catalog.CloudTrailQualifier{
			ParentField: "cluster",
			EventPaths:  []string{"requestParameters.cluster"},
		},
		LifecycleKey: "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ecs/v2/redirect?arn="+url.QueryEscape(arn)+"&region="+region)
		},
		Columns: []domain.Column{
			{Key: "service_name", Title: "Service Name", Width: 32},
			{Key: "cluster", Title: "Cluster", Width: 24},
			{Key: "status", Title: "Status", Width: 12},
			{Key: "desired_count", Title: "Desired", Width: 9},
			{Key: "running_count", Title: "Running", Width: 9},
			{Key: "launch_type", Title: "Launch Type", Width: 12},
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
		Color:          colorAnyFindingOrHealthy,
		StateEnum:      ecsSvcStatuses,
		NormalStates:   []string{"ACTIVE"},
		UnknownState:   CodeECSSvcStateUnrecognised,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.ECS} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchECSServicesPage(ctx, c.ECS, c.ECS, c.ECS, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichECSServices, Priority: 100},
		FieldKeys: []string{
			"service_name", "cluster", "status", "desired_count",
			"running_count", "launch_type", "task_definition", "arn",
			"degraded_finding", "name",
		},
		Related: []domain.RelatedDef{
			{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkECSSvcCluster, Mirror: true},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkECSSvcTargetGroups, Mirror: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSSvcAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkECSSvcELB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSSvcLogs, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkECSSvcSG},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkECSSvcRole},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECSSvcCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ecs-svc")},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkECSSvcEbRule, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkECSSvcECR, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkECSSvcTasks, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkECSSvcSecrets},
			{TargetType: "sfn", DisplayName: "Step Functions", Checker: checkECSSvcSFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkECSSvcSubnet},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkECSSvcVPC, NeedsTargetCache: true, Truncated: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "ClusterArn", TargetType: "ecs"},
			{FieldPath: "RoleArn", TargetType: "role"},
			{FieldPath: "NetworkConfiguration.AwsvpcConfiguration.Subnets", TargetType: "subnet"},
			{FieldPath: "NetworkConfiguration.AwsvpcConfiguration.SecurityGroups", TargetType: "sg"},
			{FieldPath: "LoadBalancers.TargetGroupArn", TargetType: "tg"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeECSSvcStateInactive, Phrase: "inactive (service deleted)", Severity: domain.SevBroken, Source: "wave1", Detail: "The service has been deleted and only its record remains, so it runs nothing and will never place a task again. Recreate it if the workload is still needed; otherwise remove whatever still points at it.", States: []string{"INACTIVE"}, Class: catalog.LifecycleEnded, SeverityDeviates: true},
			{Code: CodeECSSvcStateDraining, Phrase: "draining", Severity: domain.SevWarn, Source: "wave1", Detail: "The service is being wound down and its tasks are being deregistered from their load balancer, so it carries less traffic each minute. Confirm the replacement is already serving before the last task goes.", States: []string{"DRAINING"}, Class: catalog.LifecycleTearingDown},
			unrecognisedStateDef(CodeECSSvcStateUnrecognised, "<status>", "wave1", "Amazon ECS reports a service status a9s does not recognise, so a9s cannot tell whether the service is placing its tasks. Check the service's events before relying on it."),
			{Code: CodeECSSvcNoTasksRunning, Phrase: "no tasks running", Severity: domain.SevBroken, Source: "wave1", Detail: "The service is asking for tasks and none of them are running, so it is serving nothing. Read the service's events and the stopped tasks' reasons — an image pull failure, a failing health check or no capacity in the cluster are the usual causes."},
			{Code: CodeECSSvcTasksBelowDesired, Phrase: "running below desired count", Severity: domain.SevWarn, Source: "wave1", Detail: "Fewer tasks are running than the service asks for, so it is carrying its traffic on reduced capacity. Read the service's events for placement failures and check the cluster has room for the missing tasks."},
			{Code: ecsSvcCodeDeploymentFailed, Phrase: "not running its desired tasks", Severity: domain.SevBroken, Source: "wave2", Detail: "This service is not running the tasks it was asked to run: a deployment failed, the tasks cannot be placed, or the load balancer is failing their health checks. Read the service's events and task-stopped reasons to find which, then fix the task definition, the capacity, or the health check that is rejecting them."},
			{Code: ecsSvcCodePublicIP, Phrase: "tasks get public IPs", Severity: domain.SevWarn, Source: "wave2", Detail: "Every task this service launches gets its own routable public address, so each one is reachable from the internet on whatever its security groups leave open. Turn off public address assignment on the service and reach the tasks through a load balancer or NAT gateway."},
			DetailsDeniedFindingDef("ecs-svc", ""),
			DetailsUnavailableFindingDef("ecs-svc"),
		},
	},
	{
		Name:           "ECS Clusters",
		ShortName:      "ecs",
		RowTypes:       []reflect.Type{reflect.TypeFor[ecstypes.Cluster]()},
		RefToID:        ecsRefToID,
		HumanizeFields: []string{"status"},
		Aliases:        []string{"ecs", "ecs-clusters"},
		Category:       "COMPUTE",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ecs/v2/clusters/"+url.PathEscape(r.ID)+"?region="+region)
		},
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 32},
			{Key: "status", Title: "Status", Width: 12},
			{Key: "running_tasks", Title: "Running", Width: 9},
			{Key: "pending_tasks", Title: "Pending", Width: 9},
			{Key: "services_count", Title: "Services", Width: 10},
		},
		Color:          colorAnyFindingOrHealthy,
		StateEnum:      ecsClusterStatuses,
		NormalStates:   []string{"ACTIVE"},
		UnknownState:   CodeECSStateUnrecognised,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.ECS} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchECSClustersPage(ctx, c.ECS, c.ECS, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichECSClusters, Priority: 100},
		FieldKeys: []string{"cluster_name", "status", "running_tasks", "pending_tasks", "services_count", "degraded_finding", "name"},
		Related: []domain.RelatedDef{
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkECSServices, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECSCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkECSKMS},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkECSASG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkECSEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ecs")},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkECSTasks, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSLogs, NeedsTargetCache: true, Truncated: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Configuration.ExecuteCommandConfiguration.KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeECSStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is still being set up, so task placement can fail until its capacity providers are ready. Wait for it to become active before deploying services onto it.", States: []string{"PROVISIONING"}, Class: catalog.LifecycleTransitional},
			{Code: CodeECSStateDeprovisioning, Phrase: "deprovisioning", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is being torn down, so it will stop accepting task placements and anything still running on it is on borrowed time. Move the remaining services to another cluster if this was not meant to happen.", States: []string{"DEPROVISIONING"}, Class: catalog.LifecycleTearingDown},
			{Code: CodeECSStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster could not be brought up, so nothing can be scheduled on it at all. Check the capacity provider and the underlying Auto Scaling group or Fargate configuration for the error, then recreate the cluster.", States: []string{"FAILED"}, Class: catalog.LifecycleFailedLive},
			{Code: CodeECSStateInactive, Phrase: "inactive (cluster deleted)", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster has been deleted; AWS keeps returning it while old task records exist, but it can host nothing. Point any service or scheduled task that still references it at a live cluster.", States: []string{"INACTIVE"}, Class: catalog.LifecycleEnded, SeverityDeviates: true},
			unrecognisedStateDef(CodeECSStateUnrecognised, "<status>", "wave1", "Amazon ECS reports a cluster status a9s does not recognise, so a9s cannot tell whether tasks can be placed on it. Check the cluster in the ECS console before relying on it."),
			{Code: ecsCodeClusterIssue, Phrase: "tasks pending or not running", Severity: domain.SevWarn, Source: "wave2", Detail: "The cluster has tasks stuck pending, or fewer running than the services on it asked for, which usually means it has run out of processor, memory or network-interface capacity. Check the capacity provider's scaling and the placement constraints of the services that are short.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal}},
			DetailsDeniedFindingDef("ecs", ""),
			DetailsUnavailableFindingDef("ecs"),
		},
	},
	{
		Name:           "ECS Tasks",
		ShortName:      "ecs-task",
		RowTypes:       []reflect.Type{reflect.TypeFor[ecstypes.Task]()},
		RefToID:        ecsTaskRefToID,
		HumanizeFields: []string{"stop_code", "launch_type", "status", "LastStatus", "DesiredStatus", "Connectivity"},
		Aliases:        []string{"ecs-task", "ecs-tasks", "tasks"},
		Category:       "COMPUTE",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ecs/v2/redirect?arn="+url.QueryEscape(arn)+"&region="+region)
		},
		Columns: []domain.Column{
			{Key: "task_id", Title: "Task ID", Width: 38},
			{Key: "cluster", Title: "Cluster", Width: 24},
			{Key: "status", Title: "Status", Width: 12},
			{Title: "Stop Code", Path: "StopCode", Width: 24},
			{Title: "Health", Path: "HealthStatus", Width: 12},
			{Key: "task_definition", Title: "Task Definition", Width: 30},
			{Key: "launch_type", Title: "Launch", Width: 10},
			{Key: "cpu", Title: "CPU", Width: 6},
			{Key: "memory", Title: "Memory", Width: 8},
		},
		Color:          colorAnyFindingOrHealthy,
		StateEnum:      ecsTaskStatuses,
		NormalStates:   []string{"RUNNING"},
		UnknownState:   CodeECSTaskStateUnrecognised,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.ECS} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return fetchECSTasksPageWithJoin(ctx, c.ECS, c.ECS, c.ECS, c.ECS, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichECSTasks, Priority: 100, ReadsNonLive: true},
		// task_role/execution_role/secret_arns/ssm_param_names — emitted by
		// ecsJoinTaskDefinition's DescribeTaskDefinition join; required by
		// the ecs-task:role, ecs-task:secrets, and ecs-task:ssm pivots.
		FieldKeys: []string{"task_id", "cluster", "status", "stop_code", "health_status", "task_definition", "launch_type", "cpu", "memory", "efs_file_system_ids", "task_role", "execution_role", "secret_arns", "ssm_param_names", "log_groups", "log_groups_unread", "container_images", "arn", "task_def_join_error", "degraded_finding", "name"},
		Related: []domain.RelatedDef{
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkECSTaskService, Mirror: true},
			{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkECSTaskCluster, Mirror: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSTaskLogs, Truncated: true, Mirror: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkECSTaskRole, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSTaskAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ecs-task")},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkECSTaskEC2, Truncated: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkECSTaskECR, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkECSTaskENI},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkECSTaskSecrets, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkECSTaskSG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkECSTaskSSM, NeedsTargetCache: true, Truncated: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkECSTaskSubnet},
		},
		// ecstypes.Task: ClusterArn (parent cluster for this task execution)
		Navigable: []domain.NavigableField{
			{FieldPath: "ClusterArn", TargetType: "ecs"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeECSTaskStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1", Detail: "The task is waiting for its network interface and volumes before its containers start, so it is not serving yet. If it stays here, check the subnet's free addresses and the interface limit on the instance or account.", States: []string{"PROVISIONING"}, Class: catalog.LifecycleTransitional},
			{Code: CodeECSTaskStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1", Detail: "The task has been placed but its containers are not up, usually while an image is pulled. A long stay here points at a slow or failing image pull — check the registry credentials and the network path to it.", States: []string{"PENDING"}, Class: catalog.LifecycleTransitional},
			{Code: CodeECSTaskStateActivating, Phrase: "activating", Severity: domain.SevWarn, Source: "wave1", Detail: "The containers are running but the task's supporting work — service discovery, sidecar startup, load balancer registration — has not finished, so it takes no traffic yet. Wait; if it stays here, read the container dependencies in the task definition.", States: []string{"ACTIVATING"}, Class: catalog.LifecycleTransitional},
			{Code: CodeECSTaskStateDeactivating, Phrase: "deactivating", Severity: domain.SevWarn, Source: "wave1", Detail: "The task is being taken out of service and deregistered from its targets, so it is shedding traffic. Confirm a replacement task is already healthy if this service has to stay available.", States: []string{"DEACTIVATING"}, Class: catalog.LifecycleTearingDown},
			{Code: CodeECSTaskStateStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1", Detail: "The task's containers are being shut down and will not come back, so anything in flight on them ends here. Check the stop reason once it settles to see whether a deployment, a scale-in or a failure caused it.", States: []string{"STOPPING"}, Class: catalog.LifecycleTearingDown},
			{Code: CodeECSTaskStateDeprovisioning, Phrase: "deprovisioning", Severity: domain.SevWarn, Source: "wave1", Detail: "The containers have exited and AWS is releasing the task's network interface and volumes. Nothing to act on; the task disappears from this list shortly.", States: []string{"DEPROVISIONING"}, Class: catalog.LifecycleTearingDown},
			{Code: CodeECSTaskStateStopped, Phrase: "stopped (task exited)", Severity: domain.SevDim, Source: "wave1", States: []string{"STOPPED"}, Class: catalog.LifecycleEnded},
			{Code: CodeECSTaskStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1", States: []string{"DELETED"}, Class: catalog.LifecycleEnded},
			unrecognisedStateDef(CodeECSTaskStateUnrecognised, "<status>", "wave1", "Amazon ECS reports a task status a9s does not recognise, so a9s cannot tell whether the task is running its containers. Check the task in the ECS console before relying on it."),
			ecsTaskStopCodeFailedDef,
			{Code: CodeECSTaskHealthUnhealthy, Phrase: "unhealthy", Severity: domain.SevBroken, Source: "wave1", Detail: "The container health check inside this task is failing. A task run by a service is replaced by the scheduler under its deployment settings, and a load balancer stops routing to it only if its target group also marks it unhealthy; a task started on its own is neither replaced nor drained. Read the container logs from when the check started failing, and confirm the check command and its grace period suit the application's startup."},
			{Code: ecsTaskCodeTaskFailed, Phrase: "task failed", Severity: domain.SevBroken, Source: "wave2", AppliesIn: ecsTaskStopCodeFailedDef.AppliesIn, Detail: "This task ended in a failure rather than a clean stop, so the work it was doing did not complete. Read its containers' exit codes and stopped reasons to see which one went and why."},
			{Code: ecsTaskCodePrivileged, Phrase: "privileged container", Severity: domain.SevBroken, Source: "wave2", Detail: "A container in this task runs privileged, so it holds the host's full device and kernel-capability set and a container escape becomes a host compromise. Drop the privileged flag and grant only the specific Linux capabilities the workload needs."},
			{Code: ecsTaskCodeHostNamespace, Phrase: "shares the host network or process namespace", Severity: domain.SevWarn, Source: "wave2", Detail: "This task shares the host's network or process namespace, so its containers can see and reach every other process and loopback service on that instance. Switch the task definition to the awsvpc network mode and leave the process-namespace setting unset."},
			{Code: ecsTaskCodeWritableRoot, Phrase: "writable root filesystem", Severity: domain.SevWarn, Source: "wave2", Detail: "A container in this task can write to its own root filesystem, so anything that lands code on it persists for the life of the task. Make the container's root filesystem read-only and mount a volume for the paths it genuinely writes."},
			{Code: ecsTaskCodeNoLogging, Phrase: "container without log driver", Severity: domain.SevWarn, Source: "wave2", Detail: "A container in this task has no log driver, so its stdout and stderr never leave where it runs: on Fargate they are not kept at all, and on EC2 they stay in the Docker daemon's default log on that instance until the task is cleaned up. Give the container a log driver pointing at awslogs or your log router."},
			{Code: ecsTaskCodeEnvSecret, Phrase: "credential in container environment", Severity: domain.SevBroken, Source: "wave2", Detail: "A credential is stored as a plaintext environment variable in this task definition, readable by anyone who can call ecs:DescribeTaskDefinition. Move the value to Secrets Manager or Systems Manager Parameter Store and reference it through the container's `secrets` block."},
			DetailsDeniedFindingDef("ecs-task", ""),
			DetailsUnavailableFindingDef("ecs-task"),
		},
	},
	{
		Name:      "Lambda Functions",
		ShortName: "lambda",
		RowTypes:  []reflect.Type{reflect.TypeFor[lambdatypes.FunctionConfiguration]()},
		// Concurrency is a GetFunction field and lands on FunctionEnriched
		// when the detail opens.
		ComputedDetailPaths: []string{"Concurrency"},
		RefToID:             lambdaRefToID,
		HumanizeFields:      []string{"last_update_status"},
		Aliases:             []string{"lambda", "functions"},
		Category:            "COMPUTE",
		CloudTrailKey:       "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "lambda/home?region="+region+"#/functions/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "function_name", Title: "Function Name", Width: 36},
			{Key: "runtime", Title: "Runtime", Width: 16},
			{Key: "memory", Title: "Memory", Width: 8},
			{Key: "timeout", Title: "Timeout", Width: 8},
			{Key: "state", Title: "Status", Width: 10},
			{Key: "handler", Title: "Handler", Width: 30},
			{Key: "last_modified", Title: "Last Modified", Width: 22},
		},
		Children: []domain.ChildViewDef{
			{
				ChildType:      "lambda_invocations",
				Key:            "enter",
				ContextKeys:    map[string]string{"function_name": "function_name", "log_group": "log_group"},
				DisplayNameKey: "function_name",
			},
		},
		Color:          colorLambda,
		StateEnum:      enumValues[lambdatypes.State](),
		NormalStates:   []string{string(lambdatypes.StateActive)},
		UnknownState:   CodeLambdaUnrecognisedState,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.Lambda} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchLambdaFunctionsPage(ctx, c.Lambda, continuationToken)
		}),
		FieldKeys: []string{
			"function_name", "runtime", "memory",
			"timeout", "handler", "last_modified", "code_size", "log_group",
			"package_type", "event_source_arn", "dlq_target_arn", "arn",
		},
		IssueEnricherFieldKeys: []string{"state", "last_update_status"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkLambdaRole, Mirror: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkLambdaAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkLambdaLogs, NeedsTargetCache: true, Truncated: true, Distinct: "the group this function writes to, LoggingConfig.LogGroup or the /aws/lambda/<name> default"},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkLambdaSG, Mirror: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkLambdaVPC},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkLambdaKMS},
			{TargetType: "sqs", DisplayName: "SQS Queues", Checker: checkLambdaSQS, Truncated: true, Mirror: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkLambdaCFN, NeedsTargetCache: false, Truncated: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkLambdaEBRule, NeedsTargetCache: false, Truncated: true, Mirror: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkLambdaSubnet},
			{TargetType: "efs", DisplayName: "EFS File Systems", Checker: checkLambdaEFS, Mirror: true},
			{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkLambdaAPIGW, NeedsTargetCache: true, Truncated: true, Distinct: "APIs with an integration whose URI is this function"},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkLambdaCF, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ddb", DisplayName: "DynamoDB Tables", Checker: checkLambdaDDB, Truncated: true, Mirror: true},
			{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkLambdaKinesis, Truncated: true, Mirror: true},
			{TargetType: "msk", DisplayName: "MSK Clusters", Checker: checkLambdaMSK, Truncated: true, Mirror: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("lambda")},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkLambdaTG, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkLambdaSNS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sns-sub", DisplayName: "SNS Subscriptions", Checker: checkLambdaSNSSub, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkLambdaS3, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkLambdaECR, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkLambdaENI, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkLambdaSecrets, NeedsTargetCache: true, Truncated: true, Distinct: "the secret ARNs in FunctionConfiguration.Environment.Variables"},
			{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkLambdaSSM, NeedsTargetCache: true, Truncated: true},
		},
		Wave2:        IssueEnricher{Fn: EnrichLambdaPosture, Priority: 100},
		DetailEnrich: enrichLambda,
		Navigable: []domain.NavigableField{
			{FieldPath: "Role", TargetType: "role"},
			{FieldPath: "KMSKeyArn", TargetType: "kms"},
			{FieldPath: "VpcConfig.VpcId", TargetType: "vpc"},
			{FieldPath: "VpcConfig.SubnetIds", TargetType: "subnet"},
			{FieldPath: "VpcConfig.SecurityGroupIds", TargetType: "sg"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeLambdaLastUpdateFailed, Phrase: "last update failed to apply", Severity: domain.SevBroken, Source: "wave2", Detail: "The last configuration or code update did not take, so the function still runs the previous version while the console shows what you asked for. Read the update status reason — a bad VPC configuration, an invalid role or a missing layer are typical — fix it, and apply the update again."},
			{Code: CodeLambdaDeprecatedRuntime, Phrase: "runtime is end-of-life", Severity: domain.SevBroken, Source: "wave1", Detail: "This function runs on a runtime AWS no longer patches, so language and base-image security fixes will never reach it. Existing functions keep being invoked, but AWS first stops you creating new functions on it and then stops you updating this one, which turns an urgent fix into a migration under pressure. Move to a supported runtime version and redeploy while the update path is still open."},
			{Code: CodeLambdaStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave2", Detail: "The function is still being created or attached to its VPC, and invocations during this window are throttled or rejected. Wait for it to become active before wiring an event source to it.", States: []string{"Pending"}, Class: catalog.LifecycleTransitional},
			{Code: CodeLambdaStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave2", Detail: "The function cannot be invoked at all: its creation or VPC setup failed and it has no working execution environment. Read its state reason, fix the role, subnets or security groups it names, then update the function to retry.", States: []string{"Failed"}, Class: catalog.LifecycleFailedLive},
			{Code: CodeLambdaInactive, Phrase: "inactive, evicted after extended idle time", Severity: domain.SevDim, Source: "wave2", States: []string{"Inactive"}, Class: catalog.LifecyclePaused},
			{Code: CodeLambdaDeactivating, Phrase: "deactivating", Severity: domain.SevWarn, Source: "wave2", Detail: "Lambda is deactivating this function, so it is on its way out of service and new invocations are not to be relied on. Check who deactivated it and whether anything still sends it events.", States: []string{"Deactivating"}, Class: catalog.LifecycleTransitional},
			{Code: CodeLambdaDeactivated, Phrase: "deactivated", Severity: domain.SevDim, Source: "wave2", States: []string{"Deactivated"}, Class: catalog.LifecyclePaused},
			{Code: CodeLambdaActiveNonInvocable, Phrase: "active but not invocable", Severity: domain.SevBroken, Source: "wave2", Detail: "The function exists and is configured, but Lambda reports it cannot be invoked, so every event sent to it fails. Read its state reason for the cause, fix what it names, and update the function.", States: []string{"ActiveNonInvocable"}, Class: catalog.LifecycleFailedLive},
			{Code: CodeLambdaDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave2", Detail: "The function is being deleted, so its event sources and callers start failing once it is gone. If this was not intended, redeploy it from its source before anything depends on the gap.", States: []string{"Deleting"}, Class: catalog.LifecycleTearingDown},
			unrecognisedStateDef(CodeLambdaUnrecognisedState, "<state>", "wave2", "Lambda reports a function state a9s does not recognise, so a9s cannot tell whether the function can be invoked. Check the function in the AWS console before relying on it."),
			{Code: CodeLambdaNoDLQ, Phrase: "no dead-letter queue configured", Severity: domain.SevWarn, Source: "wave1", Detail: "Asynchronous invocations that exhaust their retries are dropped silently, so a bad deployment or a downstream outage loses events with no record of what was lost. Set a dead-letter queue or an on-failure destination so failed events can be inspected and replayed."},
			{Code: CodeLambdaEnvSecret, Phrase: "credential in environment variables", Severity: domain.SevBroken, Source: "wave1", Detail: "A credential is stored as a plaintext environment variable on this function, readable by anyone who can call lambda:GetFunctionConfiguration. Move the value to Secrets Manager or Systems Manager Parameter Store, read it at cold start, and rotate the exposed one."},
			{Code: lambdaCodeFunctionURLPublic, Phrase: "function endpoint open without authentication", Severity: domain.SevBroken, Source: "wave2", Detail: "The function has a web endpoint that requires no authentication, so anyone on the internet who learns the address can invoke it without credentials. Set the endpoint to require signed requests, or put an authorizing layer in front of it."},
		},
		// GetPolicy answers a function with no resource policy with
		// ResourceNotFoundException (AWS Lambda API Reference, GetPolicy).
		Policy: &catalog.PolicyDecl{Noun: "resource", Subject: "function", Public: lambdaCodePublicPolicy, Cross: lambdaCodeCrossAccountPolicy, PublicSeverity: domain.SevBroken, Source: "wave2", Required: true},
	},
	{
		Name:           "Auto Scaling Groups",
		ShortName:      "asg",
		RowTypes:       []reflect.Type{reflect.TypeFor[asgtypes.AutoScalingGroup]()},
		RefToID:        asgRefToID,
		HumanizeFields: []string{"HealthCheckType"},
		Aliases:        []string{"asg", "autoscaling", "auto-scaling"},
		Category:       "COMPUTE",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#AutoScalingGroupDetails:id="+url.PathEscape(r.ID)+";view=details")
		},
		Columns: []domain.Column{
			{Key: "asg_name", Title: "ASG Name", Width: 36},
			{Key: "min_size", Title: "Min", Width: 6},
			{Key: "max_size", Title: "Max", Width: 6},
			{Key: "desired", Title: "Desired", Width: 8},
			{Key: "instances", Title: "Instances", Width: 10},
			{Key: "status", Title: "Status", Width: 12},
		},
		Children: []domain.ChildViewDef{
			{ChildType: "asg_activities", Key: "enter", ContextKeys: map[string]string{"asg_name": "asg_name"}, DisplayNameKey: "asg_name"},
		},
		Color:          colorAnyFindingOrHealthy,
		StateEnum:      asgStatuses,
		UnknownState:   CodeASGUnrecognisedStatus,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.AutoScaling} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchAutoScalingGroupsPage(ctx, c.AutoScaling, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichASGScalingActivities, Priority: 100, ReadsNonLive: true},
		FieldKeys: []string{
			"asg_name", "min_size", "max_size", "desired", "instances", "status",
			"instances_unhealthy_count", "in_service_count", "suspended_processes",
			"vpc_zone_identifier",
		},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkASGEC2, Mirror: true},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkASGTG, Truncated: true, Mirror: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkASGSubnets, Mirror: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkASGAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkASGNG, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkASGAMI, NeedsTargetCache: false, Distinct: "the image this group launches with, LaunchConfiguration.ImageId or LaunchTemplateData.ImageId"},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkASGELB, NeedsTargetCache: false, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkASGRole, NeedsTargetCache: false, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkASGSG, NeedsTargetCache: false},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkASGSNS, NeedsTargetCache: false, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkASGVPC, NeedsTargetCache: false, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("asg")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "TargetGroupARNs", TargetType: "tg"},
			{FieldPath: "VPCZoneIdentifier", TargetType: "subnet"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeASGStateDeleting, Phrase: "delete in progress", Severity: domain.SevWarn, Source: "wave1", Detail: "The group is being removed and is terminating its instances; the deletion cannot be cancelled. If this was not intended, recreate the group from its launch template and scaling settings once it is gone, and restore the capacity by hand in the meantime.", States: []string{"Delete in progress"}, Class: catalog.LifecycleTearingDown},
			unrecognisedStateDef(CodeASGUnrecognisedStatus, "<status>", "wave1", "Auto Scaling reports a group status a9s does not recognise; the only documented one is a delete in progress. Check the group in the AWS console before relying on it."),
			{Code: CodeASGUnderprovisioned, Phrase: "<N> of <M> instances in service", Severity: domain.SevBroken, Source: "wave1", Detail: "The group is running fewer instances in service than it is meant to, so the workload behind it carries its traffic short-handed. Read the group's scaling activities for launch failures: insufficient capacity in the Availability Zone, a bad launch template, or a failing health check are the usual causes."},
			{Code: CodeASGUnhealthyInstances, Phrase: "<N unhealthy instance(s)>", Severity: domain.SevWarn, Source: "wave1", Detail: "Some instances in this group are failing their health checks and will be terminated and replaced, briefly reducing capacity. Look at those instances' system logs before they go, so the replacements do not simply repeat the failure."},
			{Code: CodeASGScalingSuspended, Phrase: "scaling suspended", Severity: domain.SevWarn, Source: "wave1", Detail: "At least one scaling process is suspended, and which one decides what the group has stopped doing. Suspending launch stops it adding capacity; suspending terminate stops it removing instances and leaves replacements half-finished; suspending the health check stops it noticing an instance has failed. Resume the process named here unless a deployment tool is holding it deliberately."},
			{Code: asgCodeScalingActivityFailed, Phrase: "latest scaling activity failed", Severity: domain.SevBroken, Source: "wave2", Detail: "The group's most recent attempt to launch or terminate an instance failed, so it is not at the size its policies asked for. The activity's status message names the cause — capacity, a launch template error, or an IAM permission — fix that and the group retries.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal, catalog.LifecycleTransitional, catalog.LifecycleTearingDown}},
			{Code: CodeASGLegacyLaunchConfig, Phrase: "uses a launch configuration", Severity: domain.SevWarn, Source: "wave1", Detail: "The group launches from a launch configuration, an immutable legacy resource AWS no longer develops — it cannot carry IMDSv2 defaults, newer instance types, or versioned edits. Copy it to a launch template and point the group at that."},
			{Code: CodeASGSingleAZ, Phrase: "single availability zone", Severity: domain.SevWarn, Source: "wave1", Detail: "Every instance in this group sits in one availability zone, so a single zone failure takes the whole group down. Add subnets from at least one more zone to the group."},
			{Code: CodeASGNoELBHealthCheck, Phrase: "no load balancer health check", Severity: domain.SevWarn, Source: "wave1", Detail: "The group is behind a load balancer but only watches EC2 status checks, so an instance whose application has stopped answering stays in service. Set the group's health check type to the load balancer's."},
			{Code: asgCodeLaunchConfigIMDSv1, Phrase: "launch configuration allows IMDSv1", Severity: domain.SevWarn, Source: "wave2", Detail: "Instances this group launches answer metadata requests without a session token, so an SSRF bug on any of them leaks the attached role's credentials. Launch configurations cannot be edited — copy this one to a launch template that requires session tokens and repoint the group."},
			{Code: asgCodeLaunchConfigPublicIP, Phrase: "launch configuration assigns public IPs", Severity: domain.SevWarn, Source: "wave2", Detail: "Every instance this group launches gets a routable public address, so each new instance is reachable from the internet on whatever its security groups leave open. Copy the launch configuration to a launch template with public address assignment off."},
			{Code: asgCodeLaunchConfigSecret, Phrase: "credential in launch configuration user data", Severity: domain.SevBroken, Source: "wave2", Detail: "A credential is pasted into the launch configuration's user data, so it is readable by anyone who can call autoscaling:DescribeLaunchConfigurations and lands on every instance the group starts. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it."},
		},
	},
	{
		Name:          "EBS Volumes",
		ShortName:     "ebs",
		RowTypes:      []reflect.Type{reflect.TypeFor[ec2types.Volume]()},
		RefToID:       ebsRefToID,
		Aliases:       []string{"ebs", "volumes", "ebs-vol"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#VolumeDetails:volumeId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24},
			{Key: "volume_id", Title: "Volume ID", Width: 22},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "size", Title: "Size (GiB)", Width: 10},
			{Key: "type", Title: "Type", Width: 8},
			{Key: "iops", Title: "IOPS", Width: 8},
			{Key: "encrypted", Title: "Encrypted", Width: 10},
			// Wide enough for the two instance ids a Multi-Attach volume
			// commonly carries; the related panel counts them all.
			{Key: "attached_to", Title: "Attached To", Width: 40},
			{Key: "az", Title: "AZ", Width: 16},
			{Key: "created", Title: "Created", Width: 18},
		},
		Color:          colorAnyFindingOrHealthy,
		StateEnum:      enumValues[ec2types.VolumeState](),
		NormalStates:   []string{string(ec2types.VolumeStateAvailable), string(ec2types.VolumeStateInUse)},
		UnknownState:   CodeEBSUnrecognisedState,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.EC2} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchEBSVolumesPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichEBSVolumeStatus, Priority: 10, Reads: []string{"backup", "ebs-snap"}},
		FieldKeys: []string{"volume_id", "name", "state", "size", "type", "iops", "encrypted", "attached_to", "az", "created"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEBSEC2, NeedsTargetCache: false, Mirror: true},
			{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEBSSnap, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEBSKMS, NeedsTargetCache: false, Mirror: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkEBSAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "backup", DisplayName: "Backup", Checker: checkEBSBackup, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkEBSCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ebs")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Attachments.InstanceId", TargetType: "ec2"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEBSStateCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1", Detail: "The volume is still being created, or restored from a snapshot, and cannot be attached until it is available. A restore from a large snapshot takes a while; wait rather than recreating it.", States: []string{"creating"}, Class: catalog.LifecycleTransitional},
			{Code: CodeEBSStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "The volume is being deleted and its data is going with it. If that was not intended, check now whether a snapshot covers it, because there is nothing to stop once the deletion completes.", States: []string{"deleting"}, Class: catalog.LifecycleTearingDown},
			{Code: CodeEBSStateError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1", Detail: "AWS has marked this volume as failed, so its data is not reachable and it cannot be attached. Restore the most recent snapshot into a new volume — the failed one will not recover.", States: []string{"error"}, Class: catalog.LifecycleFailedLive},
			{Code: CodeEBSStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1", States: []string{"deleted"}, Class: catalog.LifecycleEnded},
			unrecognisedStateDef(CodeEBSUnrecognisedState, "<state>", "wave1", "EC2 reports a volume state a9s does not recognise, so a9s cannot tell whether the volume can be attached. Check the volume in the AWS console before relying on it."),
			{Code: CodeEBSOrphanUnattached, Phrase: "orphan: unattached <N>d", Severity: domain.SevWarn, Source: "wave1", Detail: "The volume has been unattached since it was created, so it is billed hourly for no workload; the age is in the status. Snapshot it if the data matters, then delete it.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal}},
			{Code: CodeEBSUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1", Detail: "The volume's data is written to disk unencrypted, and encryption cannot be turned on in place. Snapshot it, copy the snapshot with a KMS key, and restore that copy over the volume at the next window you can take."},
			{Code: ebsCodeVolumeIODegraded, Phrase: "volume I/O degraded", Severity: domain.SevBroken, Source: "wave2", Detail: "AWS reports this volume's I/O as degraded or its data as potentially inconsistent, so reads may return stale or corrupt blocks. Take a snapshot while you still can, then restore onto a fresh volume and check the filesystem before trusting it."},
			{Code: CodeEBSNotInBackupPlan, Phrase: "not covered by a backup plan", Severity: domain.SevWarn, Source: "wave2", Detail: "No backup plan backs up this volume, so nothing is scheduled to copy it and a deletion is final. Add it to a plan by its ARN or by the EBS volume resource type; a plan that takes it in only by tag, by service name or by selecting everything backs it up only once EBS is opted in under this Region's AWS Backup service opt-in."},
			{Code: CodeEBSNoSnapshot, Phrase: "no snapshot exists", Severity: domain.SevWarn, Source: "wave2", Detail: "This volume is attached and in use, and no snapshot of it exists, so there is no point to restore from. Take one, or put the volume in a backup plan that will.", AppliesInStates: []string{string(ec2types.VolumeStateInUse)}},
		},
	},
	{
		Name:          "EBS Snapshots",
		ShortName:     "ebs-snap",
		RowTypes:      []reflect.Type{reflect.TypeFor[ec2types.Snapshot]()},
		Aliases:       []string{"ebs-snap", "snapshots", "snap"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#Snapshots:snapshotId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24},
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 24},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "volume_id", Title: "Volume ID", Width: 22},
			{Key: "size", Title: "Size (GiB)", Width: 10},
			{Key: "encrypted", Title: "Encrypted", Width: 10},
			{Key: "description", Title: "Description", Width: 30},
			{Key: "started", Title: "Started", Width: 18},
			{Key: "progress", Title: "Progress", Width: 10},
		},
		Color:          colorAnyFindingOrHealthy,
		StateEnum:      enumValues[ec2types.SnapshotState](),
		NormalStates:   []string{string(ec2types.SnapshotStateCompleted)},
		UnknownState:   CodeEBSSnapStateUnrecognised,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.EC2} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchEBSSnapshotsPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: enrichEBSSnapCrossRef, Priority: 100, Reads: []string{"ebs"}},
		FieldKeys: []string{"snapshot_id", "name", "state", "volume_id", "size", "encrypted", "description", "started", "progress"},
		FetchByIDs: fetchByIDsWithClients(func(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
			return FetchEBSSnapshotsByIDs(ctx, c.EC2, ids)
		}),
		Related: []domain.RelatedDef{
			{TargetType: "ami", DisplayName: "AMIs", Checker: checkEBSSnapAMI, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ebs", DisplayName: "EBS Volume", Checker: checkEBSSnapEBS, NeedsTargetCache: false, Truncated: true, Mirror: true},
			{TargetType: "ec2", DisplayName: "EC2 Instance", Checker: checkEBSSnapEC2, NeedsTargetCache: true, Truncated: true, Distinct: "the instance CreateImage named in Snapshot.Description"},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEBSSnapKMS, NeedsTargetCache: false},
			{TargetType: "backup", DisplayName: "Backup", Checker: checkEBSSnapBackup, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ebs-snap")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VolumeId", TargetType: "ebs"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEBSSnapStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot is still being written and cannot be used to restore a volume or copied to another region yet. Wait for it to complete before relying on it as the recovery point for anything.", States: []string{string(ec2types.SnapshotStatePending)}, Class: catalog.LifecycleTransitional},
			{Code: CodeEBSSnapStateError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1", Detail: "This snapshot failed and holds no usable copy of the volume, so any recovery plan naming it has a hole in it. Take a fresh snapshot of the source volume and delete this one.", States: []string{string(ec2types.SnapshotStateError)}, Class: catalog.LifecycleFailedLive},
			unrecognisedStateDef(CodeEBSSnapStateUnrecognised, "<state>", "wave1", "EC2 reports a snapshot state a9s does not recognise, so a9s cannot tell whether a volume can be restored from it. Check the snapshot in the AWS console before relying on it."),
			{Code: CodeEBSSnapStateRecycleBin, Phrase: "in recycle bin", Severity: domain.SevWarn, Source: "wave1", Detail: "This snapshot was deleted and is being held in the Recycle Bin, so nothing can be restored from it until it is recovered, and a retention rule will remove it for good when the period ends. Recover it from the Recycle Bin if the recovery point is still wanted.", States: []string{string(ec2types.SnapshotStateRecoverable), string(ec2types.SnapshotStateRecovering)}, Class: catalog.LifecyclePendingDeletion},
			{Code: CodeEBSSnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot's contents are stored unencrypted, and any volume restored from it starts unencrypted too. Copy it with a KMS key, restore from the copy, then delete this one."},
			{Code: CodeEBSSnapAgedAutomated, Phrase: "automated, <N>d old", Severity: domain.SevWarn, Source: "wave1", Detail: "This automated snapshot is old and no retention policy prunes it, so it is billed indefinitely; the age is in the status. Add a lifecycle policy, or delete it."},
			{Code: CodeEBSSnapOrphan, Phrase: "orphan: source volume deleted", Severity: domain.SevWarn, Source: "wave2", Detail: "The volume this snapshot came from no longer exists, so nothing is refreshing it and it will never get any newer. Keep it deliberately as an archive with an owner, or delete it — either way its stored data is billed every month."},
			{Code: ebsSnapCodePublic, Phrase: "shared with all AWS accounts", Severity: domain.SevBroken, Source: "wave2", Detail: "This snapshot is shared with every AWS account, so anyone can restore a volume from it and read whatever the source disk held. Stop sharing the snapshot with the `all` group."},
		},
	},
	{
		Name:          "AMIs",
		ShortName:     "ami",
		RowTypes:      []reflect.Type{reflect.TypeFor[ec2types.Image]()},
		Aliases:       []string{"ami", "amis", "images"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		RefToID:       amiRefToID,
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#ImageDetails:imageId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 32},
			{Key: "image_id", Title: "Image ID", Width: 22},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "deprecated", Title: "Deprecated", Width: 22},
			{Key: "architecture", Title: "Arch", Width: 10},
			{Key: "platform", Title: "Platform", Width: 16},
			{Key: "root_device_type", Title: "Root Device", Width: 14},
			{Key: "creation_date", Title: "Created", Width: 22},
			{Key: "public", Title: "Public", Width: 8},
		},
		StubCreator: func(id string) domain.Resource {
			return domain.Resource{
				ID:   id,
				Name: id,
				Fields: map[string]string{
					"image_id": id,
					"name":     id,
				},
			}
		},
		Color:          colorAMI,
		StateEnum:      enumValues[ec2types.ImageState](),
		NormalStates:   []string{string(ec2types.ImageStateAvailable)},
		UnknownState:   CodeAMIUnrecognisedState,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.EC2} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchAMIsPage(ctx, c.EC2, continuationToken)
		}),
		FetchByIDs: fetchByIDsWithClients(func(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
			return FetchAMIsByIDs(ctx, c.EC2, ids)
		}),
		FieldKeys: []string{
			"image_id", "name", "state", "architecture", "platform",
			"root_device_type", "creation_date", "public", "deprecated",
		},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkAMIEC2, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkAMIEBSSnaps, NeedsTargetCache: false, Mirror: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkAMIASG, NeedsTargetCache: true, Truncated: true, Distinct: "groups whose launch configuration or template version names this image, and groups running an instance whose Instance.ImageId is this image"},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkAMICFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkAMIKMS, Truncated: true},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkAMING, NeedsTargetCache: true, Truncated: true, Mirror: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ami")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "BlockDeviceMappings.Ebs.SnapshotId", TargetType: "ebs-snap"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeAMIStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1", Detail: "The image is still being registered and cannot be launched from until it finishes. Wait rather than pointing a launch template or Auto Scaling group at it.", States: []string{"pending", "transient"}, Class: catalog.LifecycleTransitional},
			{Code: CodeAMIStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "Registration of this image failed, so anything that launches from its identifier will error. Rebuild the image and update the launch templates or pipelines that reference it.", States: []string{"failed", "error", "invalid"}, Class: catalog.LifecycleFailedLive},
			{Code: CodeAMIStateDim, Phrase: "<image state>", Severity: domain.SevDim, Source: "wave1", States: []string{"deregistered", "disabled"}, Class: catalog.LifecycleEnded},
			unrecognisedStateDef(CodeAMIUnrecognisedState, "<image state>", "wave1", "EC2 reports an image state a9s does not recognise, so a9s cannot tell whether the image can be launched. Check the image in the AWS console before relying on it."),
			{Code: CodeAMIDeprecated, Phrase: "deprecated", Severity: domain.SevWarn, Source: "wave1", Detail: "AWS has marked this image as past its deprecation date, so it stays launchable but is no longer maintained or recommended, and anything still launching from it inherits whatever it was frozen with. Point the launch templates and Auto Scaling groups that reference it at a current image, then deregister it once nothing does.", AppliesIn: []catalog.LifecycleClass{catalog.LifecycleNormal}},
			{Code: CodeAMIPublic, Phrase: "shared with all AWS accounts", Severity: domain.SevBroken, Source: "wave1", Detail: "This image is shared with every AWS account, so anyone can launch it and read whatever the snapshot behind it contains. Remove the `all` group from the image's launch permission."},
		},
	},
	{
		Name:      "Launch Templates",
		ShortName: "lt",
		RowTypes:  []reflect.Type{reflect.TypeFor[*LTRaw]()},
		ReplayKeys: map[string]string{
			"Template.LaunchTemplateName": "name", "Template.DefaultVersionNumber": "default_version",
			"Template.LatestVersionNumber": "latest_version", "Template.CreatedBy": "created_by", "Template.CreateTime": "created",
		},
		Aliases:       []string{"lt", "launch-template", "launchtemplate", "launch-templates", "lts"},
		Category:      "COMPUTE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#LaunchTemplateDetails:launchTemplateId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 32},
			{Key: "status", Title: "Status", Width: 32},
			{Key: "default_version", Title: "Default", Width: 10},
			{Key: "latest_version", Title: "Latest", Width: 10},
			{Key: "created_by", Title: "Created By", Width: 24},
			{Key: "created", Title: "Created", Width: 18},
		},
		Color:          colorAnyFindingOrHealthy,
		FetcherClients: clientsOf(func(c *ServiceClients) []any { return []any{c.EC2} }),
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchLaunchTemplatesPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichLTDeprecatedAMI, Priority: 100, Reads: []string{"ami"}},
		FieldKeys: []string{"name", "default_version", "latest_version", "created_by", "created", "degraded_finding"},
		Related: []domain.RelatedDef{
			{TargetType: "ami", DisplayName: "AMI", Checker: checkLTAMI},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkLTASG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkLTEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkLTKMS},
			{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkLTNG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkLTSG},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkLTSubnet},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("lt")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "DefaultVersion.LaunchTemplateData.NetworkInterfaces.SubnetId", TargetType: "subnet"},
		},
		Findings: []catalog.FindingDef{
			{Code: ltCodeIMDSv1, Phrase: "IMDSv1 allowed", Severity: domain.SevWarn, Source: "wave2", Detail: "Instances launched from this template answer metadata requests without a session token, so a request-forgery bug in anything they run can read the attached role's credentials. Set the template's metadata options to require tokens, then launch a new version."},
			{Code: ltCodeUnencrypted, Phrase: "EBS encryption disabled", Severity: domain.SevWarn, Source: "wave2", Detail: "A block device in this template sets encryption off explicitly and the Region does not encrypt new EBS volumes by default, so every instance launched from it gets an unencrypted volume. Remove the override or set it to encrypted and publish a new template version, or turn on EBS encryption by default for the Region."},
			{Code: ltCodeDeprecatedAMI, Phrase: "deprecated AMI", Severity: domain.SevWarn, Source: "wave2", Detail: "The template's default version launches an image AWS has deprecated, so every instance this template creates starts from something no longer maintained. Update the default version to a current image before the next scale-out uses it."},
			{Code: ltCodeUserDataSecret, Phrase: "credential in user data", Severity: domain.SevBroken, Source: "wave2", Detail: "A credential is pasted into the default version's user data, so it is readable by anyone who can call ec2:DescribeLaunchTemplateVersions and lands on every instance launched from this template. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it."},
			DetailsDeniedFindingDef("lt", "Reading this template's default version was denied, so its metadata, block-device and image settings are unjudged rather than clean. Grant the role you browse with permission to read launch template versions, then refresh."),
			DetailsUnavailableFindingDef("lt"),
		},
	},
}

var computeChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:         "Lambda Invocations",
		ShortName:    "lambda_invocations",
		RowTypes:     []reflect.Type{reflect.TypeFor[cwlogstypes.FilteredLogEvent]()},
		LifecycleKey: "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return cloudWatchLogStreamConsoleURL(region, r.Fields["log_group"], r.Fields["log_stream"])
		},
		Columns: resource.LambdaInvocationColumns(),
		Color:   colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"request_id", "timestamp", "status", "duration_ms",
			"billed_duration_ms", "memory_size_mb", "memory_used_mb",
			"memory_used", "init_duration_ms", "restore_duration_ms", "cold_start",
			"error_type", "xray_trace_id", "log_group", "log_stream", "report_ms",
			"billed_duration_ms_raw", "duration_ms_raw",
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "lambda_invocation_logs",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group": "@parent.log_group", "request_id": "request_id", "log_stream": "log_stream", "report_ms": "report_ms"},
			DisplayNameKey: "request_id",
		}},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchLambdaInvocations(ctx, c.CloudWatchLogs, parentCtx["function_name"], parentCtx["log_group"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeLambdaInvocationTimeout, Phrase: "timed out", Severity: domain.SevBroken, Source: "wave1", Detail: "This invocation was cut off at the function's configured timeout, so its work stopped part-way and anything it had not yet written was lost. Find what it was waiting on in the logs — usually a downstream call with no timeout of its own — and either fix that or raise the function's limit."},
			{Code: CodeLambdaInvocationError, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "This invocation ended in an error rather than a result: the function or its runtime crashed, ran out of memory, or returned an error, and the caller got that error instead. The error type names which kind; open the invocation's log for the message and stack trace."},
		},
	},
	{
		Name:         "Lambda Invocation Logs",
		ShortName:    "lambda_invocation_logs",
		RowTypes:     []reflect.Type{reflect.TypeFor[cwlogstypes.FilteredLogEvent]()},
		TitleOmitsID: true,
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return cloudWatchLogStreamConsoleURL(region, r.Fields["log_group"], r.Fields["log_stream"])
		},
		Columns:   resource.LambdaInvocationLogColumns(),
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{"timestamp", "message", "log_group", "log_stream", "status"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			if reportMs, err := strconv.ParseInt(parentCtx["report_ms"], 10, 64); err == nil && parentCtx["log_stream"] != "" {
				return fetchInvocationStreamLog(ctx, c.CloudWatchLogs, parentCtx["log_group"], parentCtx["log_stream"], parentCtx["request_id"], reportMs, continuationToken)
			}
			return FetchLambdaInvocationLogs(ctx, c.CloudWatchLogs, parentCtx["log_group"], parentCtx["request_id"], continuationToken)
		}),
		Findings: logEventFindingDefs,
	},
	{
		Name:         "Scaling Activities",
		ShortName:    "asg_activities",
		RowTypes:     []reflect.Type{reflect.TypeFor[asgtypes.Activity]()},
		LifecycleKey: "status_code",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			name := r.Fields["asg_name"]
			if name == "" {
				return ""
			}
			return consolelink.Regional(region, "ec2/home?region="+region+"#AutoScalingGroupDetails:id="+url.PathEscape(name)+";view=activity")
		},
		Columns:   resource.AsgActivityColumns(),
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{"start_time", "status_code", "description", "cause", "asg_name"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchAsgActivities(ctx, c.AutoScaling, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeAsgActivityFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "This scaling action did not complete, so the group did not reach the size it was aiming for. The activity's status message names the cause; fix it before the next scaling event repeats it."},
			{Code: CodeAsgActivityCancelled, Phrase: "cancelled", Severity: domain.SevWarn, Source: "wave1", Detail: "The action was cancelled before it finished, usually because a newer scaling decision superseded it. Check the activities around it to confirm the group ended up at the size you expect."},
		},
	},
}
