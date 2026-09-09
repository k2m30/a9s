// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func networkingDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"elb": {
			Detail: []DetailField{
				{Path: "LoadBalancerName"}, {Path: "LoadBalancerArn"}, {Path: "DNSName"}, {Path: "Type"},
				{Path: "Scheme"}, {Path: "State"}, {Path: "VpcId"}, {Path: "AvailabilityZones"},
				{Path: "SecurityGroups"}, {Path: "IpAddressType"}, {Path: "CanonicalHostedZoneId"},
				{Path: "CreatedTime"},
			},
		},
		"tg": {
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
			Detail: []DetailField{
				{Path: "GroupId"}, {Path: "GroupName"}, {Path: "VpcId"}, {Path: "Description"},
				{Path: "OwnerId"}, {Path: "SecurityGroupArn"}, {Path: "IpPermissions"},
				{Path: "IpPermissionsEgress"}, {Path: "Tags"},
				{Key: "risk_summary", Label: "Risk Summary"},
			},
		},
		"vpc": {
			Detail: []DetailField{
				{Path: "VpcId"}, {Path: "CidrBlock"}, {Path: "State"}, {Path: "IsDefault"},
				{Path: "InstanceTenancy"}, {Path: "DhcpOptionsId"}, {Path: "OwnerId"},
				{Path: "CidrBlockAssociationSet"}, {Path: "Ipv6CidrBlockAssociationSet"}, {Path: "Tags"},
			},
		},
		"subnet": {
			Detail: []DetailField{
				{Path: "SubnetId"}, {Path: "VpcId"}, {Path: "CidrBlock"}, {Path: "AvailabilityZone"},
				{Path: "AvailabilityZoneId"}, {Path: "State"}, {Path: "AvailableIpAddressCount"},
				{Path: "MapPublicIpOnLaunch"}, {Path: "DefaultForAz"}, {Path: "SubnetArn"}, {Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"rtb": {
			Detail: []DetailField{
				{Path: "RouteTableId"}, {Path: "VpcId"}, {Path: "Routes"}, {Path: "Associations"},
				{Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"nat": {
			Detail: []DetailField{
				{Path: "NatGatewayId"}, {Path: "VpcId"}, {Path: "SubnetId"}, {Path: "State"},
				{Path: "ConnectivityType"}, {Path: "NatGatewayAddresses"}, {Path: "CreateTime"},
				{Path: "FailureCode"}, {Path: "FailureMessage"}, {Path: "Tags"},
			},
		},
		"igw": {
			Detail: []DetailField{
				{Path: "InternetGatewayId"}, {Path: "Attachments"}, {Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"eip": {
			Detail: []DetailField{
				{Path: "AllocationId"}, {Path: "PublicIp"}, {Path: "AssociationId"}, {Path: "InstanceId"},
				{Path: "Domain"}, {Path: "NetworkBorderGroup"}, {Path: "SubnetId"},
				{Path: "PrivateIpAddress"}, {Path: "NetworkInterfaceId"}, {Path: "Tags"},
			},
		},
		"vpce": {
			Detail: []DetailField{
				{Path: "VpcEndpointId"}, {Path: "ServiceName"}, {Path: "VpcEndpointType"},
				{Path: "State"}, {Path: "VpcId"}, {Path: "SubnetIds"}, {Path: "NetworkInterfaceIds"},
				{Path: "RouteTableIds"}, {Path: "Groups"}, {Path: "PrivateDnsEnabled"},
				{Path: "PolicyDocument"}, {Path: "CreationTimestamp"},
				{Path: "OwnerId"}, {Path: "Tags"},
			},
		},
		"tgw": {
			Detail: []DetailField{
				{Path: "TransitGatewayId"}, {Path: "TransitGatewayArn"}, {Path: "State"},
				{Path: "OwnerId"}, {Path: "Description"}, {Path: "Options"},
				{Path: "CreationTime"}, {Path: "Tags"},
			},
		},
		"eni": {
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
		"transfer": {
			Detail: []DetailField{
				{Path: "ServerId"}, {Path: "Arn"}, {Path: "Domain"}, {Path: "EndpointType"},
				{Path: "IdentityProviderType"}, {Path: "IdentityProviderDetails"},
				{Path: "UserCount"}, {Path: "SecurityPolicyName"}, {Path: "Protocols"},
				{Path: "Certificate"}, {Path: "LoggingRole"}, {Path: "StructuredLogDestinations"},
				{Path: "EndpointDetails"}, {Path: "HostKeyFingerprint"},
				{Path: "As2ServiceManagedEgressIpAddresses"},
			},
		},
		"vpc-peer": {
			Detail: []DetailField{
				{Path: "VpcPeeringConnectionId"}, {Path: "Status"}, {Path: "ExpirationTime"},
				{Path: "RequesterVpcInfo"}, {Path: "AccepterVpcInfo"}, {Path: "Tags"},
			},
		},
		// Child views for networking resources
		"elb_listeners": {
			Detail: []DetailField{
				{Path: "ListenerArn"}, {Path: "Port"}, {Path: "Protocol"}, {Path: "DefaultActions"},
				{Path: "SslPolicy"}, {Path: "Certificates"}, {Path: "AlpnPolicy"}, {Path: "MutualAuthentication"},
			},
		},
		"elb_listener_rules": {
			Detail: []DetailField{
				{Path: "RuleArn"}, {Path: "Priority"}, {Path: "Conditions"}, {Path: "Actions"}, {Path: "IsDefault"},
			},
		},
		"tg_health": {
			Detail: []DetailField{
				{Path: "Target.Id"}, {Path: "Target.Port"}, {Path: "Target.AvailabilityZone"},
				{Path: "TargetHealth.State"}, {Path: "TargetHealth.Reason"}, {Path: "TargetHealth.Description"},
				{Path: "HealthCheckPort"}, {Path: "AnomalyDetection"},
			},
		},
		"transfer_agreements": {
			Detail: []DetailField{
				{Path: "AgreementId"}, {Path: "ServerId"}, {Path: "Description"}, {Path: "Status"},
				{Key: "local_profile", Label: "Local Profile"},
				{Key: "partner_profile", Label: "Partner Profile"},
				{Path: "BaseDirectory"},
				{Path: "AccessRole"}, {Path: "EnforceMessageSigning"}, {Path: "PreserveFilename"},
			},
		},
	}
}
