// aws_eb_related_test.go — the Elastic Beanstalk related-panel checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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
		"elb":   true,
		"tg":    true,
		"sg":    true,
		"role":  true,
		"s3":    true,
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

// DescribeEnvironmentResources names the environment's load balancer; the
// elb list (ELBv2) is what it is counted against. A load balancer the list
// holds is counted by its row; with no elb list to read, the count is unknown.
func TestRelated_Eb_ELB_MatchByEnvironmentResources(t *testing.T) {
	const envName = "my-eb-env"
	const lbName = "awseb-AWSEBLB-ABCDEF123456"
	const lbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/awseb-AWSEBLB-ABCDEF123456/0123456789abcdef"

	fakeEB := newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
		EnvironmentName: aws.String(envName),
		LoadBalancers:   []ebtypes.LoadBalancer{{Name: aws.String(lbName)}},
	})
	res := resource.Resource{
		ID:        envName,
		Name:      envName,
		Fields:    map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{EnvironmentName: aws.String(envName)},
	}
	checker := ebCheckerByTarget(t, "elb")

	lbs, err := awsclient.FetchLoadBalancersPage(context.Background(), newFakeELBv2WithLBsAndListeners(
		[]elbv2types.LoadBalancer{{LoadBalancerName: aws.String(lbName), LoadBalancerArn: aws.String(lbARN), Type: elbv2types.LoadBalancerTypeEnumApplication}}, nil), "")
	if err != nil {
		t.Fatalf("elb list: %v", err)
	}
	cache := resource.ResourceCache{"elb": resource.ResourceCacheEntry{Resources: lbs.Resources}}
	result := checker(context.Background(), &awsclient.ServiceClients{ElasticBeanstalk: fakeEB}, res, cache)
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != lbName || result.Count() != 1 {
		t.Errorf("with the elb list loaded: ResourceIDs = %v Count = %d, want [%s] 1", result.ResourceIDs(), result.Count(), lbName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}

	unloaded := checker(context.Background(), &awsclient.ServiceClients{ElasticBeanstalk: fakeEB}, res, resource.ResourceCache{})
	if unloaded.EffectiveState() != domain.RelatedUnknown && unloaded.EffectiveState() != domain.RelatedError {
		t.Errorf("with no elb list: state = %v ids %v, want unknown", unloaded.EffectiveState(), unloaded.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no load balancers)", result.Count())
	}
}

func TestRelated_Eb_ELB_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

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

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1 (role from IamInstanceProfile)", result.Count())
	}
	wantRoleName := "aws-elasticbeanstalk-ec2-role"
	found := false
	for _, id := range result.ResourceIDs() {
		if id == wantRoleName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %s", result.ResourceIDs(), wantRoleName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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
	fakeIAM := &fakeIAMForASG{}
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no IAM option settings)", result.Count())
	}
}

func TestRelated_Eb_Role_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

// An environment's bucket is the source bundle of the version it runs
// (EnvironmentDescription.VersionLabel); the application's other versions are
// not deployed to it.
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
		{
			ApplicationName: aws.String(appName),
			VersionLabel:    aws.String("v0.9.0"),
			SourceBundle: &ebtypes.S3Location{
				S3Bucket: aws.String("acme-old-bundles"),
				S3Key:    aws.String("my-app/v0.9.0.zip"),
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
			VersionLabel:    aws.String("v1.0.0"),
		},
	}

	checker := ebCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != s3Bucket {
		t.Errorf("ResourceIDs = %v, want [%s] (the running version's bundle only)", ids, s3Bucket)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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
			VersionLabel:    aws.String("v1.0.0"),
		},
	}

	checker := ebCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no application versions)", result.Count())
	}
}

func TestRelated_Eb_S3_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

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

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1 (SG from launchconfiguration option)", result.Count())
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == sgID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %s", result.ResourceIDs(), sgID)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no SG option settings)", result.Count())
	}
}

func TestRelated_Eb_SG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

// DescribeEnvironmentResources names the load balancer and
// DescribeLoadBalancers gives its ARN; a target group names every load
// balancer that forwards to it in LoadBalancerArns, so the environment's
// groups are the tg rows that name that ARN.
func TestRelated_Eb_TG_MatchByTargetGroupLoadBalancerArns(t *testing.T) {
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
		nil,
	)
	tgCache := resource.ResourceCache{"tg": {Resources: []resource.Resource{
		{ID: "awseb-AWSEBTA-ABCDEF123456", Name: "awseb-AWSEBTA-ABCDEF123456", RawStruct: elbv2types.TargetGroup{
			TargetGroupName:  aws.String("awseb-AWSEBTA-ABCDEF123456"),
			TargetGroupArn:   aws.String(tgARN),
			LoadBalancerArns: []string{lbARN},
		}},
		{ID: "other-app-tg", Name: "other-app-tg", RawStruct: elbv2types.TargetGroup{
			TargetGroupName:  aws.String("other-app-tg"),
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/other-app-tg/fedcba9876543210"),
			LoadBalancerArns: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other-app/fedcba9876543210"},
		}},
	}}}
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
	result := checker(context.Background(), clients, res, tgCache)

	if result.Count() != 1 || result.Truncated() {
		t.Errorf("Count = %d truncated=%v, want exactly 1", result.Count(), result.Truncated())
	}
	// tg rows are keyed by the target group name, not the ARN.
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "awseb-AWSEBTA-ABCDEF123456" {
		t.Errorf("ResourceIDs = %v, want [awseb-AWSEBTA-ABCDEF123456]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_Eb_TG_NoLoadBalancers(t *testing.T) {
	envName := "my-eb-env"

	fakeEB := newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
		EnvironmentName: aws.String(envName),
		LoadBalancers:   []ebtypes.LoadBalancer{},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
		ELBv2:            &fakeELBv2ForEB{},
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no load balancers)", result.Count())
	}
}

func TestRelated_Eb_TG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eb-env",
		Fields:    map[string]string{},
		RawStruct: "not-an-eb-env",
	}

	checker := ebCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}
