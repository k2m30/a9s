package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["vpce"] = vpceFixtures
	demoData["tgw"] = tgwFixtures
	demoData["eni"] = eniFixtures

	RegisterChildDemo("tg_health", func(parentCtx map[string]string) []resource.Resource {
		return tgHealthFixtures(parentCtx["target_group_arn"])
	})
}

// ---------------------------------------------------------------------------
// VPC Endpoints (ec2types.VpcEndpoint)
// Fields: vpce_id, service_name, type, state, vpc_id
// ---------------------------------------------------------------------------

func vpceFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "vpce-0aaa111111111111a",
			Name:   "com.amazonaws.us-east-1.s3",
			Status: "available",
			Fields: map[string]string{
				"vpce_id":      "vpce-0aaa111111111111a",
				"service_name": "com.amazonaws.us-east-1.s3",
				"type":         "Gateway",
				"state":        "available",
				"vpc_id":       prodVPCID,
			},
			RawStruct: ec2types.VpcEndpoint{
				VpcEndpointId:       aws.String("vpce-0aaa111111111111a"),
				ServiceName:         aws.String("com.amazonaws.us-east-1.s3"),
				VpcEndpointType:     ec2types.VpcEndpointTypeGateway,
				State:               ec2types.StateAvailable,
				VpcId:               aws.String(prodVPCID),
				RouteTableIds:       []string{"rtb-0aaa111111111111a", "rtb-0ccc333333333333c"},
				SubnetIds:           []string{prodPrivateSubnetA, prodPrivateSubnetB},
				NetworkInterfaceIds: []string{"eni-0ccc333333333333c"},
				Groups: []ec2types.SecurityGroupIdentifier{
					{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
				},
				PrivateDnsEnabled: aws.Bool(false),
				PolicyDocument:    aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"*","Resource":"*"}]}`),
				OwnerId:           aws.String("123456789012"),
				CreationTimestamp: aws.Time(mustParseTime("2025-06-15T12:00:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-s3-endpoint")},
				},
			},
		},
		{
			ID:     "vpce-0bbb222222222222b",
			Name:   "com.amazonaws.us-east-1.dynamodb",
			Status: "available",
			Fields: map[string]string{
				"vpce_id":      "vpce-0bbb222222222222b",
				"service_name": "com.amazonaws.us-east-1.dynamodb",
				"type":         "Gateway",
				"state":        "available",
				"vpc_id":       prodVPCID,
			},
			RawStruct: ec2types.VpcEndpoint{
				VpcEndpointId:   aws.String("vpce-0bbb222222222222b"),
				ServiceName:     aws.String("com.amazonaws.us-east-1.dynamodb"),
				VpcEndpointType: ec2types.VpcEndpointTypeGateway,
				State:           ec2types.StateAvailable,
				VpcId:           aws.String(prodVPCID),
				RouteTableIds:   []string{"rtb-0aaa111111111111a", "rtb-0ccc333333333333c"},
				OwnerId:         aws.String("123456789012"),
				CreationTimestamp: aws.Time(mustParseTime("2025-06-15T12:05:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-dynamodb-endpoint")},
				},
			},
		},
		{
			ID:     "vpce-0ccc333333333333c",
			Name:   "com.amazonaws.us-east-1.secretsmanager",
			Status: "available",
			Fields: map[string]string{
				"vpce_id":      "vpce-0ccc333333333333c",
				"service_name": "com.amazonaws.us-east-1.secretsmanager",
				"type":         "Interface",
				"state":        "available",
				"vpc_id":       prodVPCID,
			},
			RawStruct: ec2types.VpcEndpoint{
				VpcEndpointId:     aws.String("vpce-0ccc333333333333c"),
				ServiceName:       aws.String("com.amazonaws.us-east-1.secretsmanager"),
				VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
				State:             ec2types.StateAvailable,
				VpcId:             aws.String(prodVPCID),
				SubnetIds:         []string{prodPrivateSubnetA, prodPrivateSubnetB},
				NetworkInterfaceIds: []string{"eni-0ccc333333333333c", "eni-0ddd444444444444d"},
				PrivateDnsEnabled: aws.Bool(true),
				OwnerId:           aws.String("123456789012"),
				CreationTimestamp:  aws.Time(mustParseTime("2025-08-01T09:30:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-secrets-endpoint")},
				},
			},
		},
		{
			ID:     "vpce-0ddd444444444444d",
			Name:   "com.amazonaws.us-east-1.ecr.dkr",
			Status: "pending",
			Fields: map[string]string{
				"vpce_id":      "vpce-0ddd444444444444d",
				"service_name": "com.amazonaws.us-east-1.ecr.dkr",
				"type":         "Interface",
				"state":        "pending",
				"vpc_id":       prodVPCID,
			},
			RawStruct: ec2types.VpcEndpoint{
				VpcEndpointId:     aws.String("vpce-0ddd444444444444d"),
				ServiceName:       aws.String("com.amazonaws.us-east-1.ecr.dkr"),
				VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
				State:             ec2types.StatePending,
				VpcId:             aws.String(prodVPCID),
				SubnetIds:         []string{prodPrivateSubnetA},
				PrivateDnsEnabled: aws.Bool(true),
				OwnerId:           aws.String("123456789012"),
				CreationTimestamp:  aws.Time(mustParseTime("2026-03-21T07:00:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-ecr-endpoint")},
				},
			},
		},
		// ct-events case F cross-ref: IRSA GetObject via VPC endpoint in eu-west-1.
		{
			ID:     "vpce-0abc123def456",
			Name:   "com.amazonaws.eu-west-1.s3",
			Status: "available",
			Fields: map[string]string{
				"vpce_id":      "vpce-0abc123def456",
				"service_name": "com.amazonaws.eu-west-1.s3",
				"type":         "Interface",
				"state":        "available",
				"vpc_id":       prodVPCID,
			},
			RawStruct: ec2types.VpcEndpoint{
				VpcEndpointId:     aws.String("vpce-0abc123def456"),
				ServiceName:       aws.String("com.amazonaws.eu-west-1.s3"),
				VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
				State:             ec2types.StateAvailable,
				VpcId:             aws.String(prodVPCID),
				SubnetIds:         []string{prodPrivateSubnetA, prodPrivateSubnetB},
				PrivateDnsEnabled: aws.Bool(true),
				OwnerId:           aws.String("666666666666"),
				CreationTimestamp:  aws.Time(mustParseTime("2026-01-10T11:00:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("eks-s3-endpoint-eu-west-1")},
				},
			},
		},
		// ct-events case I cross-ref: DataPipelineRole VPCE deny in eu-central-1.
		{
			ID:     "vpce-0ff11223344556677",
			Name:   "com.amazonaws.eu-central-1.s3",
			Status: "available",
			Fields: map[string]string{
				"vpce_id":      "vpce-0ff11223344556677",
				"service_name": "com.amazonaws.eu-central-1.s3",
				"type":         "Interface",
				"state":        "available",
				"vpc_id":       prodVPCID,
			},
			RawStruct: ec2types.VpcEndpoint{
				VpcEndpointId:     aws.String("vpce-0ff11223344556677"),
				ServiceName:       aws.String("com.amazonaws.eu-central-1.s3"),
				VpcEndpointType:   ec2types.VpcEndpointTypeInterface,
				State:             ec2types.StateAvailable,
				VpcId:             aws.String(prodVPCID),
				SubnetIds:         []string{prodPrivateSubnetA, prodPrivateSubnetB},
				PrivateDnsEnabled: aws.Bool(true),
				OwnerId:           aws.String("111111111111"),
				CreationTimestamp:  aws.Time(mustParseTime("2025-11-15T08:30:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("data-pipeline-s3-endpoint-eu-central-1")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Transit Gateways (ec2types.TransitGateway)
// Fields: tgw_id, name, state, owner_id, description
// ---------------------------------------------------------------------------

func tgwFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "tgw-0aaa111111111111a",
			Name:   "acme-hub-tgw",
			Status: "available",
			Fields: map[string]string{
				"tgw_id":      "tgw-0aaa111111111111a",
				"name":        "acme-hub-tgw",
				"state":       "available",
				"owner_id":    "123456789012",
				"description": "Central hub transit gateway for Acme Corp VPCs",
			},
			RawStruct: ec2types.TransitGateway{
				TransitGatewayId:  aws.String("tgw-0aaa111111111111a"),
				TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0aaa111111111111a"),
				State:             ec2types.TransitGatewayStateAvailable,
				OwnerId:           aws.String("123456789012"),
				Description:       aws.String("Central hub transit gateway for Acme Corp VPCs"),
				CreationTime:      aws.Time(mustParseTime("2025-03-01T09:00:00+00:00")),
				Options: &ec2types.TransitGatewayOptions{
					AmazonSideAsn:                aws.Int64(64512),
					AutoAcceptSharedAttachments:   ec2types.AutoAcceptSharedAttachmentsValueEnable,
					DefaultRouteTableAssociation:  ec2types.DefaultRouteTableAssociationValueEnable,
					DefaultRouteTablePropagation:  ec2types.DefaultRouteTablePropagationValueEnable,
					DnsSupport:                    ec2types.DnsSupportValueEnable,
					VpnEcmpSupport:                ec2types.VpnEcmpSupportValueEnable,
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-hub-tgw")},
					{Key: aws.String("Environment"), Value: aws.String("shared")},
				},
			},
		},
		{
			ID:     "tgw-0bbb222222222222b",
			Name:   "acme-dr-tgw",
			Status: "available",
			Fields: map[string]string{
				"tgw_id":      "tgw-0bbb222222222222b",
				"name":        "acme-dr-tgw",
				"state":       "available",
				"owner_id":    "123456789012",
				"description": "Disaster recovery cross-region transit gateway",
			},
			RawStruct: ec2types.TransitGateway{
				TransitGatewayId:  aws.String("tgw-0bbb222222222222b"),
				TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0bbb222222222222b"),
				State:             ec2types.TransitGatewayStateAvailable,
				OwnerId:           aws.String("123456789012"),
				Description:       aws.String("Disaster recovery cross-region transit gateway"),
				CreationTime:      aws.Time(mustParseTime("2025-09-15T14:00:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-dr-tgw")},
					{Key: aws.String("Environment"), Value: aws.String("dr")},
				},
			},
		},
		{
			ID:     "tgw-0ccc333333333333c",
			Name:   "acme-dev-tgw",
			Status: "deleting",
			Fields: map[string]string{
				"tgw_id":      "tgw-0ccc333333333333c",
				"name":        "acme-dev-tgw",
				"state":       "deleting",
				"owner_id":    "123456789012",
				"description": "Development transit gateway (decommissioning)",
			},
			RawStruct: ec2types.TransitGateway{
				TransitGatewayId:  aws.String("tgw-0ccc333333333333c"),
				TransitGatewayArn: aws.String("arn:aws:ec2:us-east-1:123456789012:transit-gateway/tgw-0ccc333333333333c"),
				State:             ec2types.TransitGatewayStateDeleting,
				OwnerId:           aws.String("123456789012"),
				Description:       aws.String("Development transit gateway (decommissioning)"),
				CreationTime:      aws.Time(mustParseTime("2025-01-10T08:00:00+00:00")),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-dev-tgw")},
					{Key: aws.String("Environment"), Value: aws.String("dev")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Network Interfaces (ec2types.NetworkInterface)
// Fields: eni_id, name, status, type, vpc_id, private_ip
// Note: NetworkInterface uses TagSet (not Tags).
// ---------------------------------------------------------------------------

func eniFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "eni-0aaa111111111111a",
			Name:   "prod-nat-eni-1a",
			Status: "in-use",
			Fields: map[string]string{
				"eni_id":     "eni-0aaa111111111111a",
				"name":       "prod-nat-eni-1a",
				"status":     "in-use",
				"type":       "natGateway",
				"vpc_id":     prodVPCID,
				"private_ip": "10.0.1.50",
			},
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String("eni-0aaa111111111111a"),
				Status:             ec2types.NetworkInterfaceStatusInUse,
				InterfaceType:      ec2types.NetworkInterfaceTypeNatGateway,
				VpcId:              aws.String(prodVPCID),
				SubnetId:           aws.String(prodPublicSubnetA),
				AvailabilityZone:   aws.String("us-east-1a"),
				PrivateIpAddress:   aws.String("10.0.1.50"),
				PrivateDnsName:     aws.String("ip-10-0-1-50.ec2.internal"),
				MacAddress:         aws.String("0a:1b:2c:3d:4e:01"),
				Description:        aws.String("Interface for NAT Gateway nat-0aaa111111111111a"),
				OwnerId:            aws.String("123456789012"),
				RequesterId:        aws.String("amazon-elb"),
				RequesterManaged:   aws.Bool(true),
				SourceDestCheck:    aws.Bool(false),
				Attachment: &ec2types.NetworkInterfaceAttachment{
					AttachmentId:        aws.String("eni-attach-01"),
					InstanceId:          aws.String("i-0a1b2c3d4e5f60001"),
					DeviceIndex:         aws.Int32(0),
					Status:              ec2types.AttachmentStatusAttached,
					DeleteOnTermination: aws.Bool(true),
				},
				Groups: []ec2types.GroupIdentifier{
					{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
				},
				Association: &ec2types.NetworkInterfaceAssociation{
					PublicIp:      aws.String("54.210.33.112"),
					PublicDnsName: aws.String("ec2-54-210-33-112.compute-1.amazonaws.com"),
					IpOwnerId:     aws.String("amazon"),
					AllocationId:  aws.String(relatedEC2EIPID),
				},
				TagSet: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat-eni-1a")},
				},
			},
		},
		{
			ID:     "eni-0bbb222222222222b",
			Name:   "prod-nat-eni-1b",
			Status: "in-use",
			Fields: map[string]string{
				"eni_id":     "eni-0bbb222222222222b",
				"name":       "prod-nat-eni-1b",
				"status":     "in-use",
				"type":       "natGateway",
				"vpc_id":     prodVPCID,
				"private_ip": "10.0.2.50",
			},
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String("eni-0bbb222222222222b"),
				Status:             ec2types.NetworkInterfaceStatusInUse,
				InterfaceType:      ec2types.NetworkInterfaceTypeNatGateway,
				VpcId:              aws.String(prodVPCID),
				SubnetId:           aws.String(prodPublicSubnetB),
				AvailabilityZone:   aws.String("us-east-1b"),
				PrivateIpAddress:   aws.String("10.0.2.50"),
				PrivateDnsName:     aws.String("ip-10-0-2-50.ec2.internal"),
				MacAddress:         aws.String("0a:1b:2c:3d:4e:02"),
				Description:        aws.String("Interface for NAT Gateway nat-0bbb222222222222b"),
				OwnerId:            aws.String("123456789012"),
				RequesterManaged:   aws.Bool(true),
				SourceDestCheck:    aws.Bool(false),
				Attachment: &ec2types.NetworkInterfaceAttachment{
					AttachmentId:        aws.String("eni-attach-02"),
					InstanceId:          aws.String("i-0a1b2c3d4e5f60002"),
					DeviceIndex:         aws.Int32(0),
					Status:              ec2types.AttachmentStatusAttached,
					DeleteOnTermination: aws.Bool(true),
				},
				Groups: []ec2types.GroupIdentifier{
					{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
				},
				Association: &ec2types.NetworkInterfaceAssociation{
					PublicIp:     aws.String("54.210.33.113"),
					IpOwnerId:    aws.String("amazon"),
					AllocationId: aws.String("eipalloc-0bbb222222222222b"),
				},
				TagSet: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat-eni-1b")},
				},
			},
		},
		{
			ID:     "eni-0eee555555555555e",
			Name:   "web-prod-01-primary",
			Status: "in-use",
			Fields: map[string]string{
				"eni_id":     "eni-0eee555555555555e",
				"name":       "web-prod-01-primary",
				"status":     "in-use",
				"type":       "interface",
				"vpc_id":     prodVPCID,
				"private_ip": "10.0.1.10",
			},
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String("eni-0eee555555555555e"),
				Status:             ec2types.NetworkInterfaceStatusInUse,
				InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
				VpcId:              aws.String(prodVPCID),
				SubnetId:           aws.String(prodPublicSubnetA),
				AvailabilityZone:   aws.String("us-east-1a"),
				PrivateIpAddress:   aws.String("10.0.1.10"),
				PrivateDnsName:     aws.String("ip-10-0-1-10.ec2.internal"),
				MacAddress:         aws.String("0a:1b:2c:3d:4e:05"),
				Description:        aws.String("Primary network interface for web-prod-01"),
				OwnerId:            aws.String("123456789012"),
				RequesterId:        aws.String("amazon-elb"),
				RequesterManaged:   aws.Bool(false),
				SourceDestCheck:    aws.Bool(true),
				Attachment: &ec2types.NetworkInterfaceAttachment{
					AttachmentId:        aws.String("eni-attach-05"),
					InstanceId:          aws.String("i-0a1b2c3d4e5f60001"),
					DeviceIndex:         aws.Int32(0),
					Status:              ec2types.AttachmentStatusAttached,
					DeleteOnTermination: aws.Bool(true),
				},
				Groups: []ec2types.GroupIdentifier{
					{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-web-alb-sg")},
				},
				Association: &ec2types.NetworkInterfaceAssociation{
					PublicIp:     aws.String("54.210.33.115"),
					IpOwnerId:    aws.String("amazon"),
					AllocationId: aws.String("eipalloc-0aaa111111111111a"),
				},
				TagSet: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("web-prod-01-primary")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "eni-0fff666666666666f",
			Name:   "vpce-secrets-eni-1a",
			Status: "in-use",
			Fields: map[string]string{
				"eni_id":     "eni-0fff666666666666f",
				"name":       "vpce-secrets-eni-1a",
				"status":     "in-use",
				"type":       "vpc_endpoint",
				"vpc_id":     prodVPCID,
				"private_ip": "10.0.3.100",
			},
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String("eni-0fff666666666666f"),
				Status:             ec2types.NetworkInterfaceStatusInUse,
				InterfaceType:      ec2types.NetworkInterfaceTypeVpcEndpoint,
				VpcId:              aws.String(prodVPCID),
				SubnetId:           aws.String(prodPrivateSubnetA),
				AvailabilityZone:   aws.String("us-east-1a"),
				PrivateIpAddress:   aws.String("10.0.3.100"),
				PrivateDnsName:     aws.String("ip-10-0-3-100.ec2.internal"),
				MacAddress:         aws.String("0a:1b:2c:3d:4e:06"),
				Description:        aws.String("VPC Endpoint Interface for Secrets Manager"),
				OwnerId:            aws.String("123456789012"),
				RequesterManaged:   aws.Bool(true),
				SourceDestCheck:    aws.Bool(true),
				Attachment: &ec2types.NetworkInterfaceAttachment{
					AttachmentId:        aws.String("eni-attach-06"),
					InstanceId:          aws.String("i-0a1b2c3d4e5f60003"),
					DeviceIndex:         aws.Int32(0),
					Status:              ec2types.AttachmentStatusAttached,
					DeleteOnTermination: aws.Bool(false),
				},
				Groups: []ec2types.GroupIdentifier{
					{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-api-internal-sg")},
				},
				Association: &ec2types.NetworkInterfaceAssociation{
					PublicIp:     aws.String("54.210.33.116"),
					IpOwnerId:    aws.String("amazon"),
					AllocationId: aws.String("eipalloc-0ddd444444444444d"),
				},
				TagSet: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("vpce-secrets-eni-1a")},
				},
			},
		},
		{
			ID:     "eni-0ggg777777777777g",
			Name:   "detached-eni",
			Status: "available",
			Fields: map[string]string{
				"eni_id":     "eni-0ggg777777777777g",
				"name":       "detached-eni",
				"status":     "available",
				"type":       "interface",
				"vpc_id":     prodVPCID,
				"private_ip": "10.0.3.200",
			},
			RawStruct: ec2types.NetworkInterface{
				NetworkInterfaceId: aws.String("eni-0ggg777777777777g"),
				Status:             ec2types.NetworkInterfaceStatusAvailable,
				InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
				VpcId:              aws.String(prodVPCID),
				SubnetId:           aws.String(prodPrivateSubnetA),
				AvailabilityZone:   aws.String("us-east-1a"),
				PrivateIpAddress:   aws.String("10.0.3.200"),
				PrivateDnsName:     aws.String("ip-10-0-3-200.ec2.internal"),
				MacAddress:         aws.String("0a:1b:2c:3d:4e:07"),
				Description:        aws.String("Detached ENI from terminated instance"),
				OwnerId:            aws.String("123456789012"),
				RequesterManaged:   aws.Bool(false),
				SourceDestCheck:    aws.Bool(true),
				Attachment: &ec2types.NetworkInterfaceAttachment{
					AttachmentId:        aws.String("eni-attach-07"),
					InstanceId:          aws.String("i-0a1b2c3d4e5f60009"),
					DeviceIndex:         aws.Int32(1),
					Status:              ec2types.AttachmentStatusDetached,
					DeleteOnTermination: aws.Bool(false),
				},
				Groups: []ec2types.GroupIdentifier{
					{GroupId: aws.String(prodRDSSGID), GroupName: aws.String("acme-rds-sg")},
				},
				Association: &ec2types.NetworkInterfaceAssociation{
					PublicIp:     aws.String("54.210.33.117"),
					IpOwnerId:    aws.String("amazon"),
					AllocationId: aws.String("eipalloc-0aaa111111111111a"),
				},
				TagSet: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("detached-eni")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Target Health (elbv2types.TargetHealthDescription) — child of Target Groups
// Fields: target_id, port, az, health, reason, description
// ---------------------------------------------------------------------------

func tgHealthFixtures(_ string) []resource.Resource {
	port80 := int32(80)
	port443 := int32(443)
	port8080 := int32(8080)
	port3000 := int32(3000)

	return []resource.Resource{
		{
			ID:     "i-0abc1234def56789a",
			Name:   "i-0abc1234def56789a",
			Status: "healthy",
			Fields: map[string]string{
				"target_id":   "i-0abc1234def56789a",
				"port":        "80",
				"az":          "us-east-1a",
				"health":      "healthy",
				"reason":      "",
				"description": "Health checks passed",
			},
			RawStruct: elbv2types.TargetHealthDescription{
				Target: &elbv2types.TargetDescription{
					Id:               aws.String("i-0abc1234def56789a"),
					Port:             &port80,
					AvailabilityZone: aws.String("us-east-1a"),
				},
				TargetHealth: &elbv2types.TargetHealth{
					State:       elbv2types.TargetHealthStateEnumHealthy,
					Description: aws.String("Health checks passed"),
				},
				HealthCheckPort: aws.String("80"),
			},
		},
		{
			ID:     "i-0bcd2345efg67890b",
			Name:   "i-0bcd2345efg67890b",
			Status: "healthy",
			Fields: map[string]string{
				"target_id":   "i-0bcd2345efg67890b",
				"port":        "443",
				"az":          "us-east-1b",
				"health":      "healthy",
				"reason":      "",
				"description": "Health checks passed",
			},
			RawStruct: elbv2types.TargetHealthDescription{
				Target: &elbv2types.TargetDescription{
					Id:               aws.String("i-0bcd2345efg67890b"),
					Port:             &port443,
					AvailabilityZone: aws.String("us-east-1b"),
				},
				TargetHealth: &elbv2types.TargetHealth{
					State:       elbv2types.TargetHealthStateEnumHealthy,
					Description: aws.String("Health checks passed"),
				},
				HealthCheckPort: aws.String("443"),
			},
		},
		{
			ID:     "i-0cde3456fgh78901c",
			Name:   "i-0cde3456fgh78901c",
			Status: "unhealthy",
			Fields: map[string]string{
				"target_id":   "i-0cde3456fgh78901c",
				"port":        "8080",
				"az":          "us-east-1c",
				"health":      "unhealthy",
				"reason":      "Target.FailedHealthChecks",
				"description": "Health checks failed with 503",
			},
			RawStruct: elbv2types.TargetHealthDescription{
				Target: &elbv2types.TargetDescription{
					Id:               aws.String("i-0cde3456fgh78901c"),
					Port:             &port8080,
					AvailabilityZone: aws.String("us-east-1c"),
				},
				TargetHealth: &elbv2types.TargetHealth{
					State:       elbv2types.TargetHealthStateEnumUnhealthy,
					Reason:      elbv2types.TargetHealthReasonEnumFailedHealthChecks,
					Description: aws.String("Health checks failed with 503"),
				},
				HealthCheckPort: aws.String("8080"),
			},
		},
		{
			ID:     "i-0def4567ghi89012d",
			Name:   "i-0def4567ghi89012d",
			Status: "draining",
			Fields: map[string]string{
				"target_id":   "i-0def4567ghi89012d",
				"port":        "3000",
				"az":          "us-east-1a",
				"health":      "draining",
				"reason":      "Target.DeregistrationInProgress",
				"description": "Target deregistration in progress",
			},
			RawStruct: elbv2types.TargetHealthDescription{
				Target: &elbv2types.TargetDescription{
					Id:               aws.String("i-0def4567ghi89012d"),
					Port:             &port3000,
					AvailabilityZone: aws.String("us-east-1a"),
				},
				TargetHealth: &elbv2types.TargetHealth{
					State:       elbv2types.TargetHealthStateEnumDraining,
					Reason:      elbv2types.TargetHealthReasonEnumDeregistrationInProgress,
					Description: aws.String("Target deregistration in progress"),
				},
				HealthCheckPort: aws.String("3000"),
			},
		},
	}
}
