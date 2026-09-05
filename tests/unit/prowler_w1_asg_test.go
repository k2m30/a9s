package unit

// prowler_w1_asg_test.go — behavioural pins for the asg signals of batch w1.
//
// Wave 1 (launch-config.legacy, single-az, no-elb-health-check) is asserted
// through FetchAutoScalingGroupsPage. Wave 2 (launch-config.imdsv1,
// launch-config.public-ip, launch-config.secret) reads the launch
// configurations EnrichASGScalingActivities fetches alongside the scaling
// activities; all three describe the same object, so one batched call serves
// them and the tests pin that batching.

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	pw1ASGCodeLegacyLC    = domain.FindingCode("asg.launch-config.legacy")
	pw1ASGCodeSingleAZ    = domain.FindingCode("asg.single-az")
	pw1ASGCodeNoELBHealth = domain.FindingCode("asg.no-elb-health-check")
	pw1ASGCodeLCIMDSv1    = domain.FindingCode("asg.launch-config.imdsv1")
	pw1ASGCodeLCPublicIP  = domain.FindingCode("asg.launch-config.public-ip")
	pw1ASGCodeLCSecret    = domain.FindingCode("asg.launch-config.secret")
)

// ─── fakes ──────────────────────────────────────────────────────────────────

// pw1ASGListFake serves DescribeAutoScalingGroups for the wave-1 fetcher.
type pw1ASGListFake struct {
	groups []asgtypes.AutoScalingGroup
}

func (f *pw1ASGListFake) DescribeAutoScalingGroups(_ context.Context, _ *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: f.groups}, nil
}

// pw1ASGEnrichFake serves the enricher: no failed scaling activity, plus
// launch configurations keyed by name. Requested names are recorded so the
// batching contract can be asserted.
type pw1ASGEnrichFake struct {
	awsclient.ASGAPI
	configs map[string]asgtypes.LaunchConfiguration
	lcErr   error
	lcCalls int
	lcAsked []string
	actErr  map[string]error
}

func (f *pw1ASGEnrichFake) DescribeScalingActivities(_ context.Context, in *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	if err, ok := f.actErr[aws.ToString(in.AutoScalingGroupName)]; ok {
		return nil, err
	}
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func (f *pw1ASGEnrichFake) DescribeLaunchConfigurations(_ context.Context, in *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	f.lcCalls++
	f.lcAsked = append(f.lcAsked, in.LaunchConfigurationNames...)
	if f.lcErr != nil {
		return nil, f.lcErr
	}
	var out []asgtypes.LaunchConfiguration
	for _, name := range in.LaunchConfigurationNames {
		if lc, ok := f.configs[name]; ok {
			out = append(out, lc)
		}
	}
	return &autoscaling.DescribeLaunchConfigurationsOutput{LaunchConfigurations: out}, nil
}

// pw1ASGGroup builds a healthy group: two AZs, a launch template, and an ELB
// health check when it is behind a target group.
func pw1ASGGroup(name string) asgtypes.AutoScalingGroup {
	return asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String(name),
		AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:11111111-2222-3333-4444-555555555555:autoScalingGroupName/" + name),
		MinSize:              aws.Int32(2),
		MaxSize:              aws.Int32(6),
		DesiredCapacity:      aws.Int32(2),
		DefaultCooldown:      aws.Int32(300),
		AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
		HealthCheckType:      aws.String("EC2"),
		VPCZoneIdentifier:    aws.String("subnet-0aaaa1111bbbb2222,subnet-0cccc3333dddd4444"),
		LaunchTemplate: &asgtypes.LaunchTemplateSpecification{
			LaunchTemplateId:   aws.String("lt-0aaaa1111bbbb2222"),
			LaunchTemplateName: aws.String("acme-web"),
			Version:            aws.String("$Default"),
		},
		Instances: []asgtypes.Instance{{
			InstanceId:       aws.String("i-0aaaa1111bbbb2222"),
			AvailabilityZone: aws.String("us-east-1a"),
			HealthStatus:     aws.String("Healthy"),
			LifecycleState:   asgtypes.LifecycleStateInService,
		}, {
			InstanceId:       aws.String("i-0cccc3333dddd4444"),
			AvailabilityZone: aws.String("us-east-1b"),
			HealthStatus:     aws.String("Healthy"),
			LifecycleState:   asgtypes.LifecycleStateInService,
		}},
	}
}

// pw1ASGWithLaunchConfig swaps the launch template for a launch configuration.
func pw1ASGWithLaunchConfig(name, lcName string) asgtypes.AutoScalingGroup {
	g := pw1ASGGroup(name)
	g.LaunchTemplate = nil
	g.LaunchConfigurationName = aws.String(lcName)
	return g
}

// pw1LaunchConfig builds a hardened launch configuration: IMDSv2 required, no
// public IPs, and user data that resolves its credential from SSM.
func pw1LaunchConfig(name string) asgtypes.LaunchConfiguration {
	return asgtypes.LaunchConfiguration{
		LaunchConfigurationName:  aws.String(name),
		LaunchConfigurationARN:   aws.String("arn:aws:autoscaling:us-east-1:123456789012:launchConfiguration:11111111-2222-3333-4444-555555555555:launchConfigurationName/" + name),
		ImageId:                  aws.String("ami-0aaaa1111bbbb2222"),
		InstanceType:             aws.String("t3.medium"),
		AssociatePublicIpAddress: aws.Bool(false),
		MetadataOptions: &asgtypes.InstanceMetadataOptions{
			HttpEndpoint: asgtypes.InstanceMetadataEndpointStateEnabled,
			HttpTokens:   asgtypes.InstanceMetadataHttpTokensStateRequired,
		},
		UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(
			"#!/bin/bash\nDB_PASSWORD_PARAM=/acme/prod/db-password\n/opt/app/start.sh\n"))),
	}
}

func pw1FetchASGs(t *testing.T, groups ...asgtypes.AutoScalingGroup) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), &pw1ASGListFake{groups: groups}, "")
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage: %v", err)
	}
	return out.Resources
}

// pw1EnrichASG runs the asg enricher over resources built by the real fetcher.
func pw1EnrichASG(t *testing.T, fake *pw1ASGEnrichFake, groups ...asgtypes.AutoScalingGroup) awsclient.IssueEnricherResult {
	t.Helper()
	rs := pw1FetchASGs(t, groups...)
	res, err := awsclient.EnrichASGScalingActivities(context.Background(),
		&awsclient.ServiceClients{AutoScaling: fake}, rs, nil)
	if err != nil && fake.lcErr == nil && len(fake.actErr) == 0 {
		t.Fatalf("EnrichASGScalingActivities: %v", err)
	}
	return res
}

// ─── row 15: asg.launch-config.legacy ───────────────────────────────────────

// TestASG_LegacyLaunchConfig_Present pins the warning: launch configurations
// are frozen by AWS and cannot express IMDSv2-only, instance metadata tags or
// the newer instance families.
func TestASG_LegacyLaunchConfig_Present(t *testing.T) {
	rs := pw1FetchASGs(t, pw1ASGWithLaunchConfig("acme-legacy-asg", "acme-web-lc-2019"))
	r := pw1ResourceByID(t, rs, "acme-legacy-asg")
	pw1RequireFinding(t, r.Findings, pw1ASGCodeLegacyLC, "uses a launch configuration", domain.SevWarn, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1ASGCodeLegacyLC].Rows, "Launch configuration", "acme-web-lc-2019")
}

// TestASG_LegacyLaunchConfig_LaunchTemplateIsHealthy pins the negative case.
func TestASG_LegacyLaunchConfig_LaunchTemplateIsHealthy(t *testing.T) {
	rs := pw1FetchASGs(t, pw1ASGGroup("acme-modern-asg"))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "acme-modern-asg").Findings, pw1ASGCodeLegacyLC)
}

// ─── row 16: asg.single-az ──────────────────────────────────────────────────

// TestASG_SingleAZ_OneZone pins the warning and the row naming the zone.
func TestASG_SingleAZ_OneZone(t *testing.T) {
	g := pw1ASGGroup("acme-single-az")
	g.AvailabilityZones = []string{"us-east-1a"}
	rs := pw1FetchASGs(t, g)
	r := pw1ResourceByID(t, rs, "acme-single-az")
	pw1RequireFinding(t, r.Findings, pw1ASGCodeSingleAZ, "single availability zone", domain.SevWarn, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1ASGCodeSingleAZ].Rows, "AZs", "us-east-1a")
}

// TestASG_SingleAZ_TwoZonesIsHealthy pins the negative case at the boundary
// the rule is written against.
func TestASG_SingleAZ_TwoZonesIsHealthy(t *testing.T) {
	rs := pw1FetchASGs(t, pw1ASGGroup("acme-multi-az"))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "acme-multi-az").Findings, pw1ASGCodeSingleAZ)
}

// TestASG_SingleAZ_NoZonesIsStillSingle pins the empty list: a group with no
// zones cannot survive a zone failure either.
func TestASG_SingleAZ_NoZonesIsStillSingle(t *testing.T) {
	g := pw1ASGGroup("acme-no-az")
	g.AvailabilityZones = nil
	rs := pw1FetchASGs(t, g)
	pw1RequireFinding(t, pw1ResourceByID(t, rs, "acme-no-az").Findings,
		pw1ASGCodeSingleAZ, "single availability zone", domain.SevWarn, "wave1")
}

// ─── row 17: asg.no-elb-health-check ────────────────────────────────────────

// TestASG_NoELBHealthCheck_BehindTargetGroup pins the warning: a group behind
// a load balancer that only checks EC2 status replaces a machine that failed
// its boot, but never one whose application stopped answering.
func TestASG_NoELBHealthCheck_BehindTargetGroup(t *testing.T) {
	g := pw1ASGGroup("acme-tg-ec2-health")
	g.TargetGroupARNs = []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web/1111111111111111"}
	rs := pw1FetchASGs(t, g)
	r := pw1ResourceByID(t, rs, "acme-tg-ec2-health")
	pw1RequireFinding(t, r.Findings, pw1ASGCodeNoELBHealth, "no load balancer health check", domain.SevWarn, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1ASGCodeNoELBHealth].Rows, "HealthCheckType", "EC2")
}

// TestASG_NoELBHealthCheck_ClassicLoadBalancerCounts pins that the older
// LoadBalancerNames attachment is treated the same as a target group.
func TestASG_NoELBHealthCheck_ClassicLoadBalancerCounts(t *testing.T) {
	g := pw1ASGGroup("acme-clb-ec2-health")
	g.LoadBalancerNames = []string{"acme-classic-elb"}
	rs := pw1FetchASGs(t, g)
	pw1RequireFinding(t, pw1ResourceByID(t, rs, "acme-clb-ec2-health").Findings,
		pw1ASGCodeNoELBHealth, "no load balancer health check", domain.SevWarn, "wave1")
}

// TestASG_NoELBHealthCheck_ELBTypeIsHealthy pins the negative case.
func TestASG_NoELBHealthCheck_ELBTypeIsHealthy(t *testing.T) {
	g := pw1ASGGroup("acme-tg-elb-health")
	g.TargetGroupARNs = []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web/1111111111111111"}
	g.HealthCheckType = aws.String("ELB")
	rs := pw1FetchASGs(t, g)
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "acme-tg-elb-health").Findings, pw1ASGCodeNoELBHealth)
}

// TestASG_NoELBHealthCheck_UnattachedGroupIsHealthy pins that a group behind
// no load balancer has nothing to check: EC2 health is the right setting.
func TestASG_NoELBHealthCheck_UnattachedGroupIsHealthy(t *testing.T) {
	rs := pw1FetchASGs(t, pw1ASGGroup("acme-standalone"))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, "acme-standalone").Findings, pw1ASGCodeNoELBHealth)
}

// TestASG_WaveOneConditionsAreIndependent pins that a single-AZ group on a
// launch configuration behind a target group carries all three findings.
func TestASG_WaveOneConditionsAreIndependent(t *testing.T) {
	g := pw1ASGWithLaunchConfig("acme-triple", "acme-web-lc-2019")
	g.AvailabilityZones = []string{"us-east-1a"}
	g.TargetGroupARNs = []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web/1111111111111111"}
	rs := pw1FetchASGs(t, g)
	r := pw1ResourceByID(t, rs, "acme-triple")
	for _, code := range []domain.FindingCode{pw1ASGCodeLegacyLC, pw1ASGCodeSingleAZ, pw1ASGCodeNoELBHealth} {
		if _, ok := pw1FindFinding(r.Findings, code); !ok {
			t.Errorf("missing %q on a group that trips all three wave-1 rules: %+v", code, r.Findings)
		}
	}
}

// ─── row 18: asg.launch-config.imdsv1 ───────────────────────────────────────

// TestASG_LaunchConfigIMDSv1_Optional pins the warning for a launch
// configuration that still accepts unauthenticated metadata requests.
func TestASG_LaunchConfigIMDSv1_Optional(t *testing.T) {
	const lcName = "acme-web-lc-optional"
	lc := pw1LaunchConfig(lcName)
	lc.MetadataOptions.HttpTokens = asgtypes.InstanceMetadataHttpTokensStateOptional
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}

	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-imdsv1-asg", lcName))
	pw1RequireFinding(t, res.Findings["acme-imdsv1-asg"], pw1ASGCodeLCIMDSv1,
		"launch configuration allows IMDSv1", domain.SevWarn, "wave2:asg")
	pw1RequireRow(t, pw1Rows(res, "acme-imdsv1-asg", pw1ASGCodeLCIMDSv1), "HttpTokens", "optional")
}

// TestASG_LaunchConfigIMDSv1_NilMetadataOptionsIsUnset pins the one rule in
// this batch where an absent block does trigger: a launch configuration with
// no MetadataOptions runs with tokens optional, so the machines it launches
// really do accept IMDSv1. The row says "unset" rather than "optional" so the
// operator knows nothing was configured.
func TestASG_LaunchConfigIMDSv1_NilMetadataOptionsIsUnset(t *testing.T) {
	const lcName = "acme-web-lc-unset"
	lc := pw1LaunchConfig(lcName)
	lc.MetadataOptions = nil
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}

	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-unset-asg", lcName))
	pw1RequireFinding(t, res.Findings["acme-unset-asg"], pw1ASGCodeLCIMDSv1,
		"launch configuration allows IMDSv1", domain.SevWarn, "wave2:asg")
	pw1RequireRow(t, pw1Rows(res, "acme-unset-asg", pw1ASGCodeLCIMDSv1), "HttpTokens", "unset")
}

// TestASG_LaunchConfigIMDSv1_RequiredIsHealthy pins the negative case.
func TestASG_LaunchConfigIMDSv1_RequiredIsHealthy(t *testing.T) {
	const lcName = "acme-web-lc-required"
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{
		lcName: pw1LaunchConfig(lcName),
	}}
	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-hardened-asg", lcName))
	pw1RequireNoFinding(t, res.Findings["acme-hardened-asg"], pw1ASGCodeLCIMDSv1)
}

// ─── row 19: asg.launch-config.public-ip ────────────────────────────────────

// TestASG_LaunchConfigPublicIP_Enabled pins the warning: every instance the
// group launches gets a routable address.
func TestASG_LaunchConfigPublicIP_Enabled(t *testing.T) {
	const lcName = "acme-web-lc-public"
	lc := pw1LaunchConfig(lcName)
	lc.AssociatePublicIpAddress = aws.Bool(true)
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}

	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-public-asg", lcName))
	pw1RequireFinding(t, res.Findings["acme-public-asg"], pw1ASGCodeLCPublicIP,
		"launch configuration assigns public IPs", domain.SevWarn, "wave2:asg")
	pw1RequireRow(t, pw1Rows(res, "acme-public-asg", pw1ASGCodeLCPublicIP), "Public address assignment", "true")
}

// TestASG_LaunchConfigPublicIP_FalseIsHealthy pins the negative case.
func TestASG_LaunchConfigPublicIP_FalseIsHealthy(t *testing.T) {
	const lcName = "acme-web-lc-private"
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{
		lcName: pw1LaunchConfig(lcName),
	}}
	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-private-asg", lcName))
	pw1RequireNoFinding(t, res.Findings["acme-private-asg"], pw1ASGCodeLCPublicIP)
}

// TestASG_LaunchConfigPublicIP_NilIsNotEnabled pins that an absent pointer is
// the subnet default, not an enabled setting.
func TestASG_LaunchConfigPublicIP_NilIsNotEnabled(t *testing.T) {
	const lcName = "acme-web-lc-nilpub"
	lc := pw1LaunchConfig(lcName)
	lc.AssociatePublicIpAddress = nil
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}
	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-nilpub-asg", lcName))
	pw1RequireNoFinding(t, res.Findings["acme-nilpub-asg"], pw1ASGCodeLCPublicIP)
}

// ─── row 20: asg.launch-config.secret ───────────────────────────────────────

// TestASG_LaunchConfigSecret_PlaintextUserData pins the Broken finding, the
// Where:Kind row, and that the credential never reaches the finding text.
func TestASG_LaunchConfigSecret_PlaintextUserData(t *testing.T) {
	const lcName = "acme-web-lc-secret"
	lc := pw1LaunchConfig(lcName)
	lc.UserData = aws.String(base64.StdEncoding.EncodeToString([]byte(
		"#!/bin/bash\nexport DB_PASSWORD=hunter2hunter2\n/opt/app/start.sh\n")))
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}

	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-secret-asg", lcName))
	f := pw1RequireFinding(t, res.Findings["acme-secret-asg"], pw1ASGCodeLCSecret,
		"credential in launch configuration user data", domain.SevBroken, "wave2:asg")
	pw1RequireRow(t, pw1Rows(res, "acme-secret-asg", pw1ASGCodeLCSecret), "line 2", "keyword")
	if strings.Contains(f.Phrase+f.Detail, "hunter2hunter2") {
		t.Errorf("credential value leaked into the finding text")
	}
}

// TestASG_LaunchConfigSecret_SSMParameterIsHealthy pins the negative case: a
// script that names an SSM parameter carries a reference, not a secret.
func TestASG_LaunchConfigSecret_SSMParameterIsHealthy(t *testing.T) {
	const lcName = "acme-web-lc-ssm"
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{
		lcName: pw1LaunchConfig(lcName),
	}}
	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-ssm-asg", lcName))
	pw1RequireNoFinding(t, res.Findings["acme-ssm-asg"], pw1ASGCodeLCSecret)
}

// TestASG_LaunchConfigSecret_NoUserDataIsHealthy pins the empty case.
func TestASG_LaunchConfigSecret_NoUserDataIsHealthy(t *testing.T) {
	const lcName = "acme-web-lc-noud"
	lc := pw1LaunchConfig(lcName)
	lc.UserData = nil
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}
	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-noud-asg", lcName))
	pw1RequireNoFinding(t, res.Findings["acme-noud-asg"], pw1ASGCodeLCSecret)
}

// ─── cross-cutting wave 2 ───────────────────────────────────────────────────

// TestASG_LaunchConfigConditionsAreThreeFindings pins independence across the
// three launch-configuration rules on one object.
func TestASG_LaunchConfigConditionsAreThreeFindings(t *testing.T) {
	const lcName = "acme-web-lc-terrible"
	lc := pw1LaunchConfig(lcName)
	lc.MetadataOptions = nil
	lc.AssociatePublicIpAddress = aws.Bool(true)
	lc.UserData = aws.String(base64.StdEncoding.EncodeToString([]byte(
		"#!/bin/bash\nexport ADMIN_PASSWORD=s3cr3t-value-9\n")))
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{lcName: lc}}

	res := pw1EnrichASG(t, fake, pw1ASGWithLaunchConfig("acme-terrible-asg", lcName))
	for _, code := range []domain.FindingCode{pw1ASGCodeLCIMDSv1, pw1ASGCodeLCPublicIP, pw1ASGCodeLCSecret} {
		if _, ok := pw1FindFinding(res.Findings["acme-terrible-asg"], code); !ok {
			t.Errorf("missing %q on a launch configuration that trips all three rules: %+v",
				code, res.Findings["acme-terrible-asg"])
		}
		if len(pw1Rows(res, "acme-terrible-asg", code)) == 0 {
			t.Errorf("%q has no supporting rows; each condition keeps its own", code)
		}
	}
}

// TestASG_LaunchConfigurationsAreDescribedInOneBatch pins that the launch
// configurations of several groups are fetched in one call, and that a group
// on a launch template is never asked about.
func TestASG_LaunchConfigurationsAreDescribedInOneBatch(t *testing.T) {
	lcA := pw1LaunchConfig("acme-lc-a")
	lcA.AssociatePublicIpAddress = aws.Bool(true)
	lcB := pw1LaunchConfig("acme-lc-b")
	lcB.AssociatePublicIpAddress = aws.Bool(true)
	fake := &pw1ASGEnrichFake{configs: map[string]asgtypes.LaunchConfiguration{
		"acme-lc-a": lcA, "acme-lc-b": lcB,
	}}

	res := pw1EnrichASG(t, fake,
		pw1ASGWithLaunchConfig("acme-asg-a", "acme-lc-a"),
		pw1ASGWithLaunchConfig("acme-asg-b", "acme-lc-b"),
		pw1ASGGroup("acme-asg-template"),
	)
	if fake.lcCalls != 1 {
		t.Errorf("DescribeLaunchConfigurations called %d times for 2 launch configurations: %v",
			fake.lcCalls, fake.lcAsked)
	}
	for _, asked := range fake.lcAsked {
		if asked == "" {
			t.Errorf("empty launch configuration name requested: %v", fake.lcAsked)
		}
	}
	for _, name := range []string{"acme-asg-a", "acme-asg-b"} {
		pw1RequireFinding(t, res.Findings[name], pw1ASGCodeLCPublicIP,
			"launch configuration assigns public IPs", domain.SevWarn, "wave2:asg")
	}
	pw1RequireNoFinding(t, res.Findings["acme-asg-template"], pw1ASGCodeLCPublicIP)
}

// TestASG_LaunchConfigurationErrorMarksTheAffectedGroups pins the
// partial-failure contract: groups whose launch configuration could not be
// read render "?" rather than silently reporting a clean posture.
func TestASG_LaunchConfigurationErrorMarksTheAffectedGroups(t *testing.T) {
	fake := &pw1ASGEnrichFake{
		configs: map[string]asgtypes.LaunchConfiguration{},
		lcErr:   errors.New("AccessDeniedException: autoscaling:DescribeLaunchConfigurations"),
	}
	res := pw1EnrichASG(t, fake,
		pw1ASGWithLaunchConfig("acme-asg-denied", "acme-lc-denied"),
		pw1ASGGroup("acme-asg-template"),
	)
	if !res.TruncatedIDs["acme-asg-denied"] {
		t.Errorf("TruncatedIDs missing acme-asg-denied after a failed DescribeLaunchConfigurations")
	}
	if res.TruncatedIDs["acme-asg-template"] {
		t.Errorf("a launch-template group was marked truncated by a launch-configuration failure")
	}
}

// ─── demo bench ─────────────────────────────────────────────────────────────

// TestASG_DemoBench_EachSignalHasExactlyOneWitness pins the demo fixture
// contract for the six asg signals.
func TestASG_DemoBench_EachSignalHasExactlyOneWitness(t *testing.T) {
	out, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), fakes.NewASG(), "")
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage(demo): %v", err)
	}
	for code, witness := range map[domain.FindingCode]string{
		pw1ASGCodeLegacyLC:    fixtures.ASGLegacyLaunchConfig,
		pw1ASGCodeSingleAZ:    fixtures.ASGSingleAZ,
		pw1ASGCodeNoELBHealth: fixtures.ASGNoELBHealthCheck,
	} {
		var carriers []string
		for _, r := range out.Resources {
			if _, ok := pw1FindFinding(r.Findings, code); ok {
				carriers = append(carriers, r.ID)
			}
		}
		pw1RequireOnlyWitness(t, code, witness, carriers)
	}

	res, eerr := awsclient.EnrichASGScalingActivities(context.Background(),
		&awsclient.ServiceClients{AutoScaling: fakes.NewASG()}, out.Resources, nil)
	if eerr != nil {
		t.Fatalf("EnrichASGScalingActivities(demo): %v", eerr)
	}
	for code, witness := range map[domain.FindingCode]string{
		pw1ASGCodeLCIMDSv1:   fixtures.ASGLaunchConfigIMDSv1,
		pw1ASGCodeLCPublicIP: fixtures.ASGLaunchConfigPublicIP,
		pw1ASGCodeLCSecret:   fixtures.ASGLaunchConfigSecret,
	} {
		var carriers []string
		for id, fs := range res.Findings {
			if _, ok := pw1FindFinding(fs, code); ok {
				carriers = append(carriers, id)
			}
		}
		pw1RequireOnlyWitness(t, code, witness, carriers)
	}
}

// TestASG_PostureSignalsSilentOnDeletingGroup pins common contract rule 4 on
// asg: a group AWS is deleting launches nothing more, so neither its shape nor
// its launch configuration is an open posture item.
func TestASG_PostureSignalsSilentOnDeletingGroup(t *testing.T) {
	const lcName = "acme-deleting-lc"
	lc := pw1LaunchConfig(lcName)
	lc.MetadataOptions = nil
	lc.AssociatePublicIpAddress = aws.Bool(true)
	lc.UserData = aws.String(base64.StdEncoding.EncodeToString([]byte(
		"#!/bin/bash\nexport ADMIN_PASSWORD=s3cr3t-value-9\n")))

	g := pw1ASGWithLaunchConfig("acme-deleting-asg", lcName)
	g.Status = aws.String("Delete in progress")
	g.AvailabilityZones = []string{"us-east-1a"}
	g.TargetGroupARNs = []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme/1111111111111111"}

	wave1 := pw1ResourceByID(t, pw1FetchASGs(t, g), "acme-deleting-asg")
	for _, code := range []domain.FindingCode{pw1ASGCodeLegacyLC, pw1ASGCodeSingleAZ, pw1ASGCodeNoELBHealth} {
		if _, ok := pw1FindFinding(wave1.Findings, code); ok {
			t.Errorf("%s emitted for a group AWS is deleting", code)
		}
	}

	res := pw1EnrichASG(t, &pw1ASGEnrichFake{
		configs: map[string]asgtypes.LaunchConfiguration{lcName: lc},
	}, g)
	for _, code := range []domain.FindingCode{pw1ASGCodeLCIMDSv1, pw1ASGCodeLCPublicIP, pw1ASGCodeLCSecret} {
		if _, ok := pw1FindFinding(res.Findings["acme-deleting-asg"], code); ok {
			t.Errorf("%s emitted for a group AWS is deleting", code)
		}
	}
}
