package catalog

import (
	"strconv"

	"github.com/k2m30/a9s/v3/internal/domain"
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

var networkingTypes = []ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
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
	},
	{
		Name:          "Network Interfaces",
		ShortName:     "eni",
		Aliases:       []string{"eni", "network-interfaces", "nis"},
		Category:      "NETWORKING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "eni_id", Title: "ENI ID", Width: 26, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
		},
		Color: colorENI,
	},
}
