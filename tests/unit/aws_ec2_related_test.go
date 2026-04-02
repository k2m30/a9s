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
