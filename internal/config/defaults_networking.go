package config

func networkingDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"elb": {
			List: []ListColumn{
				{Title: "Name", Path: "LoadBalancerName", Width: 32},
				{Title: "Type", Path: "Type", Width: 12},
				{Title: "Scheme", Path: "Scheme", Width: 14},
				{Title: "State", Path: "State.Code", Width: 12},
				{Title: "DNS Name", Path: "DNSName", Width: 48},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []string{
				"LoadBalancerName", "LoadBalancerArn", "DNSName", "Type",
				"Scheme", "State", "VpcId", "AvailabilityZones",
				"SecurityGroups", "IpAddressType", "CanonicalHostedZoneId",
				"CreatedTime",
			},
		},
		"tg": {
			List: []ListColumn{
				{Title: "Target Group", Path: "TargetGroupName", Width: 32},
				{Title: "Port", Path: "Port", Width: 8},
				{Title: "Protocol", Path: "Protocol", Width: 10},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Target Type", Path: "TargetType", Width: 12},
				{Title: "Health Check", Path: "HealthCheckPath", Width: 24},
			},
			Detail: []string{
				"TargetGroupName", "TargetGroupArn", "Port", "Protocol",
				"ProtocolVersion", "VpcId", "TargetType", "HealthCheckPath",
				"HealthCheckPort", "HealthCheckProtocol", "HealthCheckEnabled",
				"HealthCheckIntervalSeconds", "HealthCheckTimeoutSeconds",
				"HealthyThresholdCount", "UnhealthyThresholdCount",
				"Matcher", "LoadBalancerArns",
			},
		},
		"sg": {
			List: []ListColumn{
				{Title: "Group ID", Path: "GroupId", Width: 24},
				{Title: "Group Name", Path: "GroupName", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []string{
				"GroupId", "GroupName", "VpcId", "Description",
				"OwnerId", "SecurityGroupArn", "IpPermissions",
				"IpPermissionsEgress", "Tags",
			},
		},
		"vpc": {
			List: []ListColumn{
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Name", Path: "", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Default", Path: "IsDefault", Width: 9},
			},
			Detail: []string{
				"VpcId", "CidrBlock", "State", "IsDefault",
				"InstanceTenancy", "DhcpOptionsId", "OwnerId",
				"CidrBlockAssociationSet", "Ipv6CidrBlockAssociationSet", "Tags",
			},
		},
		"subnet": {
			List: []ListColumn{
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "AZ", Path: "AvailabilityZone", Width: 14},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Available IPs", Path: "AvailableIpAddressCount", Width: 14},
			},
			Detail: []string{
				"SubnetId", "VpcId", "CidrBlock", "AvailabilityZone",
				"AvailabilityZoneId", "State", "AvailableIpAddressCount",
				"MapPublicIpOnLaunch", "DefaultForAz", "SubnetArn", "OwnerId", "Tags",
			},
		},
		"rtb": {
			List: []ListColumn{
				{Title: "Route Table ID", Path: "RouteTableId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Routes", Path: "", Key: "routes_count", Width: 8},
				{Title: "Assoc.", Path: "", Key: "associations_count", Width: 8},
			},
			Detail: []string{
				"RouteTableId", "VpcId", "Routes", "Associations",
				"OwnerId", "Tags",
			},
		},
		"nat": {
			List: []ListColumn{
				{Title: "NAT Gateway ID", Path: "NatGatewayId", Width: 26},
				{Title: "Name", Path: "", Width: 24},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Public IP", Path: "", Key: "public_ip", Width: 16},
			},
			Detail: []string{
				"NatGatewayId", "VpcId", "SubnetId", "State",
				"ConnectivityType", "NatGatewayAddresses", "CreateTime",
				"FailureCode", "FailureMessage", "Tags",
			},
		},
		"igw": {
			List: []ListColumn{
				{Title: "IGW ID", Path: "InternetGatewayId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "VPC ID", Path: "", Key: "vpc_id", Width: 24},
				{Title: "State", Path: "", Key: "state", Width: 12},
			},
			Detail: []string{
				"InternetGatewayId", "Attachments", "OwnerId", "Tags",
			},
		},
		"eip": {
			List: []ListColumn{
				{Title: "Allocation ID", Path: "AllocationId", Width: 26},
				{Title: "Name", Path: "", Width: 24},
				{Title: "Public IP", Path: "PublicIp", Width: 16},
				{Title: "Association", Path: "AssociationId", Width: 26},
				{Title: "Instance", Path: "InstanceId", Width: 20},
				{Title: "Domain", Path: "Domain", Width: 8},
			},
			Detail: []string{
				"AllocationId", "PublicIp", "AssociationId", "InstanceId",
				"Domain", "NetworkBorderGroup", "SubnetId",
				"PrivateIpAddress", "NetworkInterfaceId", "Tags",
			},
		},
		"vpce": {
			List: []ListColumn{
				{Title: "Endpoint ID", Path: "VpcEndpointId", Width: 26},
				{Title: "Service Name", Path: "ServiceName", Width: 40},
				{Title: "Type", Path: "VpcEndpointType", Width: 12},
				{Title: "State", Path: "State", Width: 12},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []string{
				"VpcEndpointId", "ServiceName", "VpcEndpointType",
				"State", "VpcId", "SubnetIds", "NetworkInterfaceIds",
				"RouteTableIds", "Groups", "PrivateDnsEnabled",
				"PolicyDocument", "CreationTimestamp",
				"OwnerId", "Tags",
			},
		},
		"tgw": {
			List: []ListColumn{
				{Title: "TGW ID", Path: "TransitGatewayId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Owner", Path: "OwnerId", Width: 14},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"TransitGatewayId", "TransitGatewayArn", "State",
				"OwnerId", "Description", "Options",
				"CreationTime", "Tags",
			},
		},
		"eni": {
			List: []ListColumn{
				{Title: "ENI ID", Path: "NetworkInterfaceId", Width: 26},
				{Title: "Name", Path: "", Width: 24},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Type", Path: "InterfaceType", Width: 14},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Private IP", Path: "PrivateIpAddress", Width: 16},
			},
			Detail: []string{
				"NetworkInterfaceId", "Status", "InterfaceType",
				"VpcId", "SubnetId", "AvailabilityZone",
				"PrivateIpAddress", "PrivateDnsName",
				"MacAddress", "Description", "OwnerId",
				"RequesterId", "RequesterManaged",
				"SourceDestCheck", "Groups", "Attachment",
				"Association", "TagSet",
			},
		},
		// Child views for networking resources
		"elb_listeners": {
			List: []ListColumn{
				{Title: "Port", Path: "Port", Width: 8},
				{Title: "Protocol", Path: "Protocol", Width: 10},
				{Title: "Action", Key: "default_action_type", Width: 16},
				{Title: "Target", Key: "default_action_target", Width: 32},
				{Title: "SSL Policy", Path: "SslPolicy", Width: 24},
				{Title: "Certificate", Key: "certificate_short", Width: 32},
			},
			Detail: []string{
				"ListenerArn", "Port", "Protocol", "DefaultActions",
				"SslPolicy", "Certificates", "AlpnPolicy", "MutualAuthentication",
			},
		},
		"tg_health": {
			List: []ListColumn{
				{Title: "Target ID", Key: "target_id", Width: 24},
				{Title: "Port", Key: "port", Width: 8},
				{Title: "AZ", Key: "az", Width: 14},
				{Title: "Health", Key: "health", Width: 14},
				{Title: "Reason", Key: "reason", Width: 28},
				{Title: "Description", Key: "description", Width: 36},
			},
			Detail: []string{
				"Target.Id", "Target.Port", "Target.AvailabilityZone",
				"TargetHealth.State", "TargetHealth.Reason", "TargetHealth.Description",
				"HealthCheckPort", "AnomalyDetection",
			},
		},
	}
}
