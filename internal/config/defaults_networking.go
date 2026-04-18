package config

func networkingDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"elb": {
			List: []ListColumn{
				{Title: "Name", Path: "LoadBalancerName", Width: 32},
				{Title: "Type", Path: "Type", Width: 12},
				{Title: "Scheme", Path: "Scheme", Width: 14},
				{Title: "State", Path: "State.Code", Width: 12},
				{Title: "State Reason", Path: "State.Reason", Width: 32},
				{Title: "DNS Name", Path: "DNSName", Width: 48},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []DetailField{
				{Path: "LoadBalancerName"}, {Path: "LoadBalancerArn"}, {Path: "DNSName"}, {Path: "Type"},
				{Path: "Scheme"}, {Path: "State"}, {Path: "VpcId"}, {Path: "AvailabilityZones"},
				{Path: "SecurityGroups"}, {Path: "IpAddressType"}, {Path: "CanonicalHostedZoneId"},
				{Path: "CreatedTime"},
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
			Detail: []DetailField{
				{Path: "TargetGroupName"}, {Path: "TargetGroupArn"}, {Path: "Port"}, {Path: "Protocol"},
				{Path: "ProtocolVersion"}, {Path: "VpcId"}, {Path: "TargetType"}, {Path: "HealthCheckPath"},
				{Path: "HealthCheckPort"}, {Path: "HealthCheckProtocol"}, {Path: "HealthCheckEnabled"},
				{Path: "HealthCheckIntervalSeconds"}, {Path: "HealthCheckTimeoutSeconds"},
				{Path: "HealthyThresholdCount"}, {Path: "UnhealthyThresholdCount"},
				{Path: "Matcher"}, {Path: "LoadBalancerArns"},
			},
		},
		"sg": {
			List: []ListColumn{
				{Title: "Group Name", Path: "GroupName", Width: 28},
				{Title: "Group ID", Path: "GroupId", Width: 24},
				{Title: "Risk", Key: "risk_summary", Width: 22},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []DetailField{
				{Path: "GroupId"}, {Path: "GroupName"}, {Path: "VpcId"}, {Path: "Description"},
				{Path: "OwnerId"}, {Path: "SecurityGroupArn"}, {Path: "IpPermissions"},
				{Path: "IpPermissionsEgress"}, {Path: "Tags"},
				{Key: "risk_summary", Label: "Risk Summary"},
			},
		},
		"vpc": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Default", Path: "IsDefault", Width: 9},
			},
			Detail: []DetailField{
				{Path: "VpcId"}, {Path: "CidrBlock"}, {Path: "State"}, {Path: "IsDefault"},
				{Path: "InstanceTenancy"}, {Path: "DhcpOptionsId"}, {Path: "OwnerId"},
				{Path: "CidrBlockAssociationSet"}, {Path: "Ipv6CidrBlockAssociationSet"}, {Path: "Tags"},
			},
		},
		"subnet": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 28},
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "AZ", Path: "AvailabilityZone", Width: 14},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Public", Path: "MapPublicIpOnLaunch", Width: 8},
				{Title: "Available IPs", Path: "AvailableIpAddressCount", Width: 14},
			},
			Detail: []DetailField{
				{Path: "SubnetId"}, {Path: "VpcId"}, {Path: "CidrBlock"}, {Path: "AvailabilityZone"},
				{Path: "AvailabilityZoneId"}, {Path: "State"}, {Path: "AvailableIpAddressCount"},
				{Path: "MapPublicIpOnLaunch"}, {Path: "DefaultForAz"}, {Path: "SubnetArn"}, {Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"rtb": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 28},
				{Title: "Route Table ID", Path: "RouteTableId", Width: 26},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Routes", Path: "", Key: "routes_count", Width: 8},
				{Title: "Assoc.", Path: "", Key: "associations_count", Width: 8},
			},
			Detail: []DetailField{
				{Path: "RouteTableId"}, {Path: "VpcId"}, {Path: "Routes"}, {Path: "Associations"},
				{Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"nat": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "NAT Gateway ID", Path: "NatGatewayId", Width: 26},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Failure", Path: "FailureCode", Width: 22},
				{Title: "Public IP", Path: "", Key: "public_ip", Width: 16},
			},
			Detail: []DetailField{
				{Path: "NatGatewayId"}, {Path: "VpcId"}, {Path: "SubnetId"}, {Path: "State"},
				{Path: "ConnectivityType"}, {Path: "NatGatewayAddresses"}, {Path: "CreateTime"},
				{Path: "FailureCode"}, {Path: "FailureMessage"}, {Path: "Tags"},
			},
		},
		"igw": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 28},
				{Title: "IGW ID", Path: "InternetGatewayId", Width: 26},
				{Title: "VPC ID", Path: "", Key: "vpc_id", Width: 24},
				{Title: "State", Path: "", Key: "state", Width: 12},
			},
			Detail: []DetailField{
				{Path: "InternetGatewayId"}, {Path: "Attachments"}, {Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"eip": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "Allocation ID", Path: "AllocationId", Width: 26},
				{Title: "Public IP", Path: "PublicIp", Width: 16},
				{Title: "Association", Path: "AssociationId", Width: 26},
				{Title: "Instance", Path: "InstanceId", Width: 20},
				{Title: "Domain", Path: "Domain", Width: 8},
			},
			Detail: []DetailField{
				{Path: "AllocationId"}, {Path: "PublicIp"}, {Path: "AssociationId"}, {Path: "InstanceId"},
				{Path: "Domain"}, {Path: "NetworkBorderGroup"}, {Path: "SubnetId"},
				{Path: "PrivateIpAddress"}, {Path: "NetworkInterfaceId"}, {Path: "Tags"},
			},
		},
		"vpce": {
			List: []ListColumn{
				{Title: "Service Name", Path: "ServiceName", Width: 40},
				{Title: "Endpoint ID", Path: "VpcEndpointId", Width: 26},
				{Title: "Type", Path: "VpcEndpointType", Width: 12},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Last Error", Path: "LastError.Message", Width: 32},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []DetailField{
				{Path: "VpcEndpointId"}, {Path: "ServiceName"}, {Path: "VpcEndpointType"},
				{Path: "State"}, {Path: "VpcId"}, {Path: "SubnetIds"}, {Path: "NetworkInterfaceIds"},
				{Path: "RouteTableIds"}, {Path: "Groups"}, {Path: "PrivateDnsEnabled"},
				{Path: "PolicyDocument"}, {Path: "CreationTimestamp"},
				{Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"tgw": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 28},
				{Title: "TGW ID", Path: "TransitGatewayId", Width: 26},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Owner", Path: "OwnerId", Width: 14},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []DetailField{
				{Path: "TransitGatewayId"}, {Path: "TransitGatewayArn"}, {Path: "State"},
				{Path: "OwnerId"}, {Path: "Description"}, {Path: "Options"},
				{Path: "CreationTime"}, {Path: "Tags"},
			},
		},
		"eni": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "ENI ID", Path: "NetworkInterfaceId", Width: 26},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Type", Path: "InterfaceType", Width: 14},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Private IP", Path: "PrivateIpAddress", Width: 16},
			},
			Detail: []DetailField{
				{Path: "NetworkInterfaceId"}, {Path: "Status"}, {Path: "InterfaceType"},
				{Path: "VpcId"}, {Path: "SubnetId"}, {Path: "AvailabilityZone"},
				{Path: "PrivateIpAddress"}, {Path: "PrivateDnsName"},
				{Path: "MacAddress"}, {Path: "Description"}, {Path: "OwnerId"},
				{Path: "RequesterId"}, {Path: "RequesterManaged"},
				{Path: "SourceDestCheck"}, {Path: "Groups"}, {Path: "Attachment"},
				{Path: "Association"}, {Path: "TagSet"},
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
			Detail: []DetailField{
				{Path: "ListenerArn"}, {Path: "Port"}, {Path: "Protocol"}, {Path: "DefaultActions"},
				{Path: "SslPolicy"}, {Path: "Certificates"}, {Path: "AlpnPolicy"}, {Path: "MutualAuthentication"},
			},
		},
		"elb_listener_rules": {
			List: []ListColumn{
				{Title: "Priority", Key: "priority", Width: 10},
				{Title: "Conditions", Key: "conditions_summary", Width: 36},
				{Title: "Action", Key: "action_type", Width: 16},
				{Title: "Target", Key: "action_target", Width: 32},
			},
			Detail: []DetailField{
				{Path: "RuleArn"}, {Path: "Priority"}, {Path: "Conditions"}, {Path: "Actions"}, {Path: "IsDefault"},
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
			Detail: []DetailField{
				{Path: "Target.Id"}, {Path: "Target.Port"}, {Path: "Target.AvailabilityZone"},
				{Path: "TargetHealth.State"}, {Path: "TargetHealth.Reason"}, {Path: "TargetHealth.Description"},
				{Path: "HealthCheckPort"}, {Path: "AnomalyDetection"},
			},
		},
	}
}
