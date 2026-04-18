package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func elbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("elb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("elb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("elb related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_ELB_Registered(t *testing.T) {
	expected := map[string]string{
		"VpcId":                        "vpc",
		"SecurityGroups":               "sg",
		"AvailabilityZones.SubnetId":   "subnet",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("elb", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for elb", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

// --- Target Groups checker (Pattern C — cache, LoadBalancerArns match) ---

func TestRelated_ELB_TG_Found(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/abcdef1234567890"

	tgRes := resource.Resource{
		ID:   "test-tg",
		Name: "test-tg",
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/test-tg/1234567890abcdef"),
			TargetGroupName: aws.String("test-tg"),
			LoadBalancerArns: []string{elbARN},
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}
	source := resource.Resource{
		ID:   "test-lb",
		Name: "test-lb",
		Fields: map[string]string{
			"load_balancer_arn": elbARN,
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("test-lb"),
			LoadBalancerArn:  aws.String(elbARN),
		},
	}

	checker := elbCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "test-tg" {
		t.Errorf("ResourceIDs = %v, want [test-tg]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ELB_TG_NotFound(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/abcdef1234567890"
	const otherARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other-lb/1111111111111111"

	tgRes := resource.Resource{
		ID:   "other-tg",
		Name: "other-tg",
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/other-tg/9999999999999999"),
			TargetGroupName:  aws.String("other-tg"),
			LoadBalancerArns: []string{otherARN},
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}
	source := resource.Resource{
		ID:   "test-lb",
		Name: "test-lb",
		Fields: map[string]string{
			"load_balancer_arn": elbARN,
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("test-lb"),
			LoadBalancerArn:  aws.String(elbARN),
		},
	}

	checker := elbCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ELB_TG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "test-lb",
		Name: "test-lb",
		Fields: map[string]string{
			"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/abcdef1234567890",
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("test-lb"),
			LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/abcdef1234567890"),
		},
	}

	checker := elbCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ELB_TG_EmptyARN(t *testing.T) {
	tgRes := resource.Resource{
		ID:   "some-tg",
		Name: "some-tg",
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/some-tg/0000000000000001"),
			TargetGroupName:  aws.String("some-tg"),
			LoadBalancerArns: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/some-lb/1111111111111111"},
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}
	// Source has empty load_balancer_arn — should match nothing.
	source := resource.Resource{
		ID:   "",
		Name: "",
		Fields: map[string]string{
			"load_balancer_arn": "",
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerArn: nil,
		},
	}

	checker := elbCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty ARN", result.Count)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, LoadBalancer dimension) ---

func TestRelated_ELB_Alarms_Found(t *testing.T) {
	// ELB ARN suffix is used as the dimension value for "LoadBalancer" dimension.
	// Full ARN: .../loadbalancer/app/my-lb/abc123  → suffix: app/my-lb/abc123
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc123"
	const dimensionValue = "app/my-lb/abc123"

	alarmRes := resource.Resource{
		ID: "elb-5xx-errors",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("elb-5xx-errors"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("LoadBalancer"), Value: aws.String(dimensionValue)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   "my-lb",
		Name: "my-lb",
		Fields: map[string]string{
			"load_balancer_arn": elbARN,
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("my-lb"),
			LoadBalancerArn:  aws.String(elbARN),
		},
	}

	checker := elbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "elb-5xx-errors" {
		t.Errorf("ResourceIDs = %v, want [elb-5xx-errors]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ELB_Alarms_NotFound(t *testing.T) {
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc123"

	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("LoadBalancer"), Value: aws.String("app/other-lb/zzz999")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   "my-lb",
		Name: "my-lb",
		Fields: map[string]string{
			"load_balancer_arn": elbARN,
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("my-lb"),
			LoadBalancerArn:  aws.String(elbARN),
		},
	}

	checker := elbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ELB_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "my-lb",
		Name: "my-lb",
		Fields: map[string]string{
			"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc123",
		},
		RawStruct: elbv2types.LoadBalancer{
			LoadBalancerName: aws.String("my-lb"),
			LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc123"),
		},
	}

	checker := elbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- elb→cfn: requires DescribeTags per ELB (outside cache budget) ---

// TestRelated_ELB_CFN_Unknown: valid ELB → Count: -1 (tags not in DescribeLoadBalancers).
func TestRelated_ELB_CFN_Unknown(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-web",
		Name: "acme-prod-web",
		Fields: map[string]string{
			"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/abcdef1234567890",
		},
	}
	checker := elbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: tags via DescribeTags per ELB)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// TestRelated_ELB_CFN_EmptyInput: no identity → Count: 0.
func TestRelated_ELB_CFN_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := elbCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty identity)", result.Count)
	}
}

// --- elb→r53: requires per-zone ListResourceRecordSets (outside cache budget) ---

// TestRelated_ELB_R53_Unknown: ELB with dns_name → Count: -1 (records per-zone).
func TestRelated_ELB_R53_Unknown(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-web",
		Name: "acme-prod-web",
		Fields: map[string]string{
			"dns_name": "acme-prod-web-1234.us-east-1.elb.amazonaws.com",
		},
	}
	checker := elbCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: alias records per-zone)", result.Count)
	}
	if result.TargetType != "r53" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "r53")
	}
}

// TestRelated_ELB_R53_EmptyInput: no dns_name → Count: 0.
func TestRelated_ELB_R53_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := elbCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty dns_name)", result.Count)
	}
}
