package resource

func networkingResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "Load Balancers",
			ShortName:     "elb",
			Aliases:       []string{"elb", "alb", "nlb", "loadbalancers", "load-balancers"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 32, Sortable: true},
				{Key: "dns_name", Title: "DNS Name", Width: 48, Sortable: false},
				{Key: "type", Title: "Type", Width: 12, Sortable: true},
				{Key: "scheme", Title: "Scheme", Width: 14, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["state"] {
				case "active":
					return ColorHealthy
				case "provisioning":
					return ColorWarning
				case "failed", "active_impaired":
					return ColorBroken
				}
				return ColorHealthy
			},
			Children: []ChildViewDef{{
				ChildType:      "elb_listeners",
				Key:            "enter",
				ContextKeys:    map[string]string{"load_balancer_arn": "load_balancer_arn", "lb_name": "Name"},
				DisplayNameKey: "lb_name",
			}},
		},
		{
			Name:          "Target Groups",
			ShortName:     "tg",
			Aliases:       []string{"tg", "targetgroups", "target-groups"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "target_group_name", Title: "Target Group", Width: 32, Sortable: true},
				{Key: "port", Title: "Port", Width: 8, Sortable: true},
				{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "target_type", Title: "Target Type", Width: 12, Sortable: true},
				{Key: "health_check_path", Title: "Health Check", Width: 24, Sortable: false},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
			Children: []ChildViewDef{{
				ChildType:      "tg_health",
				Key:            "enter",
				ContextKeys:    map[string]string{"target_group_arn": "target_group_arn"},
				DisplayNameKey: "Name",
			}},
		},
		{
			Name:          "Security Groups",
			ShortName:     "sg",
			Aliases:       []string{"sg", "securitygroups", "security-groups"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "group_name", Title: "Group Name", Width: 28, Sortable: true},
				{Key: "group_id", Title: "Group ID", Width: 24, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "description", Title: "Description", Width: 36, Sortable: false},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "VPCs",
			ShortName:     "vpc",
			Aliases:       []string{"vpc", "vpcs"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 24, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "is_default", Title: "Default", Width: 9, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["state"] {
				case "available":
					return ColorHealthy
				case "pending":
					return ColorWarning
				}
				return ColorHealthy
			},
		},
		{
			Name:          "Subnets",
			ShortName:     "subnet",
			Aliases:       []string{"subnet", "subnets"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
				{Key: "availability_zone", Title: "AZ", Width: 14, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "available_ips", Title: "Available IPs", Width: 14, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["state"] {
				case "available":
					return ColorHealthy
				case "pending":
					return ColorWarning
				}
				return ColorHealthy
			},
		},
		{
			Name:          "Route Tables",
			ShortName:     "rtb",
			Aliases:       []string{"rtb", "routetables", "route-tables"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "route_table_id", Title: "Route Table ID", Width: 26, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "routes_count", Title: "Routes", Width: 8, Sortable: true},
				{Key: "associations_count", Title: "Assoc.", Width: 8, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "NAT Gateways",
			ShortName:     "nat",
			Aliases:       []string{"nat", "natgateways", "nat-gateways"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 24, Sortable: true},
				{Key: "nat_gateway_id", Title: "NAT Gateway ID", Width: 26, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
			},
			Color: func(r Resource) Color {
				switch r.Fields["state"] {
				case "available":
					return ColorHealthy
				case "pending", "deleting":
					return ColorWarning
				case "failed":
					return ColorBroken
				case "deleted":
					return ColorDim
				}
				return ColorHealthy
			},
		},
		{
			Name:          "Internet Gateways",
			ShortName:     "igw",
			Aliases:       []string{"igw", "internetgateways", "internet-gateways"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "igw_id", Title: "IGW ID", Width: 26, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "Elastic IPs",
			ShortName:     "eip",
			Aliases:       []string{"eip", "elastic-ips", "elasticips"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 24, Sortable: true},
				{Key: "allocation_id", Title: "Allocation ID", Width: 26, Sortable: true},
				{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: true},
				{Key: "association_id", Title: "Association", Width: 26, Sortable: true},
				{Key: "instance_id", Title: "Instance", Width: 20, Sortable: true},
				{Key: "domain", Title: "Domain", Width: 8, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "VPC Endpoints",
			ShortName:     "vpce",
			Aliases:       []string{"vpce", "vpc-endpoints", "vpcendpoints"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "service_name", Title: "Service Name", Width: 40, Sortable: true},
				{Key: "vpce_id", Title: "Endpoint ID", Width: 26, Sortable: true},
				{Key: "type", Title: "Type", Width: 12, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "Transit Gateways",
			ShortName:     "tgw",
			Aliases:       []string{"tgw", "transit-gateways", "transitgateways"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "tgw_id", Title: "TGW ID", Width: 26, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "owner_id", Title: "Owner", Width: 14, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "Network Interfaces",
			ShortName:     "eni",
			Aliases:       []string{"eni", "network-interfaces", "nis"},
			Category:      "NETWORKING",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 24, Sortable: true},
				{Key: "eni_id", Title: "ENI ID", Width: 26, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "type", Title: "Type", Width: 14, Sortable: true},
				{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
				{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
			},
			Color: func(r Resource) Color {
				switch r.Fields["status"] {
				case "in-use", "available":
					return ColorHealthy
				case "attaching", "detaching":
					return ColorWarning
				}
				return ColorHealthy
			},
		},
	}
}
