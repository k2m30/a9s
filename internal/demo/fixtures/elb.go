// Package fixtures provides ELBv2 fixture data for the ELB fake.
package fixtures

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// ELBFixtures holds all ELBv2 domain objects served by the fake.
type ELBFixtures struct {
	LoadBalancers []elbv2types.LoadBalancer
	TargetGroups  []elbv2types.TargetGroup
	// Listeners keyed by load balancer ARN
	Listeners map[string][]elbv2types.Listener
	// TargetHealth keyed by target group ARN
	TargetHealth map[string][]elbv2types.TargetHealthDescription
	// Rules keyed by listener ARN
	Rules map[string][]elbv2types.Rule
}

const (
	fixtProdELBName     = "acme-prod-web"
	fixtProdELBARN      = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
	fixtProdELBDNS      = "acme-prod-web-1234567890.us-east-1.elb.amazonaws.com"
	fixtELBProdVPCID    = "vpc-0abc123def456789a"
	fixtELBStagingVPCID = "vpc-0def456789abc123d"
	fixtELBSubnetA      = "subnet-0aaa111111111111a"
	fixtELBSubnetB      = "subnet-0bbb222222222222b"

	fixtProdListenerARN  = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/aaaa1111bbbb2222"
	fixtProdWebTGARN     = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"
	fixtProdAPITGARN     = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/0987654321fedcba"
	fixtProdListenerRule = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaaa1111bbbb2222/rule1111111111111"
)

// NewELBFixtures builds and returns a fully-populated ELBFixtures struct.
func NewELBFixtures() *ELBFixtures {
	f := &ELBFixtures{
		Listeners:    make(map[string][]elbv2types.Listener),
		TargetHealth: make(map[string][]elbv2types.TargetHealthDescription),
		Rules:        make(map[string][]elbv2types.Rule),
	}
	f.LoadBalancers = buildLoadBalancers()
	f.TargetGroups = buildTargetGroups()
	buildListeners(f)
	buildTargetHealth(f)
	buildRules(f)
	return f
}

func buildLoadBalancers() []elbv2types.LoadBalancer {
	lbs := []elbv2types.LoadBalancer{
		{
			LoadBalancerName: aws.String(fixtProdELBName),
			LoadBalancerArn:  aws.String(fixtProdELBARN),
			DNSName:          aws.String(fixtProdELBDNS),
			Type:             elbv2types.LoadBalancerTypeEnumApplication,
			Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
			State: &elbv2types.LoadBalancerState{
				Code: elbv2types.LoadBalancerStateEnumActive,
			},
			VpcId:         aws.String(fixtELBProdVPCID),
			IpAddressType: elbv2types.IpAddressTypeIpv4,
			SecurityGroups: []string{"sg-0aaa111111111111a"},
			CreatedTime:    aws.Time(time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)),
			AvailabilityZones: []elbv2types.AvailabilityZone{
				{SubnetId: aws.String(fixtELBSubnetA), ZoneName: aws.String("us-east-1a")},
				{SubnetId: aws.String(fixtELBSubnetB), ZoneName: aws.String("us-east-1b")},
			},
		},
		{
			LoadBalancerName: aws.String("acme-internal-api"),
			LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba"),
			DNSName:          aws.String("internal-acme-api-0987654321.us-east-1.elb.amazonaws.com"),
			Type:             elbv2types.LoadBalancerTypeEnumApplication,
			Scheme:           elbv2types.LoadBalancerSchemeEnumInternal,
			State: &elbv2types.LoadBalancerState{
				Code: elbv2types.LoadBalancerStateEnumActive,
			},
			VpcId:         aws.String(fixtELBProdVPCID),
			IpAddressType: elbv2types.IpAddressTypeIpv4,
			SecurityGroups: []string{"sg-0bbb222222222222b"},
			CreatedTime:    aws.Time(time.Date(2025, 8, 20, 14, 0, 0, 0, time.UTC)),
		},
		{
			LoadBalancerName: aws.String("acme-prod-nlb"),
			LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/acme-prod-nlb/abcdef1234567890"),
			DNSName:          aws.String("acme-prod-nlb-abcdef1234.us-east-1.elb.amazonaws.com"),
			Type:             elbv2types.LoadBalancerTypeEnumNetwork,
			Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
			State: &elbv2types.LoadBalancerState{
				Code: elbv2types.LoadBalancerStateEnumActive,
			},
			VpcId:         aws.String(fixtELBProdVPCID),
			IpAddressType: elbv2types.IpAddressTypeIpv4,
			CreatedTime:   aws.Time(time.Date(2025, 9, 10, 9, 0, 0, 0, time.UTC)),
		},
		{
			LoadBalancerName: aws.String("staging-web-alb"),
			LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/staging-web-alb/5555555555aaaaaa"),
			DNSName:          aws.String("staging-web-alb-5555555555.us-east-1.elb.amazonaws.com"),
			Type:             elbv2types.LoadBalancerTypeEnumApplication,
			Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
			State: &elbv2types.LoadBalancerState{
				Code: elbv2types.LoadBalancerStateEnumProvisioning,
			},
			VpcId:         aws.String(fixtELBStagingVPCID),
			IpAddressType: elbv2types.IpAddressTypeIpv4,
			CreatedTime:   aws.Time(time.Date(2026, 3, 21, 8, 0, 0, 0, time.UTC)),
		},
	}

	// Issue: State=active_impaired → Warning (partial AZ failure)
	lbs = append(lbs, elbv2types.LoadBalancer{
		LoadBalancerName: aws.String("elb-active-impaired"),
		LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/elb-active-impaired/1111aaaa2222bbbb"),
		DNSName:          aws.String("elb-active-impaired-1234567890.us-east-1.elb.amazonaws.com"),
		Type:             elbv2types.LoadBalancerTypeEnumApplication,
		Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
		State: &elbv2types.LoadBalancerState{
			Code:   elbv2types.LoadBalancerStateEnumActiveImpaired,
			Reason: aws.String("A registered instance is in an Availability Zone that is not enabled for the load balancer."),
		},
		VpcId:         aws.String(fixtELBProdVPCID),
		IpAddressType: elbv2types.IpAddressTypeIpv4,
		SecurityGroups: []string{"sg-0aaa111111111111a"},
		CreatedTime:    aws.Time(time.Date(2025, 10, 5, 11, 0, 0, 0, time.UTC)),
		AvailabilityZones: []elbv2types.AvailabilityZone{
			{SubnetId: aws.String(fixtELBSubnetA), ZoneName: aws.String("us-east-1a")},
		},
	})

	// Issue: State=failed → Broken (load balancer provisioning failed)
	lbs = append(lbs, elbv2types.LoadBalancer{
		LoadBalancerName: aws.String("elb-failed"),
		LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/elb-failed/3333cccc4444dddd"),
		DNSName:          aws.String("elb-failed-9876543210.us-east-1.elb.amazonaws.com"),
		Type:             elbv2types.LoadBalancerTypeEnumApplication,
		Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
		State: &elbv2types.LoadBalancerState{
			Code:   elbv2types.LoadBalancerStateEnumFailed,
			Reason: aws.String("Load balancer creation failed due to subnet configuration error."),
		},
		VpcId:         aws.String(fixtELBProdVPCID),
		IpAddressType: elbv2types.IpAddressTypeIpv4,
		SecurityGroups: []string{"sg-0aaa111111111111a"},
		CreatedTime:    aws.Time(time.Date(2026, 4, 10, 14, 30, 0, 0, time.UTC)),
		AvailabilityZones: []elbv2types.AvailabilityZone{
			{SubnetId: aws.String(fixtELBSubnetA), ZoneName: aws.String("us-east-1a")},
		},
	})

	// Generate additional ELBs
	names := []string{
		"api-services-alb", "data-pipeline-nlb", "monitoring-alb", "ci-build-alb",
		"acme-dev-web", "analytics-alb", "auth-service-alb", "reporting-alb",
		"webhooks-nlb", "cache-layer-alb", "events-alb", "grpc-nlb",
		"media-upload-alb", "search-alb", "notification-alb", "worker-alb",
		"batch-nlb", "gateway-alb",
	}
	lbTypes := []elbv2types.LoadBalancerTypeEnum{
		elbv2types.LoadBalancerTypeEnumApplication, elbv2types.LoadBalancerTypeEnumNetwork,
	}
	schemes := []elbv2types.LoadBalancerSchemeEnum{
		elbv2types.LoadBalancerSchemeEnumInternetFacing, elbv2types.LoadBalancerSchemeEnumInternal,
	}
	subnets := []string{fixtELBSubnetA, fixtELBSubnetB}
	for i, name := range names {
		lbType := lbTypes[i%len(lbTypes)]
		scheme := schemes[i%len(schemes)]
		typePrefix := "app"
		if lbType == elbv2types.LoadBalancerTypeEnumNetwork {
			typePrefix = "net"
		}
		arn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/%s/%s/%016x", typePrefix, name, i+100)
		dns := fmt.Sprintf("%s-%010d.us-east-1.elb.amazonaws.com", name, i+1000)
		vpcID := fixtELBProdVPCID
		if i >= 14 {
			vpcID = fixtELBStagingVPCID
		}
		lbs = append(lbs, elbv2types.LoadBalancer{
			LoadBalancerName: aws.String(name),
			LoadBalancerArn:  aws.String(arn),
			DNSName:          aws.String(dns),
			Type:             lbType,
			Scheme:           scheme,
			State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
			VpcId:            aws.String(vpcID),
			IpAddressType:    elbv2types.IpAddressTypeIpv4,
			SecurityGroups:   []string{"sg-0aaa111111111111a"},
			CreatedTime:      aws.Time(time.Date(2025, 1+time.Month(i%12), 1+i%28, 8, 0, 0, 0, time.UTC)),
			AvailabilityZones: []elbv2types.AvailabilityZone{
				{SubnetId: aws.String(subnets[i%len(subnets)]), ZoneName: aws.String("us-east-1a")},
			},
		})
	}
	return lbs
}

func buildTargetGroups() []elbv2types.TargetGroup {
	return []elbv2types.TargetGroup{
		{
			TargetGroupName:            aws.String("acme-web-tg"),
			TargetGroupArn:             aws.String(fixtProdWebTGARN),
			Port:                       aws.Int32(443),
			Protocol:                   elbv2types.ProtocolEnumHttps,
			VpcId:                      aws.String(fixtELBProdVPCID),
			TargetType:                 elbv2types.TargetTypeEnumInstance,
			HealthCheckPath:            aws.String("/healthz"),
			HealthCheckEnabled:         aws.Bool(true),
			HealthCheckIntervalSeconds: aws.Int32(30),
			HealthCheckTimeoutSeconds:  aws.Int32(5),
			HealthyThresholdCount:      aws.Int32(3),
			UnhealthyThresholdCount:    aws.Int32(3),
			LoadBalancerArns:           []string{fixtProdELBARN},
		},
		{
			TargetGroupName:            aws.String("acme-api-tg"),
			TargetGroupArn:             aws.String(fixtProdAPITGARN),
			Port:                       aws.Int32(8080),
			Protocol:                   elbv2types.ProtocolEnumHttp,
			VpcId:                      aws.String(fixtELBProdVPCID),
			TargetType:                 elbv2types.TargetTypeEnumIp,
			HealthCheckPath:            aws.String("/api/health"),
			HealthCheckEnabled:         aws.Bool(true),
			HealthCheckIntervalSeconds: aws.Int32(15),
			HealthCheckTimeoutSeconds:  aws.Int32(3),
			HealthyThresholdCount:      aws.Int32(2),
			UnhealthyThresholdCount:    aws.Int32(2),
			LoadBalancerArns:           []string{fixtProdELBARN},
		},
		{
			TargetGroupName:    aws.String("acme-grpc-tg"),
			TargetGroupArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-grpc-tg/1111111111111111"),
			Port:               aws.Int32(50051),
			Protocol:           elbv2types.ProtocolEnumHttp,
			ProtocolVersion:    aws.String("GRPC"),
			VpcId:              aws.String(fixtELBProdVPCID),
			TargetType:         elbv2types.TargetTypeEnumIp,
			HealthCheckEnabled: aws.Bool(true),
			LoadBalancerArns:   []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba"},
		},
		{
			TargetGroupName:    aws.String("staging-web-tg"),
			TargetGroupArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/staging-web-tg/2222222222222222"),
			Port:               aws.Int32(80),
			Protocol:           elbv2types.ProtocolEnumHttp,
			VpcId:              aws.String(fixtELBStagingVPCID),
			TargetType:         elbv2types.TargetTypeEnumInstance,
			HealthCheckEnabled: aws.Bool(true),
			LoadBalancerArns:   []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/staging-web-alb/5555555555aaaaaa"},
		},
	}
}

func buildListeners(f *ELBFixtures) {
	f.Listeners[fixtProdELBARN] = []elbv2types.Listener{
		{
			ListenerArn:     aws.String(fixtProdListenerARN),
			LoadBalancerArn: aws.String(fixtProdELBARN),
			Port:            aws.Int32(443),
			Protocol:        elbv2types.ProtocolEnumHttps,
			DefaultActions: []elbv2types.Action{
				{
					Type:           elbv2types.ActionTypeEnumForward,
					TargetGroupArn: aws.String(fixtProdWebTGARN),
				},
			},
		},
		{
			ListenerArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/bbbb2222cccc3333"),
			LoadBalancerArn: aws.String(fixtProdELBARN),
			Port:            aws.Int32(80),
			Protocol:        elbv2types.ProtocolEnumHttp,
			DefaultActions: []elbv2types.Action{
				{
					Type: elbv2types.ActionTypeEnumRedirect,
					RedirectConfig: &elbv2types.RedirectActionConfig{
						Protocol:   aws.String("HTTPS"),
						Port:       aws.String("443"),
						StatusCode: elbv2types.RedirectActionStatusCodeEnumHttp301,
					},
				},
			},
		},
	}
}

func buildTargetHealth(f *ELBFixtures) {
	f.TargetHealth[fixtProdWebTGARN] = []elbv2types.TargetHealthDescription{
		{
			Target: &elbv2types.TargetDescription{
				Id:   aws.String("i-0a1b2c3d4e5f60001"),
				Port: aws.Int32(443),
			},
			HealthCheckPort: aws.String("443"),
			TargetHealth: &elbv2types.TargetHealth{
				State: elbv2types.TargetHealthStateEnumHealthy,
			},
		},
		{
			Target: &elbv2types.TargetDescription{
				Id:   aws.String("i-0a1b2c3d4e5f60002"),
				Port: aws.Int32(443),
			},
			HealthCheckPort: aws.String("443"),
			TargetHealth: &elbv2types.TargetHealth{
				State: elbv2types.TargetHealthStateEnumHealthy,
			},
		},
		{
			Target: &elbv2types.TargetDescription{
				Id:   aws.String("i-0a1b2c3d4e5f60003"),
				Port: aws.Int32(443),
			},
			HealthCheckPort: aws.String("443"),
			TargetHealth: &elbv2types.TargetHealth{
				State:       elbv2types.TargetHealthStateEnumUnhealthy,
				Reason:      elbv2types.TargetHealthReasonEnumFailedHealthChecks,
				Description: aws.String("Health checks failed"),
			},
		},
	}
	f.TargetHealth[fixtProdAPITGARN] = []elbv2types.TargetHealthDescription{
		{
			Target: &elbv2types.TargetDescription{
				Id:   aws.String("10.0.1.50"),
				Port: aws.Int32(8080),
			},
			HealthCheckPort: aws.String("8080"),
			TargetHealth: &elbv2types.TargetHealth{
				State: elbv2types.TargetHealthStateEnumHealthy,
			},
		},
	}
}

func buildRules(f *ELBFixtures) {
	f.Rules[fixtProdListenerARN] = []elbv2types.Rule{
		{
			RuleArn:  aws.String(fixtProdListenerRule),
			Priority: aws.String("1"),
			Conditions: []elbv2types.RuleCondition{
				{
					Field:  aws.String("path-pattern"),
					Values: []string{"/api/*"},
				},
			},
			Actions: []elbv2types.Action{
				{
					Type:           elbv2types.ActionTypeEnumForward,
					TargetGroupArn: aws.String(fixtProdAPITGARN),
				},
			},
			IsDefault: aws.Bool(false),
		},
		{
			RuleArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaaa1111bbbb2222/default"),
			Priority: aws.String("default"),
			Conditions: []elbv2types.RuleCondition{},
			Actions: []elbv2types.Action{
				{
					Type:           elbv2types.ActionTypeEnumForward,
					TargetGroupArn: aws.String(fixtProdWebTGARN),
				},
			},
			IsDefault: aws.Bool(true),
		},
	}
}
