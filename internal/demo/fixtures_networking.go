package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
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
	demoData["vpc"] = vpcFixtures

	RegisterChildDemo("elb_listeners", func(parentCtx map[string]string) []resource.Resource {
		return elbListenerFixtures(parentCtx["load_balancer_arn"])
	})
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
				"name":              "acme-prod-web",
				"dns_name":          "acme-prod-web-1234567890.us-east-1.elb.amazonaws.com",
				"type":              "application",
				"scheme":            "internet-facing",
				"state":             "active",
				"vpc_id":            prodVPCID,
				"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef",
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
