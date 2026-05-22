package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorELB(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "active", "":
		return domain.ColorHealthy
	case "provisioning", "active_impaired":
		return domain.ColorWarning
	case "failed":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func colorTG(_ domain.Resource) domain.Color { return domain.ColorHealthy }

func colorSG(r domain.Resource) domain.Color {
	if r.Fields["wide_open"] == "true" {
		return domain.ColorBroken
	}
	count, _ := strconv.Atoi(r.Fields["dangerous_open_count"])
	if count > 0 {
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func colorVPC(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return domain.ColorHealthy
	case "pending":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorSubnet(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return domain.ColorHealthy
	case "pending":
		return domain.ColorWarning
	case "unavailable", "failed", "failed-insufficient-capacity":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func colorRTB(r domain.Resource) domain.Color {
	blackhole, _ := strconv.Atoi(r.Fields["blackhole_routes_count"])
	if blackhole > 0 {
		return domain.ColorBroken
	}
	assoc, _ := strconv.Atoi(r.Fields["associations_count"])
	if assoc == 0 && r.Fields["is_main"] != "true" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorNAT(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return domain.ColorHealthy
	case "pending", "deleting":
		return domain.ColorWarning
	case "failed":
		return domain.ColorBroken
	case "deleted":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorIGW(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "attaching", "detaching":
		return domain.ColorWarning
	}
	attachments, _ := strconv.Atoi(r.Fields["attachments_count"])
	if attachments == 0 {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorEIP(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	if r.Fields["association_id"] == "" && r.Fields["instance_id"] == "" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorVPCE(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "Available", "":
		return domain.ColorHealthy
	case "PendingAcceptance", "Pending", "Deleting":
		return domain.ColorWarning
	case "Failed", "Rejected", "Expired", "Partial":
		return domain.ColorBroken
	case "Deleted":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorTGW(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "available", "":
		return domain.ColorHealthy
	case "pending", "modifying", "deleting":
		return domain.ColorWarning
	case "failed":
		return domain.ColorBroken
	case "deleted":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

func colorENI(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	switch r.Fields["status"] {
	case "in-use":
		return domain.ColorHealthy
	case "available":
		if r.Fields["requester_managed"] == "true" {
			return domain.ColorHealthy
		}
		return domain.ColorWarning
	case "attaching", "detaching":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var networkingTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Load Balancers",
		ShortName:     "elb",
		Aliases:       []string{"elb", "alb", "nlb", "loadbalancers", "load-balancers"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 32, Sortable: true},
			{Key: "dns_name", Title: "DNS Name", Width: 48, Sortable: false},
			{Key: "type", Title: "Type", Width: 12, Sortable: true},
			{Key: "scheme", Title: "Scheme", Width: 14, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "elb_listeners",
			Key:            "enter",
			ContextKeys:    map[string]string{"load_balancer_arn": "load_balancer_arn", "lb_name": "Name"},
			DisplayNameKey: "lb_name",
		}},
		Color: colorELB,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchLoadBalancersPage(ctx, c.ELBv2, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichELBAttributes, Priority: 100},
		FieldKeys: []string{"name", "dns_name", "type", "scheme", "state", "vpc_id", "load_balancer_arn"},
		Related: []domain.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkELBTargetGroups, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkELBAlarms, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkELBSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkELBVPC},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkELBCFN},
			{TargetType: "r53", DisplayName: "Route 53 Records", Checker: checkELBR53},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkELBACM},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkELBCF},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkELBENI, NeedsTargetCache: true},
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
	},
	{
		Name:          "Target Groups",
		ShortName:     "tg",
		Aliases:       []string{"tg", "targetgroups", "target-groups"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "target_group_name", Title: "Target Group", Width: 32, Sortable: true},
			{Key: "port", Title: "Port", Width: 8, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchTargetGroupsPage(ctx, c.ELBv2, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichTargetGroupHealth, Priority: 10},
		IssueEnricherFieldKeys: []string{"health_summary"},
		FieldKeys:              []string{"target_group_name", "port", "protocol", "vpc_id", "target_type", "health_check_path"},
		Related: []domain.RelatedDef{
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkTGELB, NeedsTargetCache: false},
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkTGECSSvc, NeedsTargetCache: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkTGASG, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkTGAlarm, NeedsTargetCache: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkTGVPC},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkTGBackup},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkTGCFN},
			{TargetType: "dbc", DisplayName: "DocumentDB Clusters", Checker: checkTGDBC},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkTGDBI},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkTGEC2},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkTGLambda},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTGLogs},
			{TargetType: "dbi-snap", DisplayName: "DB Instance Snapshots", Checker: checkTGDBISnap},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkTGSG},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkTGSubnet},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("tg")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "LoadBalancerArns", TargetType: "elb"},
		},
	},
	{
		Name:          "Security Groups",
		ShortName:     "sg",
		Aliases:       []string{"sg", "securitygroups", "security-groups"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "group_name", Title: "Group Name", Width: 28, Sortable: true},
			{Key: "group_id", Title: "Group ID", Width: 24, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color: colorSG,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchSecurityGroupsPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"group_id", "group_name", "vpc_id", "description", "dangerous_open_count", "wide_open", "risk_summary"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkSGVPC, NeedsTargetCache: false},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkSGEC2, NeedsTargetCache: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkSGENI, NeedsTargetCache: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkSGELB, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSGLambda, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSGCFN, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Referencing SGs", Checker: checkSGSG, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("sg")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
		},
	},
	{
		Name:          "VPCs",
		ShortName:     "vpc",
		Aliases:       []string{"vpc", "vpcs"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "is_default", Title: "Default", Width: 9, Sortable: true},
		},
		Color: colorVPC,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchVPCsPage(ctx, c.EC2, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichVPCFlowLogs, Priority: 100},
		IssueEnricherFieldKeys: []string{"flow_logs"},
		FieldKeys:              []string{"vpc_id", "name", "cidr_block", "state", "is_default"},
		Related: []domain.RelatedDef{
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkVPCSubnet, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkVPCSG, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkVPCEC2, NeedsTargetCache: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkVPCELB, NeedsTargetCache: true},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkVPCNAT, NeedsTargetCache: true},
			{TargetType: "igw", DisplayName: "Internet Gateways", Checker: checkVPCIGW, NeedsTargetCache: true},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVPCRTB, NeedsTargetCache: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkVPCVPCE, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkVPCCFN, NeedsTargetCache: false},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkVPCENI, NeedsTargetCache: true},
			{TargetType: "tgw", DisplayName: "Transit Gateways", Checker: checkVPCTGW, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("vpc")},
		},
	},
	{
		Name:          "Subnets",
		ShortName:     "subnet",
		Aliases:       []string{"subnet", "subnets"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchSubnetsPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"subnet_id", "name", "vpc_id", "cidr_block", "availability_zone", "state", "available_ips"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkSubnetEC2, NeedsTargetCache: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkSubnetENI, NeedsTargetCache: true},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkSubnetNAT, NeedsTargetCache: true},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkSubnetELB, NeedsTargetCache: true},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkSubnetRTB, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSubnetCFN, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkSubnetVPC},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkSubnetASG, NeedsTargetCache: true},
			{TargetType: "efs", DisplayName: "EFS File Systems", Checker: checkSubnetEFS},
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkSubnetEKS, NeedsTargetCache: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkSubnetVPCE, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("subnet")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
		},
	},
	{
		Name:          "Route Tables",
		ShortName:     "rtb",
		Aliases:       []string{"rtb", "routetables", "route-tables"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "route_table_id", Title: "Route Table ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "routes_count", Title: "Routes", Width: 8, Sortable: true},
			{Key: "associations_count", Title: "Assoc.", Width: 8, Sortable: true},
		},
		Color: colorRTB,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchRouteTablesPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"route_table_id", "name", "vpc_id", "routes_count", "associations_count", "blackhole_routes_count", "is_main"},
		Related: []domain.RelatedDef{
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRTBSubnet, NeedsTargetCache: true},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkRTBNAT, NeedsTargetCache: true},
			{TargetType: "igw", DisplayName: "Internet Gateways", Checker: checkRTBIGW, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRTBCFN, NeedsTargetCache: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkRTBVPC},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkRTBENI, NeedsTargetCache: true},
			{TargetType: "tgw", DisplayName: "Transit Gateways", Checker: checkRTBTGW, NeedsTargetCache: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkRTBVPCE, NeedsTargetCache: true},
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
	},
	{
		Name:          "NAT Gateways",
		ShortName:     "nat",
		Aliases:       []string{"nat", "natgateways", "nat-gateways"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "nat_gateway_id", Title: "NAT Gateway ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
		},
		Color: colorNAT,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchNatGatewaysPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"nat_gateway_id", "name", "vpc_id", "subnet_id", "state", "public_ip"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkNATVPC, NeedsTargetCache: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkNATSubnet, NeedsTargetCache: true},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkNATRTB, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkNATAlarm, NeedsTargetCache: true},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkNATEIP, NeedsTargetCache: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkNATENI, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("nat")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetId", TargetType: "subnet"},
			{FieldPath: "NatGatewayAddresses.AllocationId", TargetType: "eip"},
		},
	},
	{
		Name:          "Internet Gateways",
		ShortName:     "igw",
		Aliases:       []string{"igw", "internetgateways", "internet-gateways"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "igw_id", Title: "IGW ID", Width: 26, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
		},
		Color: colorIGW,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchInternetGatewaysPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"igw_id", "name", "vpc_id", "state", "attachments_count"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkIGWVPC, NeedsTargetCache: true},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkIGWRTB, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("igw")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "Attachments.VpcId", TargetType: "vpc"},
		},
	},
	{
		Name:          "Elastic IPs",
		ShortName:     "eip",
		Aliases:       []string{"eip", "elastic-ips", "elasticips"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "allocation_id", Title: "Allocation ID", Width: 26, Sortable: true},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: true},
			{Key: "association_id", Title: "Association", Width: 26, Sortable: true},
			{Key: "instance_id", Title: "Instance", Width: 20, Sortable: true},
			{Key: "domain", Title: "Domain", Width: 8, Sortable: true},
		},
		Color: colorEIP,
		Fetcher: func(ctx context.Context, clients any, _ string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			resources, err := FetchElasticIPs(ctx, c.EC2)
			if err != nil {
				return resource.FetchResult{}, err
			}
			return resource.FetchResult{
				Resources:  resources,
				Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
			}, nil
		},
		FieldKeys: []string{"allocation_id", "name", "public_ip", "association_id", "instance_id", "domain", "status"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEIPEC2},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEIPENI},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkEIPNAT, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEIPAlarm},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEIPASG},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkEIPCFN},
			{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkEIPECS},
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkEIPECSSvc},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkEIPECSTask},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEIPLogs},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eip")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "InstanceId", TargetType: "ec2"},
			{FieldPath: "NetworkInterfaceId", TargetType: "eni"},
		},
	},
	{
		Name:          "VPC Endpoints",
		ShortName:     "vpce",
		Aliases:       []string{"vpce", "vpc-endpoints", "vpcendpoints"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "service_name", Title: "Service Name", Width: 40, Sortable: true},
			{Key: "vpce_id", Title: "Endpoint ID", Width: 26, Sortable: true},
			{Key: "type", Title: "Type", Width: 12, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
		},
		Color: colorVPCE,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchVPCEndpointsPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"vpce_id", "service_name", "type", "state", "vpc_id"},
		Related: []domain.RelatedDef{
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkVPCESubnet, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkVPCESG, NeedsTargetCache: false},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVPCERTB, NeedsTargetCache: false},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkVPCEENI, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkVPCEVPC},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkVPCEACM},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkVPCEAlarm},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkVPCECF},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkVPCELogs},
			{TargetType: "r53", DisplayName: "Route 53 Zones", Checker: checkVPCER53},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkVPCES3},
			{TargetType: "tg", DisplayName: "Target Groups", Checker: checkVPCETG},
			{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkVPCEWAF},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("vpce")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetIds", TargetType: "subnet"},
			{FieldPath: "NetworkInterfaceIds", TargetType: "eni"},
			{FieldPath: "Groups.GroupId", TargetType: "sg"},
			{FieldPath: "RouteTableIds", TargetType: "rtb"},
		},
	},
	{
		Name:          "Transit Gateways",
		ShortName:     "tgw",
		Aliases:       []string{"tgw", "transit-gateways", "transitgateways"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "tgw_id", Title: "TGW ID", Width: 26, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "owner_id", Title: "Owner", Width: 14, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color: colorTGW,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchTransitGatewaysPage(ctx, c.EC2, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichTGWAttachments, Priority: 100},
		IssueEnricherFieldKeys: []string{"att_status"},
		FieldKeys:              []string{"tgw_id", "name", "state", "owner_id", "description"},
		Related: []domain.RelatedDef{
			{TargetType: "vpc", DisplayName: "VPCs", Checker: checkTGWVPC, NeedsTargetCache: false},
			{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkTGWRTB, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkTGWRole, NeedsTargetCache: false},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkTGWSubnet, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("tgw")},
		},
	},
	{
		Name:          "Network Interfaces",
		ShortName:     "eni",
		Aliases:       []string{"eni", "network-interfaces", "nis"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "eni_id", Title: "ENI ID", Width: 26, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
		},
		Color: colorENI,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchNetworkInterfacesPage(ctx, c.EC2, continuationToken)
		},
		FieldKeys: []string{"eni_id", "name", "status", "type", "vpc_id", "private_ip", "requester_managed"},
		Related: []domain.RelatedDef{
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkENIEC2, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkENISG, NeedsTargetCache: true},
			{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkENIEIP, NeedsTargetCache: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkENIVPC},
			{TargetType: "subnet", DisplayName: "Subnet", Checker: checkENISubnet},
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkENIELB},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkENILambda},
			{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkENINAT, NeedsTargetCache: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkENIVPCE, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("eni")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "SubnetId", TargetType: "subnet"},
			{FieldPath: "Groups.GroupId", TargetType: "sg"},
			{FieldPath: "Attachment.InstanceId", TargetType: "ec2"},
			{FieldPath: "Association.AllocationId", TargetType: "eip"},
		},
	},
}

var networkingChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "ELB Listeners",
		ShortName: "elb_listeners",
		Columns:   resource.ELBListenerColumns(),
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
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchELBListeners(ctx, c.ELBv2, parentCtx, continuationToken)
		},
	},
	{
		Name:      "Listener Rules",
		ShortName: "elb_listener_rules",
		Columns:   resource.ELBListenerRuleColumns(),
		CopyField: "conditions_summary",
		FieldKeys: []string{
			"priority", "conditions_summary", "action_type", "action_target", "is_default",
		},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchELBListenerRules(ctx, c.ELBv2, parentCtx, continuationToken)
		},
	},
	{
		Name:      "Target Health",
		ShortName: "tg_health",
		Columns:   resource.TargetHealthColumns(),
		FieldKeys: []string{"target_id", "port", "az", "health", "reason", "description"},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchTargetHealth(ctx, c.ELBv2, parentCtx["target_group_arn"], continuationToken)
		},
	},
}
