package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["sg"] = sgFixtures
	demoData["subnet"] = subnetFixtures
}

// ---------------------------------------------------------------------------
// Security Groups (ec2types.SecurityGroup)
// Fields: group_id, group_name, vpc_id, description
// ---------------------------------------------------------------------------

func sgFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "sg-0aaa111111111111a",
			Name:   "acme-web-alb-sg",
			Status: "",
			Fields: map[string]string{
				"group_id":    "sg-0aaa111111111111a",
				"group_name":  "acme-web-alb-sg",
				"vpc_id":      prodVPCID,
				"description": "Security group for production web ALB",
			},
			RawStruct: ec2types.SecurityGroup{
				GroupId:     aws.String("sg-0aaa111111111111a"),
				GroupName:   aws.String("acme-web-alb-sg"),
				VpcId:       aws.String(prodVPCID),
				Description: aws.String("Security group for production web ALB"),
				OwnerId:     aws.String("123456789012"),
				IpPermissions: []ec2types.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(443),
						ToPort:     aws.Int32(443),
						IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("HTTPS from anywhere")}},
					},
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(80),
						ToPort:     aws.Int32(80),
						IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("HTTP from anywhere (redirect)")}},
					},
				},
				IpPermissionsEgress: []ec2types.IpPermission{
					{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-web-alb-sg")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "sg-0bbb222222222222b",
			Name:   "acme-api-internal-sg",
			Status: "",
			Fields: map[string]string{
				"group_id":    "sg-0bbb222222222222b",
				"group_name":  "acme-api-internal-sg",
				"vpc_id":      prodVPCID,
				"description": "Internal API service security group",
			},
			RawStruct: ec2types.SecurityGroup{
				GroupId:     aws.String("sg-0bbb222222222222b"),
				GroupName:   aws.String("acme-api-internal-sg"),
				VpcId:       aws.String(prodVPCID),
				Description: aws.String("Internal API service security group"),
				OwnerId:     aws.String("123456789012"),
				IpPermissions: []ec2types.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(8080),
						ToPort:     aws.Int32(8080),
						IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("API traffic from VPC")}},
					},
				},
				IpPermissionsEgress: []ec2types.IpPermission{
					{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-api-internal-sg")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "sg-0ccc333333333333c",
			Name:   "acme-rds-sg",
			Status: "",
			Fields: map[string]string{
				"group_id":    "sg-0ccc333333333333c",
				"group_name":  "acme-rds-sg",
				"vpc_id":      prodVPCID,
				"description": "RDS PostgreSQL access from app tier",
			},
			RawStruct: ec2types.SecurityGroup{
				GroupId:     aws.String("sg-0ccc333333333333c"),
				GroupName:   aws.String("acme-rds-sg"),
				VpcId:       aws.String(prodVPCID),
				Description: aws.String("RDS PostgreSQL access from app tier"),
				OwnerId:     aws.String("123456789012"),
				IpPermissions: []ec2types.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(5432),
						ToPort:     aws.Int32(5432),
						IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("PostgreSQL from VPC")}},
					},
				},
				IpPermissionsEgress: []ec2types.IpPermission{
					{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-rds-sg")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "sg-0ddd444444444444d",
			Name:   "acme-bastion-sg",
			Status: "",
			Fields: map[string]string{
				"group_id":    "sg-0ddd444444444444d",
				"group_name":  "acme-bastion-sg",
				"vpc_id":      prodVPCID,
				"description": "Bastion host SSH access",
			},
			RawStruct: ec2types.SecurityGroup{
				GroupId:     aws.String("sg-0ddd444444444444d"),
				GroupName:   aws.String("acme-bastion-sg"),
				VpcId:       aws.String(prodVPCID),
				Description: aws.String("Bastion host SSH access"),
				OwnerId:     aws.String("123456789012"),
				IpPermissions: []ec2types.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(22),
						ToPort:     aws.Int32(22),
						IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("203.0.113.0/24"), Description: aws.String("Office VPN")}},
					},
				},
				IpPermissionsEgress: []ec2types.IpPermission{
					{IpProtocol: aws.String("-1"), IpRanges: []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}}},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-bastion-sg")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "sg-0fff888888888888f",
			Name:   "staging-default-sg",
			Status: "",
			Fields: map[string]string{
				"group_id":    "sg-0fff888888888888f",
				"group_name":  "staging-default-sg",
				"vpc_id":      stagingVPCID,
				"description": "Default staging VPC security group",
			},
			RawStruct: ec2types.SecurityGroup{
				GroupId:     aws.String("sg-0fff888888888888f"),
				GroupName:   aws.String("staging-default-sg"),
				VpcId:       aws.String(stagingVPCID),
				Description: aws.String("Default staging VPC security group"),
				OwnerId:     aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-default-sg")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Subnets (ec2types.Subnet)
// Fields: subnet_id, name, vpc_id, cidr_block, availability_zone, state, available_ips
// ---------------------------------------------------------------------------

func subnetFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     prodPublicSubnetA,
			Name:   "prod-public-1a",
			Status: "available",
			Fields: map[string]string{
				"subnet_id":         prodPublicSubnetA,
				"name":              "prod-public-1a",
				"vpc_id":            prodVPCID,
				"cidr_block":        "10.0.1.0/24",
				"availability_zone": "us-east-1a",
				"state":             "available",
				"available_ips":     "243",
			},
			RawStruct: ec2types.Subnet{
				SubnetId:                aws.String(prodPublicSubnetA),
				VpcId:                   aws.String(prodVPCID),
				CidrBlock:              aws.String("10.0.1.0/24"),
				AvailabilityZone:        aws.String("us-east-1a"),
				AvailabilityZoneId:      aws.String("use1-az1"),
				State:                   ec2types.SubnetStateAvailable,
				AvailableIpAddressCount: aws.Int32(243),
				MapPublicIpOnLaunch:     aws.Bool(true),
				DefaultForAz:            aws.Bool(false),
				SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + prodPublicSubnetA),
				OwnerId:                 aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-public-1a")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Tier"), Value: aws.String("public")},
				},
			},
		},
		{
			ID:     prodPublicSubnetB,
			Name:   "prod-public-1b",
			Status: "available",
			Fields: map[string]string{
				"subnet_id":         prodPublicSubnetB,
				"name":              "prod-public-1b",
				"vpc_id":            prodVPCID,
				"cidr_block":        "10.0.2.0/24",
				"availability_zone": "us-east-1b",
				"state":             "available",
				"available_ips":     "248",
			},
			RawStruct: ec2types.Subnet{
				SubnetId:                aws.String(prodPublicSubnetB),
				VpcId:                   aws.String(prodVPCID),
				CidrBlock:              aws.String("10.0.2.0/24"),
				AvailabilityZone:        aws.String("us-east-1b"),
				AvailabilityZoneId:      aws.String("use1-az2"),
				State:                   ec2types.SubnetStateAvailable,
				AvailableIpAddressCount: aws.Int32(248),
				MapPublicIpOnLaunch:     aws.Bool(true),
				DefaultForAz:            aws.Bool(false),
				SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + prodPublicSubnetB),
				OwnerId:                 aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-public-1b")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Tier"), Value: aws.String("public")},
				},
			},
		},
		{
			ID:     prodPrivateSubnetA,
			Name:   "prod-private-1a",
			Status: "available",
			Fields: map[string]string{
				"subnet_id":         prodPrivateSubnetA,
				"name":              "prod-private-1a",
				"vpc_id":            prodVPCID,
				"cidr_block":        "10.0.3.0/24",
				"availability_zone": "us-east-1a",
				"state":             "available",
				"available_ips":     "230",
			},
			RawStruct: ec2types.Subnet{
				SubnetId:                aws.String(prodPrivateSubnetA),
				VpcId:                   aws.String(prodVPCID),
				CidrBlock:              aws.String("10.0.3.0/24"),
				AvailabilityZone:        aws.String("us-east-1a"),
				AvailabilityZoneId:      aws.String("use1-az1"),
				State:                   ec2types.SubnetStateAvailable,
				AvailableIpAddressCount: aws.Int32(230),
				MapPublicIpOnLaunch:     aws.Bool(false),
				DefaultForAz:            aws.Bool(false),
				SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + prodPrivateSubnetA),
				OwnerId:                 aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-private-1a")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Tier"), Value: aws.String("private")},
				},
			},
		},
		{
			ID:     prodPrivateSubnetB,
			Name:   "prod-private-1b",
			Status: "available",
			Fields: map[string]string{
				"subnet_id":         prodPrivateSubnetB,
				"name":              "prod-private-1b",
				"vpc_id":            prodVPCID,
				"cidr_block":        "10.0.4.0/24",
				"availability_zone": "us-east-1b",
				"state":             "available",
				"available_ips":     "250",
			},
			RawStruct: ec2types.Subnet{
				SubnetId:                aws.String(prodPrivateSubnetB),
				VpcId:                   aws.String(prodVPCID),
				CidrBlock:              aws.String("10.0.4.0/24"),
				AvailabilityZone:        aws.String("us-east-1b"),
				AvailabilityZoneId:      aws.String("use1-az2"),
				State:                   ec2types.SubnetStateAvailable,
				AvailableIpAddressCount: aws.Int32(250),
				MapPublicIpOnLaunch:     aws.Bool(false),
				DefaultForAz:            aws.Bool(false),
				SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + prodPrivateSubnetB),
				OwnerId:                 aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-private-1b")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Tier"), Value: aws.String("private")},
				},
			},
		},
		{
			ID:     stagingSubnetA,
			Name:   "staging-1a",
			Status: "available",
			Fields: map[string]string{
				"subnet_id":         stagingSubnetA,
				"name":              "staging-1a",
				"vpc_id":            stagingVPCID,
				"cidr_block":        "10.1.1.0/24",
				"availability_zone": "us-east-1a",
				"state":             "available",
				"available_ips":     "251",
			},
			RawStruct: ec2types.Subnet{
				SubnetId:                aws.String(stagingSubnetA),
				VpcId:                   aws.String(stagingVPCID),
				CidrBlock:              aws.String("10.1.1.0/24"),
				AvailabilityZone:        aws.String("us-east-1a"),
				AvailabilityZoneId:      aws.String("use1-az1"),
				State:                   ec2types.SubnetStateAvailable,
				AvailableIpAddressCount: aws.Int32(251),
				MapPublicIpOnLaunch:     aws.Bool(true),
				DefaultForAz:            aws.Bool(false),
				SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/" + stagingSubnetA),
				OwnerId:                 aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-1a")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}
