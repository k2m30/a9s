// aws_eb_related_test.go contains TDD Red tests for the Elastic Beanstalk
// related-panel checkers T020–T024. Tests are written before the coder replaces
// the stubs in stubs_related.go with real implementations — initial failures are
// expected.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ebCheckerByTarget looks up the EB checker for the given target type via the registry.
func ebCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("eb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("eb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("eb related checker for %s not found", target)
	return nil
}

// TestRelated_EB_Registered verifies that all 10 EB related defs are registered
// with non-nil checkers.
func TestRelated_EB_Registered(t *testing.T) {
	defs := resource.GetRelated("eb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for eb")
	}

	checkerExpected := map[string]bool{
		"cfn":   true,
		"logs":  true,
		"asg":   true,
		"ec2":   true,
		"alarm": true,
		"elb":   true, // T020
		"tg":    true, // T024
		"sg":    true, // T023
		"role":  true, // T021
		"s3":    true, // T022
	}
	for target, wantChecker := range checkerExpected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				hasChecker := def.Checker != nil
				if hasChecker != wantChecker {
					t.Errorf("eb %q: Checker presence = %v, want %v", target, hasChecker, wantChecker)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// ---------------------------------------------------------------------------
// T020 — checkEbELB (forward: DescribeEnvironmentResources.LoadBalancers[].Name)
// ---------------------------------------------------------------------------

// TestRelated_Eb_ELB_MatchByEnvironmentResources verifies that checkEbELB returns
// the load balancer name from DescribeEnvironmentResources.
func TestRelated_Eb_ELB_MatchByEnvironmentResources(t *testing.T) {
	elbName := "awseb-e-abc12345-AWSEBLoad-ABCDEF123456"
	envName := "my-eb-env"

	fakeEB := newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
		EnvironmentName: aws.String(envName),
		LoadBalancers: []ebtypes.LoadBalancer{
			{Name: aws.String(elbName)},
		},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != elbName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, elbName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Eb_ELB_NoLoadBalancers verifies that checkEbELB returns Count=0
// when DescribeEnvironmentResources returns no load balancers.
func TestRelated_Eb_ELB_NoLoadBalancers(t *testing.T) {
	envName := "my-eb-env"

	fakeEB := newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
		EnvironmentName: aws.String(envName),
		LoadBalancers:   []ebtypes.LoadBalancer{},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no load balancers)", result.Count)
	}
}

// TestRelated_Eb_ELB_WrongRawStruct verifies that checkEbELB returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_Eb_ELB_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T021 — checkEbRole (forward: DescribeConfigurationSettings →
//         IamInstanceProfile + ServiceRole → iam:GetInstanceProfile)
// ---------------------------------------------------------------------------

// TestRelated_Eb_Role_MatchByIamInstanceProfile verifies that checkEbRole resolves
// role ARNs from the IamInstanceProfile option setting.
func TestRelated_Eb_Role_MatchByIamInstanceProfile(t *testing.T) {
	envName := "my-eb-env"
	appName := "my-app"
	profileName := "aws-elasticbeanstalk-ec2-role"
	roleARN := "arn:aws:iam::123456789012:role/aws-elasticbeanstalk-ec2-role"

	fakeEB := newFakeEBWithConfigSettings([]ebtypes.ConfigurationOptionSetting{
		{
			Namespace:  aws.String("aws:autoscaling:launchconfiguration"),
			OptionName: aws.String("IamInstanceProfile"),
			Value:      aws.String(profileName),
		},
	})
	fakeIAM := newFakeIAMWithInstanceProfile([]iamtypes.Role{
		{Arn: aws.String(roleARN), RoleName: aws.String("aws-elasticbeanstalk-ec2-role")},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
		IAM:              fakeIAM,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
			ApplicationName: aws.String(appName),
		},
	}

	checker := ebCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (role from IamInstanceProfile)", result.Count)
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

// TestRelated_Eb_Role_NoRoleSettings verifies that checkEbRole returns Count=0
// when DescribeConfigurationSettings has no IAM-related option settings.
func TestRelated_Eb_Role_NoRoleSettings(t *testing.T) {
	envName := "my-eb-env"
	appName := "my-app"

	fakeEB := newFakeEBWithConfigSettings([]ebtypes.ConfigurationOptionSetting{
		{
			Namespace:  aws.String("aws:elasticbeanstalk:environment"),
			OptionName: aws.String("EnvironmentType"),
			Value:      aws.String("LoadBalanced"),
		},
	})
	fakeIAM := &fakeIAMBatch2{}
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
		IAM:              fakeIAM,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
			ApplicationName: aws.String(appName),
		},
	}

	checker := ebCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no IAM option settings)", result.Count)
	}
}

// TestRelated_Eb_Role_WrongRawStruct verifies that checkEbRole returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_Eb_Role_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T022 — checkEbS3 (forward: DescribeApplicationVersions.SourceBundle.S3Bucket)
// ---------------------------------------------------------------------------

// TestRelated_Eb_S3_MatchBySourceBundle verifies that checkEbS3 returns the
// S3 bucket from DescribeApplicationVersions.SourceBundle.S3Bucket.
func TestRelated_Eb_S3_MatchBySourceBundle(t *testing.T) {
	envName := "my-eb-env"
	appName := "my-app"
	s3Bucket := "elasticbeanstalk-us-east-1-123456789012"

	fakeEB := newFakeEBWithAppVersions([]ebtypes.ApplicationVersionDescription{
		{
			ApplicationName: aws.String(appName),
			VersionLabel:    aws.String("v1.0.0"),
			SourceBundle: &ebtypes.S3Location{
				S3Bucket: aws.String(s3Bucket),
				S3Key:    aws.String("my-app/v1.0.0.zip"),
			},
		},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
			ApplicationName: aws.String(appName),
		},
	}

	checker := ebCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (S3 bucket from SourceBundle)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == s3Bucket {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %s", result.ResourceIDs, s3Bucket)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Eb_S3_NoApplicationVersions verifies that checkEbS3 returns Count=0
// when DescribeApplicationVersions returns no versions.
func TestRelated_Eb_S3_NoApplicationVersions(t *testing.T) {
	envName := "my-eb-env"
	appName := "my-app"

	fakeEB := newFakeEBWithAppVersions([]ebtypes.ApplicationVersionDescription{})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
			ApplicationName: aws.String(appName),
		},
	}

	checker := ebCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no application versions)", result.Count)
	}
}

// TestRelated_Eb_S3_WrongRawStruct verifies that checkEbS3 returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_Eb_S3_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T023 — checkEbSG (forward: DescribeConfigurationSettings →
//         aws:autoscaling:launchconfiguration:SecurityGroups +
//         aws:elbv2:loadbalancer:SecurityGroups)
// ---------------------------------------------------------------------------

// TestRelated_Eb_SG_MatchByLaunchConfigSecurityGroups verifies that checkEbSG
// returns the security group ID from the launchconfiguration option setting.
func TestRelated_Eb_SG_MatchByLaunchConfigSecurityGroups(t *testing.T) {
	envName := "my-eb-env"
	appName := "my-app"
	sgID := "sg-0abc111111111111a"

	fakeEB := newFakeEBWithConfigSettings([]ebtypes.ConfigurationOptionSetting{
		{
			Namespace:  aws.String("aws:autoscaling:launchconfiguration"),
			OptionName: aws.String("SecurityGroups"),
			Value:      aws.String(sgID),
		},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
			ApplicationName: aws.String(appName),
		},
	}

	checker := ebCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (SG from launchconfiguration option)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == sgID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %s", result.ResourceIDs, sgID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Eb_SG_NoSecurityGroupSettings verifies that checkEbSG returns Count=0
// when DescribeConfigurationSettings has no security group option settings.
func TestRelated_Eb_SG_NoSecurityGroupSettings(t *testing.T) {
	envName := "my-eb-env"
	appName := "my-app"

	fakeEB := newFakeEBWithConfigSettings([]ebtypes.ConfigurationOptionSetting{
		{
			Namespace:  aws.String("aws:elasticbeanstalk:environment"),
			OptionName: aws.String("EnvironmentType"),
			Value:      aws.String("LoadBalanced"),
		},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
			ApplicationName: aws.String(appName),
		},
	}

	checker := ebCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SG option settings)", result.Count)
	}
}

// TestRelated_Eb_SG_WrongRawStruct verifies that checkEbSG returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_Eb_SG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// T024 — checkEbTG (forward: DescribeEnvironmentResources.LoadBalancers[] →
//                   elbv2:DescribeListeners.DefaultActions[].TargetGroupArn)
// ---------------------------------------------------------------------------

// TestRelated_Eb_TG_MatchByListenerDefaultAction verifies that checkEbTG resolves
// the target group ARN from the EB environment's load balancer listeners.
// DescribeEnvironmentResources returns the LB by name; checkEbTG must resolve
// name→ARN via DescribeLoadBalancers before calling DescribeListeners.
func TestRelated_Eb_TG_MatchByListenerDefaultAction(t *testing.T) {
	envName := "my-eb-env"
	lbName := "awseb-AWSEBLB-ABCDEF123456"
	lbARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/awseb-AWSEBLB-ABCDEF123456/0123456789abcdef"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/awseb-AWSEBTA-ABCDEF123456/0123456789abcdef"

	fakeEB := newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
		EnvironmentName: aws.String(envName),
		LoadBalancers: []ebtypes.LoadBalancer{
			{Name: aws.String(lbName)},
		},
	})
	fakeELBv2 := newFakeELBv2WithLBsAndListeners(
		[]elbv2types.LoadBalancer{
			{
				LoadBalancerName: aws.String(lbName),
				LoadBalancerArn:  aws.String(lbARN),
			},
		},
		[]elbv2types.Listener{
			{
				LoadBalancerArn: aws.String(lbARN),
				DefaultActions: []elbv2types.Action{
					{
						Type:           elbv2types.ActionTypeEnumForward,
						TargetGroupArn: aws.String(tgARN),
					},
				},
			},
		},
	)
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
		ELBv2:            fakeELBv2,
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "tg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != tgARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, tgARN)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Eb_TG_NoLoadBalancers verifies that checkEbTG returns Count=0
// when DescribeEnvironmentResources returns no load balancers.
func TestRelated_Eb_TG_NoLoadBalancers(t *testing.T) {
	envName := "my-eb-env"

	fakeEB := newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
		EnvironmentName: aws.String(envName),
		LoadBalancers:   []ebtypes.LoadBalancer{},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
		ELBv2:            &fakeELBv2Batch2{},
	}

	res := resource.Resource{
		ID:     envName,
		Name:   envName,
		Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String(envName),
		},
	}

	checker := ebCheckerByTarget(t, "tg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no load balancers)", result.Count)
	}
}

// TestRelated_Eb_TG_WrongRawStruct verifies that checkEbTG returns Count=-1
// when RawStruct is the wrong type.
func TestRelated_Eb_TG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}
