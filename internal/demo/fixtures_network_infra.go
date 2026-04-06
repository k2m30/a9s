package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["rtb"] = rtbFixtures
	demoData["nat"] = natFixtures
	demoData["igw"] = igwFixtures
	demoData["eip"] = eipFixtures
}

// ---------------------------------------------------------------------------
// Route Tables (ec2types.RouteTable)
// Fields: route_table_id, name, vpc_id, routes_count, associations_count
// Status is set to isMain ("true"/"false") by the fetcher.
// ---------------------------------------------------------------------------

func rtbFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rtb-0aaa111111111111a",
			Name:   "prod-main",
			Status: "true",
			Fields: map[string]string{
				"route_table_id":     "rtb-0aaa111111111111a",
				"name":               "prod-main",
				"vpc_id":             prodVPCID,
				"routes_count":       "2",
				"associations_count": "2",
			},
			RawStruct: ec2types.RouteTable{
				RouteTableId: aws.String("rtb-0aaa111111111111a"),
				VpcId:        aws.String(prodVPCID),
				OwnerId:      aws.String("123456789012"),
				Routes: []ec2types.Route{
					{
						DestinationCidrBlock: aws.String("10.0.0.0/16"),
						GatewayId:            aws.String("local"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRouteTable,
					},
					{
						DestinationCidrBlock: aws.String("0.0.0.0/0"),
						NatGatewayId:         aws.String("nat-0aaa111111111111a"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRoute,
					},
				},
				Associations: []ec2types.RouteTableAssociation{
					{
						Main:                    aws.Bool(true),
						RouteTableAssociationId: aws.String("rtbassoc-0aaa111111111111a"),
						RouteTableId:            aws.String("rtb-0aaa111111111111a"),
					},
					{
						Main:                    aws.Bool(false),
						RouteTableAssociationId: aws.String("rtbassoc-0aaa222222222222a"),
						RouteTableId:            aws.String("rtb-0aaa111111111111a"),
						SubnetId:                aws.String(prodPrivateSubnetA),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-main")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "rtb-0bbb222222222222b",
			Name:   "prod-public",
			Status: "false",
			Fields: map[string]string{
				"route_table_id":     "rtb-0bbb222222222222b",
				"name":               "prod-public",
				"vpc_id":             prodVPCID,
				"routes_count":       "3",
				"associations_count": "2",
			},
			RawStruct: ec2types.RouteTable{
				RouteTableId: aws.String("rtb-0bbb222222222222b"),
				VpcId:        aws.String(prodVPCID),
				OwnerId:      aws.String("123456789012"),
				Routes: []ec2types.Route{
					{
						DestinationCidrBlock: aws.String("10.0.0.0/16"),
						GatewayId:            aws.String("local"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRouteTable,
					},
					{
						DestinationCidrBlock: aws.String("0.0.0.0/0"),
						GatewayId:            aws.String("igw-0aaa111111111111a"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRoute,
					},
					{
						DestinationCidrBlock: aws.String("10.1.0.0/16"),
						NatGatewayId:         aws.String("nat-0aaa111111111111a"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRoute,
					},
				},
				Associations: []ec2types.RouteTableAssociation{
					{
						Main:                    aws.Bool(false),
						RouteTableAssociationId: aws.String("rtbassoc-0bbb222222222222b"),
						RouteTableId:            aws.String("rtb-0bbb222222222222b"),
						SubnetId:                aws.String(prodPublicSubnetA),
					},
					{
						Main:                    aws.Bool(false),
						RouteTableAssociationId: aws.String("rtbassoc-0ccc333333333333c"),
						RouteTableId:            aws.String("rtb-0bbb222222222222b"),
						SubnetId:                aws.String(prodPublicSubnetB),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-public")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "rtb-0ccc333333333333c",
			Name:   "prod-private",
			Status: "false",
			Fields: map[string]string{
				"route_table_id":     "rtb-0ccc333333333333c",
				"name":               "prod-private",
				"vpc_id":             prodVPCID,
				"routes_count":       "2",
				"associations_count": "2",
			},
			RawStruct: ec2types.RouteTable{
				RouteTableId: aws.String("rtb-0ccc333333333333c"),
				VpcId:        aws.String(prodVPCID),
				OwnerId:      aws.String("123456789012"),
				Routes: []ec2types.Route{
					{
						DestinationCidrBlock: aws.String("10.0.0.0/16"),
						GatewayId:            aws.String("local"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRouteTable,
					},
					{
						DestinationCidrBlock: aws.String("0.0.0.0/0"),
						NatGatewayId:         aws.String("nat-0aaa111111111111a"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRoute,
					},
				},
				Associations: []ec2types.RouteTableAssociation{
					{
						Main:                    aws.Bool(false),
						RouteTableAssociationId: aws.String("rtbassoc-0ddd444444444444d"),
						RouteTableId:            aws.String("rtb-0ccc333333333333c"),
						SubnetId:                aws.String(prodPrivateSubnetA),
					},
					{
						Main:                    aws.Bool(false),
						RouteTableAssociationId: aws.String("rtbassoc-0eee555555555555e"),
						RouteTableId:            aws.String("rtb-0ccc333333333333c"),
						SubnetId:                aws.String(prodPrivateSubnetB),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-private")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "rtb-0ddd444444444444d",
			Name:   "staging-main",
			Status: "true",
			Fields: map[string]string{
				"route_table_id":     "rtb-0ddd444444444444d",
				"name":               "staging-main",
				"vpc_id":             stagingVPCID,
				"routes_count":       "3",
				"associations_count": "2",
			},
			RawStruct: ec2types.RouteTable{
				RouteTableId: aws.String("rtb-0ddd444444444444d"),
				VpcId:        aws.String(stagingVPCID),
				OwnerId:      aws.String("123456789012"),
				Routes: []ec2types.Route{
					{
						DestinationCidrBlock: aws.String("10.2.0.0/16"),
						GatewayId:            aws.String("local"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRouteTable,
					},
					{
						DestinationCidrBlock: aws.String("0.0.0.0/0"),
						GatewayId:            aws.String("igw-0bbb222222222222b"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRoute,
					},
					{
						DestinationCidrBlock: aws.String("10.2.0.0/16"),
						NatGatewayId:         aws.String("nat-0ccc333333333333c"),
						State:                ec2types.RouteStateActive,
						Origin:               ec2types.RouteOriginCreateRoute,
					},
				},
				Associations: []ec2types.RouteTableAssociation{
					{
						Main:                    aws.Bool(true),
						RouteTableAssociationId: aws.String("rtbassoc-0fff666666666666f"),
						RouteTableId:            aws.String("rtb-0ddd444444444444d"),
					},
					{
						Main:                    aws.Bool(false),
						RouteTableAssociationId: aws.String("rtbassoc-0ggg777777777777g"),
						RouteTableId:            aws.String("rtb-0ddd444444444444d"),
						SubnetId:                aws.String(stagingSubnetA),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-main")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// NAT Gateways (ec2types.NatGateway)
// Fields: nat_gateway_id, name, vpc_id, subnet_id, state, public_ip
// ---------------------------------------------------------------------------

func natFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "nat-0aaa111111111111a",
			Name:   "prod-nat-1a",
			Status: "available",
			Fields: map[string]string{
				"nat_gateway_id": "nat-0aaa111111111111a",
				"name":           "prod-nat-1a",
				"vpc_id":         prodVPCID,
				"subnet_id":      prodPublicSubnetA,
				"state":          "available",
				"public_ip":      "54.210.33.200",
			},
			RawStruct: ec2types.NatGateway{
				NatGatewayId:     aws.String("nat-0aaa111111111111a"),
				VpcId:            aws.String(prodVPCID),
				SubnetId:         aws.String(prodPublicSubnetA),
				State:            ec2types.NatGatewayStateAvailable,
				ConnectivityType: ec2types.ConnectivityTypePublic,
				CreateTime:       aws.Time(mustParseTime("2025-06-01T10:00:00+00:00")),
				FailureCode:      aws.String("Gateway.NotAttached"),
				FailureMessage:   aws.String("Network vpc-0abc123def456789a has no Internet gateway attached"),
				NatGatewayAddresses: []ec2types.NatGatewayAddress{
					{
						AllocationId: aws.String("eipalloc-0aaa111111111111a"),
						PublicIp:     aws.String("54.210.33.200"),
						PrivateIp:    aws.String("10.0.1.50"),
						IsPrimary:    aws.Bool(true),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat-1a")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "nat-0bbb222222222222b",
			Name:   "prod-nat-1b",
			Status: "available",
			Fields: map[string]string{
				"nat_gateway_id": "nat-0bbb222222222222b",
				"name":           "prod-nat-1b",
				"vpc_id":         prodVPCID,
				"subnet_id":      prodPublicSubnetB,
				"state":          "available",
				"public_ip":      "54.210.33.201",
			},
			RawStruct: ec2types.NatGateway{
				NatGatewayId:     aws.String("nat-0bbb222222222222b"),
				VpcId:            aws.String(prodVPCID),
				SubnetId:         aws.String(prodPublicSubnetB),
				State:            ec2types.NatGatewayStateAvailable,
				ConnectivityType: ec2types.ConnectivityTypePublic,
				CreateTime:       aws.Time(mustParseTime("2025-06-01T10:05:00+00:00")),
				NatGatewayAddresses: []ec2types.NatGatewayAddress{
					{
						AllocationId: aws.String("eipalloc-0bbb222222222222b"),
						PublicIp:     aws.String("54.210.33.201"),
						PrivateIp:    aws.String("10.0.2.50"),
						IsPrimary:    aws.Bool(true),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat-1b")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "nat-0ccc333333333333c",
			Name:   "staging-nat",
			Status: "deleting",
			Fields: map[string]string{
				"nat_gateway_id": "nat-0ccc333333333333c",
				"name":           "staging-nat",
				"vpc_id":         stagingVPCID,
				"subnet_id":      stagingSubnetA,
				"state":          "deleting",
				"public_ip":      "52.87.100.10",
			},
			RawStruct: ec2types.NatGateway{
				NatGatewayId:     aws.String("nat-0ccc333333333333c"),
				VpcId:            aws.String(stagingVPCID),
				SubnetId:         aws.String(stagingSubnetA),
				State:            ec2types.NatGatewayStateDeleting,
				ConnectivityType: ec2types.ConnectivityTypePublic,
				CreateTime:       aws.Time(mustParseTime("2025-11-15T08:00:00+00:00")),
				NatGatewayAddresses: []ec2types.NatGatewayAddress{
					{
						AllocationId: aws.String("eipalloc-0ccc333333333333c"),
						PublicIp:     aws.String("52.87.100.10"),
						PrivateIp:    aws.String("10.1.1.50"),
						IsPrimary:    aws.Bool(true),
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-nat")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Internet Gateways (ec2types.InternetGateway)
// Fields: igw_id, name, vpc_id, state
// ---------------------------------------------------------------------------

func igwFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "igw-0aaa111111111111a",
			Name:   "prod-igw",
			Status: "attached",
			Fields: map[string]string{
				"igw_id": "igw-0aaa111111111111a",
				"name":   "prod-igw",
				"vpc_id": prodVPCID,
				"state":  "attached",
			},
			RawStruct: ec2types.InternetGateway{
				InternetGatewayId: aws.String("igw-0aaa111111111111a"),
				OwnerId:           aws.String("123456789012"),
				Attachments: []ec2types.InternetGatewayAttachment{
					{
						VpcId: aws.String(prodVPCID),
						State: ec2types.AttachmentStatusAttached,
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-igw")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "igw-0bbb222222222222b",
			Name:   "staging-igw",
			Status: "attached",
			Fields: map[string]string{
				"igw_id": "igw-0bbb222222222222b",
				"name":   "staging-igw",
				"vpc_id": stagingVPCID,
				"state":  "attached",
			},
			RawStruct: ec2types.InternetGateway{
				InternetGatewayId: aws.String("igw-0bbb222222222222b"),
				OwnerId:           aws.String("123456789012"),
				Attachments: []ec2types.InternetGatewayAttachment{
					{
						VpcId: aws.String(stagingVPCID),
						State: ec2types.AttachmentStatusAttached,
					},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-igw")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Elastic IPs (ec2types.Address)
// Fields: allocation_id, name, public_ip, association_id, instance_id, domain
// ---------------------------------------------------------------------------

func eipFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "eipalloc-0aaa111111111111a",
			Name:   "prod-nat-eip-1a",
			Status: "vpc",
			Fields: map[string]string{
				"allocation_id":  "eipalloc-0aaa111111111111a",
				"name":           "prod-nat-eip-1a",
				"public_ip":      "54.210.33.200",
				"association_id": "eipassoc-0aaa111111111111a",
				"instance_id":    "",
				"domain":         "vpc",
			},
			RawStruct: ec2types.Address{
				AllocationId:       aws.String("eipalloc-0aaa111111111111a"),
				PublicIp:           aws.String("54.210.33.200"),
				AssociationId:      aws.String("eipassoc-0aaa111111111111a"),
				InstanceId:         aws.String("i-0a1b2c3d4e5f60001"),
				SubnetId:           aws.String("subnet-0aaa111111111111a"),
				Domain:             ec2types.DomainTypeVpc,
				NetworkBorderGroup: aws.String("us-east-1"),
				NetworkInterfaceId: aws.String("eni-0aaa111111111111a"),
				PrivateIpAddress:   aws.String("10.0.1.50"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat-eip-1a")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "eipalloc-0bbb222222222222b",
			Name:   "prod-nat-eip-1b",
			Status: "vpc",
			Fields: map[string]string{
				"allocation_id":  "eipalloc-0bbb222222222222b",
				"name":           "prod-nat-eip-1b",
				"public_ip":      "54.210.33.201",
				"association_id": "eipassoc-0bbb222222222222b",
				"instance_id":    "",
				"domain":         "vpc",
			},
			RawStruct: ec2types.Address{
				AllocationId:       aws.String("eipalloc-0bbb222222222222b"),
				PublicIp:           aws.String("54.210.33.201"),
				AssociationId:      aws.String("eipassoc-0bbb222222222222b"),
				InstanceId:         aws.String("i-0a1b2c3d4e5f60002"),
				Domain:             ec2types.DomainTypeVpc,
				NetworkBorderGroup: aws.String("us-east-1"),
				NetworkInterfaceId: aws.String("eni-0bbb222222222222b"),
				PrivateIpAddress:   aws.String("10.0.2.50"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat-eip-1b")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "eipalloc-0ddd444444444444d",
			Name:   "bastion-eip",
			Status: "vpc",
			Fields: map[string]string{
				"allocation_id":  "eipalloc-0ddd444444444444d",
				"name":           "bastion-eip",
				"public_ip":      "52.87.221.44",
				"association_id": "eipassoc-0ddd444444444444d",
				"instance_id":    "i-0a1b2c3d4e5f60005",
				"domain":         "vpc",
			},
			RawStruct: ec2types.Address{
				AllocationId:       aws.String("eipalloc-0ddd444444444444d"),
				PublicIp:           aws.String("52.87.221.44"),
				AssociationId:      aws.String("eipassoc-0ddd444444444444d"),
				InstanceId:         aws.String("i-0a1b2c3d4e5f60005"),
				NetworkInterfaceId: aws.String("eni-0eee555555555555e"),
				Domain:             ec2types.DomainTypeVpc,
				NetworkBorderGroup: aws.String("us-east-1"),
				PrivateIpAddress:   aws.String("10.0.0.5"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("bastion-eip")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "eipalloc-0ccc333333333333c",
			Name:   "staging-nat-eip",
			Status: "vpc",
			Fields: map[string]string{
				"allocation_id":  "eipalloc-0ccc333333333333c",
				"name":           "staging-nat-eip",
				"public_ip":      "52.87.100.10",
				"association_id": "eipassoc-0ccc333333333333c",
				"instance_id":    "i-0a1b2c3d4e5f60003",
				"domain":         "vpc",
			},
			RawStruct: ec2types.Address{
				AllocationId:       aws.String("eipalloc-0ccc333333333333c"),
				PublicIp:           aws.String("52.87.100.10"),
				AssociationId:      aws.String("eipassoc-0ccc333333333333c"),
				InstanceId:         aws.String("i-0a1b2c3d4e5f60003"),
				NetworkInterfaceId: aws.String("eni-0eee555555555555e"),
				Domain:             ec2types.DomainTypeVpc,
				NetworkBorderGroup: aws.String("us-east-1"),
				PrivateIpAddress:   aws.String("10.1.1.50"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-nat-eip")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
		{
			ID:     "eipalloc-0eee555555555555e",
			Name:   "unassociated-eip",
			Status: "vpc",
			Fields: map[string]string{
				"allocation_id":  "eipalloc-0eee555555555555e",
				"name":           "unassociated-eip",
				"public_ip":      "34.201.55.100",
				"association_id": "",
				"instance_id":    "",
				"domain":         "vpc",
			},
			RawStruct: ec2types.Address{
				AllocationId:       aws.String("eipalloc-0eee555555555555e"),
				PublicIp:           aws.String("34.201.55.100"),
				InstanceId:         aws.String("i-0a1b2c3d4e5f60003"),
				NetworkInterfaceId: aws.String("eni-0fff666666666666f"),
				Domain:             ec2types.DomainTypeVpc,
				NetworkBorderGroup: aws.String("us-east-1"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("unassociated-eip")},
				},
			},
		},
	}
}
