package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_ASG_Registered verifies all 11 related defs are registered with correct checker presence.
func TestRelated_ASG_Registered(t *testing.T) {
	defs := resource.GetRelated("asg")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for asg")
	}

	checkerExpected := map[string]bool{
		"ec2":    true, // non-nil
		"tg":     true, // non-nil
		"subnet": true, // non-nil
		"alarm":  true, // non-nil
		"ng":     true, // non-nil
		"ami":    true, // non-nil (T009)
		"elb":    true, // non-nil (T010)
		"role":   true, // non-nil (T011)
		"sg":     true, // non-nil (T012)
		"sns":    true, // non-nil (T013)
		"vpc":    true, // non-nil (T014)
	}
	for target, wantChecker := range checkerExpected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				hasChecker := def.Checker != nil
				if hasChecker != wantChecker {
					t.Errorf("asg %q: Checker presence = %v, want %v", target, hasChecker, wantChecker)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func asgCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("asg") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("asg related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("asg related checker for %s not found", target)
	return nil
}

// --- checkAsgAlarm tests (Pattern D — dimension-based) ---

func TestRelated_ASG_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "asg-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("asg-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("my-asg"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "asg-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [asg-cpu-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ASG_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "asg-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("asg-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("other-asg"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ASG_Alarm_EmptyID(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "asg-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("asg-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("my-asg"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_ASG_Alarm_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- checkAsgNG tests (Pattern C — target-cache match by ASG name) ---

func TestRelated_ASG_NG_MatchByASGName(t *testing.T) {
	ngRes := resource.Resource{
		ID:     "my-node-group",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("my-node-group"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("my-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-node-group" {
		t.Errorf("ResourceIDs = %v, want [my-node-group]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ASG_NG_NoMatch(t *testing.T) {
	ngRes := resource.Resource{
		ID:     "other-node-group",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("other-node-group"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("other-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ASG_NG_EmptyID(t *testing.T) {
	ngRes := resource.Resource{
		ID:     "my-node-group",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("my-node-group"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("my-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	res := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_ASG_NG_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkASGEC2 tests (Pattern F — no cache, reads Instances[] from RawStruct)
// ---------------------------------------------------------------------------

// TestRelated_ASG_EC2_MatchByInstances verifies that checkASGEC2 returns the
// instance IDs from the ASG Instances slice.
func TestRelated_ASG_EC2_MatchByInstances(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0abc111111111111a"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0bbb222222222222b"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
		},
	}

	checker := asgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	if result.ResourceIDs[0] != "i-0abc111111111111a" || result.ResourceIDs[1] != "i-0bbb222222222222b" {
		t.Errorf("ResourceIDs = %v, want [i-0abc111111111111a, i-0bbb222222222222b]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_EC2_NoInstances verifies that checkASGEC2 returns Count=0 when
// the ASG has no instances.
func TestRelated_ASG_EC2_NoInstances(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			Instances:            []asgtypes.Instance{},
		},
	}

	checker := asgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no instances)", result.Count)
	}
}

// TestRelated_ASG_EC2_NoRawStruct verifies that checkASGEC2 returns Count=-1 when
// the resource has no RawStruct (cannot extract instance data).
func TestRelated_ASG_EC2_NoRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: nil,
	}

	checker := asgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkASGTG tests (Pattern C — cache match by TargetGroupARN)
// ---------------------------------------------------------------------------

// TestRelated_ASG_TG_MatchByARN verifies that checkASGTG returns Count=1 when the
// ASG's TargetGroupARNs contains an ARN matching a target group in the cache.
func TestRelated_ASG_TG_MatchByARN(t *testing.T) {
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abcdef123456"
	tgRes := resource.Resource{
		ID:     "my-tg",
		Fields: map[string]string{},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:  aws.String(tgARN),
			TargetGroupName: aws.String("my-tg"),
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			TargetGroupARNs:      []string{tgARN},
		},
	}

	checker := asgCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (TG matched by ARN)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-tg" {
		t.Errorf("ResourceIDs = %v, want [my-tg]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_TG_NoTargetGroups verifies that checkASGTG returns Count=0 when
// the ASG has no TargetGroupARNs.
func TestRelated_ASG_TG_NoTargetGroups(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			TargetGroupARNs:      []string{},
		},
	}

	checker := asgCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no TargetGroupARNs)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkASGSubnets tests (Pattern F — no cache, parses VPCZoneIdentifier)
// ---------------------------------------------------------------------------

// TestRelated_ASG_Subnets_ParsesMultiple verifies that checkASGSubnets correctly
// parses a comma-separated VPCZoneIdentifier into individual subnet IDs.
func TestRelated_ASG_Subnets_ParsesMultiple(t *testing.T) {
	subnetA := "subnet-0aaa111111111111a"
	subnetB := "subnet-0bbb222222222222b"
	subnetC := "subnet-0ccc333333333333c"

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			VPCZoneIdentifier:    aws.String(subnetA + "," + subnetB + "," + subnetC),
		},
	}

	checker := asgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3 (3 subnets)", result.Count)
	}
	if len(result.ResourceIDs) != 3 {
		t.Fatalf("ResourceIDs length = %d, want 3; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_Subnets_EmptyIdentifier verifies that checkASGSubnets returns
// Count=0 when VPCZoneIdentifier is empty.
func TestRelated_ASG_Subnets_EmptyIdentifier(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			VPCZoneIdentifier:    aws.String(""),
		},
	}

	checker := asgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPCZoneIdentifier)", result.Count)
	}
}

// TestRelated_ASG_Subnets_NoRawStruct verifies that checkASGSubnets returns
// Count=-1 when the resource has no RawStruct.
func TestRelated_ASG_Subnets_NoRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: nil,
	}

	checker := asgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T009 — checkASGAMI (forward: LC.ImageId or LT.LaunchTemplateData.ImageId)
// ---------------------------------------------------------------------------

// TestRelated_ASG_AMI_MatchByLaunchTemplate verifies that checkASGAMI returns
// the AMI ID from a launch template version.
func TestRelated_ASG_AMI_MatchByLaunchTemplate(t *testing.T) {
	ltID := "lt-0abc1234567890abc"
	amiID := "ami-0abc111111111111a"

	fakeEC2 := newFakeEC2WithLaunchTemplateVersions([]ec2types.LaunchTemplateVersion{
		{
			LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
				ImageId: aws.String(amiID),
			},
		},
	})
	clients := &awsclient.ServiceClients{
		EC2:          fakeEC2,
		AutoScaling:  &fakeASGBatch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			LaunchTemplate: &asgtypes.LaunchTemplateSpecification{
				LaunchTemplateId: aws.String(ltID),
				Version:          aws.String("$Latest"),
			},
		},
	}

	checker := asgCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != amiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, amiID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_AMI_NoLaunchConfigOrTemplate verifies that checkASGAMI returns
// Count=0 when the ASG has no LaunchConfigurationName and no LaunchTemplate.
func TestRelated_ASG_AMI_NoLaunchConfigOrTemplate(t *testing.T) {
	clients := &awsclient.ServiceClients{
		EC2:         &fakeEC2Batch2{},
		AutoScaling: &fakeASGBatch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			// No LaunchConfigurationName, no LaunchTemplate
		},
	}

	checker := asgCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no LC or LT)", result.Count)
	}
}

// TestRelated_ASG_AMI_WrongRawStruct verifies that checkASGAMI returns
// Count=-1 when RawStruct is the wrong type.
func TestRelated_ASG_AMI_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: "not-an-asg",
	}

	checker := asgCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T010 — checkASGELB (forward: LoadBalancerNames + resolve TG ARNs via ELBv2)
// ---------------------------------------------------------------------------

// TestRelated_ASG_ELB_MatchByClassicELBNames verifies that checkASGELB returns
// classic ELB names directly from LoadBalancerNames.
func TestRelated_ASG_ELB_MatchByClassicELBNames(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			LoadBalancerNames:    []string{"my-classic-elb-1", "my-classic-elb-2"},
		},
	}

	// checkASGELB reads classic ELB names without calling AWS when only LoadBalancerNames present
	checker := asgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_ELB_NoLoadBalancers verifies that checkASGELB returns Count=0
// when the ASG has no LoadBalancerNames and no TargetGroupARNs.
func TestRelated_ASG_ELB_NoLoadBalancers(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			LoadBalancerNames:    []string{},
			TargetGroupARNs:      []string{},
		},
	}

	checker := asgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ELBs or TG ARNs)", result.Count)
	}
}

// TestRelated_ASG_ELB_WrongRawStruct verifies that checkASGELB returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_ASG_ELB_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: "not-an-asg",
	}

	checker := asgCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T011 — checkASGRole (forward: ServiceLinkedRoleARN + LC/LT IamInstanceProfile)
// ---------------------------------------------------------------------------

// TestRelated_ASG_Role_MatchByServiceLinkedRole verifies that checkASGRole
// returns the ServiceLinkedRoleARN directly as a role ID.
func TestRelated_ASG_Role_MatchByServiceLinkedRole(t *testing.T) {
	roleARN := "arn:aws:iam::123456789012:role/aws-service-role/autoscaling.amazonaws.com/AWSServiceRoleForAutoScaling"

	fakeASG := &fakeASGBatch2{}
	fakeIAM := &fakeIAMBatch2{}
	clients := &awsclient.ServiceClients{
		AutoScaling: fakeASG,
		IAM:         fakeIAM,
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			ServiceLinkedRoleARN: aws.String(roleARN),
		},
	}

	checker := asgCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (service-linked role found)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == roleARN {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %s", result.ResourceIDs, roleARN)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_Role_MatchByLaunchConfigInstanceProfile verifies that
// checkASGRole resolves the role from a LaunchConfig IamInstanceProfile.
func TestRelated_ASG_Role_MatchByLaunchConfigInstanceProfile(t *testing.T) {
	roleARN := "arn:aws:iam::123456789012:role/my-ec2-role"
	lcName := "my-launch-config"
	profileName := "my-instance-profile"

	fakeASG := newFakeASGWithLaunchConfig(lcName, profileName, nil)
	fakeIAM := newFakeIAMWithInstanceProfile([]iamtypes.Role{
		{Arn: aws.String(roleARN), RoleName: aws.String("my-ec2-role")},
	})
	clients := &awsclient.ServiceClients{
		AutoScaling: fakeASG,
		IAM:         fakeIAM,
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName:    aws.String("my-asg"),
			LaunchConfigurationName: aws.String(lcName),
		},
	}

	checker := asgCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (role from instance profile)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == roleARN {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %s", result.ResourceIDs, roleARN)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_Role_NoRoles verifies that checkASGRole returns Count=0
// when the ASG has no ServiceLinkedRoleARN and no launch config or template.
func TestRelated_ASG_Role_NoRoles(t *testing.T) {
	clients := &awsclient.ServiceClients{
		AutoScaling: &fakeASGBatch2{},
		IAM:         &fakeIAMBatch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			// No ServiceLinkedRoleARN, no LC, no LT
		},
	}

	checker := asgCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no roles)", result.Count)
	}
}

// TestRelated_ASG_Role_WrongRawStruct verifies that checkASGRole returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_ASG_Role_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: "not-an-asg",
	}

	checker := asgCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T012 — checkASGSG (forward: LC.SecurityGroups or LT.LaunchTemplateData.SecurityGroupIds)
// ---------------------------------------------------------------------------

// TestRelated_ASG_SG_MatchByLaunchConfig verifies that checkASGSG returns the
// security group IDs from the ASG's launch configuration.
func TestRelated_ASG_SG_MatchByLaunchConfig(t *testing.T) {
	lcName := "my-launch-config"
	sgID := "sg-0abc111111111111a"

	fakeASG := newFakeASGWithLaunchConfig(lcName, "", []string{sgID})
	clients := &awsclient.ServiceClients{
		AutoScaling: fakeASG,
		EC2:         &fakeEC2Batch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName:    aws.String("my-asg"),
			LaunchConfigurationName: aws.String(lcName),
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != sgID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, sgID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_SG_NoLaunchConfigOrTemplate verifies that checkASGSG returns
// Count=0 when the ASG has no launch config and no launch template.
func TestRelated_ASG_SG_NoLaunchConfigOrTemplate(t *testing.T) {
	clients := &awsclient.ServiceClients{
		AutoScaling: &fakeASGBatch2{},
		EC2:         &fakeEC2Batch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			// No LaunchConfigurationName, no LaunchTemplate
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no LC or LT)", result.Count)
	}
}

// TestRelated_ASG_SG_WrongRawStruct verifies that checkASGSG returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_ASG_SG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: "not-an-asg",
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T013 — checkASGSNS (forward: DescribeNotificationConfigurations.TopicARN +
//                              DescribeLifecycleHooks.NotificationTargetARN)
// ---------------------------------------------------------------------------

// TestRelated_ASG_SNS_MatchByNotificationConfig verifies that checkASGSNS returns
// the SNS topic ARN from a notification configuration.
func TestRelated_ASG_SNS_MatchByNotificationConfig(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789012:my-asg-notifications"
	asgName := "my-asg"

	fakeASG := newFakeASGWithNotifications([]asgtypes.NotificationConfiguration{
		{
			AutoScalingGroupName: aws.String(asgName),
			TopicARN:             aws.String(topicARN),
			NotificationType:     aws.String("autoscaling:EC2_INSTANCE_LAUNCH"),
		},
	})
	clients := &awsclient.ServiceClients{
		AutoScaling: fakeASG,
	}

	res := resource.Resource{
		ID:     asgName,
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String(asgName),
		},
	}

	checker := asgCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != topicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, topicARN)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_SNS_NoNotifications verifies that checkASGSNS returns Count=0
// when the ASG has no notification configurations and no lifecycle hooks.
func TestRelated_ASG_SNS_NoNotifications(t *testing.T) {
	clients := &awsclient.ServiceClients{
		AutoScaling: &fakeASGBatch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
		},
	}

	checker := asgCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no notifications)", result.Count)
	}
}

// TestRelated_ASG_SNS_WrongRawStruct verifies that checkASGSNS returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_ASG_SNS_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: "not-an-asg",
	}

	checker := asgCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T014 — checkASGVPC (forward: VPCZoneIdentifier → ec2:DescribeSubnets.VpcId)
// ---------------------------------------------------------------------------

// TestRelated_ASG_VPC_MatchBySubnets verifies that checkASGVPC resolves the
// VPC ID from the subnets in VPCZoneIdentifier.
func TestRelated_ASG_VPC_MatchBySubnets(t *testing.T) {
	subnetID := "subnet-0aaa111111111111a"
	vpcID := "vpc-0abc123456789def0"

	fakeEC2 := newFakeEC2WithSubnets([]ec2types.Subnet{
		{
			SubnetId: aws.String(subnetID),
			VpcId:    aws.String(vpcID),
		},
	})
	clients := &awsclient.ServiceClients{
		EC2: fakeEC2,
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			VPCZoneIdentifier:    aws.String(subnetID),
		},
	}

	checker := asgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != vpcID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, vpcID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_VPC_NoSubnets verifies that checkASGVPC returns Count=0
// when VPCZoneIdentifier is empty.
func TestRelated_ASG_VPC_NoSubnets(t *testing.T) {
	clients := &awsclient.ServiceClients{
		EC2: &fakeEC2Batch2{},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			VPCZoneIdentifier:    aws.String(""),
		},
	}

	checker := asgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPCZoneIdentifier)", result.Count)
	}
}

// TestRelated_ASG_VPC_WrongRawStruct verifies that checkASGVPC returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_ASG_VPC_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: "not-an-asg",
	}

	checker := asgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}
