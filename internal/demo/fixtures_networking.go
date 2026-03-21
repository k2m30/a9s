package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/internal/resource"
)

// Consistent VPC IDs for cross-type references (Acme Corp Production scenario).
const (
	prodVPCID    = "vpc-0abc123def456789a"
	stagingVPCID = "vpc-0def456789abc123d"
)

// Consistent subnet IDs referenced by other networking resources.
const (
	prodPublicSubnetA  = "subnet-0aaa111111111111a"
	prodPublicSubnetB  = "subnet-0bbb222222222222b"
	prodPrivateSubnetA = "subnet-0ccc333333333333c"
	prodPrivateSubnetB = "subnet-0ddd444444444444d"
	stagingSubnetA     = "subnet-0eee555555555555e"
	stagingSubnetB     = "subnet-0fff666666666666f"
)

func init() {
	demoData["elb"] = elbFixtures
	demoData["tg"] = tgFixtures
	demoData["sg"] = sgFixtures
	demoData["vpc"] = vpcFixtures
	demoData["subnet"] = subnetFixtures
	demoData["rtb"] = rtbFixtures
	demoData["nat"] = natFixtures
	demoData["igw"] = igwFixtures
	demoData["eip"] = eipFixtures
	demoData["vpce"] = vpceFixtures
	demoData["tgw"] = tgwFixtures
	demoData["eni"] = eniFixtures
}

// ---------------------------------------------------------------------------
// ELB (elbv2types.LoadBalancer)
// Fields: name, dns_name, type, scheme, state, vpc_id
// ---------------------------------------------------------------------------

func elbFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-prod-web",
			Name:   "acme-prod-web",
			Status: "active",
			Fields: map[string]string{
				"name":     "acme-prod-web",
				"dns_name": "acme-prod-web-1234567890.us-east-1.elb.amazonaws.com",
				"type":     "application",
				"scheme":   "internet-facing",
				"state":    "active",
				"vpc_id":   prodVPCID,
			},
			RawStruct: elbv2types.LoadBalancer{
				LoadBalancerName: aws.String("acme-prod-web"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"),
				DNSName:          aws.String("acme-prod-web-1234567890.us-east-1.elb.amazonaws.com"),
				Type:             elbv2types.LoadBalancerTypeEnumApplication,
				Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumActive,
				},
				VpcId:                 aws.String(prodVPCID),
				CanonicalHostedZoneId: aws.String("Z35SXDOTRQ7X7K"),
				IpAddressType:         elbv2types.IpAddressTypeIpv4,
				SecurityGroups:        []string{"sg-0aaa111111111111a"},
				CreatedTime:           aws.Time(mustParseTime("2025-06-15T10:30:00+00:00")),
				AvailabilityZones: []elbv2types.AvailabilityZone{
					{SubnetId: aws.String(prodPublicSubnetA), ZoneName: aws.String("us-east-1a")},
					{SubnetId: aws.String(prodPublicSubnetB), ZoneName: aws.String("us-east-1b")},
				},
			},
		},
		{
			ID:     "acme-internal-api",
			Name:   "acme-internal-api",
			Status: "active",
			Fields: map[string]string{
				"name":     "acme-internal-api",
				"dns_name": "internal-acme-api-0987654321.us-east-1.elb.amazonaws.com",
				"type":     "application",
				"scheme":   "internal",
				"state":    "active",
				"vpc_id":   prodVPCID,
			},
			RawStruct: elbv2types.LoadBalancer{
				LoadBalancerName: aws.String("acme-internal-api"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba"),
				DNSName:          aws.String("internal-acme-api-0987654321.us-east-1.elb.amazonaws.com"),
				Type:             elbv2types.LoadBalancerTypeEnumApplication,
				Scheme:           elbv2types.LoadBalancerSchemeEnumInternal,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumActive,
				},
				VpcId:                 aws.String(prodVPCID),
				CanonicalHostedZoneId: aws.String("Z35SXDOTRQ7X7K"),
				IpAddressType:         elbv2types.IpAddressTypeIpv4,
				SecurityGroups:        []string{"sg-0bbb222222222222b"},
				CreatedTime:           aws.Time(mustParseTime("2025-08-20T14:00:00+00:00")),
				AvailabilityZones: []elbv2types.AvailabilityZone{
					{SubnetId: aws.String(prodPrivateSubnetA), ZoneName: aws.String("us-east-1a")},
					{SubnetId: aws.String(prodPrivateSubnetB), ZoneName: aws.String("us-east-1b")},
				},
			},
		},
		{
			ID:     "acme-prod-nlb",
			Name:   "acme-prod-nlb",
			Status: "active",
			Fields: map[string]string{
				"name":     "acme-prod-nlb",
				"dns_name": "acme-prod-nlb-abcdef1234.us-east-1.elb.amazonaws.com",
				"type":     "network",
				"scheme":   "internet-facing",
				"state":    "active",
				"vpc_id":   prodVPCID,
			},
			RawStruct: elbv2types.LoadBalancer{
				LoadBalancerName: aws.String("acme-prod-nlb"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/acme-prod-nlb/abcdef1234567890"),
				DNSName:          aws.String("acme-prod-nlb-abcdef1234.us-east-1.elb.amazonaws.com"),
				Type:             elbv2types.LoadBalancerTypeEnumNetwork,
				Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumActive,
				},
				VpcId:                 aws.String(prodVPCID),
				CanonicalHostedZoneId: aws.String("Z26RNL4JYFTOTI"),
				IpAddressType:         elbv2types.IpAddressTypeIpv4,
				CreatedTime:           aws.Time(mustParseTime("2025-09-10T09:00:00+00:00")),
				AvailabilityZones: []elbv2types.AvailabilityZone{
					{SubnetId: aws.String(prodPublicSubnetA), ZoneName: aws.String("us-east-1a")},
					{SubnetId: aws.String(prodPublicSubnetB), ZoneName: aws.String("us-east-1b")},
				},
			},
		},
		{
			ID:     "staging-web-alb",
			Name:   "staging-web-alb",
			Status: "provisioning",
			Fields: map[string]string{
				"name":     "staging-web-alb",
				"dns_name": "staging-web-alb-5555555555.us-east-1.elb.amazonaws.com",
				"type":     "application",
				"scheme":   "internet-facing",
				"state":    "provisioning",
				"vpc_id":   stagingVPCID,
			},
			RawStruct: elbv2types.LoadBalancer{
				LoadBalancerName: aws.String("staging-web-alb"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/staging-web-alb/5555555555aaaaaa"),
				DNSName:          aws.String("staging-web-alb-5555555555.us-east-1.elb.amazonaws.com"),
				Type:             elbv2types.LoadBalancerTypeEnumApplication,
				Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumProvisioning,
				},
				VpcId:                 aws.String(stagingVPCID),
				CanonicalHostedZoneId: aws.String("Z35SXDOTRQ7X7K"),
				IpAddressType:         elbv2types.IpAddressTypeIpv4,
				SecurityGroups:        []string{"sg-0fff888888888888f"},
				CreatedTime:           aws.Time(mustParseTime("2026-03-21T08:00:00+00:00")),
				AvailabilityZones: []elbv2types.AvailabilityZone{
					{SubnetId: aws.String(stagingSubnetA), ZoneName: aws.String("us-east-1a")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Target Groups (elbv2types.TargetGroup)
// Fields: target_group_name, port, protocol, vpc_id, target_type, health_check_path
// ---------------------------------------------------------------------------

func tgFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-web-tg",
			Name:   "acme-web-tg",
			Status: "",
			Fields: map[string]string{
				"target_group_name": "acme-web-tg",
				"port":              "443",
				"protocol":          "HTTPS",
				"vpc_id":            prodVPCID,
				"target_type":       "instance",
				"health_check_path": "/healthz",
			},
			RawStruct: elbv2types.TargetGroup{
				TargetGroupName:            aws.String("acme-web-tg"),
				TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"),
				Port:                       aws.Int32(443),
				Protocol:                   elbv2types.ProtocolEnumHttps,
				ProtocolVersion:            aws.String("HTTP2"),
				VpcId:                      aws.String(prodVPCID),
				TargetType:                 elbv2types.TargetTypeEnumInstance,
				HealthCheckPath:            aws.String("/healthz"),
				HealthCheckPort:            aws.String("443"),
				HealthCheckProtocol:        elbv2types.ProtocolEnumHttps,
				HealthCheckEnabled:         aws.Bool(true),
				HealthCheckIntervalSeconds: aws.Int32(30),
				HealthCheckTimeoutSeconds:  aws.Int32(5),
				HealthyThresholdCount:      aws.Int32(3),
				UnhealthyThresholdCount:    aws.Int32(3),
				Matcher:                    &elbv2types.Matcher{HttpCode: aws.String("200")},
				LoadBalancerArns: []string{
					"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef",
				},
			},
		},
		{
			ID:     "acme-api-tg",
			Name:   "acme-api-tg",
			Status: "",
			Fields: map[string]string{
				"target_group_name": "acme-api-tg",
				"port":              "8080",
				"protocol":          "HTTP",
				"vpc_id":            prodVPCID,
				"target_type":       "ip",
				"health_check_path": "/api/health",
			},
			RawStruct: elbv2types.TargetGroup{
				TargetGroupName:            aws.String("acme-api-tg"),
				TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/0987654321fedcba"),
				Port:                       aws.Int32(8080),
				Protocol:                   elbv2types.ProtocolEnumHttp,
				ProtocolVersion:            aws.String("HTTP1"),
				VpcId:                      aws.String(prodVPCID),
				TargetType:                 elbv2types.TargetTypeEnumIp,
				HealthCheckPath:            aws.String("/api/health"),
				HealthCheckPort:            aws.String("8080"),
				HealthCheckProtocol:        elbv2types.ProtocolEnumHttp,
				HealthCheckEnabled:         aws.Bool(true),
				HealthCheckIntervalSeconds: aws.Int32(15),
				HealthCheckTimeoutSeconds:  aws.Int32(5),
				HealthyThresholdCount:      aws.Int32(2),
				UnhealthyThresholdCount:    aws.Int32(3),
				Matcher:                    &elbv2types.Matcher{HttpCode: aws.String("200-299")},
				LoadBalancerArns: []string{
					"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba",
				},
			},
		},
		{
			ID:     "acme-grpc-tg",
			Name:   "acme-grpc-tg",
			Status: "",
			Fields: map[string]string{
				"target_group_name": "acme-grpc-tg",
				"port":              "50051",
				"protocol":          "HTTP",
				"vpc_id":            prodVPCID,
				"target_type":       "ip",
				"health_check_path": "/grpc.health.v1.Health/Check",
			},
			RawStruct: elbv2types.TargetGroup{
				TargetGroupName:            aws.String("acme-grpc-tg"),
				TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-grpc-tg/aabbccddee112233"),
				Port:                       aws.Int32(50051),
				Protocol:                   elbv2types.ProtocolEnumHttp,
				ProtocolVersion:            aws.String("GRPC"),
				VpcId:                      aws.String(prodVPCID),
				TargetType:                 elbv2types.TargetTypeEnumIp,
				HealthCheckPath:            aws.String("/grpc.health.v1.Health/Check"),
				HealthCheckPort:            aws.String("50051"),
				HealthCheckProtocol:        elbv2types.ProtocolEnumHttp,
				HealthCheckEnabled:         aws.Bool(true),
				HealthCheckIntervalSeconds: aws.Int32(10),
				HealthCheckTimeoutSeconds:  aws.Int32(5),
				HealthyThresholdCount:      aws.Int32(2),
				UnhealthyThresholdCount:    aws.Int32(2),
				Matcher:                    &elbv2types.Matcher{GrpcCode: aws.String("0")},
			},
		},
		{
			ID:     "acme-nlb-tcp-tg",
			Name:   "acme-nlb-tcp-tg",
			Status: "",
			Fields: map[string]string{
				"target_group_name": "acme-nlb-tcp-tg",
				"port":              "6379",
				"protocol":          "TCP",
				"vpc_id":            prodVPCID,
				"target_type":       "instance",
				"health_check_path": "",
			},
			RawStruct: elbv2types.TargetGroup{
				TargetGroupName:            aws.String("acme-nlb-tcp-tg"),
				TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-nlb-tcp-tg/112233aabbccddee"),
				Port:                       aws.Int32(6379),
				Protocol:                   elbv2types.ProtocolEnumTcp,
				VpcId:                      aws.String(prodVPCID),
				TargetType:                 elbv2types.TargetTypeEnumInstance,
				HealthCheckPort:            aws.String("6379"),
				HealthCheckProtocol:        elbv2types.ProtocolEnumTcp,
				HealthCheckEnabled:         aws.Bool(true),
				HealthCheckIntervalSeconds: aws.Int32(30),
				HealthCheckTimeoutSeconds:  aws.Int32(10),
				HealthyThresholdCount:      aws.Int32(3),
				UnhealthyThresholdCount:    aws.Int32(3),
				LoadBalancerArns: []string{
					"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/acme-prod-nlb/abcdef1234567890",
				},
			},
		},
	}
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
// VPCs (ec2types.Vpc)
// Fields: vpc_id, name, cidr_block, state, is_default
// ---------------------------------------------------------------------------

func vpcFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     prodVPCID,
			Name:   "acme-prod",
			Status: "available",
			Fields: map[string]string{
				"vpc_id":     prodVPCID,
				"name":       "acme-prod",
				"cidr_block": "10.0.0.0/16",
				"state":      "available",
				"is_default": "false",
			},
			RawStruct: ec2types.Vpc{
				VpcId:           aws.String(prodVPCID),
				CidrBlock:       aws.String("10.0.0.0/16"),
				State:           ec2types.VpcStateAvailable,
				IsDefault:       aws.Bool(false),
				InstanceTenancy: ec2types.TenancyDefault,
				DhcpOptionsId:   aws.String("dopt-0abc123def456789a"),
				OwnerId:         aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-prod")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     stagingVPCID,
			Name:   "acme-staging",
			Status: "available",
			Fields: map[string]string{
				"vpc_id":     stagingVPCID,
				"name":       "acme-staging",
				"cidr_block": "10.1.0.0/16",
				"state":      "available",
				"is_default": "false",
			},
			RawStruct: ec2types.Vpc{
				VpcId:           aws.String(stagingVPCID),
				CidrBlock:       aws.String("10.1.0.0/16"),
				State:           ec2types.VpcStateAvailable,
				IsDefault:       aws.Bool(false),
				InstanceTenancy: ec2types.TenancyDefault,
				DhcpOptionsId:   aws.String("dopt-0def456789abc123d"),
				OwnerId:         aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-staging")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
		{
			ID:     "vpc-0default00000000",
			Name:   "default",
			Status: "available",
			Fields: map[string]string{
				"vpc_id":     "vpc-0default00000000",
				"name":       "default",
				"cidr_block": "172.31.0.0/16",
				"state":      "available",
				"is_default": "true",
			},
			RawStruct: ec2types.Vpc{
				VpcId:           aws.String("vpc-0default00000000"),
				CidrBlock:       aws.String("172.31.0.0/16"),
				State:           ec2types.VpcStateAvailable,
				IsDefault:       aws.Bool(true),
				InstanceTenancy: ec2types.TenancyDefault,
				DhcpOptionsId:   aws.String("dopt-0default0000000"),
				OwnerId:         aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("default")},
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
				"route_table_id":    "rtb-0aaa111111111111a",
				"name":              "prod-main",
				"vpc_id":            prodVPCID,
				"routes_count":      "2",
				"associations_count": "1",
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
				"route_table_id":    "rtb-0bbb222222222222b",
				"name":              "prod-public",
				"vpc_id":            prodVPCID,
				"routes_count":      "2",
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
				"route_table_id":    "rtb-0ccc333333333333c",
				"name":              "prod-private",
				"vpc_id":            prodVPCID,
				"routes_count":      "2",
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
				"route_table_id":    "rtb-0ddd444444444444d",
				"name":              "staging-main",
				"vpc_id":            stagingVPCID,
				"routes_count":      "2",
				"associations_count": "1",
			},
			RawStruct: ec2types.RouteTable{
				RouteTableId: aws.String("rtb-0ddd444444444444d"),
				VpcId:        aws.String(stagingVPCID),
				OwnerId:      aws.String("123456789012"),
				Routes: []ec2types.Route{
					{
						DestinationCidrBlock: aws.String("10.1.0.0/16"),
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
				},
				Associations: []ec2types.RouteTableAssociation{
					{
						Main:                    aws.Bool(true),
						RouteTableAssociationId: aws.String("rtbassoc-0fff666666666666f"),
						RouteTableId:            aws.String("rtb-0ddd444444444444d"),
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
		{
			ID:     "igw-0ccc333333333333c",
			Name:   "detached-igw",
			Status: "detached",
			Fields: map[string]string{
				"igw_id": "igw-0ccc333333333333c",
				"name":   "detached-igw",
				"vpc_id": "",
				"state":  "detached",
			},
			RawStruct: ec2types.InternetGateway{
				InternetGatewayId: aws.String("igw-0ccc333333333333c"),
				OwnerId:           aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("detached-igw")},
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
				Domain:             ec2types.DomainTypeVpc,
				NetworkBorderGroup: aws.String("us-east-1"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("unassociated-eip")},
				},
			},
		},
	}
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
				VpcEndpointId:   aws.String("vpce-0aaa111111111111a"),
				ServiceName:     aws.String("com.amazonaws.us-east-1.s3"),
				VpcEndpointType: ec2types.VpcEndpointTypeGateway,
				State:           ec2types.StateAvailable,
				VpcId:           aws.String(prodVPCID),
				RouteTableIds:   []string{"rtb-0aaa111111111111a", "rtb-0ccc333333333333c"},
				OwnerId:         aws.String("123456789012"),
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
				RequesterManaged:   aws.Bool(true),
				SourceDestCheck:    aws.Bool(false),
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
				Groups: []ec2types.GroupIdentifier{
					{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-web-alb-sg")},
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
				TagSet: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("vpce-secrets-eni-1a")},
				},
			},
		},
		{
			ID:     "eni-0ggg777777777777g",
			Name:   "",
			Status: "available",
			Fields: map[string]string{
				"eni_id":     "eni-0ggg777777777777g",
				"name":       "",
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
			},
		},
	}
}
