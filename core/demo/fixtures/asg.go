// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides ASG fixture data for the ASG fake.
package fixtures

import (
	"encoding/base64"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

// ASGFixtures holds all AutoScaling domain objects served by the fake.
type ASGFixtures struct {
	// AutoScalingGroups is the full list returned by DescribeAutoScalingGroups.
	AutoScalingGroups []asgtypes.AutoScalingGroup
	// Activities maps ASG name → []Activity.
	Activities map[string][]asgtypes.Activity
	// LaunchConfigurations maps LC name → LaunchConfiguration.
	LaunchConfigurations map[string]asgtypes.LaunchConfiguration
	// NotificationConfigurations maps ASG name → []NotificationConfiguration —
	// required for the asg:sns related-panel pivot (checkASGSNS).
	NotificationConfigurations map[string][]asgtypes.NotificationConfiguration
	// LifecycleHooks maps ASG name → []LifecycleHook — reverse-scanned by
	// checkASGSNS for SNS-ARN NotificationTargetARN values.
	LifecycleHooks map[string][]asgtypes.LifecycleHook
}

// NewASGFixtures builds and returns a fully-populated ASGFixtures struct.
var sharedASGFixtures = sync.OnceValue(func() *ASGFixtures {
	groups := buildASGGroups()
	activities := buildASGActivities()
	lcs := buildLaunchConfigurations()
	return &ASGFixtures{
		AutoScalingGroups:          groups,
		Activities:                 activities,
		LaunchConfigurations:       lcs,
		NotificationConfigurations: buildASGNotificationConfigurations(),
		LifecycleHooks:             buildASGLifecycleHooks(),
	}
})

// buildASGNotificationConfigurations wires acme-web-prod-asg's scaling
// notifications to the shared ops-alerts SNS topic — required for the
// asg:sns related-panel pivot (checkASGSNS).
func buildASGNotificationConfigurations() map[string][]asgtypes.NotificationConfiguration {
	return map[string][]asgtypes.NotificationConfiguration{
		"acme-web-prod-asg": {
			{
				AutoScalingGroupName: aws.String("acme-web-prod-asg"),
				TopicARN:             aws.String(relatedAlarmSNSARN),
				NotificationType:     aws.String("autoscaling:EC2_INSTANCE_LAUNCH"),
			},
		},
	}
}

// buildASGLifecycleHooks provides a lifecycle hook targeting the shared
// ops-alerts SNS topic — an alternate asg:sns witness path via
// NotificationTargetARN, mirroring the real AWS lifecycle-hook shape.
func buildASGLifecycleHooks() map[string][]asgtypes.LifecycleHook {
	return map[string][]asgtypes.LifecycleHook{
		"acme-web-prod-asg": {
			{
				AutoScalingGroupName:  aws.String("acme-web-prod-asg"),
				LifecycleHookName:     aws.String("acme-web-prod-drain-hook"),
				LifecycleTransition:   aws.String("autoscaling:EC2_INSTANCE_TERMINATING"),
				NotificationTargetARN: aws.String(relatedAlarmSNSARN),
				DefaultResult:         aws.String("CONTINUE"),
				HeartbeatTimeout:      aws.Int32(300),
			},
		},
	}
}

func NewASGFixtures() *ASGFixtures {
	return sharedASGFixtures()
}

// ASG posture witnesses — one demo group per Prowler-derived finding.
const (
	// ASGLegacyLaunchConfig is the only group still launching from a launch
	// configuration; every other group uses a launch template. It is
	// therefore also the only group the launch-configuration checks below
	// can apply to.
	ASGLegacyLaunchConfig = "acme-web-prod-asg"
	// ASGSingleAZ is the only group confined to one availability zone.
	ASGSingleAZ = "acme-staging-asg"
	// ASGNoELBHealthCheck is the only group attached to a target group while
	// still deciding health from EC2 status checks alone.
	ASGNoELBHealthCheck = "awseb-e-acmeprodapi-asg"
	// ASGLaunchConfigIMDSv1 / ASGLaunchConfigPublicIP / ASGLaunchConfigSecret
	// all name the same group: acme-web-prod-lc is the only launch
	// configuration in the demo, so all three signals land on its group.
	ASGLaunchConfigIMDSv1   = ASGLegacyLaunchConfig
	ASGLaunchConfigPublicIP = ASGLegacyLaunchConfig
	ASGLaunchConfigSecret   = ASGLegacyLaunchConfig
)

const (
	asgSubnetA = "subnet-0aaa111111111111a"
	asgSubnetB = "subnet-0bbb222222222222b"
	asgSubnetC = "subnet-0ccc333333333333c"
)

// asgAZsFor maps a group's VPCZoneIdentifier to the availability zones its
// subnets sit in, so AvailabilityZones can never drift from the subnet list.
// asg-staging is the single-AZ witness purely because it has one subnet.
func asgAZsFor(vpcZoneIdentifier string) []string {
	azBySubnet := map[string]string{
		asgSubnetA: "us-east-1a",
		asgSubnetB: "us-east-1b",
		asgSubnetC: "us-east-1c",
	}
	var azs []string
	for subnet := range strings.SplitSeq(vpcZoneIdentifier, ",") {
		if az, ok := azBySubnet[strings.TrimSpace(subnet)]; ok {
			azs = append(azs, az)
		}
	}
	return azs
}

func buildASGGroups() []asgtypes.AutoScalingGroup {
	groups := buildASGGroupsRaw()
	for i := range groups {
		groups[i].AvailabilityZones = asgAZsFor(aws.ToString(groups[i].VPCZoneIdentifier))
	}
	return groups
}

func buildASGGroupsRaw() []asgtypes.AutoScalingGroup {
	return []asgtypes.AutoScalingGroup{
		{
			AutoScalingGroupName:    aws.String("acme-web-prod-asg"),
			AutoScalingGroupARN:     aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:11111111-1111-1111-1111-111111111111:autoScalingGroupName/acme-web-prod-asg"),
			MinSize:                 aws.Int32(2),
			MaxSize:                 aws.Int32(10),
			DesiredCapacity:         aws.Int32(4),
			HealthCheckType:         aws.String("ELB"),
			HealthCheckGracePeriod:  aws.Int32(300),
			LaunchConfigurationName: aws.String("acme-web-prod-lc"),
			VPCZoneIdentifier:       aws.String(asgSubnetA + "," + asgSubnetB + "," + asgSubnetC),
			// TargetGroupARNs — required for tg→asg related-panel pivot
			// (checkTGASG). acme-web-tg (elb.go fixtProdWebTGARN) already
			// registers this ASG's instances as healthy targets.
			TargetGroupARNs: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"},
			CreatedTime:     aws.Time(mustTime("2025-01-15T10:00:00Z")),
			// Instances — required for ami→asg (via ec2 cache image_id match) and
			// ec2→asg related-panel pivots. Both instances launched from
			// fixtProdAMIID1 (ec2.go), so checkAMIASG's ec2-cache cross-reference
			// resolves this ASG for that AMI.
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0a1b2c3d4e5f60001"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0a1b2c3d4e5f60002"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("web")},
			},
		},
		{
			AutoScalingGroupName:   aws.String("acme-worker-batch-asg"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:22222222-2222-2222-2222-222222222222:autoScalingGroupName/acme-worker-batch-asg"),
			MinSize:                aws.Int32(0),
			MaxSize:                aws.Int32(20),
			DesiredCapacity:        aws.Int32(5),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(60),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-02-01T08:00:00Z")),
			// MixedInstancesPolicy — required for the lt->asg related-panel
			// pivot (mixed-instances path). References prod-web-lt (lt.go).
			MixedInstancesPolicy: &asgtypes.MixedInstancesPolicy{
				LaunchTemplate: &asgtypes.LaunchTemplate{
					LaunchTemplateSpecification: &asgtypes.LaunchTemplateSpecification{
						LaunchTemplateId: aws.String(ProdWebLTID),
						Version:          aws.String("$Default"),
					},
				},
			},
			// AmazonECSManaged — required for ecs→asg related-panel pivot.
			// Marks this ASG as owned by an ECS cluster capacity provider.
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("batch-worker")},
				{Key: aws.String("AmazonECSManaged"), Value: aws.String("true")},
			},
		},
		{
			AutoScalingGroupName:   aws.String("acme-staging-asg"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:33333333-3333-3333-3333-333333333333:autoScalingGroupName/acme-staging-asg"),
			MinSize:                aws.Int32(1),
			MaxSize:                aws.Int32(3),
			DesiredCapacity:        aws.Int32(2),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(120),
			VPCZoneIdentifier:      aws.String(asgSubnetA),
			CreatedTime:            aws.Time(mustTime("2025-03-10T12:00:00Z")),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0eee555555555555e"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			// LaunchTemplate — required for the lt->asg related-panel pivot
			// (plain single-template path). References prod-web-lt (lt.go).
			LaunchTemplate: &asgtypes.LaunchTemplateSpecification{
				LaunchTemplateId: aws.String(ProdWebLTID),
				Version:          aws.String("$Default"),
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		{
			AutoScalingGroupName: aws.String("awseb-e-acmeprodapi-asg"),
			AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:44444444-4444-4444-4444-444444444444:autoScalingGroupName/awseb-e-acmeprodapi-asg"),
			MinSize:              aws.Int32(1),
			MaxSize:              aws.Int32(4),
			DesiredCapacity:      aws.Int32(2),
			// The asg.no-elb-health-check witness: registered behind the api
			// target group yet still deciding health from EC2 status checks.
			// Healthy on every other signal so its phrase renders alone.
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(180),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-01-20T09:00:00Z")),
			TargetGroupARNs:        []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/0987654321fedcba"},
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0fff666666666666f"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0aaa777777777777a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("elasticbeanstalk:environment-name"), Value: aws.String("acme-prod-api")},
			},
		},
		{
			AutoScalingGroupName:   aws.String("eks-acme-prod-ng-general"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:55555555-5555-5555-5555-555555555555:autoScalingGroupName/eks-acme-prod-ng-general"),
			MinSize:                aws.Int32(2),
			MaxSize:                aws.Int32(8),
			DesiredCapacity:        aws.Int32(3),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(15),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB + "," + asgSubnetC),
			Status:                 aws.String("Delete in progress"),
			CreatedTime:            aws.Time(mustTime("2025-03-05T12:00:00Z")),
			// Instances — required for eks→ec2 related-panel pivot (via
			// ListNodegroups/DescribeNodegroup → this ASG → DescribeAutoScalingGroups).
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0a1b2c3d4e5f60003"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("eks:cluster-name"), Value: aws.String("acme-prod")},
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("general-pool")},
			},
		},
		// Issue: MinSize=5, Instances count < MinSize → Broken (underprovisioned)
		{
			AutoScalingGroupName:   aws.String("asg-underprovisioned"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:66666666-6666-6666-6666-666666666666:autoScalingGroupName/asg-underprovisioned"),
			MinSize:                aws.Int32(5),
			MaxSize:                aws.Int32(10),
			DesiredCapacity:        aws.Int32(5),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(300),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-06-01T10:00:00Z")),
			// Only 2 instances running while MinSize=5
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0aaa111111111111a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0bbb222222222222b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("api-worker")},
			},
		},
		// Issue: SuspendedProcesses includes Launch + HealthCheck → Warning
		// (scaling disabled). In-service instance count meets MinSize and
		// zero instances are Unhealthy, so this fixture demonstrates the
		// suspended-processes branch alone (asg.scaling.suspended), not the
		// underprovisioned or unhealthy branches that take precedence in
		// colorASG/the fetcher's Finding emission.
		{
			AutoScalingGroupName:   aws.String("asg-suspended"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:77777777-7777-7777-7777-777777777777:autoScalingGroupName/asg-suspended"),
			MinSize:                aws.Int32(1),
			MaxSize:                aws.Int32(5),
			DesiredCapacity:        aws.Int32(2),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(120),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-04-20T08:00:00Z")),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0ccc333333333333c"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0ddd444444444444d"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			SuspendedProcesses: []asgtypes.SuspendedProcess{
				{ProcessName: aws.String("Launch"), SuspensionReason: aws.String("User suspended the process")},
				{ProcessName: aws.String("HealthCheck"), SuspensionReason: aws.String("User suspended the process")},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("batch-processor")},
			},
		},
		// Issue: one Unhealthy instance while in-service count still meets
		// MinSize → Warning (asg.instances.unhealthy). Not underprovisioned
		// (2 in service >= MinSize=2), no suspended processes.
		{
			AutoScalingGroupName:   aws.String("asg-unhealthy-instance"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:88888888-8888-8888-8888-888888888888:autoScalingGroupName/asg-unhealthy-instance"),
			MinSize:                aws.Int32(2),
			MaxSize:                aws.Int32(6),
			DesiredCapacity:        aws.Int32(3),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(120),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-05-12T08:00:00Z")),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0eee555555555555e"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0fff666666666666f"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0aaa777777777777a"), HealthStatus: aws.String("Unhealthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("web-worker")},
			},
		},
		// Issue: latest scaling activity (MaxRecords=1, newest-first) has
		// StatusCode=Failed → EnrichASGScalingActivities fires
		// asg.scaling-activity-failed. In-service count meets MinSize, so
		// this fixture demonstrates the scaling-activity-failed branch alone.
		{
			AutoScalingGroupName:   aws.String("asg-scaling-failed"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:99999999-9999-9999-9999-999999999999:autoScalingGroupName/asg-scaling-failed"),
			MinSize:                aws.Int32(2),
			MaxSize:                aws.Int32(8),
			DesiredCapacity:        aws.Int32(2),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(120),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-07-01T08:00:00Z")),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0bbb888888888888b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0ccc999999999999c"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("payments-worker")},
			},
		},
	}
}

func buildASGActivities() map[string][]asgtypes.Activity {
	asgNames := []string{
		"acme-web-prod-asg",
		"acme-worker-batch-asg",
		"acme-staging-asg",
		"awseb-e-acmeprodapi-asg",
		"eks-acme-prod-ng-general",
	}
	result := make(map[string][]asgtypes.Activity, len(asgNames)+1)
	for _, name := range asgNames {
		result[name] = buildActivitiesFor(name)
	}
	// asg-scaling-failed's most-recent activity (index 0 — DescribeScalingActivities
	// returns newest-first) is itself Failed, unlike buildActivitiesFor's groups
	// where the newest activity is always Successful. Pins
	// EnrichASGScalingActivities's asg.scaling-activity-failed finding.
	result["asg-scaling-failed"] = []asgtypes.Activity{
		{
			ActivityId:           aws.String("act-demo-101"),
			AutoScalingGroupName: aws.String("asg-scaling-failed"),
			StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
			StatusMessage:        aws.String("Instance became unhealthy while waiting for instance to be in InService state"),
			Description:          aws.String("Launching a new EC2 instance: insufficient capacity"),
			Cause:                aws.String("At 2026-03-22T11:00:00Z an instance was launched in response to a difference between desired and actual capacity, but it failed to reach a healthy state"),
			StartTime:            aws.Time(mustTime("2026-03-22T11:00:00Z")),
			EndTime:              aws.Time(mustTime("2026-03-22T11:03:00Z")),
			Progress:             aws.Int32(0),
		},
		{
			ActivityId:           aws.String("act-demo-102"),
			AutoScalingGroupName: aws.String("asg-scaling-failed"),
			StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
			Description:          aws.String("Launching a new EC2 instance"),
			StartTime:            aws.Time(mustTime("2026-03-22T10:30:00Z")),
			EndTime:              aws.Time(mustTime("2026-03-22T10:35:00Z")),
			Progress:             aws.Int32(100),
		},
	}
	return result
}

// buildLaunchConfigurations returns a map of LC name → LaunchConfiguration for demo mode.
// Only the LC referenced by acme-web-prod-asg is populated; it carries the AMI and SG IDs
// needed by checkASGAMI and checkASGSG in demo mode.
func buildLaunchConfigurations() map[string]asgtypes.LaunchConfiguration {
	return map[string]asgtypes.LaunchConfiguration{
		"acme-web-prod-lc": {
			LaunchConfigurationName: aws.String("acme-web-prod-lc"),
			ImageId:                 aws.String(fixtProdAMIID1),
			InstanceType:            aws.String("m5.large"),
			SecurityGroups:          []string{"sg-0web111111111111w"},
			KeyName:                 aws.String("acme-prod-key"),
			CreatedTime:             aws.Time(mustTime("2025-01-10T09:00:00Z")),
			// IamInstanceProfile — required for the asg:role related-panel pivot
			// (checkASGRole via asgResolveInstanceProfile →
			// asgInstanceProfileToRoles → iam:GetInstanceProfile). Resolves to
			// acme-ec2-instance-role via fixtures/iam.go's InstanceProfiles map.
			IamInstanceProfile: aws.String("acme-ec2-instance-profile"),
			// The three launch-configuration witnesses. MetadataOptions is
			// deliberately absent: a launch configuration without it defaults
			// to IMDSv1-permitted, and launch configurations cannot be edited
			// to add it — that immutability is the point of the finding.
			AssociatePublicIpAddress: aws.Bool(true),
			UserData:                 aws.String(base64.StdEncoding.EncodeToString([]byte(asgLegacyUserData))),
		},
	}
}

// asgLegacyUserData is the acme-web-prod-lc bootstrap script — the
// asg.launch-config.secret witness, with the database password pasted in
// rather than resolved from Secrets Manager at boot.
const asgLegacyUserData = `#!/bin/bash
set -euo pipefail
yum install -y httpd
export DB_PASSWORD=Pr0dWebLegacy2024
/opt/acme/bin/web-server --db-user acme
`

func buildActivitiesFor(asgName string) []asgtypes.Activity {
	return []asgtypes.Activity{
		{
			ActivityId:           aws.String("act-demo-001"),
			AutoScalingGroupName: aws.String(asgName),
			StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
			Description:          aws.String("Launching a new EC2 instance"),
			StartTime:            aws.Time(mustTime("2026-03-22T10:00:00Z")),
			EndTime:              aws.Time(mustTime("2026-03-22T10:05:00Z")),
			Progress:             aws.Int32(100),
		},
		{
			ActivityId:           aws.String("act-demo-002"),
			AutoScalingGroupName: aws.String(asgName),
			StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
			Description:          aws.String("Terminating EC2 instance: instance replaced"),
			StartTime:            aws.Time(mustTime("2026-03-22T09:30:00Z")),
			EndTime:              aws.Time(mustTime("2026-03-22T09:35:00Z")),
			Progress:             aws.Int32(100),
		},
		{
			ActivityId:           aws.String("act-demo-003"),
			AutoScalingGroupName: aws.String(asgName),
			StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
			Description:          aws.String("Launching a new EC2 instance: capacity limit reached"),
			StartTime:            aws.Time(mustTime("2026-03-22T08:15:00Z")),
			EndTime:              aws.Time(mustTime("2026-03-22T08:16:00Z")),
			Progress:             aws.Int32(0),
		},
		{
			ActivityId:           aws.String("act-demo-004"),
			AutoScalingGroupName: aws.String(asgName),
			StatusCode:           asgtypes.ScalingActivityStatusCodeInProgress,
			Description:          aws.String("Launching a new EC2 instance: scale out triggered"),
			StartTime:            aws.Time(mustTime("2026-03-22T07:45:00Z")),
			Progress:             aws.Int32(50),
		},
		// Cancelled — required for asg_activities' Findings-based coloring
		// witness (asgActivityFindings, core/aws/asg_activities.go): a
		// StatusCode=Cancelled activity is the only demo witness of this
		// branch.
		{
			ActivityId:           aws.String("act-demo-005"),
			AutoScalingGroupName: aws.String(asgName),
			StatusCode:           asgtypes.ScalingActivityStatusCodeCancelled,
			Description:          aws.String("Terminating EC2 instance: cancelled by user"),
			Cause:                aws.String("An operator cancelled the scale-in activity before it completed"),
			StartTime:            aws.Time(mustTime("2026-03-22T07:00:00Z")),
			EndTime:              aws.Time(mustTime("2026-03-22T07:01:00Z")),
			Progress:             aws.Int32(0),
		},
	}
}

func init() {
	Register(Pin{ShortName: "asg", Rows: 9, Issues: 7, CoverageGaps: []string{"dim"}})
}
