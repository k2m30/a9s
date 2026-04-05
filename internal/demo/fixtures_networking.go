package demo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["elb"] = elbFixtures
	demoData["tg"] = tgFixtures
	demoData["vpc"] = vpcFixtures

	RegisterChildDemo("elb_listeners", func(parentCtx map[string]string) []resource.Resource {
		return elbListenerFixtures(parentCtx["load_balancer_arn"])
	})
	RegisterChildDemo("elb_listener_rules", func(parentCtx map[string]string) []resource.Resource {
		return elbListenerRuleFixtures()
	})
}

// ---------------------------------------------------------------------------
// ELB (elbv2types.LoadBalancer)
// Fields: name, dns_name, type, scheme, state, vpc_id
// ---------------------------------------------------------------------------

func elbFixtures() []resource.Resource {
	elbs := []resource.Resource{
		{
			ID:     prodELBName,
			Name:   prodELBName,
			Status: "active",
			Fields: map[string]string{
				"name":              prodELBName,
				"dns_name":          prodELBDNS,
				"type":              "application",
				"scheme":            "internet-facing",
				"state":             "active",
				"vpc_id":            prodVPCID,
				"load_balancer_arn": prodELBARN,
			},
			RawStruct: elbv2types.LoadBalancer{
				LoadBalancerName: aws.String(prodELBName),
				LoadBalancerArn:  aws.String(prodELBARN),
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
				"name":              "acme-internal-api",
				"dns_name":          "internal-acme-api-0987654321.us-east-1.elb.amazonaws.com",
				"type":              "application",
				"scheme":            "internal",
				"state":             "active",
				"vpc_id":            prodVPCID,
				"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba",
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
				"name":              "acme-prod-nlb",
				"dns_name":          "acme-prod-nlb-abcdef1234.us-east-1.elb.amazonaws.com",
				"type":              "network",
				"scheme":            "internet-facing",
				"state":             "active",
				"vpc_id":            prodVPCID,
				"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/acme-prod-nlb/abcdef1234567890",
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
				"name":              "staging-web-alb",
				"dns_name":          "staging-web-alb-5555555555.us-east-1.elb.amazonaws.com",
				"type":              "application",
				"scheme":            "internet-facing",
				"state":             "provisioning",
				"vpc_id":            stagingVPCID,
				"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/staging-web-alb/5555555555aaaaaa",
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

	// Generate 18 more ELBs to reach 22 total
	elbTypes := []elbv2types.LoadBalancerTypeEnum{
		elbv2types.LoadBalancerTypeEnumApplication, elbv2types.LoadBalancerTypeEnumNetwork,
		elbv2types.LoadBalancerTypeEnumApplication, elbv2types.LoadBalancerTypeEnumApplication,
	}
	elbSchemes := []elbv2types.LoadBalancerSchemeEnum{
		elbv2types.LoadBalancerSchemeEnumInternetFacing, elbv2types.LoadBalancerSchemeEnumInternal,
	}
	for i := 0; i < 18; i++ {
		name := elbNamePool[i]
		lbType := elbTypes[i%len(elbTypes)]
		scheme := elbSchemes[i%len(elbSchemes)]
		typeStr := "application"
		if lbType == elbv2types.LoadBalancerTypeEnumNetwork {
			typeStr = "network"
		}
		schemeStr := "internet-facing"
		if scheme == elbv2types.LoadBalancerSchemeEnumInternal {
			schemeStr = "internal"
		}
		arnSuffix := fmt.Sprintf("%s/%016x", name, i+100)
		arn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/%s", arnSuffix)
		if lbType == elbv2types.LoadBalancerTypeEnumNetwork {
			arn = fmt.Sprintf("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/%s", arnSuffix)
		}
		dns := fmt.Sprintf("%s-%010d.us-east-1.elb.amazonaws.com", name, i+1000)
		vpcID := prodVPCID
		if i >= 14 {
			vpcID = stagingVPCID
		}
		createTime := fmt.Sprintf("2025-%02d-%02dT%02d:00:00+00:00", 1+(i%12), 1+i, 8+(i%12))
		elbs = append(elbs, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "active",
			Fields: map[string]string{
				"name":              name,
				"dns_name":          dns,
				"type":              typeStr,
				"scheme":            schemeStr,
				"state":             "active",
				"vpc_id":            vpcID,
				"load_balancer_arn": arn,
			},
			RawStruct: elbv2types.LoadBalancer{
				LoadBalancerName: aws.String(name),
				LoadBalancerArn:  aws.String(arn),
				DNSName:          aws.String(dns),
				Type:             lbType,
				Scheme:           scheme,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumActive,
				},
				VpcId:         aws.String(vpcID),
				IpAddressType: elbv2types.IpAddressTypeIpv4,
				CreatedTime:   aws.Time(mustParseTime(createTime)),
			},
		})
	}

	return elbs
}

// ---------------------------------------------------------------------------
// Target Groups (elbv2types.TargetGroup)
// Fields: target_group_name, port, protocol, vpc_id, target_type, health_check_path
// ---------------------------------------------------------------------------

func tgFixtures() []resource.Resource {
	tgs := []resource.Resource{
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

	// Generate 18 more target groups to reach 22 total
	tgProtocols := []elbv2types.ProtocolEnum{
		elbv2types.ProtocolEnumHttp, elbv2types.ProtocolEnumHttps,
		elbv2types.ProtocolEnumHttp, elbv2types.ProtocolEnumTcp,
	}
	tgTargetTypes := []elbv2types.TargetTypeEnum{
		elbv2types.TargetTypeEnumIp, elbv2types.TargetTypeEnumInstance,
	}
	tgPorts := []int32{8080, 443, 3000, 8443, 9090, 5000, 8080, 443, 3000, 8443, 9090, 5000, 8080, 443, 3000, 8443, 9090, 5000}
	healthPaths := []string{"/health", "/healthz", "/api/health", "/ready", "/ping", "/status"}
	for i := 0; i < 18; i++ {
		name := tgNamePool[i]
		port := tgPorts[i]
		proto := tgProtocols[i%len(tgProtocols)]
		targetType := tgTargetTypes[i%len(tgTargetTypes)]
		healthPath := healthPaths[i%len(healthPaths)]
		arn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/%s/%016x", name, i+200)
		tgs = append(tgs, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"target_group_name": name,
				"port":              fmt.Sprintf("%d", port),
				"protocol":          string(proto),
				"vpc_id":            prodVPCID,
				"target_type":       string(targetType),
				"health_check_path": healthPath,
			},
			RawStruct: elbv2types.TargetGroup{
				TargetGroupName:            aws.String(name),
				TargetGroupArn:             aws.String(arn),
				Port:                       aws.Int32(port),
				Protocol:                   proto,
				VpcId:                      aws.String(prodVPCID),
				TargetType:                 targetType,
				HealthCheckPath:            aws.String(healthPath),
				HealthCheckPort:            aws.String(fmt.Sprintf("%d", port)),
				HealthCheckEnabled:         aws.Bool(true),
				HealthCheckIntervalSeconds: aws.Int32(30),
				HealthCheckTimeoutSeconds:  aws.Int32(5),
				HealthyThresholdCount:      aws.Int32(3),
				UnhealthyThresholdCount:    aws.Int32(3),
				Matcher:                    &elbv2types.Matcher{HttpCode: aws.String("200")},
			},
		})
	}

	return tgs
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
				CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
					{
						AssociationId: aws.String("vpc-cidr-assoc-01"),
						CidrBlock:     aws.String("10.0.0.0/16"),
						CidrBlockState: &ec2types.VpcCidrBlockState{
							State: ec2types.VpcCidrBlockStateCodeAssociated,
						},
					},
				},
				Ipv6CidrBlockAssociationSet: []ec2types.VpcIpv6CidrBlockAssociation{
					{
						AssociationId: aws.String("vpc-cidr-assoc-ipv6-01"),
						Ipv6CidrBlock: aws.String("2600:1f18:1234:5678::/56"),
						Ipv6CidrBlockState: &ec2types.VpcCidrBlockState{
							State: ec2types.VpcCidrBlockStateCodeAssociated,
						},
					},
				},
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
// ELB Listeners (elbv2types.Listener) — child of Load Balancers
// Fields: port, protocol, default_action_type, default_action_target,
//
//	ssl_policy, certificate_short
//
// ---------------------------------------------------------------------------

func elbListenerFixtures(_ string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/aaa111",
			Name:   "443",
			Status: "",
			Fields: map[string]string{
				"port":                  "443",
				"protocol":              "HTTPS",
				"default_action_type":   "forward",
				"default_action_target": "acme-web-tg",
				"ssl_policy":            "ELBSecurityPolicy-TLS13-1-2-2021-06",
				"certificate_short":     "abc-def-123",
				"listener_display":      ":443 HTTPS",
			},
			RawStruct: elbv2types.Listener{
				ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/aaa111"),
				Port:        aws.Int32(443),
				Protocol:    elbv2types.ProtocolEnumHttps,
				SslPolicy:   aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
				Certificates: []elbv2types.Certificate{{
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123"),
				}},
				DefaultActions: []elbv2types.Action{{
					Type:           elbv2types.ActionTypeEnumForward,
					TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"),
				}},
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/bbb222",
			Name:   "80",
			Status: "",
			Fields: map[string]string{
				"port":                  "80",
				"protocol":              "HTTP",
				"default_action_type":   "redirect",
				"default_action_target": "HTTPS://#{host}:443#{path}?#{query}",
				"ssl_policy":            "",
				"certificate_short":     "",
				"listener_display":      ":80 HTTP",
			},
			RawStruct: elbv2types.Listener{
				ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/bbb222"),
				Port:        aws.Int32(80),
				Protocol:    elbv2types.ProtocolEnumHttp,
				DefaultActions: []elbv2types.Action{{
					Type: elbv2types.ActionTypeEnumRedirect,
					RedirectConfig: &elbv2types.RedirectActionConfig{
						Protocol:   aws.String("HTTPS"),
						Port:       aws.String("443"),
						StatusCode: elbv2types.RedirectActionStatusCodeEnumHttp301,
					},
				}},
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/ccc333",
			Name:   "8443",
			Status: "",
			Fields: map[string]string{
				"port":                  "8443",
				"protocol":              "HTTPS",
				"default_action_type":   "forward",
				"default_action_target": "acme-api-tg",
				"ssl_policy":            "ELBSecurityPolicy-2016-08",
				"certificate_short":     "xyz-789-456",
				"listener_display":      ":8443 HTTPS",
			},
			RawStruct: elbv2types.Listener{
				ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/ccc333"),
				Port:        aws.Int32(8443),
				Protocol:    elbv2types.ProtocolEnumHttps,
				SslPolicy:   aws.String("ELBSecurityPolicy-2016-08"),
				Certificates: []elbv2types.Certificate{{
					CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/xyz-789-456"),
				}},
				DefaultActions: []elbv2types.Action{{
					Type:           elbv2types.ActionTypeEnumForward,
					TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/0987654321fedcba"),
				}},
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/net/acme-prod-nlb/abcdef1234567890/ddd444",
			Name:   "5000",
			Status: "",
			Fields: map[string]string{
				"port":                  "5000",
				"protocol":              "TCP",
				"default_action_type":   "forward",
				"default_action_target": "acme-nlb-tcp-tg",
				"ssl_policy":            "",
				"certificate_short":     "",
				"listener_display":      ":5000 TCP",
			},
			RawStruct: elbv2types.Listener{
				ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/net/acme-prod-nlb/abcdef1234567890/ddd444"),
				Port:        aws.Int32(5000),
				Protocol:    elbv2types.ProtocolEnumTcp,
				DefaultActions: []elbv2types.Action{{
					Type:           elbv2types.ActionTypeEnumForward,
					TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-nlb-tcp-tg/112233aabbccddee"),
				}},
			},
		},
	}
}

func elbListenerRuleFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule1",
			Name:   "100",
			Status: "",
			Fields: map[string]string{
				"priority":           "100",
				"conditions_summary": "path: /api/*",
				"action_type":        "forward",
				"action_target":      "acme-api-tg",
				"is_default":         "false",
			},
			RawStruct: elbv2types.Rule{
				RuleArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule1"),
				Priority:  aws.String("100"),
				IsDefault: aws.Bool(false),
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule2",
			Name:   "200",
			Status: "",
			Fields: map[string]string{
				"priority":           "200",
				"conditions_summary": "host: admin.example.com AND path: /dashboard/*",
				"action_type":        "forward",
				"action_target":      "acme-admin-tg",
				"is_default":         "false",
			},
			RawStruct: elbv2types.Rule{
				RuleArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule2"),
				Priority:  aws.String("200"),
				IsDefault: aws.Bool(false),
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule3",
			Name:   "300",
			Status: "",
			Fields: map[string]string{
				"priority":           "300",
				"conditions_summary": "path: /health",
				"action_type":        "fixed-response",
				"action_target":      "200 text/plain",
				"is_default":         "false",
			},
			RawStruct: elbv2types.Rule{
				RuleArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule3"),
				Priority:  aws.String("300"),
				IsDefault: aws.Bool(false),
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule4",
			Name:   "400",
			Status: "",
			Fields: map[string]string{
				"priority":           "400",
				"conditions_summary": "path: /old/*",
				"action_type":        "redirect",
				"action_target":      "HTTPS://#{host}:443/new/#{path}?#{query}",
				"is_default":         "false",
			},
			RawStruct: elbv2types.Rule{
				RuleArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/rule4"),
				Priority:  aws.String("400"),
				IsDefault: aws.Bool(false),
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/default",
			Name:   "default",
			Status: "",
			Fields: map[string]string{
				"priority":           "default",
				"conditions_summary": "",
				"action_type":        "forward",
				"action_target":      "acme-web-tg",
				"is_default":         "true",
			},
			RawStruct: elbv2types.Rule{
				RuleArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaa111/default"),
				Priority:  aws.String("default"),
				IsDefault: aws.Bool(true),
			},
		},
	}
}
