// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides ELBv2 fixture data for the ELB fake.
package fixtures

import (
	"fmt"
	"sync"
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
	// ResourceTags maps an ELB/TG ARN to its elbv2:DescribeTags tag set.
	// Backs the elb→cfn and tg→cfn related-panel pivots (checkELBCFN / checkTGCFN).
	ResourceTags map[string][]elbv2types.Tag
	// LoadBalancerAttributes maps a load balancer ARN to its
	// elbv2:DescribeLoadBalancerAttributes response. Backs the elb→s3
	// related-panel pivot (checkELBS3).
	LoadBalancerAttributes map[string][]elbv2types.LoadBalancerAttribute
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

	// fixtLambdaProcessorTGARN backs the lambda:tg related-panel pivot
	// witness — a Lambda-type target group registering process-orders
	// (lambda.go) as its target.
	fixtLambdaProcessorTGARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/lambda-processor-tg/3333333333333333"
)

// GRPCTargetGroupARN is the demo witness for a finding that carries more
// supporting rows than the detail view shows: acme-grpc-tg registers more
// wholly unhealthy targets than the row cap, which is the only way `--demo`
// renders the closing "… +K more" row of a capped Attention list.
const GRPCTargetGroupARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-grpc-tg/1111111111111111"

// Prowler-gap witnesses for the elb type. Each names the ONE demo load
// balancer that carries the corresponding Wave-2 finding; no other load
// balancer has the attribute or the listener that would trip it.
const (
	// ELBDesyncMonitor only observes ambiguous HTTP requests instead of
	// rejecting them.
	ELBDesyncMonitor = "monitoring-alb"
	// ELBKeepsInvalidHeaders forwards invalid HTTP header fields to its
	// targets.
	ELBKeepsInvalidHeaders = "acme-dev-web"
	// ELBPlainHTTP serves an HTTP listener that does not redirect to HTTPS.
	ELBPlainHTTP = "auth-service-alb"
	// ELBPlainTCPListener is the network-load-balancer half of the same
	// signal: a TCP listener on a port nothing encrypts by convention. Port
	// 443 is NOT that port — a TCP listener there is TLS passthrough, and the
	// session terminates on the target.
	ELBPlainTCPListener = "data-pipeline-nlb"
	// ELBWeakTLS terminates TLS on a pre-TLS-1.2 security policy.
	ELBWeakTLS = "events-alb"
)

// NewELBFixtures builds and returns a fully-populated ELBFixtures struct.
var sharedELBFixtures = sync.OnceValue(func() *ELBFixtures {
	f := &ELBFixtures{
		Listeners:    make(map[string][]elbv2types.Listener),
		TargetHealth: make(map[string][]elbv2types.TargetHealthDescription),
		Rules:        make(map[string][]elbv2types.Rule),
		// ResourceTags — the prod ALB and its web TG both carry the stack tag,
		// backing elb→cfn and tg→cfn. acme-eks-cluster is a real stack
		// fixture (cfn.go).
		ResourceTags: map[string][]elbv2types.Tag{
			fixtProdELBARN: {
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
			},
			fixtProdWebTGARN: {
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
			},
		},
		// LoadBalancerAttributes — the prod ALB has access logging enabled
		// to the a9s-demo-logs bucket (s3.go LogsBucketName), backing elb→s3.
		// acme-internal-api has deletion protection disabled — pins
		// elbCodeMisconfigured ("elb.misconfigured") firing dynamically in
		// demo mode; before this entry no fixture LB carried
		// deletion_protection.enabled=false, so the Wave-2 enricher had
		// nothing to classify against (the OWNER GAP qa_finding_dynamic_witness_test.go
		// pins as knownUnwitnessedFindings["elb:elb.misconfigured"]).
		LoadBalancerAttributes: map[string][]elbv2types.LoadBalancerAttribute{
			fixtProdELBARN: {
				{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
				{Key: aws.String("access_logs.s3.bucket"), Value: aws.String(LogsBucketName)},
				{Key: aws.String("access_logs.s3.prefix"), Value: aws.String("acme-prod-web")},
			},
			"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-internal-api/0987654321fedcba": {
				{Key: aws.String("deletion_protection.enabled"), Value: aws.String("false")},
			},
		},
	}
	f.LoadBalancers = buildLoadBalancers()
	buildWitnessAttributes(f)
	f.TargetGroups = buildTargetGroups()
	buildListeners(f)
	buildTargetHealth(f)
	buildRules(f)
	return f
})

func NewELBFixtures() *ELBFixtures {
	return sharedELBFixtures()
}

// lbARNByName resolves a demo load balancer's ARN from its name so the
// witness tables below key off the same generated ARN the fetcher emits.
func lbARNByName(lbs []elbv2types.LoadBalancer, name string) string {
	for _, lb := range lbs {
		if aws.ToString(lb.LoadBalancerName) == name {
			return aws.ToString(lb.LoadBalancerArn)
		}
	}
	return ""
}

// buildWitnessAttributes gives each attribute-driven elb finding exactly one
// witness. Every load balancer named here also carries the healthy value for
// the sibling attribute, so neither witness trips the other's finding; every
// load balancer NOT named here has no attribute entry at all, which the
// enricher reads as "not reported" rather than "misconfigured".
func buildWitnessAttributes(f *ELBFixtures) {
	f.LoadBalancerAttributes[lbARNByName(f.LoadBalancers, ELBDesyncMonitor)] = []elbv2types.LoadBalancerAttribute{
		{Key: aws.String("routing.http.desync_mitigation_mode"), Value: aws.String("monitor")},
		{Key: aws.String("routing.http.drop_invalid_header_fields.enabled"), Value: aws.String("true")},
	}
	f.LoadBalancerAttributes[lbARNByName(f.LoadBalancers, ELBKeepsInvalidHeaders)] = []elbv2types.LoadBalancerAttribute{
		{Key: aws.String("routing.http.desync_mitigation_mode"), Value: aws.String("defensive")},
		{Key: aws.String("routing.http.drop_invalid_header_fields.enabled"), Value: aws.String("false")},
	}
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
			VpcId:          aws.String(fixtELBProdVPCID),
			IpAddressType:  elbv2types.IpAddressTypeIpv4,
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
			VpcId:          aws.String(fixtELBProdVPCID),
			IpAddressType:  elbv2types.IpAddressTypeIpv4,
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
			// AvailabilityZones/SecurityGroups — required for the apigw:elb
			// related-panel pivot (checkApigwELB), which intersects the
			// PublicAPIGWID VpcLink's SubnetIds/SecurityGroupIds
			// (apigw.go fixture) against this NLB's subnet/SG membership.
			AvailabilityZones: []elbv2types.AvailabilityZone{
				{SubnetId: aws.String(fixtProdPrivateSubnetA)},
			},
			SecurityGroups: []string{APIGWVpcLinkSecurityGroupID},
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
		VpcId:          aws.String(fixtELBProdVPCID),
		IpAddressType:  elbv2types.IpAddressTypeIpv4,
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
		VpcId:          aws.String(fixtELBProdVPCID),
		IpAddressType:  elbv2types.IpAddressTypeIpv4,
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
			TargetGroupArn:     aws.String(GRPCTargetGroupARN),
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
		// Lambda-type target group — required for the lambda:tg related-panel
		// pivot witness. checkLambdaTG calls DescribeTargetHealth and matches
		// Target.Id against the function ARN; process-orders is a real
		// lambda.go fixture.
		{
			TargetGroupName:    aws.String("lambda-processor-tg"),
			TargetGroupArn:     aws.String(fixtLambdaProcessorTGARN),
			Protocol:           elbv2types.ProtocolEnumHttps,
			VpcId:              aws.String(fixtELBProdVPCID),
			TargetType:         elbv2types.TargetTypeEnumLambda,
			HealthCheckEnabled: aws.Bool(false),
			LoadBalancerArns:   []string{fixtProdELBARN},
		},
	}
}

func buildListeners(f *ELBFixtures) {
	// The only demo listeners speaking plain HTTP with a forward action —
	// every other HTTP listener redirects to HTTPS. Two of them, supplied
	// 8080 before 80 because DescribeListeners promises no order: the merged
	// phrase has to sort the ports itself, and a fixture already in order
	// could not tell whether it does.
	plainARN := lbARNByName(f.LoadBalancers, ELBPlainHTTP)
	f.Listeners[plainARN] = []elbv2types.Listener{
		{
			ListenerArn:     aws.String(plainARN + "/listener/aaaa1112"),
			LoadBalancerArn: aws.String(plainARN),
			Port:            aws.Int32(8080),
			Protocol:        elbv2types.ProtocolEnumHttp,
			DefaultActions: []elbv2types.Action{
				{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(fixtProdAPITGARN)},
			},
		},
		{
			ListenerArn:     aws.String(plainARN + "/listener/aaaa1111"),
			LoadBalancerArn: aws.String(plainARN),
			Port:            aws.Int32(80),
			Protocol:        elbv2types.ProtocolEnumHttp,
			DefaultActions: []elbv2types.Action{
				{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(fixtProdAPITGARN)},
			},
		},
	}

	// The only demo network load balancer with a listener, so no other NLB
	// can trip the TCP half of the plain-listener rule.
	plainTCPARN := lbARNByName(f.LoadBalancers, ELBPlainTCPListener)
	f.Listeners[plainTCPARN] = []elbv2types.Listener{
		{
			ListenerArn:     aws.String(plainTCPARN + "/listener/cccc3333"),
			LoadBalancerArn: aws.String(plainTCPARN),
			Port:            aws.Int32(8080),
			Protocol:        elbv2types.ProtocolEnumTcp,
			DefaultActions: []elbv2types.Action{
				{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(fixtProdAPITGARN)},
			},
		},
	}

	// The only demo listeners on a pre-TLS-1.2 security policy. Both carry a
	// certificate so the wave-1 "no certificate configured" signal stays on
	// its own witness, and they are supplied 8443 before 443 for the same
	// reason the cleartext pair is out of order.
	weakTLSARN := lbARNByName(f.LoadBalancers, ELBWeakTLS)
	f.Listeners[weakTLSARN] = []elbv2types.Listener{
		{
			ListenerArn:     aws.String(weakTLSARN + "/listener/bbbb2223"),
			LoadBalancerArn: aws.String(weakTLSARN),
			Port:            aws.Int32(8443),
			Protocol:        elbv2types.ProtocolEnumHttps,
			SslPolicy:       aws.String("ELBSecurityPolicy-TLS-1-0-2015-04"),
			Certificates: []elbv2types.Certificate{
				{CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111")},
			},
			DefaultActions: []elbv2types.Action{
				{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(fixtProdWebTGARN)},
			},
		},
		{
			ListenerArn:     aws.String(weakTLSARN + "/listener/bbbb2222"),
			LoadBalancerArn: aws.String(weakTLSARN),
			Port:            aws.Int32(443),
			Protocol:        elbv2types.ProtocolEnumHttps,
			SslPolicy:       aws.String("ELBSecurityPolicy-2016-08"),
			Certificates: []elbv2types.Certificate{
				{CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111")},
			},
			DefaultActions: []elbv2types.Action{
				{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(fixtProdWebTGARN)},
			},
		},
	}

	f.Listeners[fixtProdELBARN] = []elbv2types.Listener{
		{
			ListenerArn:     aws.String(fixtProdListenerARN),
			LoadBalancerArn: aws.String(fixtProdELBARN),
			Port:            aws.Int32(443),
			Protocol:        elbv2types.ProtocolEnumHttps,
			// Certificates — required for the elb→acm related-panel pivot
			// (checkELBACM). ProdACMCertARN1 (acm.go) is the same cert this
			// ELB's ARN appears under in ACM's InUseBy fixture (acm→elb).
			Certificates: []elbv2types.Certificate{
				{CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111")},
			},
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
	// acme-grpc-tg — every target reports literal "unhealthy": the sole demo
	// witness for the tg Broken color bucket (EnrichTargetGroupHealth: "!"
	// only when every reporting target is unhealthy, not merely a mix), and
	// the sole demo witness for a finding with more supporting rows than the
	// detail view shows. Twelve is the smallest count that puts two rows past
	// awsclient.FindingRowCap, so the closing "… +2 more" row renders with a
	// count no reader can mistake for the number of targets.
	grpcTargets := make([]elbv2types.TargetHealthDescription, 0, 12)
	for i := range 12 {
		grpcTargets = append(grpcTargets, elbv2types.TargetHealthDescription{
			Target: &elbv2types.TargetDescription{
				Id:   aws.String(fmt.Sprintf("10.0.6.%d", 80+i)),
				Port: aws.Int32(50051),
			},
			TargetHealth: &elbv2types.TargetHealth{
				State:       elbv2types.TargetHealthStateEnumUnhealthy,
				Reason:      elbv2types.TargetHealthReasonEnumFailedHealthChecks,
				Description: aws.String("Health checks failed"),
			},
		})
	}
	f.TargetHealth[GRPCTargetGroupARN] = grpcTargets
	// Lambda-type target — Target.Id is the function ARN (no Port for
	// lambda targets). process-orders is a real lambda.go fixture; matches
	// the lambda:tg related-panel pivot witness (checkLambdaTG).
	f.TargetHealth[fixtLambdaProcessorTGARN] = []elbv2types.TargetHealthDescription{
		{
			Target: &elbv2types.TargetDescription{
				Id: aws.String("arn:aws:lambda:us-east-1:123456789012:function:process-orders"),
			},
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
			RuleArn:    aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-prod-web/1234567890abcdef/aaaa1111bbbb2222/default"),
			Priority:   aws.String("default"),
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

func init() {
	Register(Pin{ShortName: "elb", Rows: 24, Issues: 3, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "tg", Rows: 5, Issues: 0, CoverageGaps: []string{"dim"}})
}
