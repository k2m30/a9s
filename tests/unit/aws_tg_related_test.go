package unit_test

import (
	"context"
	"testing"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tgCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func tgCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("tg") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("tg related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("tg related checker for %s not found", target)
	return nil
}

const (
	tgTestARN      = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123"
	tgTestELBARN   = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/def456"
	tgOtherELBARN  = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other-alb/999999"
	tgOtherTGARN   = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/other-tg/xyz789"
)

// tgSrcResource returns a canonical test resource for the TG.
func tgSrcResource() resource.Resource {
	tgARN := tgTestARN
	return resource.Resource{
		ID:   "my-tg",
		Name: "my-tg",
		Fields: map[string]string{
			"target_group_name": "my-tg",
			"target_group_arn":  tgARN,
			"vpc_id":            "vpc-abc123",
			"target_type":       "instance",
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:   &tgARN,
			LoadBalancerArns: []string{tgTestELBARN},
			VpcId:            strPtr("vpc-abc123"),
		},
	}
}

// --- ELB checker tests (Pattern F — reads LoadBalancerArns from TG RawStruct) ---

func TestRelated_TG_ELB_Match(t *testing.T) {
	res := tgSrcResource()

	checker := tgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_TG_ELB_Empty(t *testing.T) {
	tgARN := tgTestARN
	res := resource.Resource{
		ID:   "my-tg-no-elb",
		Name: "my-tg-no-elb",
		Fields: map[string]string{
			"target_group_arn": tgARN,
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:   &tgARN,
			LoadBalancerArns: []string{},
		},
	}

	checker := tgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- ECS service checker tests (Pattern C — reverse cache lookup) ---

func TestRelated_TG_ECSSvc_Match(t *testing.T) {
	res := tgSrcResource()
	tgARN := tgTestARN
	cache := resource.ResourceCache{
		"ecs-svc": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "my-svc",
				RawStruct: ecstypes.Service{
					LoadBalancers: []ecstypes.LoadBalancer{
						{TargetGroupArn: &tgARN},
					},
				},
			},
		}},
	}

	checker := tgCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_TG_ECSSvc_NoMatch(t *testing.T) {
	res := tgSrcResource()
	otherARN := tgOtherTGARN
	cache := resource.ResourceCache{
		"ecs-svc": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "other-svc",
				RawStruct: ecstypes.Service{
					LoadBalancers: []ecstypes.LoadBalancer{
						{TargetGroupArn: &otherARN},
					},
				},
			},
		}},
	}

	checker := tgCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- ASG checker tests (Pattern C — reverse cache lookup) ---

func TestRelated_TG_ASG_Match(t *testing.T) {
	res := tgSrcResource()
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "my-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					TargetGroupARNs: []string{tgTestARN},
				},
			},
		}},
	}

	checker := tgCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_TG_ASG_NoMatch(t *testing.T) {
	res := tgSrcResource()
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "other-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					TargetGroupARNs: []string{tgOtherTGARN},
				},
			},
		}},
	}

	checker := tgCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- Nil clients / empty cache tests ---

// TestRelated_TG_NilClients verifies that cache-dependent checkers return -1
// when both clients are nil and the cache has no entry.
func TestRelated_TG_NilClients(t *testing.T) {
	res := tgSrcResource()
	emptyCache := resource.ResourceCache{}

	for _, target := range []string{"ecs-svc", "asg"} {
		checker := tgCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (nil clients, empty cache)", target, result.Count)
		}
	}
}

// --- Alarm checker tests (Pattern C — reverse cache lookup via TargetGroup dimension) ---

func TestRelated_TG_Alarm_Match(t *testing.T) {
	res := tgSrcResource()
	tgARNSuffix := "targetgroup/my-tg/abc123"
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "tg-unhealthy-alarm",
			RawStruct: cwtypes.MetricAlarm{
				Dimensions: []cwtypes.Dimension{
					{Name: strPtr("TargetGroup"), Value: strPtr(tgARNSuffix)},
				},
			},
		}}},
	}

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (alarm with matching TargetGroup dimension)", result.Count)
	}
}

func TestRelated_TG_Alarm_NoMatch(t *testing.T) {
	res := tgSrcResource()
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "other-alarm",
			RawStruct: cwtypes.MetricAlarm{
				Dimensions: []cwtypes.Dimension{
					{Name: strPtr("TargetGroup"), Value: strPtr("targetgroup/other-tg/xyz789")},
				},
			},
		}}},
	}

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (alarm with different TargetGroup dimension)", result.Count)
	}
}

// --- Stub checker assertions ---

func TestRelated_TG_CfnStub(t *testing.T) {
	defs := resource.GetRelated("tg")
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Error("tg cfn: expected nil Checker (stub)")
			}
			return
		}
	}
	t.Error("tg cfn related def not found")
}

// --- NavigableFields test ---

func TestNavigableFields_TG(t *testing.T) {
	fields := resource.GetNavigableFields("tg")
	found := false
	for _, f := range fields {
		if f.FieldPath == "VpcId" && f.TargetType == "vpc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("tg NavigableField VpcId→vpc not registered")
	}
}

// --- Demo checker test ---

func TestRelatedDemo_TG_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("tg")
	if checker == nil {
		t.Fatal("no demo checker registered for tg")
	}

	// Use a known fixture ID that returns non-zero counts.
	src := resource.Resource{ID: "acme-web-tg"}
	results := checker(src)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"elb": false, "ecs-svc": false, "asg": false, "alarm": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// At least one result should have Count > 0.
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
