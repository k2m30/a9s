package unit_test

import (
	"context"
	"errors"
	"testing"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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
	tgTestARN    = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123"
	tgTestELBARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/def456"
	tgOtherTGARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/other-tg/xyz789"
)

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
			VpcId:            new("vpc-abc123"),
		},
	}
}

func TestRelated_TG_ELB_Match(t *testing.T) {
	res := tgSrcResource()
	tgARN := tgTestELBARN

	// LoadBalancerArns proves membership only; the elb row ID is the
	// LoadBalancerName, which the ARN does not yield, so the elb cache supplies it.
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "my-alb",
				Fields: map[string]string{"load_balancer_arn": tgARN},
			},
		}},
	}

	checker := tgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-alb" {
		t.Errorf("ResourceIDs = %v, want [my-alb]", result.ResourceIDs())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_TG_NilClients(t *testing.T) {
	res := tgSrcResource()
	emptyCache := resource.ResourceCache{}

	for _, target := range []string{"ecs-svc", "asg"} {
		checker := tgCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, emptyCache)
		if result.State() != domain.RelatedUnknown {
			t.Errorf("target=%s: Count = %d, want -1 (nil clients, empty cache)", target, result.Count())
		}
	}
}

func TestRelated_TG_Alarm_Match(t *testing.T) {
	res := tgSrcResource()
	tgARNSuffix := "targetgroup/my-tg/abc123"
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "tg-unhealthy-alarm",
			RawStruct: cwtypes.MetricAlarm{
				Namespace: new("AWS/ApplicationELB"),
				Dimensions: []cwtypes.Dimension{
					{Name: new("TargetGroup"), Value: new(tgARNSuffix)},
				},
			},
		}}},
	}

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (alarm with matching TargetGroup dimension)", result.Count())
	}
}

func TestRelated_TG_Alarm_NoMatch(t *testing.T) {
	res := tgSrcResource()
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "other-alarm",
			RawStruct: cwtypes.MetricAlarm{
				Dimensions: []cwtypes.Dimension{
					{Name: new("TargetGroup"), Value: new("targetgroup/other-tg/xyz789")},
				},
			},
		}}},
	}

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (alarm with different TargetGroup dimension)", result.Count())
	}
}

func TestRelated_TG_Alarm_NilCache(t *testing.T) {
	res := tgSrcResource()

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil alarm cache is not a proven zero)", result.State())
	}
}

func TestRelated_TG_Alarm_WrongDimensionName_NotCounted(t *testing.T) {
	res := tgSrcResource()
	tgARNSuffix := "targetgroup/my-tg/abc123"
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID: "load-balancer-alarm",
			RawStruct: cwtypes.MetricAlarm{
				Dimensions: []cwtypes.Dimension{
					{Name: new("LoadBalancer"), Value: new(tgARNSuffix)},
				},
			},
		}}},
	}

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (dimension name is LoadBalancer, not TargetGroup)", result.Count())
	}
}

func TestRelated_TG_Alarm_Error(t *testing.T) {
	res := tgSrcResource()
	wantErr := errors.New("boom: DescribeAlarms throttled")

	original := resource.GetPaginatedFetcher("alarm")
	resource.SetPaginatedForTest("alarm", func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, wantErr
	})
	t.Cleanup(func() {
		if original != nil {
			resource.SetPaginatedForTest("alarm", original)
		} else {
			resource.CleanupPaginatedForTest("alarm")
		}
	})

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), &awsclient.ServiceClients{}, res, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError", result.State())
	}
	if result.Err() == nil {
		t.Error("Err = nil, want the propagated fetch error")
	}
}

// A truncated page with a match renders "(1+)", not "(1)".
func TestRelated_TG_Alarm_Truncated_PropagatesTrue(t *testing.T) {
	res := tgSrcResource()
	tgARNSuffix := "targetgroup/my-tg/abc123"
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{
				ID: "tg-unhealthy-alarm",
				RawStruct: cwtypes.MetricAlarm{
					Namespace: new("AWS/ApplicationELB"),
					Dimensions: []cwtypes.Dimension{
						{Name: new("TargetGroup"), Value: new(tgARNSuffix)},
					},
				},
			}},
			IsTruncated: true,
		},
	}

	checker := tgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (truncated cache page with a match must render as '(1+)')")
	}
}

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
