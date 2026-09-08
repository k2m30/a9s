// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strconv"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func colorELB(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(elbStateFindings(r.Fields["state"]))
}

func colorTG(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorVPC(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(vpcStateFindings(r.Fields["state"]))
}

func colorSubnet(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(subnetFindings(r.Fields["state"], r.Fields["auto_public_ip"]))
}

func colorRTB(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	blackhole, _ := strconv.Atoi(r.Fields["blackhole_routes_count"])
	associations, _ := strconv.Atoi(r.Fields["associations_count"])
	return colorFromFindings(rtbFindings(blackhole, associations, r.Fields["is_main"]))
}

func colorNAT(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(natStateFindings(r.Fields["state"]))
}

func colorIGW(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	attachments, _ := strconv.Atoi(r.Fields["attachments_count"])
	return colorFromFindings(igwFindings(r.Fields["state"], attachments))
}

func colorVPCE(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(vpceFindings(r.Fields["state"], r.Fields["policy_exposure"]))
}

func colorTGW(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(tgwFindings(r.Fields["state"], r.Fields["auto_accept"]))
}

func colorENI(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(eniFindings(r.Fields["status"], r.Fields["requester_managed"]))
}

var networkingTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Load Balancers",
		ShortName:     "elb",
		Aliases:       []string{"elb", "alb", "nlb", "loadbalancers", "load-balancers"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["load_balancer_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ec2/home?region="+region+"#LoadBalancer:loadBalancerArn="+arn)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 32, Sortable: true},
			{Key: "dns_name", Title: "DNS Name", Width: 48, Sortable: false},
			{Key: "type", Title: "Type", Width: 12, Sortable: true},
			{Key: "scheme", Title: "Scheme", Width: 14, Sortable: true},
			{Key: "state", Title: "Status", Width: 12, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "elb_listeners",
			Key:            "enter",
			ContextKeys:    map[string]string{"load_balancer_arn": "load_balancer_arn", "lb_name": "Name"},
			DisplayNameKey: "lb_name",
		}},
		Color: colorELB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchLoadBalancersPage(ctx, c.ELBv2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichELBAttributes, Priority: 100},
		FieldKeys: []string{"name", "dns_name", "type", "scheme", "state", "vpc_id", "load_balancer_arn"},
		Related: []domain.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkELBTargetGroups, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkELBAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkELBSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkELBVPC},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkELBCFN},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkELBACM},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkELBCF, Truncated: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkELBENI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkELBS3},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkELBSubnet},
			{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkELBWAF},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("elb")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SecurityGroups", TargetType: "sg"},
			{FieldPath: "AvailabilityZones.SubnetId", TargetType: "subnet"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeELBStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1", Detail: "The load balancer is still being built and is not yet accepting traffic. This normally clears in a few minutes; if it does not, its subnets are usually out of free IP addresses."},
			{Code: CodeELBStateActiveImpaired, Phrase: "active impaired", Severity: domain.SevWarn, Source: "wave1", Detail: "The load balancer is serving traffic but could not set up or scale in at least one availability zone, so capacity there is degraded. Check that every attached subnet has spare IP addresses."},
			{Code: CodeELBStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The load balancer could not be created and will not recover on its own. It has to be deleted and recreated; nothing routes through it in the meantime."},
			{Code: elbCodeMisconfigured, Phrase: "deletion protection or access logs disabled", Severity: domain.SevWarn, Source: "wave2"},
			{Code: elbCodeDesyncMitigationOff, Phrase: "HTTP desync mitigation off", Severity: domain.SevWarn, Source: "wave2", Detail: "The load balancer forwards requests it knows are ambiguous instead of rejecting them, so a crafted request can be interpreted one way by the balancer and another by the target. Set the desync mitigation mode to defensive or strictest."},
			{Code: elbCodeInvalidHeadersKept, Phrase: "invalid HTTP headers not dropped", Severity: domain.SevWarn, Source: "wave2", Detail: "Headers that are not valid HTTP are passed through to the targets instead of being dropped, which is how request smuggling reaches an application. Turn on dropping of invalid header fields."},
			{Code: elbCodePlainHTTPListener, Phrase: "<port(s) LIST> in the clear", Severity: domain.SevWarn, Source: "wave2", Detail: "A listener on this load balancer carries traffic in the clear, so credentials and session cookies cross the network readable by anyone on the path; the ports are listed below. Terminate TLS on the listener, or redirect it to an HTTPS listener."},
			{Code: elbCodeWeakTLSPolicy, Phrase: "weak TLS policy on <port(s) LIST>", Severity: domain.SevWarn, Source: "wave2", Detail: "A listener's security policy still negotiates older protocol versions or ciphers without forward secrecy, so a client can be steered onto a breakable connection; the ports are listed below. Move the listener to one of the modern security policies that require version 1.2 or later."},
		},
	},
	{
		Name:          "Target Groups",
		ShortName:     "tg",
		Aliases:       []string{"tg", "targetgroups", "target-groups"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["target_group_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ec2/home?region="+region+"#TargetGroup:targetGroupArn="+arn)
		},
		Columns: []domain.Column{
			{Key: "target_group_name", Title: "Target Group", Width: 32, Sortable: true},
			{Key: "port", Title: "Port", Width: 8, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
			{Key: "health_summary", Title: "Status", Width: 14, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "target_type", Title: "Target Type", Width: 12, Sortable: true},
			{Key: "health_check_path", Title: "Health Check", Width: 24, Sortable: false},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "tg_health",
			Key:            "enter",
			ContextKeys:    map[string]string{"target_group_arn": "target_group_arn"},
			DisplayNameKey: "Name",
		}},
		Color: colorTG,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchTargetGroupsPage(ctx, c.ELBv2, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichTargetGroupHealth, Priority: 10},
		IssueEnricherFieldKeys: []string{"health_summary"},
		// target_group_arn is required by checkLambdaTG (lambda:tg pivot) to
		// call DescribeTargetHealth on cache-restored rows — without it in
		// FieldKeys the ARN does not survive a YAML cache round-trip.
		FieldKeys: []string{"target_group_name", "target_group_arn", "port", "protocol", "vpc_id", "target_type", "health_check_path"},
		Related: []domain.RelatedDef{
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkTGELB, NeedsTargetCache: false, Truncated: true},
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkTGECSSvc, NeedsTargetCache: true, Truncated: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkTGASG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkTGAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkTGVPC},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkTGCFN},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkTGEC2},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkTGLambda},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("tg")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "LoadBalancerArns", TargetType: "elb"},
		},
		Findings: []catalog.FindingDef{
			{Code: tgCodeAllTargetsUnhealthy, Phrase: "all <N target(s)> unhealthy", Severity: domain.SevBroken, Source: "wave2", Detail: "Every registered target is failing its health check, so the load balancer has nowhere to send a request and whatever sits in front of this group is down. Check the targets themselves, then the health-check path, port and matcher the group is configured with."},
			{Code: tgCodeUnhealthyTargets, Phrase: "unhealthy targets: <N>/<M>", Severity: domain.SevWarn, Source: "wave2", Detail: "Some of this group's targets are failing their health checks, so every request is landing on the ones that are left. Each failing target is listed with the reason its health check gave; fix or replace them before the remaining targets run out of headroom."},
		},
	},
	{
		Name:          "Security Groups",
		ShortName:     "sg",
		Aliases:       []string{"sg", "securitygroups", "security-groups"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#SecurityGroup:groupId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "group_name", Title: "Group Name", Width: 28, Sortable: true},
			{Key: "group_id", Title: "Group ID", Width: 24, Sortable: true},
			{Key: "risk_summary", Title: "Status", Width: 22, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color: colorAnyFindingOrHealthy,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchSecurityGroupsPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichSGUsage, Priority: 100},
		FieldKeys: []string{"group_id", "group_name", "vpc_id", "description", "dangerous_open_count", "wide_open", "open_ports", "risk_summary"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkSGVPC, NeedsTargetCache: false},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkSGEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkSGENI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkSGELB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSGLambda, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSGCFN, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Referencing SGs", Checker: checkSGSG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sg")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
		},
		Findings: []catalog.FindingDef{
			{Code: sgCodeWideOpen, Phrase: "all ports open to 0.0.0.0/0", Severity: domain.SevBroken, Source: "wave1", Detail: "One ingress rule opens every port and protocol to the whole internet, so nothing this group protects is reachable only from where you intended. Replace it with rules naming the ports each workload actually serves and the addresses allowed to reach them."},
			{Code: sgCodeDangerousPorts, Phrase: "<port(s) LIST> open to 0.0.0.0/0", Severity: domain.SevBroken, Source: "wave1", Detail: "An administrative or database port on this group accepts connections from any address on the internet, which is how credential-stuffing and direct database access start. Narrow the rule to the addresses that need it, or move the access behind a bastion or private link."},
			{Code: sgCodeDefaultWithRules, Phrase: sgDefaultWithRulesPhrase, Severity: domain.SevWarn, Source: "wave1", Detail: "The VPC's default security group still carries rules, and AWS attaches it to any resource launched without an explicit group. Remove every ingress rule and every egress rule other than the AWS-created allow-all, and give each workload its own group."},
			{Code: sgCodeUnused, Phrase: sgUnusedPhrase, Severity: domain.SevWarn, Source: "wave2", Detail: "No network interface in this account references this group, so its rules protect nothing and its name still gets picked from the console list. Delete it, or attach it to the workload it was written for."},
		},
	},
	{
		Name:          "VPCs",
		ShortName:     "vpc",
		Aliases:       []string{"vpc", "vpcs"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#VpcDetails:VpcId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "is_default", Title: "Default", Width: 9, Sortable: true},
		},
		Color: colorVPC,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchVPCsPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichVPCFlowLogs, Priority: 100},
		IssueEnricherFieldKeys: []string{"flow_logs"},
		FieldKeys:              []string{"vpc_id", "name", "cidr_block", "state", "is_default"},
		Related: []domain.RelatedDef{
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkVPCSubnet, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkVPCSG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkVPCEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkVPCELB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkVPCNAT, NeedsTargetCache: true, Truncated: true},
			{TargetType: "igw", DisplayName: "Internet Gateways", Checker: checkVPCIGW, NeedsTargetCache: true, Truncated: true},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVPCRTB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkVPCVPCE, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkVPCCFN, NeedsTargetCache: false},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkVPCENI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "tgw", DisplayName: "Transit Gateways", Checker: checkVPCTGW, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("vpc")},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeVPCStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			{Code: vpcCodeNoFlowLogs, Phrase: "no active VPC flow logs", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "Subnets",
		ShortName:     "subnet",
		Aliases:       []string{"subnet", "subnets"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#SubnetDetails:subnetId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
			{Key: "availability_zone", Title: "AZ", Width: 14, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "available_ips", Title: "Available IPs", Width: 14, Sortable: true},
		},
		Color: colorSubnet,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchSubnetsPage(ctx, c.EC2, continuationToken)
		}),
		FieldKeys: []string{"subnet_id", "name", "vpc_id", "cidr_block", "availability_zone", "state", "available_ips"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkSubnetEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkSubnetENI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkSubnetNAT, NeedsTargetCache: true, Truncated: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkSubnetELB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkSubnetRTB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSubnetCFN, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkSubnetVPC},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkSubnetASG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "efs", DisplayName: "EFS File Systems", Checker: checkSubnetEFS, Truncated: true},
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkSubnetEKS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkSubnetVPCE, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("subnet")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeSubnetStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeSubnetStateUnavailable, Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSubnetStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSubnetStateFailedInsufficientCapacity, Phrase: "failed-insufficient-capacity", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeSubnetAutoPublicIP, Phrase: SubnetAutoPublicIPPhrase, Severity: domain.SevWarn, Source: "wave1", Detail: "Every instance launched into this subnet is given a public address by default, so a workload reaches the internet whether or not its owner intended it to. Turn the subnet's auto-assign public address setting off and attach an elastic address to the instances that genuinely need one."},
		},
	},
	{
		Name:          "Route Tables",
		ShortName:     "rtb",
		Aliases:       []string{"rtb", "routetables", "route-tables"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#RouteTableDetails:RouteTableId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "route_table_id", Title: "Route Table ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "routes_count", Title: "Routes", Width: 8, Sortable: true},
			{Key: "associations_count", Title: "Assoc.", Width: 8, Sortable: true},
		},
		Color: colorRTB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchRouteTablesPage(ctx, c.EC2, continuationToken)
		}),
		FieldKeys: []string{"route_table_id", "name", "vpc_id", "routes_count", "associations_count", "blackhole_routes_count", "is_main"},
		Related: []domain.RelatedDef{
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRTBSubnet},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkRTBNAT},
			{TargetType: "igw", DisplayName: "Internet Gateways", Checker: checkRTBIGW},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRTBCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkRTBVPC},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkRTBENI},
			{TargetType: "tgw", DisplayName: "Transit Gateways", Checker: checkRTBTGW},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkRTBVPCE, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("rtb")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "Associations.SubnetId", TargetType: "subnet"},
			{FieldPath: "Routes.NatGatewayId", TargetType: "nat"},
			{FieldPath: "Routes.GatewayId", TargetType: "igw"},
			{FieldPath: "Routes.NetworkInterfaceId", TargetType: "eni"},
			{FieldPath: "Routes.TransitGatewayId", TargetType: "tgw"},
			{FieldPath: "Routes.VpcPeeringConnectionId", TargetType: "vpc"},
		},
		Findings: []catalog.FindingDef{
			{Code: rtbCodeBlackholeRoute, Phrase: "blackhole route (target deleted)", Severity: domain.SevBroken, Source: "wave1"},
			{Code: rtbCodeOrphanUnassociated, Phrase: "no subnet associations", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "NAT Gateways",
		ShortName:     "nat",
		Aliases:       []string{"nat", "natgateways", "nat-gateways"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#NatGatewayDetails:natGatewayId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "nat_gateway_id", Title: "NAT Gateway ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
		},
		Color: colorNAT,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchNatGatewaysPage(ctx, c.EC2, continuationToken)
		}),
		FieldKeys: []string{"nat_gateway_id", "name", "vpc_id", "subnet_id", "state", "public_ip"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkNATVPC},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkNATSubnet},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkNATRTB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkNATAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkNATEIP},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkNATENI},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("nat")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetId", TargetType: "subnet"},
			{FieldPath: "NatGatewayAddresses.AllocationId", TargetType: "eip"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeNATStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeNATStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeNATStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeNATStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
		},
	},
	{
		Name:          "Internet Gateways",
		ShortName:     "igw",
		Aliases:       []string{"igw", "internetgateways", "internet-gateways"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#InternetGateway:internetGatewayId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "igw_id", Title: "IGW ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "state", Title: "Status", Width: 12, Sortable: true},
		},
		Color: colorIGW,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchInternetGatewaysPage(ctx, c.EC2, continuationToken)
		}),
		FieldKeys: []string{"igw_id", "name", "vpc_id", "state", "attachments_count"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkIGWVPC},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkIGWRTB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("igw")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Attachments.VpcId", TargetType: "vpc"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeIGWStateAttaching, Phrase: "attaching", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeIGWStateDetaching, Phrase: "detaching", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeIGWNoAttachments, Phrase: "no VPC attachments", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "Elastic IPs",
		ShortName:     "eip",
		Aliases:       []string{"eip", "elastic-ips", "elasticips"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#ElasticIpDetails:AllocationId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "allocation_id", Title: "Allocation ID", Width: 26, Sortable: true},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: true},
			{Key: "association_id", Title: "Association", Width: 26, Sortable: true},
			{Key: "instance_id", Title: "Instance", Width: 20, Sortable: true},
			{Key: "domain", Title: "Domain", Width: 8, Sortable: true},
		},
		Color: colorAnyFindingOrHealthy,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, _ string) (resource.FetchResult, error) {
			resources, err := FetchElasticIPs(ctx, c.EC2)
			if err != nil {
				return resource.FetchResult{}, err
			}
			return resource.FetchResult{
				Resources:  resources,
				Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
			}, nil
		}),
		FieldKeys: []string{"allocation_id", "name", "public_ip", "association_id", "instance_id", "domain", "status"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEIPEC2},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEIPENI},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkEIPNAT, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEIPAlarm, Truncated: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEIPASG},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkEIPCFN},
			{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkEIPECS, Truncated: true},
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkEIPECSSvc, Truncated: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkEIPECSTask, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eip")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "InstanceId", TargetType: "ec2"},
			{FieldPath: "NetworkInterfaceId", TargetType: "eni"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEIPUnassociated, Phrase: "unassociated", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "VPC Endpoints",
		ShortName:     "vpce",
		Aliases:       []string{"vpce", "vpc-endpoints", "vpcendpoints"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#EndpointDetails:vpcEndpointId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "service_name", Title: "Service Name", Width: 40, Sortable: true},
			{Key: "vpce_id", Title: "Endpoint ID", Width: 26, Sortable: true},
			{Key: "type", Title: "Type", Width: 12, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
		},
		Color: colorVPCE,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchVPCEndpointsPage(ctx, c.EC2, continuationToken)
		}),
		FieldKeys: []string{"vpce_id", "service_name", "type", "state", "vpc_id"},
		Related: []domain.RelatedDef{
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkVPCESubnet, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkVPCESG, NeedsTargetCache: false},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVPCERTB, NeedsTargetCache: false},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkVPCEENI, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkVPCEVPC},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkVPCEAlarm, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkVPCELogs},
			{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkVPCER53},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("vpce")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetIds", TargetType: "subnet"},
			{FieldPath: "NetworkInterfaceIds", TargetType: "eni"},
			{FieldPath: "Groups.GroupId", TargetType: "sg"},
			{FieldPath: "RouteTableIds", TargetType: "rtb"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeVPCEStatePendingAcceptance, Phrase: "pending acceptance", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeVPCEStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeVPCEStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeVPCEStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeVPCEStateRejected, Phrase: "rejected", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeVPCEStateExpired, Phrase: "expired", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeVPCEStatePartial, Phrase: "partial", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeVPCEStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"},
			{Code: CodeVPCEPolicyOpen, Phrase: VPCEPolicyOpenPhrase, Severity: domain.SevWarn, Source: "wave1", Detail: "The endpoint policy grants every action to every principal, so any identity that can reach this endpoint can use it to talk to resources in other accounts. Replace it with a policy naming the principals and resources this VPC is allowed to reach."},
		},
	},
	{
		Name:          "Transit Gateways",
		ShortName:     "tgw",
		Aliases:       []string{"tgw", "transit-gateways", "transitgateways"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#TransitGateways:filter="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "tgw_id", Title: "TGW ID", Width: 26, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "owner_id", Title: "Owner", Width: 14, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color: colorTGW,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchTransitGatewaysPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichTGWAttachments, Priority: 100},
		IssueEnricherFieldKeys: []string{"att_status"},
		FieldKeys:              []string{"tgw_id", "name", "state", "owner_id", "description"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkTGWVPC, NeedsTargetCache: false},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkTGWRTB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkTGWRole, NeedsTargetCache: false},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkTGWSubnet, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("tgw")},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeTGWStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1", Detail: "The gateway is still being created and does not route yet. Attachments created now stay pending until it comes up."},
			{Code: CodeTGWStateModifying, Phrase: "modifying", Severity: domain.SevWarn, Source: "wave1", Detail: "A configuration change is being applied. Routing across the gateway can be inconsistent until it settles."},
			{Code: CodeTGWStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "The gateway is being torn down. Every attachment on it goes away and any traffic still routed through it will stop."},
			{Code: CodeTGWStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The gateway could not be created and will not recover. It has to be recreated, and anything routed through it has no path."},
			{Code: CodeTGWStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1", Detail: "This gateway is gone. AWS keeps returning it for a while after deletion, so route tables that still point at it are dead references worth cleaning up."},
			{Code: tgwCodeAttachmentFailed, Phrase: "attachment failed", Severity: domain.SevBroken, Source: "wave2", Detail: "The network behind this attachment has no path across the gateway. Failed attachments do not retry; delete and recreate the attachment."},
			{Code: tgwCodeAttachmentTransitional, Phrase: "attachment between states", Severity: domain.SevWarn, Source: "wave2", Detail: "The attachment is between states — being modified, rolled back, or waiting for the gateway owner to accept it — and traffic across it is not reliable until it settles. Pending acceptance is the one state that needs a person: the owning account has to approve it."},
			{Code: CodeTGWAutoAccept, Phrase: TGWAutoAcceptPhrase, Severity: domain.SevWarn, Source: "wave1", Detail: "Any account this gateway is shared with can attach a VPC to it without review, putting that VPC on your routed network the moment it asks. Turn auto-accept off and approve each attachment explicitly."},
		},
	},
	{
		Name:          "Network Interfaces",
		ShortName:     "eni",
		Aliases:       []string{"eni", "network-interfaces", "nis"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ec2/home?region="+region+"#NetworkInterface:networkInterfaceId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "eni_id", Title: "ENI ID", Width: 26, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
		},
		Color: colorENI,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchNetworkInterfacesPage(ctx, c.EC2, continuationToken)
		}),
		FieldKeys: []string{"eni_id", "name", "status", "type", "vpc_id", "private_ip", "requester_managed", "description", "requester_id", "security_groups"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkENIEC2},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkENISG},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkENIEIP},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkENIVPC},
			{TargetType: "subnet", DisplayName: "Subnet", Checker: checkENISubnet},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkENIELB},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkENILambda},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkENINAT, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkENIVPCE, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eni")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetId", TargetType: "subnet"},
			{FieldPath: "Groups.GroupId", TargetType: "sg"},
			{FieldPath: "Attachment.InstanceId", TargetType: "ec2"},
			{FieldPath: "Association.AllocationId", TargetType: "eip"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeENIStateAttaching, Phrase: "attaching", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeENIStateDetaching, Phrase: "detaching", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeENIStateAvailable, Phrase: "available", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "Transfer Family",
		ShortName:     "transfer",
		Aliases:       []string{"transfer", "sftp", "as2", "ftps"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "transfer/home#/servers/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "server_id", Title: "Server Id", Width: 32, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "domain", Title: "Domain", Width: 10, Sortable: true},
			{Key: "endpoint_type", Title: "Endpoint", Width: 14, Sortable: true},
			{Key: "identity_provider_type", Title: "Identity Provider", Width: 20, Sortable: true},
			{Key: "user_count", Title: "Users", Width: 8, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "transfer_agreements",
			Key:            "e",
			ContextKeys:    map[string]string{"server_id": "ID"},
			DisplayNameKey: "server_id",
		}},
		Color:   colorAnyFindingOrHealthy,
		Fetcher: fetcherWithClients(FetchTransferServersPage),
		FieldKeys: []string{
			"server_id", "status", "domain", "endpoint_type",
			"identity_provider_type", "user_count", "arn",
		},
		Related: []domain.RelatedDef{
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkTransferACM},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkTransferEIP},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkTransferLambda},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTransferLogs},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkTransferRole},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkTransferSubnet},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkTransferVPC},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkTransferVPCE},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("transfer")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "LoggingRole", TargetType: "role"},
			{FieldPath: "Certificate", TargetType: "acm"},
			{FieldPath: "EndpointDetails.VpcId", TargetType: "vpc"},
			{FieldPath: "EndpointDetails.SubnetIds", TargetType: "subnet"},
			{FieldPath: "EndpointDetails.AddressAllocationIds", TargetType: "eip"},
			{FieldPath: "EndpointDetails.VpcEndpointId", TargetType: "vpce"},
			{FieldPath: "IdentityProviderDetails.Function", TargetType: "lambda"},
		},
		Findings: []catalog.FindingDef{
			{Code: transferCodeOffline, Phrase: "offline: not accepting transfers", Severity: domain.SevWarn, Source: "wave1", Detail: "Server is offline; partners cannot connect until it is started."},
			{Code: transferCodeStarting, Phrase: "starting", Severity: domain.SevWarn, Source: "wave1", Detail: "Server is starting; not yet fully able to respond."},
			{Code: transferCodeStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1", Detail: "Server is stopping; transfers are draining."},
			{Code: transferCodeStartFailed, Phrase: "start failed", Severity: domain.SevBroken, Source: "wave1", Detail: "Server failed to come online; partner transfers are down."},
			{Code: transferCodeStopFailed, Phrase: "stop failed", Severity: domain.SevWarn, Source: "wave1", Detail: "Stop failed; the server may still be serving transfers."},
			{Code: transferCodeLegacyPolicy, Phrase: "legacy security policy", Severity: domain.SevWarn, Source: "wave1", Detail: "The server's security policy still allows weak ciphers and old TLS versions, so a client can be steered onto a breakable connection. Move the server to a current security policy."},
			{Code: transferCodeNoLogging, Phrase: "no activity logging", Severity: domain.SevWarn, Source: "wave1", Detail: "Neither a logging role nor structured log destinations are configured."},
			DetailsDeniedFindingDef("transfer", "Access to server details was denied; only the listed fields are visible."),
			DetailsUnavailableFindingDef("transfer"),
		},
	},
	{
		Name:          "VPC Peering",
		ShortName:     "vpc-peer",
		Aliases:       []string{"vpc-peer", "pcx", "peering"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "vpc/home?region="+region+"#PeeringConnectionDetails:VpcPeeringConnectionId="+r.ID)
		},
		Columns: []domain.Column{
			{Key: "pcx_id", Title: "Pcx Id", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 34, Sortable: true},
			{Key: "requester_vpc", Title: "Requester VPC", Width: 22, Sortable: true},
			{Key: "requester_owner", Title: "Requester Owner", Width: 14, Sortable: true},
			{Key: "accepter_vpc", Title: "Accepter VPC", Width: 22, Sortable: true},
			{Key: "accepter_owner", Title: "Accepter Owner", Width: 14, Sortable: true},
			{Key: "expires", Title: "Expires", Width: 17, Sortable: true},
		},
		Color: colorAnyFindingOrHealthy,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchVpcPeeringConnectionsPage(ctx, c.EC2, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichVpcPeerRoutes, Priority: 100},
		FieldKeys: []string{"pcx_id", "status", "requester_vpc", "requester_owner", "accepter_vpc", "accepter_owner", "expires"},
		Related: []domain.RelatedDef{
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVpcPeerRTB, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkVpcPeerVPC, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("vpc-peer")},
		},
		// No Navigable fields — see docs/resources/vpc-peer-impl-plan.md §0.
		Findings: []catalog.FindingDef{
			{Code: vpcPeerCodeProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1", Detail: "Peering connection is being provisioned."},
			{Code: vpcPeerCodeInitiating, Phrase: "initiating", Severity: domain.SevWarn, Source: "wave1", Detail: "Peering request is being initiated."},
			{Code: vpcPeerCodePendingAcceptance, Phrase: "pending acceptance: expires in <N>d", Severity: domain.SevWarn, Source: "wave1", Detail: "The peer has not accepted this request yet, and AWS expires it a week after creation; the countdown is in the status and the date is listed below. Ask the accepter to approve it."},
			{Code: vpcPeerCodeExpired, Phrase: "expired: never accepted", Severity: domain.SevWarn, Source: "wave1", Detail: "The peering request expired unaccepted; recreate it if still needed."},
			{Code: vpcPeerCodeRejected, Phrase: "rejected", Severity: domain.SevBroken, Source: "wave1", Detail: "The accepter rejected this peering request, so nothing will ever route across it; AWS keeps the record listed for a while. Delete it and request again once the other side agrees."},
			{Code: vpcPeerCodeFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The peering connection failed to establish and will not recover on its own; the status message is listed below. Delete it and request a new one."},
			{Code: vpcPeerCodeDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "Peering connection is being deleted."},
			{Code: vpcPeerCodeDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1", Detail: "AWS keeps deleted connections listed for a window."},
			{Code: vpcPeerCodeCidrOverlap, Phrase: "CIDR overlap with peer", Severity: domain.SevWarn, Source: "wave1", Detail: "The requester and accepter VPCs have overlapping address ranges, so routes into the overlap are blackholed; the range is listed below. Re-address one side, or peer a VPC that does not overlap."},
			{Code: vpcPeerCodeNoLocalRoute, Phrase: "no local route to peer", Severity: domain.SevWarn, Source: "wave2", Detail: "No loaded route table routes to this peering connection."},
			{Code: vpcPeerCodeRouteBlackholed, Phrase: "route to peer blackholed", Severity: domain.SevWarn, Source: "wave2", Detail: "A route references this connection but its state is blackhole."},
		},
	},
}

var networkingChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "ELB Listeners",
		ShortName: "elb_listeners",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			if r.ID == "" {
				return ""
			}
			return consolelink.Regional(region, "ec2/home?region="+region+"#ELBListenerV2:listenerArn="+r.ID)
		},
		Columns: resource.ELBListenerColumns(),
		Color:   colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"port", "protocol", "default_action_type", "default_action_target",
			"ssl_policy", "certificate_short", "listener_display",
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "elb_listener_rules",
			Key:            "enter",
			ContextKeys:    map[string]string{"listener_arn": "ID"},
			DisplayNameKey: "listener_display",
		}},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchELBListeners(ctx, c.ELBv2, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeELBListenerNoCertificate, Phrase: "no certificate configured", Severity: domain.SevBroken, Source: "wave1"},
		},
	},
	{
		Name:      "Listener Rules",
		ShortName: "elb_listener_rules",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			if r.ID == "" {
				return ""
			}
			return consolelink.Regional(region, "ec2/home?region="+region+"#ListenerRuleDetails:ruleArn="+r.ID)
		},
		Columns:   resource.ELBListenerRuleColumns(),
		CopyField: "conditions_summary",
		FieldKeys: []string{
			"priority", "conditions_summary", "action_type", "action_target", "is_default",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchELBListenerRules(ctx, c.ELBv2, parentCtx, continuationToken)
		}),
	},
	{
		Name:      "Target Health",
		ShortName: "tg_health",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["target_group_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ec2/home?region="+region+"#TargetGroup:targetGroupArn="+arn)
		},
		Columns:      resource.TargetHealthColumns(),
		Color:        colorAnyFindingOrHealthy,
		LifecycleKey: "health",
		FieldKeys:    []string{"target_id", "port", "az", "health", "reason", "reason_human", "description", "target_group_arn"},
		Findings: []catalog.FindingDef{
			{Code: CodeTGHealthUnhealthy, Phrase: "<health check reason>", Severity: domain.SevBroken, Source: "wave1", Detail: "This target is failing the group's health check, so the load balancer has stopped sending it requests; the reason the check gave is the phrase. Fix the target or the check's path, port and matcher."},
			{Code: CodeTGHealthUnavailable, Phrase: "<health check reason>", Severity: domain.SevWarn, Source: "wave1", Detail: "The load balancer cannot health-check this target at all, usually because a security group or network path blocks the check. Open the check's port from the load balancer to the target."},
			{Code: CodeTGHealthDraining, Phrase: "<health check reason>", Severity: domain.SevWarn, Source: "wave1", Detail: "The target is deregistering and finishing the requests it already holds. It stops receiving new ones and leaves the group when the deregistration delay elapses."},
			{Code: CodeTGHealthInitial, Phrase: "<health check reason>", Severity: domain.SevDim, Source: "wave1", Detail: "The target has just registered and has not passed enough health checks to receive traffic yet. It becomes healthy once the configured threshold of consecutive successes is met."},
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchTargetHealth(ctx, c.ELBv2, parentCtx["target_group_arn"], continuationToken)
		}),
	},
	{
		Name:      "Agreements",
		ShortName: "transfer_agreements",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			sid := r.Fields["server_id"]
			if sid == "" {
				return ""
			}
			return consolelink.Regional(region, "transfer/home#/servers/"+sid)
		},
		Columns: []domain.Column{
			{Key: "agreement_id", Title: "Agreement Id", Width: 24, Sortable: true},
			{Key: "description", Title: "Description", Width: 32, Sortable: false},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "local_profile", Title: "Local Profile", Width: 16, Sortable: true},
			{Key: "partner_profile", Title: "Partner Profile", Width: 16, Sortable: true},
			{Key: "base_directory", Title: "Base Directory", Width: 30, Sortable: false},
		},
		Color: colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"agreement_id", "description", "status", "local_profile", "partner_profile", "base_directory", "server_id",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchTransferAgreements(ctx, c.Transfer, parentCtx["server_id"], continuationToken)
		}),
		DetailEnrich: enrichTransferAgreement,
		Findings: []catalog.FindingDef{
			{Code: transferCodeAgreementInactive, Phrase: "inactive: partner traffic rejected", Severity: domain.SevWarn, Source: "wave1", Detail: "Agreement is inactive; partner traffic is rejected."},
			{Code: transferCodeCertExpired, Phrase: "expired", Severity: domain.SevBroken, Source: "wave1", Detail: "The certificate has expired, so partner connections that present or verify it now fail. Import a renewed certificate and point the profile at it."},
			{Code: transferCodeCertExpiring, Phrase: "expires in <N>d", Severity: domain.SevWarn, Source: "wave1", Detail: "The certificate expires soon; once it does, partner connections that present or verify it will fail. Import a renewed certificate before the inactive date."},
		},
	},
}
