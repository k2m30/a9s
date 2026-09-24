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
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
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
		"tg": {Resources: []resource.Resource{
			{
				ID: "tg-web",
				RawStruct: elbv2types.TargetGroup{
					VpcId:      aws.String("vpc-123"),
					TargetType: elbv2types.TargetTypeEnumInstance,
				},
			},
		}},
		"asg": {Resources: []resource.Resource{
			{
				ID: "asg-web",
				RawStruct: asgtypes.AutoScalingGroup{
					Instances: []asgtypes.Instance{{InstanceId: aws.String("i-abc123")}},
				},
			},
		}},
		"alarm": {Resources: []resource.Resource{
			{
				ID: "cpu-high",
				RawStruct: cwtypes.MetricAlarm{
					Namespace:  aws.String("AWS/EC2"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-abc123")}},
				},
			},
		}},
		"cfn": {Resources: []resource.Resource{
			{
				ID: "app-stack",
				RawStruct: cfntypes.Stack{
					StackName: aws.String("app-stack"),
				},
			},
		}},
		"eip": {Resources: []resource.Resource{
			{
				ID: "eipalloc-abc",
				RawStruct: ec2types.Address{
					InstanceId: aws.String("i-abc123"),
				},
			},
		}},
		"ebs-snap": {Resources: []resource.Resource{
			{
				ID: "snap-abc",
				RawStruct: ec2types.Snapshot{
					VolumeId: aws.String("vol-abc"),
				},
			},
		}},
	}

	// tg is not here: membership is DescribeTargetHealth's answer, which a
	// call without clients cannot read (TestRelated_EC2_TG_Found).
	targets := []string{"asg", "alarm", "cfn", "eip", "ebs-snap"}
	for _, target := range targets {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), nil, instance, cache)
		if got.Count() < 0 {
			t.Fatalf("%s checker returned unknown count: %+v", target, got)
		}
		if got.Count() == 0 {
			t.Fatalf("%s checker returned zero count with matching fixture cache: %+v", target, got)
		}
		if len(got.ResourceIDs()) == 0 {
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

func TestEC2RelatedCheckers_EBS_MatchesVolumeIDs(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-multi",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-multi"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-abc")}},
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-def")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 2 {
		t.Errorf("checkEC2EBS: expected count=2, got %d", got.Count())
	}
	if len(got.ResourceIDs()) != 2 {
		t.Errorf("checkEC2EBS: expected 2 ResourceIDs, got %v", got.ResourceIDs())
	}
	if got.ResourceIDs()[0] != "vol-abc" || got.ResourceIDs()[1] != "vol-def" {
		t.Errorf("checkEC2EBS: expected sorted [vol-abc, vol-def], got %v", got.ResourceIDs())
	}
}

func TestEC2RelatedCheckers_EBS_NoVolumes(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-none",
		RawStruct: ec2types.Instance{
			InstanceId:          aws.String("i-ebs-none"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 0 {
		t.Errorf("checkEC2EBS with empty BlockDeviceMappings: expected count=0, got %d", got.Count())
	}
	if len(got.ResourceIDs()) != 0 {
		t.Errorf("checkEC2EBS with empty BlockDeviceMappings: expected no ResourceIDs, got %v", got.ResourceIDs())
	}
}

func TestEC2RelatedCheckers_EBS_NilEbs(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-nil",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-nil"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: nil},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 0 {
		t.Errorf("checkEC2EBS with nil Ebs: expected count=0, got %d", got.Count())
	}
}

func TestEC2RelatedCheckers_EBS_NilVolumeId(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-nil-volid",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-nil-volid"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: nil}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 0 {
		t.Errorf("checkEC2EBS with nil VolumeId: expected count=0, got %d", got.Count())
	}
}

func TestEC2RelatedCheckers_EBS_SingleVolume(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-single",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-single"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-only")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 1 {
		t.Errorf("checkEC2EBS single volume: expected count=1, got %d", got.Count())
	}
	if len(got.ResourceIDs()) != 1 || got.ResourceIDs()[0] != "vol-only" {
		t.Errorf("checkEC2EBS single volume: expected ResourceIDs=[vol-only], got %v", got.ResourceIDs())
	}
}

func TestEC2RelatedCheckers_EBS_DeduplicatesVolumeIDs(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-dedup",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-dedup"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-dup")}},
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-dup")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 1 {
		t.Errorf("checkEC2EBS should deduplicate volume IDs; expected count=1, got %d", got.Count())
	}
}

func TestEC2RelatedCheckers_EBS_NonEC2RawStruct(t *testing.T) {
	instance := resource.Resource{
		ID:        "i-wrong-type",
		RawStruct: "not-an-ec2-instance",
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count() != 0 {
		t.Errorf("checkEC2EBS with wrong RawStruct type: expected count=0, got %d", got.Count())
	}
}

func TestResourceCacheEntry_IsTruncated_Propagates(t *testing.T) {
	instance := resource.Resource{
		ID: "i-truncated-test",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-truncated-test"),
			VpcId:      aws.String("vpc-999"),
		},
	}

	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID: "alarm-other-instance",
					RawStruct: cwtypes.MetricAlarm{
						Dimensions: []cwtypes.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-other")},
						},
					},
				},
			},
			IsTruncated: true,
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	got := checker(context.Background(), nil, instance, cache)

	if got.Count() != 0 {
		t.Errorf("alarm checker with truncated cache and 0 matches: want Count=0, got Count=%d", got.Count())
	}
	if !got.Truncated() {
		t.Errorf("alarm checker with truncated cache and 0 matches: want Truncated=true, got false")
	}
}

func TestNavigableFields_EC2_Registered(t *testing.T) {
	expected := map[string]string{
		"VpcId":                            "vpc",
		"SubnetId":                         "subnet",
		"ImageId":                          "ami",
		"BlockDeviceMappings.Ebs.VolumeId": "ebs",
		"SecurityGroups.GroupId":           "sg",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigableForTest("ec2", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not registered for ec2", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("ec2 navigable field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

func TestNavigableFields_EC2_FieldPathsResolve(t *testing.T) {
	ec2Client := fakes.NewEC2()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil {
		t.Fatalf("FetchEC2Instances via EC2 fake: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("demo fixture returned no resources for ec2")
	}
	r := resources[0]

	fields := resource.GetNavigableFields("ec2")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ec2")
	}

	for _, nav := range fields {
		items := fieldpath.ExtractFieldList(r.RawStruct, r.Fields, []string{nav.FieldPath}, nil)
		found := false
		for _, item := range items {
			if item.Value != "" && item.Value != "-" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("NavigableField.FieldPath %q resolved to empty/missing value in ec2 demo fixture", nav.FieldPath)
		}
	}
}

// ec2TargetHealthFake answers DescribeTargetHealth with the instances
// registered in each target group, keyed by target group ARN.
type ec2TargetHealthFake struct {
	awsclient.ELBv2API
	registered map[string][]string
}

func (f *ec2TargetHealthFake) DescribeTargetHealth(_ context.Context, in *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	out := &elbv2.DescribeTargetHealthOutput{}
	for _, id := range f.registered[aws.ToString(in.TargetGroupArn)] {
		out.TargetHealthDescriptions = append(out.TargetHealthDescriptions, elbv2types.TargetHealthDescription{
			Target:       &elbv2types.TargetDescription{Id: aws.String(id), Port: aws.Int32(80)},
			TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
		})
	}
	return out, nil
}

// An instance's target groups are the ones DescribeTargetHealth reports it
// registered in; a group in the same VPC that does not register it is not.
func TestRelated_EC2_TG_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			VpcId:      aws.String("vpc-match"),
		},
	}
	const registeredARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg-1/1111111111111111"
	const otherARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg-3/3333333333333333"
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "tg-1",
				RawStruct: elbv2types.TargetGroup{
					TargetGroupArn: aws.String(registeredARN),
					VpcId:          aws.String("vpc-match"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
			},
			{
				ID: "tg-3",
				RawStruct: elbv2types.TargetGroup{
					TargetGroupArn: aws.String(otherARN),
					VpcId:          aws.String("vpc-match"),
					TargetType:     elbv2types.TargetTypeEnumInstance,
				},
			},
		}},
	}
	clients := &awsclient.ServiceClients{ELBv2: &ec2TargetHealthFake{registered: map[string][]string{
		registeredARN: {"i-match"},
		otherARN:      {"i-other"},
	}}}

	checker := ec2CheckerByTarget(t, "tg")
	result := checker(context.Background(), clients, instance, cache)

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "tg-1" {
		t.Errorf("ResourceIDs = %v (state %v), want [tg-1]", ids, result.State())
	}
}

func TestRelated_EC2_TG_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			VpcId:      aws.String("vpc-match"),
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "tg-2",
				RawStruct: elbv2types.TargetGroup{
					VpcId:      aws.String("vpc-other"),
					TargetType: elbv2types.TargetTypeEnumInstance,
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_TG_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			VpcId:      aws.String("vpc-match"),
		},
	}

	checker := ec2CheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_TG_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty instance ID", result.Count())
	}
}

func TestRelated_EC2_ASG_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "asg-1",
				RawStruct: asgtypes.AutoScalingGroup{
					Instances: []asgtypes.Instance{{InstanceId: aws.String("i-match")}},
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0", result.Count())
	}
}

func TestRelated_EC2_ASG_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "asg-1",
				RawStruct: asgtypes.AutoScalingGroup{
					Instances: []asgtypes.Instance{{InstanceId: aws.String("i-other")}},
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_ASG_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}

	checker := ec2CheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_ASG_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty instance ID", result.Count())
	}
}

func TestRelated_EC2_Alarm_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "alarm-1",
				RawStruct: cwtypes.MetricAlarm{
					Namespace: aws.String("AWS/EC2"),
					Dimensions: []cwtypes.Dimension{
						{Name: aws.String("InstanceId"), Value: aws.String("i-match")},
					},
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0", result.Count())
	}
}

func TestRelated_EC2_Alarm_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "alarm-1",
				RawStruct: cwtypes.MetricAlarm{
					Dimensions: []cwtypes.Dimension{
						{Name: aws.String("InstanceId"), Value: aws.String("i-other")},
					},
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_Alarm_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_Alarm_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty instance ID", result.Count())
	}
}

func TestRelated_EC2_CFN_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "my-stack",
				RawStruct: cfntypes.Stack{
					StackName: aws.String("my-stack"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0", result.Count())
	}
}

func TestRelated_EC2_CFN_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "other-stack",
				RawStruct: cfntypes.Stack{
					StackName: aws.String("other-stack"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_CFN_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_CFN_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for instance with no CFN stack tag", result.Count())
	}
}

func TestRelated_EC2_EIP_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "eipalloc-abc",
				RawStruct: ec2types.Address{
					InstanceId: aws.String("i-match"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0", result.Count())
	}
}

func TestRelated_EC2_EIP_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "eipalloc-abc",
				RawStruct: ec2types.Address{
					InstanceId: aws.String("i-other"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_EIP_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}

	checker := ec2CheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_EIP_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "eip")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty instance ID", result.Count())
	}
}

func TestRelated_EC2_EBSSnap_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-match")}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "snap-1",
				RawStruct: ec2types.Snapshot{
					VolumeId: aws.String("vol-match"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0", result.Count())
	}
}

func TestRelated_EC2_EBSSnap_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-match")}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "snap-1",
				RawStruct: ec2types.Snapshot{
					VolumeId: aws.String("vol-other"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_EBSSnap_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-match")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_EBSSnap_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for instance with no volumes", result.Count())
	}
}

func TestRelated_EC2_NG_Found(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:cluster-name"), Value: aws.String("my-cluster")},
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("my-ng")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "my-ng",
				RawStruct: ekstypes.Nodegroup{
					ClusterName:   aws.String("my-cluster"),
					NodegroupName: aws.String("my-ng"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0", result.Count())
	}
}

func TestRelated_EC2_NG_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:cluster-name"), Value: aws.String("my-cluster")},
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("my-ng")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "other-ng",
				RawStruct: ekstypes.Nodegroup{
					ClusterName:   aws.String("other-cluster"),
					NodegroupName: aws.String("other-ng"),
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_NG_CacheMissNoClients(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:cluster-name"), Value: aws.String("my-cluster")},
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("my-ng")},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown, empty cache)", result.Count())
	}
}

func TestRelated_EC2_NG_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for instance with no EKS tags", result.Count())
	}
}

func TestRelated_EC2_CTEvents_NotFound(t *testing.T) {
	instance := resource.Resource{
		ID: "i-match",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-match"),
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "evt-1",
				RawStruct: cloudtrailtypes.Event{
					Resources: []cloudtrailtypes.Resource{
						{ResourceName: aws.String("i-other")},
					},
				},
			},
		}},
	}

	checker := ec2CheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, instance, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_EC2_CTEvents_EmptySourceID(t *testing.T) {
	instance := resource.Resource{
		ID:        "",
		RawStruct: ec2types.Instance{},
	}

	checker := ec2CheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty instance ID", result.Count())
	}
}
