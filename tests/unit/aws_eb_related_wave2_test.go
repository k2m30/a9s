// aws_eb_related_wave2_test.go — coverage wave 2 for eb_related.go checkers
// Covers: checkEbCFN, checkEbLogs, checkEbASG, checkEbEC2, checkEbAlarm
// Each has: happy-path (match → IDs), no-match, and one edge case.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// checkEbCFN — Pattern C: cache scan, stack name prefix "awseb-{envID}"
// ---------------------------------------------------------------------------

func TestRelated_Eb_CFN_MatchByEnvIDPrefix(t *testing.T) {
	const envID = "e-abcdef1234"
	const envName = "prod-env"
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/awseb-e-abcdef1234-stack/abc"

	cfnRes := resource.Resource{
		ID:   stackID,
		Name: "awseb-e-abcdef1234-stack",
		Fields: map[string]string{
			"stack_name": "awseb-e-abcdef1234-stack",
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}

	src := resource.Resource{
		ID:   envID,
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String(envID),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != stackID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, stackID)
	}
}

func TestRelated_Eb_CFN_NoMatchDifferentEnv(t *testing.T) {
	const envID = "e-abcdef1234"
	const otherEnvID = "e-zzzzzzz999"

	cfnRes := resource.Resource{
		ID:   "awseb-" + otherEnvID + "-stack",
		Name: "awseb-" + otherEnvID + "-stack",
		Fields: map[string]string{
			"stack_name": "awseb-" + otherEnvID + "-stack",
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}

	src := resource.Resource{
		ID:   envID,
		Name: "prod-env",
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String(envID),
			EnvironmentName: aws.String("prod-env"),
		},
	}

	checker := ebCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: no RawStruct — falls back to res.ID as envID.
func TestRelated_Eb_CFN_FallsBackToResID(t *testing.T) {
	const envID = "e-fallback1234"
	stackName := "awseb-" + envID + "-stack"

	cfnRes := resource.Resource{
		ID:   stackName,
		Name: stackName,
		Fields: map[string]string{
			"stack_name": stackName,
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}

	// RawStruct is nil (wrong type) — should fall back to res.ID
	src := resource.Resource{
		ID:        envID,
		Name:      "prod-env",
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (fallback to res.ID)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != stackName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, stackName)
	}
}

// ---------------------------------------------------------------------------
// checkEbLogs — Pattern C: log group prefix "/aws/elasticbeanstalk/{envName}/"
// ---------------------------------------------------------------------------

func TestRelated_Eb_Logs_MatchByEnvNamePrefix(t *testing.T) {
	const envName = "my-beanstalk-env"
	logGroupID := "/aws/elasticbeanstalk/" + envName + "/var/log/eb-activity.log"

	logRes := resource.Resource{
		ID:   logGroupID,
		Name: logGroupID,
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	src := resource.Resource{
		ID:   "e-abc123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-abc123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != logGroupID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, logGroupID)
	}
}

func TestRelated_Eb_Logs_NoMatchDifferentEnv(t *testing.T) {
	const envName = "my-beanstalk-env"
	otherLogGroup := "/aws/elasticbeanstalk/other-env/var/log/eb-activity.log"

	logRes := resource.Resource{
		ID:   otherLogGroup,
		Name: otherLogGroup,
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	src := resource.Resource{
		ID:   "e-abc123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-abc123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: multiple log groups for same env — all returned.
func TestRelated_Eb_Logs_MultipleGroupsSameEnv(t *testing.T) {
	const envName = "prod-java-env"
	prefix := "/aws/elasticbeanstalk/" + envName + "/"

	logRes1 := resource.Resource{ID: prefix + "var/log/web.stdout.log"}
	logRes2 := resource.Resource{ID: prefix + "var/log/eb-activity.log"}
	otherRes := resource.Resource{ID: "/aws/elasticbeanstalk/staging-env/var/log/web.stdout.log"}

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{logRes1, logRes2, otherRes},
		},
	}

	src := resource.Resource{
		ID:   "e-abc123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-abc123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEbASG — Pattern C: ASG tag "elasticbeanstalk:environment-name"
// ---------------------------------------------------------------------------

func TestRelated_Eb_ASG_MatchByTag(t *testing.T) {
	const envName = "prod-python-env"
	const asgName = "awseb-e-abc123-AWSEBAutoScalingGroup"

	asgRes := resource.Resource{
		ID:   asgName,
		Name: asgName,
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String(asgName),
			Tags: []asgtypes.TagDescription{
				{
					Key:   aws.String("elasticbeanstalk:environment-name"),
					Value: aws.String(envName),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}

	src := resource.Resource{
		ID:   "e-abc123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-abc123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != asgName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, asgName)
	}
}

func TestRelated_Eb_ASG_NoMatchDifferentTag(t *testing.T) {
	const envName = "prod-python-env"
	const asgName = "awseb-e-abc123-AWSEBAutoScalingGroup"

	asgRes := resource.Resource{
		ID:   asgName,
		Name: asgName,
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String(asgName),
			Tags: []asgtypes.TagDescription{
				{
					Key:   aws.String("elasticbeanstalk:environment-name"),
					Value: aws.String("staging-python-env"), // different env
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}

	src := resource.Resource{
		ID:   "e-abc123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-abc123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: ASG with wrong RawStruct type is skipped, not counted.
func TestRelated_Eb_ASG_SkipsWrongRawStructASG(t *testing.T) {
	const envName = "prod-python-env"
	const asgName = "awseb-e-abc123-AWSEBAutoScalingGroup"

	// Wrong RawStruct — assertStruct[asgtypes.AutoScalingGroup] will fail.
	asgRes := resource.Resource{
		ID:        asgName,
		Name:      asgName,
		RawStruct: "not-an-asg",
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}

	src := resource.Resource{
		ID:   "e-abc123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-abc123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (skipped wrong RawStruct ASG)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEbEC2 — Pattern C: EC2 tag "elasticbeanstalk:environment-name"
// ---------------------------------------------------------------------------

func TestRelated_Eb_EC2_MatchByTag(t *testing.T) {
	const envName = "prod-node-env"
	const instanceID = "i-0a1b2c3d4e5f60001"

	ec2Res := resource.Resource{
		ID:   instanceID,
		Name: instanceID,
		RawStruct: ec2types.Instance{
			InstanceId: aws.String(instanceID),
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("elasticbeanstalk:environment-name"),
					Value: aws.String(envName),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	src := resource.Resource{
		ID:   "e-nodenv123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-nodenv123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != instanceID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, instanceID)
	}
}

func TestRelated_Eb_EC2_NoMatchDifferentEnvTag(t *testing.T) {
	const envName = "prod-node-env"

	ec2Res := resource.Resource{
		ID:   "i-0zzzzzzzzzzzzzzz",
		Name: "i-0zzzzzzzzzzzzzzz",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0zzzzzzzzzzzzzzz"),
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("elasticbeanstalk:environment-name"),
					Value: aws.String("staging-node-env"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	src := resource.Resource{
		ID:   "e-nodenv123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-nodenv123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: EC2 instance with no EB tag is skipped.
func TestRelated_Eb_EC2_SkipsInstanceWithoutEBTag(t *testing.T) {
	const envName = "prod-node-env"

	ec2Res := resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "i-0a1b2c3d4e5f60001",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0a1b2c3d4e5f60001"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("worker")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	src := resource.Resource{
		ID:   "e-nodenv123",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-nodenv123"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no EB tag on instance)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEbAlarm — Pattern D: alarm dimension or name substring match
// ---------------------------------------------------------------------------

func TestRelated_Eb_Alarm_MatchByDimension(t *testing.T) {
	const envName = "prod-php-env"
	const alarmName = "eb-prod-php-env-EnvironmentHealth"

	alarmRes := resource.Resource{
		ID:   alarmName,
		Name: alarmName,
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String(alarmName),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("EnvironmentName"),
					Value: aws.String(envName),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	src := resource.Resource{
		ID:   "e-phpenv999",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-phpenv999"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != alarmName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, alarmName)
	}
}

func TestRelated_Eb_Alarm_NoMatchNeitherDimensionNorName(t *testing.T) {
	const envName = "prod-php-env"

	alarmRes := resource.Resource{
		ID:   "unrelated-alarm",
		Name: "unrelated-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("unrelated-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("EnvironmentName"),
					Value: aws.String("other-env"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	src := resource.Resource{
		ID:   "e-phpenv999",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-phpenv999"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: alarm matched by name substring fallback (no dimension match).
func TestRelated_Eb_Alarm_FallbackMatchByNameSubstring(t *testing.T) {
	const envName = "prod-php-env"
	// alarm name contains envName but dimension value does not match
	alarmName := "custom-" + envName + "-alert"

	alarmRes := resource.Resource{
		ID:   alarmName,
		Name: alarmName,
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String(alarmName),
			Dimensions: []cwtypes.Dimension{
				{
					// Dimension name differs but not matched
					Name:  aws.String("SomeOtherDimension"),
					Value: aws.String("unrelated-value"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	src := resource.Resource{
		ID:   "e-phpenv999",
		Name: envName,
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentId:   aws.String("e-phpenv999"),
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (name substring fallback)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != alarmName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, alarmName)
	}
}
