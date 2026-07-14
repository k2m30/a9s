package unit_test

import (
	"time"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// ===========================================================================
// Realistic SDK struct builders
// ===========================================================================

func realisticVPC() ec2types.Vpc {
	return ec2types.Vpc{
		VpcId:     new("vpc-0abc1234def56789a"),
		CidrBlock: new("10.0.0.0/16"),
		State:     ec2types.VpcStateAvailable,
		IsDefault: new(false),
		OwnerId:   new("123456789012"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-vpc")},
			{Key: new("env"), Value: new("production")},
		},
	}
}

func realisticSecurityGroup() ec2types.SecurityGroup {
	return ec2types.SecurityGroup{
		GroupId:     new("sg-0abc1234def56789a"),
		GroupName:   new("web-sg"),
		VpcId:       new("vpc-0abc1234"),
		Description: new("Web server security group"),
		OwnerId:     new("123456789012"),
		IpPermissions: []ec2types.IpPermission{
			{
				FromPort:   new(int32(443)),
				ToPort:     new(int32(443)),
				IpProtocol: new("tcp"),
				IpRanges: []ec2types.IpRange{
					{CidrIp: new("0.0.0.0/0"), Description: new("HTTPS from anywhere")},
				},
			},
		},
		IpPermissionsEgress: []ec2types.IpPermission{
			{
				IpProtocol: new("-1"),
				IpRanges:   []ec2types.IpRange{{CidrIp: new("0.0.0.0/0")}},
			},
		},
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("web-sg")},
		},
	}
}

func realisticNodeGroup() ekstypes.Nodegroup {
	return ekstypes.Nodegroup{
		NodegroupName: new("prod-ng-01"),
		ClusterName:   new("prod-cluster"),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"t3.large", "t3.xlarge"},
		AmiType:       ekstypes.AMITypesAl2X8664,
		CapacityType:  ekstypes.CapacityTypesOnDemand,
		DiskSize:      new(int32(100)),
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: new(int32(3)),
			MinSize:     new(int32(1)),
			MaxSize:     new(int32(5)),
		},
		NodeRole: new("arn:aws:iam::123456789012:role/eks-node-role"),
		Subnets:  []string{"subnet-0abc1234", "subnet-0def5678"},
		Tags:     map[string]string{"env": "production"},
	}
}

func realisticSubnet() ec2types.Subnet {
	return ec2types.Subnet{
		SubnetId:                new("subnet-0abc1234def56789a"),
		VpcId:                   new("vpc-0abc1234"),
		CidrBlock:               new("10.0.1.0/24"),
		AvailabilityZone:        new("us-east-1a"),
		State:                   ec2types.SubnetStateAvailable,
		AvailableIpAddressCount: new(int32(251)),
		MapPublicIpOnLaunch:     new(true),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("public-subnet-1a")},
		},
	}
}

func realisticNATGateway() ec2types.NatGateway {
	return ec2types.NatGateway{
		NatGatewayId:     new("nat-0abc1234def56789a"),
		VpcId:            new("vpc-0abc1234"),
		SubnetId:         new("subnet-0abc1234"),
		State:            ec2types.NatGatewayStateAvailable,
		ConnectivityType: ec2types.ConnectivityTypePublic,
		NatGatewayAddresses: []ec2types.NatGatewayAddress{
			{
				AllocationId:       new("eipalloc-0abc1234"),
				PublicIp:           new("54.123.45.67"),
				PrivateIp:          new("10.0.1.100"),
				NetworkInterfaceId: new("eni-0abc1234"),
			},
		},
		CreateTime: new(testTime),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-nat")},
		},
	}
}

func realisticInternetGateway() ec2types.InternetGateway {
	return ec2types.InternetGateway{
		InternetGatewayId: new("igw-0abc1234def56789a"),
		Attachments: []ec2types.InternetGatewayAttachment{
			{VpcId: new("vpc-0abc1234"), State: ec2types.AttachmentStatusAttached},
		},
		OwnerId: new("123456789012"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-igw")},
		},
	}
}

func realisticEIP() ec2types.Address {
	return ec2types.Address{
		AllocationId:     new("eipalloc-0abc1234def56789a"),
		PublicIp:         new("54.123.45.67"),
		AssociationId:    new("eipassoc-0abc1234"),
		InstanceId:       new("i-0abc1234"),
		Domain:           ec2types.DomainTypeVpc,
		PrivateIpAddress: new("10.0.1.42"),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-eip")},
		},
	}
}

func realisticTransitGateway() ec2types.TransitGateway {
	return ec2types.TransitGateway{
		TransitGatewayId: new("tgw-0abc1234def56789a"),
		State:            ec2types.TransitGatewayStateAvailable,
		OwnerId:          new("123456789012"),
		Description:      new("Production transit gateway"),
		Options: &ec2types.TransitGatewayOptions{
			AutoAcceptSharedAttachments:  ec2types.AutoAcceptSharedAttachmentsValueEnable,
			DefaultRouteTableAssociation: ec2types.DefaultRouteTableAssociationValueEnable,
			DnsSupport:                   ec2types.DnsSupportValueEnable,
			VpnEcmpSupport:               ec2types.VpnEcmpSupportValueEnable,
		},
		CreationTime: new(testTime),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-tgw")},
		},
	}
}

func realisticVPCEndpoint() ec2types.VpcEndpoint {
	return ec2types.VpcEndpoint{
		VpcEndpointId:     new("vpce-0abc1234def56789a"),
		ServiceName:       new("com.amazonaws.us-east-1.s3"),
		VpcEndpointType:   ec2types.VpcEndpointTypeGateway,
		State:             ec2types.StateAvailable,
		VpcId:             new("vpc-0abc1234"),
		CreationTimestamp: new(testTime),
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("s3-endpoint")},
		},
	}
}

func realisticENI() ec2types.NetworkInterface {
	return ec2types.NetworkInterface{
		NetworkInterfaceId: new("eni-0abc1234def56789a"),
		Status:             ec2types.NetworkInterfaceStatusInUse,
		InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
		VpcId:              new("vpc-0abc1234"),
		SubnetId:           new("subnet-0abc1234"),
		PrivateIpAddress:   new("10.0.1.42"),
		MacAddress:         new("02:ab:cd:ef:12:34"),
		Description:        new("Primary network interface"),
		Groups: []ec2types.GroupIdentifier{
			{GroupId: new("sg-0abc1234"), GroupName: new("web-sg")},
		},
		TagSet: []ec2types.Tag{
			{Key: new("Name"), Value: new("prod-eni")},
		},
	}
}

func realisticDBISnapshot() rdstypes.DBSnapshot {
	return rdstypes.DBSnapshot{
		DBSnapshotIdentifier: new("dbi-snap-prod-20250615"),
		DBInstanceIdentifier: new("prod-db-01"),
		Status:               new("available"),
		Engine:               new("mysql"),
		EngineVersion:        new("8.0.35"),
		SnapshotType:         new("automated"),
		SnapshotCreateTime:   new(testTime),
		AllocatedStorage:     new(int32(100)),
	}
}

func realisticDBCSnapshot() docdbtypes.DBClusterSnapshot {
	return docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: new("dbc-snap-prod-20250615"),
		DBClusterIdentifier:         new("docdb-prod-cluster"),
		Status:                      new("available"),
		Engine:                      new("docdb"),
		SnapshotType:                new("automated"),
		SnapshotCreateTime:          new(testTime),
	}
}

func realisticSNSSubscription() snstypes.Subscription {
	return snstypes.Subscription{
		SubscriptionArn: new("arn:aws:sns:us-east-1:123456789012:alerts:a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"),
		TopicArn:        new("arn:aws:sns:us-east-1:123456789012:alerts"),
		Protocol:        new("email"),
		Endpoint:        new("user@example.com"),
		Owner:           new("123456789012"),
	}
}

// ===========================================================================
// 1. VPC
// ===========================================================================

var _ = time.Now
