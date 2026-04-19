package unit_test

// aws_elb_related_extra_test.go — additional coverage for elb_related.go.
// Covers: checkELBSG, checkELBVPC, checkELBCFN (with fake DescribeTags),
// checkELBACM (with fake DescribeListeners), checkELBCF (cf cache),
// checkELBENI (eni cache), checkELBS3 (with fake DescribeLoadBalancerAttributes),
// checkELBSubnet, checkELBWAF (with fake GetWebACLForResource).
// elbCheckerByTarget is defined in aws_elb_related_test.go (same package).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeELBv2Tags — implements ELBv2DescribeTagsAPI for checkELBCFN tests
// ---------------------------------------------------------------------------

type fakeELBv2Tags struct {
	output *elbv2.DescribeTagsOutput
	err    error
}

func (f *fakeELBv2Tags) DescribeTags(_ context.Context, _ *elbv2.DescribeTagsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

// fakeELBv2Full embeds fakeELBv2Batch2 (satisfies ELBv2API) plus DescribeTags
// (ELBv2DescribeTagsAPI) and GetWebACLForResource is on a separate WAF fake.
type fakeELBv2Full struct {
	fakeELBv2Batch2
	describeTagsFn           func(*elbv2.DescribeTagsInput) (*elbv2.DescribeTagsOutput, error)
	describeLBAttributesFn   func(*elbv2.DescribeLoadBalancerAttributesInput) (*elbv2.DescribeLoadBalancerAttributesOutput, error)
}

func (f *fakeELBv2Full) DescribeTags(_ context.Context, input *elbv2.DescribeTagsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	if f.describeTagsFn != nil {
		return f.describeTagsFn(input)
	}
	return &elbv2.DescribeTagsOutput{}, nil
}

func (f *fakeELBv2Full) DescribeLoadBalancerAttributes(_ context.Context, input *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	if f.describeLBAttributesFn != nil {
		return f.describeLBAttributesFn(input)
	}
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}

// ---------------------------------------------------------------------------
// fakeWAFv2ForResource — implements WAFv2GetWebACLForResourceAPI
// ---------------------------------------------------------------------------

type fakeWAFv2ForResource struct {
	output *wafv2.GetWebACLForResourceOutput
	err    error
}

func (f *fakeWAFv2ForResource) GetWebACLForResource(_ context.Context, _ *wafv2.GetWebACLForResourceInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLForResourceOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

// ListWebACLs, ListResourcesForWebACL, GetLoggingConfiguration — stubs to satisfy WAFv2API.
func (f *fakeWAFv2ForResource) ListWebACLs(_ context.Context, _ *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	return &wafv2.ListWebACLsOutput{}, nil
}

func (f *fakeWAFv2ForResource) ListResourcesForWebACL(_ context.Context, _ *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	return &wafv2.ListResourcesForWebACLOutput{}, nil
}

func (f *fakeWAFv2ForResource) GetLoggingConfiguration(_ context.Context, _ *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	return &wafv2.GetLoggingConfigurationOutput{}, nil
}

// ---------------------------------------------------------------------------
// Helper: ELB source resource with known ARN
// ---------------------------------------------------------------------------

func elbSrc(name, arn, vpcID, dnsName string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"load_balancer_arn": arn,
			"vpc_id":            vpcID,
			"dns_name":          dnsName,
			"name":              name,
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String(name),
			LoadBalancerArn:  aws.String(arn),
		},
	}
}

// --- checkELBSG (Pattern F — reads SecurityGroups) ---

func TestRelated_ELB_SG_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:   "prod-alb",
		Name: "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("prod-alb"),
			LoadBalancerArn:  aws.String(elbARN),
			SecurityGroups:   []string{"sg-alb001", "sg-alb002"},
		},
	}
	checker := elbCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.ResourceIDs[0] != "sg-alb001" {
		t.Errorf("ResourceIDs[0] = %q, want sg-alb001", result.ResourceIDs[0])
	}
}

func TestRelated_ELB_SG_NLBHasNoSGs(t *testing.T) {
	// NLBs have no security groups
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/prod-nlb/1234567890abcdef"
	source := resource.Resource{
		ID:   "prod-nlb",
		Name: "prod-nlb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("prod-nlb"),
			LoadBalancerArn:  aws.String(elbARN),
			SecurityGroups:   []string{},
		},
	}
	checker := elbCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (NLB has no SGs)", result.Count)
	}
}

func TestRelated_ELB_SG_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "prod-alb", RawStruct: "not-a-load-balancer"}
	checker := elbCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkELBVPC (Pattern F — reads vpc_id from Fields) ---

func TestRelated_ELB_VPC_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "prod-alb",
		Name: "prod-alb",
		Fields: map[string]string{"vpc_id": "vpc-prod001"},
	}
	checker := elbCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "vpc-prod001" {
		t.Errorf("ResourceIDs[0] = %q, want vpc-prod001", result.ResourceIDs[0])
	}
}

func TestRelated_ELB_VPC_EmptyVPCField(t *testing.T) {
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"vpc_id": ""},
	}
	checker := elbCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty vpc_id)", result.Count)
	}
}

// --- checkELBCFN (Pattern C — DescribeTags) ---

func TestRelated_ELB_CFN_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := &fakeELBv2Full{
		describeTagsFn: func(_ *elbv2.DescribeTagsInput) (*elbv2.DescribeTagsOutput, error) {
			return &elbv2.DescribeTagsOutput{
				TagDescriptions: []elbv2types.TagDescription{
					{
						ResourceArn: aws.String(elbARN),
						Tags: []elbv2types.Tag{
							{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("prod-alb-stack")},
						},
					},
				},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "prod-alb-stack" {
		t.Errorf("ResourceIDs[0] = %q, want prod-alb-stack", result.ResourceIDs[0])
	}
}

func TestRelated_ELB_CFN_NoCFNTag(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := &fakeELBv2Full{
		describeTagsFn: func(_ *elbv2.DescribeTagsInput) (*elbv2.DescribeTagsOutput, error) {
			return &elbv2.DescribeTagsOutput{
				TagDescriptions: []elbv2types.TagDescription{
					{
						ResourceArn: aws.String(elbARN),
						Tags: []elbv2types.Tag{
							{Key: aws.String("Environment"), Value: aws.String("prod")},
						},
					},
				},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN stack tag)", result.Count)
	}
}

// --- checkELBACM (Pattern C — DescribeListeners for certificate ARNs) ---

func TestRelated_ELB_ACM_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	const certARN = "arn:aws:acm:us-east-1:123456789012:certificate/abc1-2345-6789-0abc-defabcdef012"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := newFakeELBv2WithListeners([]elbv2types.Listener{
		{
			ListenerArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/prod-alb/abcdef/listener1"),
			LoadBalancerArn: aws.String(elbARN),
			Port:            aws.Int32(443),
			Certificates: []elbv2types.Certificate{
				{CertificateArn: aws.String(certARN)},
			},
		},
	})
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "acm")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != certARN {
		t.Errorf("ResourceIDs[0] = %q, want %s", result.ResourceIDs[0], certARN)
	}
}

func TestRelated_ELB_ACM_DeduplicatesCerts(t *testing.T) {
	// Same cert on two listeners → should appear once.
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	const certARN = "arn:aws:acm:us-east-1:123456789012:certificate/abc1-2345-6789-0abc-defabcdef012"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := newFakeELBv2WithListeners([]elbv2types.Listener{
		{
			ListenerArn:     aws.String("arn:listener1"),
			LoadBalancerArn: aws.String(elbARN),
			Port:            aws.Int32(443),
			Certificates:    []elbv2types.Certificate{{CertificateArn: aws.String(certARN)}},
		},
		{
			ListenerArn:     aws.String("arn:listener2"),
			LoadBalancerArn: aws.String(elbARN),
			Port:            aws.Int32(8443),
			Certificates:    []elbv2types.Certificate{{CertificateArn: aws.String(certARN)}},
		},
	})
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "acm")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated cert)", result.Count)
	}
}

func TestRelated_ELB_ACM_HTTPListenerNoCerts(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := newFakeELBv2WithListeners([]elbv2types.Listener{
		{
			ListenerArn:     aws.String("arn:listener-http"),
			LoadBalancerArn: aws.String(elbARN),
			Port:            aws.Int32(80),
			Certificates:    []elbv2types.Certificate{},
		},
	})
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "acm")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (HTTP-only listener, no certs)", result.Count)
	}
}

func TestRelated_ELB_ACM_NilClients(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	checker := elbCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- checkELBCF (Pattern C — cf cache, DomainName match) ---

func TestRelated_ELB_CF_Found(t *testing.T) {
	const dnsName = "prod-alb-1234567890.us-east-1.elb.amazonaws.com"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"dns_name": dnsName},
	}
	cfRes := resource.Resource{
		ID: "E1EXAMPLE1234",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E1EXAMPLE1234"),
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String(dnsName)},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}
	checker := elbCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "E1EXAMPLE1234" {
		t.Errorf("ResourceIDs[0] = %q, want E1EXAMPLE1234", result.ResourceIDs[0])
	}
}

func TestRelated_ELB_CF_NoMatch(t *testing.T) {
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"dns_name": "prod-alb-1234.us-east-1.elb.amazonaws.com"},
	}
	cfRes := resource.Resource{
		ID: "E2EXAMPLE5678",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E2EXAMPLE5678"),
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("other-origin.example.com")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}
	checker := elbCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching origin domain)", result.Count)
	}
}

func TestRelated_ELB_CF_EmptyDNSName(t *testing.T) {
	source := resource.Resource{ID: "prod-alb", Fields: map[string]string{"dns_name": ""}}
	checker := elbCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty dns_name)", result.Count)
	}
}

// --- checkELBENI (Pattern C — eni cache, "ELB app/NAME/hash" description) ---

func TestRelated_ELB_ENI_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "prod-alb",
		Name: "prod-alb",
		Fields: map[string]string{
			"name": "prod-alb",
		},
	}
	eniRes := resource.Resource{
		ID: "eni-alb001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-alb001"),
			RequesterId:        aws.String("amazon-elb"),
			Description:        aws.String("ELB app/prod-alb/abcdef1234567890"),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := elbCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "eni-alb001" {
		t.Errorf("ResourceIDs[0] = %q, want eni-alb001", result.ResourceIDs[0])
	}
}

func TestRelated_ELB_ENI_WrongRequester(t *testing.T) {
	source := resource.Resource{
		ID:   "prod-alb",
		Name: "prod-alb",
		Fields: map[string]string{"name": "prod-alb"},
	}
	eniRes := resource.Resource{
		ID: "eni-other",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-other"),
			RequesterId:        aws.String("amazon-ec2"), // not elb
			Description:        aws.String("ELB app/prod-alb/abcdef1234567890"),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := elbCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RequesterId)", result.Count)
	}
}

func TestRelated_ELB_ENI_EmptyName(t *testing.T) {
	source := resource.Resource{ID: "", Name: "", Fields: map[string]string{"name": ""}}
	checker := elbCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty name)", result.Count)
	}
}

// --- checkELBS3 (Pattern C — DescribeLoadBalancerAttributes for access_logs.s3.bucket) ---

func TestRelated_ELB_S3_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := &fakeELBv2Full{
		describeLBAttributesFn: func(_ *elbv2.DescribeLoadBalancerAttributesInput) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
			return &elbv2.DescribeLoadBalancerAttributesOutput{
				Attributes: []elbv2types.LoadBalancerAttribute{
					{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("true")},
					{Key: aws.String("access_logs.s3.bucket"), Value: aws.String("my-elb-access-logs")},
					{Key: aws.String("access_logs.s3.prefix"), Value: aws.String("prod-alb")},
				},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "my-elb-access-logs" {
		t.Errorf("ResourceIDs[0] = %q, want my-elb-access-logs", result.ResourceIDs[0])
	}
}

func TestRelated_ELB_S3_LogsNotEnabled(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	fakeELB := &fakeELBv2Full{
		describeLBAttributesFn: func(_ *elbv2.DescribeLoadBalancerAttributesInput) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
			return &elbv2.DescribeLoadBalancerAttributesOutput{
				Attributes: []elbv2types.LoadBalancerAttribute{
					{Key: aws.String("access_logs.s3.enabled"), Value: aws.String("false")},
					{Key: aws.String("access_logs.s3.bucket"), Value: aws.String("")},
				},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fakeELB}
	checker := elbCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (access logs disabled)", result.Count)
	}
}

// --- checkELBSubnet (Pattern F — reads AvailabilityZones[].SubnetId) ---

func TestRelated_ELB_Subnet_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:   "prod-alb",
		Name: "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("prod-alb"),
			LoadBalancerArn:  aws.String(elbARN),
			AvailabilityZones: []elbv2types.AvailabilityZone{
				{ZoneName: aws.String("us-east-1a"), SubnetId: aws.String("subnet-az1a")},
				{ZoneName: aws.String("us-east-1b"), SubnetId: aws.String("subnet-az1b")},
			},
		},
	}
	checker := elbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_ELB_Subnet_DeduplicatesSubnets(t *testing.T) {
	// Unlikely but guard against duplicate subnet IDs.
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:   "prod-alb",
		Name: "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("prod-alb"),
			LoadBalancerArn:  aws.String(elbARN),
			AvailabilityZones: []elbv2types.AvailabilityZone{
				{ZoneName: aws.String("us-east-1a"), SubnetId: aws.String("subnet-az1a")},
				{ZoneName: aws.String("us-east-1a"), SubnetId: aws.String("subnet-az1a")},
			},
		},
	}
	checker := elbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated)", result.Count)
	}
}

func TestRelated_ELB_Subnet_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "prod-alb", RawStruct: "not-a-load-balancer"}
	checker := elbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkELBWAF (Pattern C — GetWebACLForResource) ---

func TestRelated_ELB_WAF_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	const wafID = "waf-web-acl-id-abc123"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	clients := &awsclient.ServiceClients{
		WAFv2: &fakeWAFv2ForResource{
			output: &wafv2.GetWebACLForResourceOutput{
				WebACL: &wafv2types.WebACL{
					Id:   aws.String(wafID),
					Name: aws.String("prod-alb-waf"),
					ARN:  aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-alb-waf/" + wafID),
				},
			},
		},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != wafID {
		t.Errorf("ResourceIDs[0] = %q, want %s", result.ResourceIDs[0], wafID)
	}
}

func TestRelated_ELB_WAF_NoWebACL(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abcdef1234567890"
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	clients := &awsclient.ServiceClients{
		WAFv2: &fakeWAFv2ForResource{
			output: &wafv2.GetWebACLForResourceOutput{WebACL: nil},
		},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no WAF attached)", result.Count)
	}
}

func TestRelated_ELB_WAF_EmptyARN(t *testing.T) {
	source := resource.Resource{
		ID:     "prod-alb",
		Fields: map[string]string{"load_balancer_arn": ""},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerArn: nil,
		},
	}
	checker := elbCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN)", result.Count)
	}
}
