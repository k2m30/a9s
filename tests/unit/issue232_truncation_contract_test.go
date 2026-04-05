// issue232_truncation_contract_test.go contains specification-driven tests for
// issue #232: EC2 related checkers must return Count=-1 (unknown) when the
// cache is truncated and 0 local matches are found. A partial page cannot be
// treated as a definitive zero.
//
// Tests 1-4 FAIL against pre-fix code (ASG, EIP, NodeGroups, CT-Events discard
// the isTruncated return value). Tests 5-10 must always PASS.
package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// trunc232Instance is the EC2 instance used across all truncation-contract tests.
// It belongs to cluster "cluster-A" and stack "stack-trunc" so that node-group
// and CFN checkers can enter their matching loops.
var trunc232Instance = resource.Resource{
	ID: "i-test-trunc",
	RawStruct: ec2types.Instance{
		InstanceId: aws.String("i-test-trunc"),
		VpcId:      aws.String("vpc-trunc"),
		Tags: []ec2types.Tag{
			{Key: aws.String("eks:cluster-name"), Value: aws.String("cluster-A")},
			{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("stack-trunc")},
		},
	},
}

// ---------------------------------------------------------------------------
// Tests 1-4: buggy checkers — MUST fail against pre-fix code
// ---------------------------------------------------------------------------

// TestContract_TruncatedZeroMatch_ASG_ReturnsUnknown verifies that the ASG
// checker returns Count=-1 when the cache entry is truncated and no ASG in the
// partial list contains the instance.
func TestContract_TruncatedZeroMatch_ASG_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"asg": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "asg-other",
					RawStruct: asgtypes.AutoScalingGroup{
						AutoScalingGroupName: aws.String("asg-other"),
						Instances: []asgtypes.Instance{
							{InstanceId: aws.String("i-some-other-instance")},
						},
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "asg")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("ASG checker with truncated cache and 0 matches: want Count=-1, got Count=%d (bug: isTruncated return value is discarded)", got.Count)
	}
}

// TestContract_TruncatedZeroMatch_EIP_ReturnsUnknown verifies that the EIP
// checker returns Count=-1 when the cache entry is truncated and no EIP in the
// partial list is associated with the instance.
func TestContract_TruncatedZeroMatch_EIP_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"eip": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "eipalloc-other",
					RawStruct: ec2types.Address{
						AllocationId: aws.String("eipalloc-other"),
						InstanceId:   aws.String("i-other"),
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "eip")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("EIP checker with truncated cache and 0 matches: want Count=-1, got Count=%d (bug: isTruncated return value is discarded)", got.Count)
	}
}

// TestContract_TruncatedZeroMatch_NodeGroups_ReturnsUnknown verifies that the
// NodeGroups checker returns Count=-1 when the cache entry is truncated and
// the only node group in the partial list belongs to a different cluster.
//
// The instance has tag eks:cluster-name=cluster-A; the fixture NG has
// ClusterName=cluster-B so the checker skips it via the clusterName != rawClusterName
// guard — but because the cache is truncated it cannot conclude there are 0 NGs.
func TestContract_TruncatedZeroMatch_NodeGroups_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"ng": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "ng-cluster-b-workers",
					RawStruct: ekstypes.Nodegroup{
						ClusterName:   aws.String("cluster-B"),
						NodegroupName: aws.String("workers"),
					},
					Fields: map[string]string{
						"cluster_name":   "cluster-B",
						"nodegroup_name": "workers",
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ng")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("NodeGroups checker with truncated cache and 0 matches: want Count=-1, got Count=%d (bug: isTruncated return value is discarded)", got.Count)
	}
}

// TestContract_TruncatedZeroMatch_CloudTrailEvents_ReturnsUnknown verifies that
// the CloudTrail events checker returns Count=-1 when the cache entry is
// truncated and no event in the partial list references the instance.
func TestContract_TruncatedZeroMatch_CloudTrailEvents_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"ct-events": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "event-unrelated-001",
					RawStruct: cloudtrailtypes.Event{
						EventId: aws.String("event-unrelated-001"),
						Resources: []cloudtrailtypes.Resource{
							{ResourceName: aws.String("i-completely-different")},
						},
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ct-events")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("CloudTrail events checker with truncated cache and 0 matches: want Count=-1, got Count=%d (bug: isTruncated return value is discarded)", got.Count)
	}
}

// ---------------------------------------------------------------------------
// Tests 5-6: truncation MUST NOT suppress confirmed matches (positive control)
// ---------------------------------------------------------------------------

// TestContract_TruncatedWithMatch_ASG_ReturnsCount ensures that when an ASG
// in the (truncated) partial list does contain the instance, the checker still
// returns a positive count — truncation only upgrades zero-match, not all results.
func TestContract_TruncatedWithMatch_ASG_ReturnsCount(t *testing.T) {
	cache := resource.ResourceCache{
		"asg": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "asg-matching",
					RawStruct: asgtypes.AutoScalingGroup{
						AutoScalingGroupName: aws.String("asg-matching"),
						Instances: []asgtypes.Instance{
							{InstanceId: aws.String("i-test-trunc")},
						},
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "asg")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count < 1 {
		t.Errorf("ASG checker with matching instance in truncated cache: want Count>=1, got Count=%d", got.Count)
	}
}

// TestContract_TruncatedWithMatch_EIP_ReturnsCount ensures that when an EIP in
// the (truncated) partial list is associated with the instance, the checker
// still returns a positive count.
func TestContract_TruncatedWithMatch_EIP_ReturnsCount(t *testing.T) {
	cache := resource.ResourceCache{
		"eip": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "eipalloc-match",
					RawStruct: ec2types.Address{
						AllocationId: aws.String("eipalloc-match"),
						InstanceId:   aws.String("i-test-trunc"),
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "eip")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count < 1 {
		t.Errorf("EIP checker with matching EIP in truncated cache: want Count>=1, got Count=%d", got.Count)
	}
}

// ---------------------------------------------------------------------------
// Tests 7-10: regression pins for already-correct checkers
// ---------------------------------------------------------------------------

// TestContract_TruncatedZeroMatch_TG_ReturnsUnknown pins the correct behavior
// of the target-group checker, which already handles truncation properly.
func TestContract_TruncatedZeroMatch_TG_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"tg": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "tg-other-vpc",
					RawStruct: elbv2types.TargetGroup{
						VpcId:      aws.String("vpc-completely-different"),
						TargetType: elbv2types.TargetTypeEnumInstance,
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "tg")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("TG checker (regression pin) with truncated cache and 0 matches: want Count=-1, got Count=%d", got.Count)
	}
}

// TestContract_TruncatedZeroMatch_Alarm_ReturnsUnknown pins the correct behavior
// of the CloudWatch alarm checker, which already handles truncation properly.
func TestContract_TruncatedZeroMatch_Alarm_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"alarm": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "alarm-other-instance",
					RawStruct: cwtypes.MetricAlarm{
						AlarmName: aws.String("alarm-other-instance"),
						Dimensions: []cwtypes.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-different-instance")},
						},
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("Alarm checker (regression pin) with truncated cache and 0 matches: want Count=-1, got Count=%d", got.Count)
	}
}

// TestContract_TruncatedZeroMatch_CFN_ReturnsUnknown pins the correct behavior
// of the CloudFormation checker, which already handles truncation properly.
// The instance has stack-name=stack-trunc; the fixture stack has a different name.
func TestContract_TruncatedZeroMatch_CFN_ReturnsUnknown(t *testing.T) {
	cache := resource.ResourceCache{
		"cfn": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "other-stack",
					RawStruct: cfntypes.Stack{
						StackName: aws.String("other-stack"),
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "cfn")
	got := checker(context.Background(), nil, trunc232Instance, cache)

	if got.Count != -1 {
		t.Errorf("CFN checker (regression pin) with truncated cache and 0 matches: want Count=-1, got Count=%d", got.Count)
	}
}

// TestContract_TruncatedZeroMatch_EBSSnap_ReturnsUnknown pins the correct behavior
// of the EBS snapshot checker, which already handles truncation properly.
// The instance has one attached volume (vol-trunc-abc); the snapshot in the
// truncated cache references a different volume.
func TestContract_TruncatedZeroMatch_EBSSnap_ReturnsUnknown(t *testing.T) {
	instanceWithVolume := resource.Resource{
		ID: "i-test-trunc",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-test-trunc"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-trunc-abc")}},
			},
		},
	}

	cache := resource.ResourceCache{
		"ebs-snap": {
			IsTruncated: true,
			Resources: []resource.Resource{
				{
					ID: "snap-unrelated",
					RawStruct: ec2types.Snapshot{
						SnapshotId: aws.String("snap-unrelated"),
						VolumeId:   aws.String("vol-completely-different"),
					},
				},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs-snap")
	got := checker(context.Background(), nil, instanceWithVolume, cache)

	if got.Count != -1 {
		t.Errorf("EBSSnap checker (regression pin) with truncated cache and 0 matches: want Count=-1, got Count=%d", got.Count)
	}
}
