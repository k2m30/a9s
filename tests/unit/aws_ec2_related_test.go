package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ec2CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ec2 related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ec2 related checker for %s not found", target)
	return nil
}

func TestEC2RelatedCheckers_NoUnknownCounts(t *testing.T) {
	instance := resource.Resource{
		ID: "i-abc123",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-abc123"),
			VpcId:      aws.String("vpc-123"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("app-stack")},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-abc")}},
			},
		},
	}

	cache := resource.ResourceCache{
		"tg": {
			{
				ID: "tg-web",
				RawStruct: elbv2types.TargetGroup{
					VpcId:      aws.String("vpc-123"),
					TargetType: elbv2types.TargetTypeEnumInstance,
				},
			},
		},
		"asg": {
			{
				ID: "asg-web",
				RawStruct: asgtypes.AutoScalingGroup{
					Instances: []asgtypes.Instance{{InstanceId: aws.String("i-abc123")}},
				},
			},
		},
		"alarm": {
			{
				ID: "cpu-high",
				RawStruct: cwtypes.MetricAlarm{
					Dimensions: []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-abc123")}},
				},
			},
		},
		"cfn": {
			{
				ID: "app-stack",
				RawStruct: cfntypes.Stack{
					StackName: aws.String("app-stack"),
				},
			},
		},
		"eip": {
			{
				ID: "eipalloc-abc",
				RawStruct: ec2types.Address{
					InstanceId: aws.String("i-abc123"),
				},
			},
		},
		"ebs-snap": {
			{
				ID: "snap-abc",
				RawStruct: ec2types.Snapshot{
					VolumeId: aws.String("vol-abc"),
				},
			},
		},
	}

	targets := []string{"tg", "asg", "alarm", "cfn", "eip", "ebs-snap"}
	for _, target := range targets {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), nil, instance, cache)
		if got.Count < 0 {
			t.Fatalf("%s checker returned unknown count: %+v", target, got)
		}
		if got.Count == 0 {
			t.Fatalf("%s checker returned zero count with matching fixture cache: %+v", target, got)
		}
		if len(got.ResourceIDs) == 0 {
			t.Fatalf("%s checker returned empty ResourceIDs with positive count: %+v", target, got)
		}
	}
}

func TestEC2NavigableFields_IncludeSecurityGroupsGroupID(t *testing.T) {
	fields := resource.GetNavigableFields("ec2")
	if len(fields) == 0 {
		t.Fatal("resource.GetNavigableFields(\"ec2\") returned empty")
	}

	found := false
	for _, f := range fields {
		if f.FieldPath == "SecurityGroups.GroupId" && f.TargetType == "sg" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("ec2 navigable fields must include SecurityGroups.GroupId -> sg; got: %+v", fields)
	}
}

// Bug reveal: real-profile EC2 detail counts currently depend on the destination
// list already being present in cache. Related counts should be derivable without
// pre-warming :asg/:eip first.
func TestEC2RelatedCheckers_ASGDoesNotRequirePrewarmedCache(t *testing.T) {
	instance := resource.Resource{
		ID: "i-real-asg",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-real-asg"),
			VpcId:      aws.String("vpc-123"),
		},
	}

	checker := ec2CheckerByTarget(t, "asg")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count == -1 {
		t.Fatalf("asg related count should not be unknown just because the asg list was not preloaded; got %+v", got)
	}
}

// Bug reveal: EIP count is also cache-dependent today.
func TestEC2RelatedCheckers_EIPDoesNotRequirePrewarmedCache(t *testing.T) {
	instance := resource.Resource{
		ID: "i-real-eip",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-real-eip"),
			VpcId:      aws.String("vpc-123"),
		},
	}

	checker := ec2CheckerByTarget(t, "eip")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count == -1 {
		t.Fatalf("eip related count should not be unknown just because the eip list was not preloaded; got %+v", got)
	}
}

// Bug reveal: live EC2 detail shows EKS node group rows but they are not backed
// by a checker, so the row never becomes actionable.
func TestEC2RelatedRegistry_NodeGroupsHasChecker(t *testing.T) {
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType != "ng" {
			continue
		}
		if def.Checker == nil {
			t.Fatal("ec2 related definition for ng must have a checker so EKS node group relationships can be counted and opened")
		}
		return
	}
	t.Fatal("ec2 related definition for ng not found")
}

// Bug reveal: live EC2 detail shows CloudTrail Events but the row is currently a
// non-counted placeholder.
func TestEC2RelatedRegistry_CloudTrailEventsHasChecker(t *testing.T) {
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType != "ct-events" {
			continue
		}
		if def.Checker == nil {
			t.Fatal("ec2 related definition for ct-events must have a checker so CloudTrail event relationships can be counted and opened")
		}
		return
	}
	t.Fatal("ec2 related definition for ct-events not found")
}
