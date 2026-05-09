// Package fixtures provides ASG fixture data for the ASG fake.
package fixtures

import (
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
}

// NewASGFixtures builds and returns a fully-populated ASGFixtures struct.
var sharedASGFixtures = sync.OnceValue(func() *ASGFixtures {
	groups := buildASGGroups()
	activities := buildASGActivities()
	lcs := buildLaunchConfigurations()
	return &ASGFixtures{
		AutoScalingGroups:    groups,
		Activities:           activities,
		LaunchConfigurations: lcs,
	}
})

func NewASGFixtures() *ASGFixtures {
	return sharedASGFixtures()
}

const (
	asgSubnetA = "subnet-0aaa111111111111a"
	asgSubnetB = "subnet-0bbb222222222222b"
	asgSubnetC = "subnet-0ccc333333333333c"
)

func buildASGGroups() []asgtypes.AutoScalingGroup {
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
			CreatedTime:             aws.Time(mustTime("2025-01-15T10:00:00Z")),
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
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("batch-worker")},
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
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		{
			AutoScalingGroupName:   aws.String("awseb-e-acmeprodapi-asg"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:44444444-4444-4444-4444-444444444444:autoScalingGroupName/awseb-e-acmeprodapi-asg"),
			MinSize:                aws.Int32(1),
			MaxSize:                aws.Int32(4),
			DesiredCapacity:        aws.Int32(2),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(180),
			VPCZoneIdentifier:      aws.String(asgSubnetA + "," + asgSubnetB),
			CreatedTime:            aws.Time(mustTime("2025-01-20T09:00:00Z")),
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
		// Issue: SuspendedProcesses includes Launch + HealthCheck → Warning (scaling disabled)
		{
			AutoScalingGroupName:   aws.String("asg-suspended"),
			AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:77777777-7777-7777-7777-777777777777:autoScalingGroupName/asg-suspended"),
			MinSize:                aws.Int32(1),
			MaxSize:                aws.Int32(5),
			DesiredCapacity:        aws.Int32(2),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(120),
			VPCZoneIdentifier:      aws.String(asgSubnetA),
			CreatedTime:            aws.Time(mustTime("2025-04-20T08:00:00Z")),
			SuspendedProcesses: []asgtypes.SuspendedProcess{
				{ProcessName: aws.String("Launch"), SuspensionReason: aws.String("User suspended the process")},
				{ProcessName: aws.String("HealthCheck"), SuspensionReason: aws.String("User suspended the process")},
			},
			Tags: []asgtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Service"), Value: aws.String("batch-processor")},
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
	result := make(map[string][]asgtypes.Activity, len(asgNames))
	for _, name := range asgNames {
		result[name] = buildActivitiesFor(name)
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
			ImageId:                 aws.String("ami-0abcdef1234567890"),
			InstanceType:            aws.String("m5.large"),
			SecurityGroups:          []string{"sg-0web111111111111w"},
			KeyName:                 aws.String("acme-prod-key"),
			CreatedTime:             aws.Time(mustTime("2025-01-10T09:00:00Z")),
		},
	}
}

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
	}
}
