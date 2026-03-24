package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["asg"] = asgFixtures
	demoData["eb"] = ebFixtures

	RegisterChildDemo("asg_activities", func(parentCtx map[string]string) []resource.Resource {
		return asgActivityFixtures(parentCtx["asg_name"])
	})
	RegisterChildDemo("ecs_svc_events", func(parentCtx map[string]string) []resource.Resource {
		return ecsSvcEventFixtures(parentCtx["service_name"])
	})
	RegisterChildDemo("ecs_tasks", func(parentCtx map[string]string) []resource.Resource {
		return ecsSvcTaskFixtures(parentCtx["service_name"])
	})
	RegisterChildDemo("ecs_svc_logs", func(parentCtx map[string]string) []resource.Resource {
		return ecsSvcLogFixtures(parentCtx["service_name"])
	})
}

// ---------------------------------------------------------------------------
// Auto Scaling Groups
// ---------------------------------------------------------------------------

// asgFixtures returns demo Auto Scaling Group fixtures.
func asgFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-web-prod-asg",
			Name:   "acme-web-prod-asg",
			Status: "",
			Fields: map[string]string{
				"asg_name":  "acme-web-prod-asg",
				"min_size":  "2",
				"max_size":  "10",
				"desired":   "4",
				"instances": "4",
				"status":    "",
			},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("acme-web-prod-asg"),
				AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:12345678-1234-1234-1234-123456789012:autoScalingGroupName/acme-web-prod-asg"),
				MinSize:              aws.Int32(2),
				MaxSize:              aws.Int32(10),
				DesiredCapacity:      aws.Int32(4),
				AvailabilityZones:    []string{"us-east-1a", "us-east-1b", "us-east-1c"},
				HealthCheckType:      aws.String("ELB"),
				HealthCheckGracePeriod: aws.Int32(300),
				DefaultCooldown:      aws.Int32(300),
				CreatedTime:          aws.Time(mustParseTime("2025-04-10T08:00:00Z")),
				VPCZoneIdentifier:    aws.String("subnet-0aaa111111111111a,subnet-0bbb222222222222b,subnet-0ccc333333333333c"),
				TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-prod/1234567890abcdef"},
				TerminationPolicies:  []string{"Default"},
				Instances: []asgtypes.Instance{
					{InstanceId: aws.String("i-0a1b2c3d4e5f60001"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0a1b2c3d4e5f60002"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0a1b2c3d4e5f60003"), AvailabilityZone: aws.String("us-east-1c"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0a1b2c3d4e5f60009"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				},
				Tags: []asgtypes.TagDescription{
					{Key: aws.String("Name"), Value: aws.String("acme-web-prod"), ResourceId: aws.String("acme-web-prod-asg"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
					{Key: aws.String("Environment"), Value: aws.String("prod"), ResourceId: aws.String("acme-web-prod-asg"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
				},
			},
		},
		{
			ID:     "acme-worker-batch-asg",
			Name:   "acme-worker-batch-asg",
			Status: "",
			Fields: map[string]string{
				"asg_name":  "acme-worker-batch-asg",
				"min_size":  "0",
				"max_size":  "20",
				"desired":   "5",
				"instances": "5",
				"status":    "",
			},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("acme-worker-batch-asg"),
				AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:22345678-1234-1234-1234-123456789012:autoScalingGroupName/acme-worker-batch-asg"),
				MinSize:              aws.Int32(0),
				MaxSize:              aws.Int32(20),
				DesiredCapacity:      aws.Int32(5),
				AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
				HealthCheckType:      aws.String("EC2"),
				HealthCheckGracePeriod: aws.Int32(120),
				DefaultCooldown:      aws.Int32(300),
				CreatedTime:          aws.Time(mustParseTime("2025-07-22T14:30:00Z")),
				VPCZoneIdentifier:    aws.String("subnet-0aaa111111111111a,subnet-0bbb222222222222b"),
				TerminationPolicies:  []string{"OldestInstance"},
				Instances: []asgtypes.Instance{
					{InstanceId: aws.String("i-0b1b2c3d4e5f60001"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0b1b2c3d4e5f60002"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0b1b2c3d4e5f60003"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0b1b2c3d4e5f60004"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0b1b2c3d4e5f60005"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				},
				Tags: []asgtypes.TagDescription{
					{Key: aws.String("Name"), Value: aws.String("acme-worker-batch"), ResourceId: aws.String("acme-worker-batch-asg"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
					{Key: aws.String("Environment"), Value: aws.String("prod"), ResourceId: aws.String("acme-worker-batch-asg"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
				},
			},
		},
		{
			ID:     "acme-staging-asg",
			Name:   "acme-staging-asg",
			Status: "",
			Fields: map[string]string{
				"asg_name":  "acme-staging-asg",
				"min_size":  "1",
				"max_size":  "3",
				"desired":   "2",
				"instances": "2",
				"status":    "",
			},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("acme-staging-asg"),
				AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:32345678-1234-1234-1234-123456789012:autoScalingGroupName/acme-staging-asg"),
				MinSize:              aws.Int32(1),
				MaxSize:              aws.Int32(3),
				DesiredCapacity:      aws.Int32(2),
				AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
				HealthCheckType:      aws.String("EC2"),
				HealthCheckGracePeriod: aws.Int32(300),
				DefaultCooldown:      aws.Int32(300),
				CreatedTime:          aws.Time(mustParseTime("2025-11-05T10:00:00Z")),
				VPCZoneIdentifier:    aws.String("subnet-0bbb222222222222b"),
				TerminationPolicies:  []string{"Default"},
				Instances: []asgtypes.Instance{
					{InstanceId: aws.String("i-0c1b2c3d4e5f60001"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0c1b2c3d4e5f60002"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				},
				Tags: []asgtypes.TagDescription{
					{Key: aws.String("Name"), Value: aws.String("acme-staging"), ResourceId: aws.String("acme-staging-asg"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
					{Key: aws.String("Environment"), Value: aws.String("staging"), ResourceId: aws.String("acme-staging-asg"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
				},
			},
		},
		{
			ID:     "eks-acme-prod-ng-general",
			Name:   "eks-acme-prod-ng-general",
			Status: "Delete in progress",
			Fields: map[string]string{
				"asg_name":  "eks-acme-prod-ng-general",
				"min_size":  "2",
				"max_size":  "8",
				"desired":   "3",
				"instances": "3",
				"status":    "Delete in progress",
			},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("eks-acme-prod-ng-general"),
				AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:42345678-1234-1234-1234-123456789012:autoScalingGroupName/eks-acme-prod-ng-general"),
				MinSize:              aws.Int32(2),
				MaxSize:              aws.Int32(8),
				DesiredCapacity:      aws.Int32(3),
				Status:               aws.String("Delete in progress"),
				AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
				HealthCheckType:      aws.String("EC2"),
				DefaultCooldown:      aws.Int32(300),
				CreatedTime:          aws.Time(mustParseTime("2025-09-15T06:00:00Z")),
				VPCZoneIdentifier:    aws.String("subnet-0aaa111111111111a,subnet-0bbb222222222222b"),
				TerminationPolicies:  []string{"Default"},
				Instances: []asgtypes.Instance{
					{InstanceId: aws.String("i-0d1b2c3d4e5f60001"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0d1b2c3d4e5f60002"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
					{InstanceId: aws.String("i-0d1b2c3d4e5f60003"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				},
				Tags: []asgtypes.TagDescription{
					{Key: aws.String("Name"), Value: aws.String("eks-acme-prod-ng-general"), ResourceId: aws.String("eks-acme-prod-ng-general"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
					{Key: aws.String("kubernetes.io/cluster/acme-prod"), Value: aws.String("owned"), ResourceId: aws.String("eks-acme-prod-ng-general"), ResourceType: aws.String("auto-scaling-group"), PropagateAtLaunch: aws.Bool(true)},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ASG Scaling Activities (child of Auto Scaling Groups)
// ---------------------------------------------------------------------------

// asgActivityFixtures returns demo ASG scaling activity fixtures.
func asgActivityFixtures(asgName string) []resource.Resource {
	ts1 := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	end1 := time.Date(2026, 3, 22, 10, 2, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 22, 9, 30, 0, 0, time.UTC)
	end2 := time.Date(2026, 3, 22, 9, 32, 0, 0, time.UTC)
	ts3 := time.Date(2026, 3, 22, 8, 15, 0, 0, time.UTC)
	end3 := time.Date(2026, 3, 22, 8, 16, 0, 0, time.UTC)
	ts4 := time.Date(2026, 3, 22, 7, 45, 0, 0, time.UTC)
	progress100 := int32(100)
	progress50 := int32(50)

	return []resource.Resource{
		{
			ID:     "act-demo-001",
			Name:   "2026-03-22 10:00:00",
			Status: "Successful",
			Fields: map[string]string{
				"start_time":  "2026-03-22 10:00:00",
				"status_code": "Successful",
				"description": "Launching a new EC2 instance: i-0abc1234def56789a",
				"cause":       "At 2026-03-22T10:00:00Z a monitor alarm TargetTracking-" + asgName + "-AlarmHigh was in state ALARM",
			},
			RawStruct: asgtypes.Activity{
				ActivityId:           aws.String("act-demo-001"),
				AutoScalingGroupName: aws.String(asgName),
				AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/" + asgName),
				Cause:                aws.String("At 2026-03-22T10:00:00Z a monitor alarm TargetTracking-" + asgName + "-AlarmHigh was in state ALARM"),
				Description:          aws.String("Launching a new EC2 instance: i-0abc1234def56789a"),
				StartTime:            &ts1,
				EndTime:              &end1,
				StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
				StatusMessage:        aws.String(""),
				Progress:             &progress100,
			},
		},
		{
			ID:     "act-demo-002",
			Name:   "2026-03-22 09:30:00",
			Status: "Successful",
			Fields: map[string]string{
				"start_time":  "2026-03-22 09:30:00",
				"status_code": "Successful",
				"description": "Terminating EC2 instance: i-0fed9876cba54321b",
				"cause":       "At 2026-03-22T09:30:00Z a monitor alarm TargetTracking-" + asgName + "-AlarmLow was in state ALARM",
			},
			RawStruct: asgtypes.Activity{
				ActivityId:           aws.String("act-demo-002"),
				AutoScalingGroupName: aws.String(asgName),
				AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/" + asgName),
				Cause:                aws.String("At 2026-03-22T09:30:00Z a monitor alarm TargetTracking-" + asgName + "-AlarmLow was in state ALARM"),
				Description:          aws.String("Terminating EC2 instance: i-0fed9876cba54321b"),
				StartTime:            &ts2,
				EndTime:              &end2,
				StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
				StatusMessage:        aws.String(""),
				Progress:             &progress100,
			},
		},
		{
			ID:     "act-demo-003",
			Name:   "2026-03-22 08:15:00",
			Status: "Failed",
			Fields: map[string]string{
				"start_time":  "2026-03-22 08:15:00",
				"status_code": "Failed",
				"description": "Launching a new EC2 instance. Status Reason: Your request for accessing resources in this region is being validated.",
				"cause":       "At 2026-03-22T08:15:00Z an instance was started in response to a difference between desired and actual capacity, increasing the capacity from 3 to 4.",
			},
			RawStruct: asgtypes.Activity{
				ActivityId:           aws.String("act-demo-003"),
				AutoScalingGroupName: aws.String(asgName),
				Cause:                aws.String("At 2026-03-22T08:15:00Z an instance was started in response to a difference between desired and actual capacity, increasing the capacity from 3 to 4."),
				Description:          aws.String("Launching a new EC2 instance. Status Reason: Your request for accessing resources in this region is being validated."),
				StartTime:            &ts3,
				EndTime:              &end3,
				StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
				StatusMessage:        aws.String("Your request for accessing resources in this region is being validated."),
				Progress:             &progress100,
			},
		},
		{
			ID:     "act-demo-004",
			Name:   "2026-03-22 07:45:00",
			Status: "InProgress",
			Fields: map[string]string{
				"start_time":  "2026-03-22 07:45:00",
				"status_code": "InProgress",
				"description": "Launching a new EC2 instance: i-0new1234launch567",
				"cause":       "At 2026-03-22T07:45:00Z an instance was started in response to a difference between desired and actual capacity, increasing the capacity from 3 to 4.",
			},
			RawStruct: asgtypes.Activity{
				ActivityId:           aws.String("act-demo-004"),
				AutoScalingGroupName: aws.String(asgName),
				Cause:                aws.String("At 2026-03-22T07:45:00Z an instance was started in response to a difference between desired and actual capacity, increasing the capacity from 3 to 4."),
				Description:          aws.String("Launching a new EC2 instance: i-0new1234launch567"),
				StartTime:            &ts4,
				StatusCode:           asgtypes.ScalingActivityStatusCodeInProgress,
				Progress:             &progress50,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Elastic Beanstalk Environments
// ---------------------------------------------------------------------------

// ebFixtures returns demo Elastic Beanstalk environment fixtures.
func ebFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "e-acmeprodapi",
			Name:   "acme-prod-api",
			Status: "Ready",
			Fields: map[string]string{
				"environment_name": "acme-prod-api",
				"environment_id":   "e-acmeprodapi",
				"application_name": "acme-api",
				"status":           "Ready",
				"health":           "Green",
				"version_label":    "v2.4.1",
				"solution_stack":   "64bit Amazon Linux 2023 v4.0.0 running Docker",
				"platform_arn":     "arn:aws:elasticbeanstalk:us-east-1::platform/Docker running on 64bit Amazon Linux 2023/4.0.0",
				"endpoint_url":     "acme-prod-api.us-east-1.elasticbeanstalk.com",
				"date_created":     "2025-05-20 09:00:00",
				"environment_arn":  "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-api/acme-prod-api",
			},
			RawStruct: ebtypes.EnvironmentDescription{
				EnvironmentName: aws.String("acme-prod-api"),
				EnvironmentId:   aws.String("e-acmeprodapi"),
				ApplicationName: aws.String("acme-api"),
				Status:          ebtypes.EnvironmentStatusReady,
				Health:          ebtypes.EnvironmentHealthGreen,
				HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
				VersionLabel:    aws.String("v2.4.1"),
				SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.0 running Docker"),
				PlatformArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1::platform/Docker running on 64bit Amazon Linux 2023/4.0.0"),
				EndpointURL:    aws.String("acme-prod-api.us-east-1.elasticbeanstalk.com"),
				CNAME:          aws.String("acme-prod-api.us-east-1.elasticbeanstalk.com"),
				DateCreated:    aws.Time(mustParseTime("2025-05-20T09:00:00Z")),
				DateUpdated:    aws.Time(mustParseTime("2026-03-10T14:22:00Z")),
				EnvironmentArn: aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-api/acme-prod-api"),
			},
		},
		{
			ID:     "e-acmestagapi",
			Name:   "acme-staging-api",
			Status: "Ready",
			Fields: map[string]string{
				"environment_name": "acme-staging-api",
				"environment_id":   "e-acmestagapi",
				"application_name": "acme-api",
				"status":           "Ready",
				"health":           "Yellow",
				"version_label":    "v2.5.0-rc1",
				"solution_stack":   "64bit Amazon Linux 2023 v4.0.0 running Docker",
				"platform_arn":     "arn:aws:elasticbeanstalk:us-east-1::platform/Docker running on 64bit Amazon Linux 2023/4.0.0",
				"endpoint_url":     "acme-staging-api.us-east-1.elasticbeanstalk.com",
				"date_created":     "2025-08-12 11:30:00",
				"environment_arn":  "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-api/acme-staging-api",
			},
			RawStruct: ebtypes.EnvironmentDescription{
				EnvironmentName: aws.String("acme-staging-api"),
				EnvironmentId:   aws.String("e-acmestagapi"),
				ApplicationName: aws.String("acme-api"),
				Status:          ebtypes.EnvironmentStatusReady,
				Health:          ebtypes.EnvironmentHealthYellow,
				HealthStatus:    ebtypes.EnvironmentHealthStatusWarning,
				VersionLabel:    aws.String("v2.5.0-rc1"),
				SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.0.0 running Docker"),
				PlatformArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1::platform/Docker running on 64bit Amazon Linux 2023/4.0.0"),
				EndpointURL:    aws.String("acme-staging-api.us-east-1.elasticbeanstalk.com"),
				CNAME:          aws.String("acme-staging-api.us-east-1.elasticbeanstalk.com"),
				DateCreated:    aws.Time(mustParseTime("2025-08-12T11:30:00Z")),
				DateUpdated:    aws.Time(mustParseTime("2026-03-18T16:05:00Z")),
				EnvironmentArn: aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-api/acme-staging-api"),
			},
		},
		{
			ID:     "e-acmeprodweb",
			Name:   "acme-prod-web",
			Status: "Updating",
			Fields: map[string]string{
				"environment_name": "acme-prod-web",
				"environment_id":   "e-acmeprodweb",
				"application_name": "acme-webapp",
				"status":           "Updating",
				"health":           "Green",
				"version_label":    "v3.1.0",
				"solution_stack":   "64bit Amazon Linux 2023 v6.0.0 running Node.js 20",
				"platform_arn":     "arn:aws:elasticbeanstalk:us-east-1::platform/Node.js 20 running on 64bit Amazon Linux 2023/6.0.0",
				"endpoint_url":     "acme-prod-web.us-east-1.elasticbeanstalk.com",
				"date_created":     "2025-03-01 08:00:00",
				"environment_arn":  "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-webapp/acme-prod-web",
			},
			RawStruct: ebtypes.EnvironmentDescription{
				EnvironmentName: aws.String("acme-prod-web"),
				EnvironmentId:   aws.String("e-acmeprodweb"),
				ApplicationName: aws.String("acme-webapp"),
				Status:          ebtypes.EnvironmentStatusUpdating,
				Health:          ebtypes.EnvironmentHealthGreen,
				HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
				VersionLabel:    aws.String("v3.1.0"),
				SolutionStackName: aws.String("64bit Amazon Linux 2023 v6.0.0 running Node.js 20"),
				PlatformArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1::platform/Node.js 20 running on 64bit Amazon Linux 2023/6.0.0"),
				EndpointURL:    aws.String("acme-prod-web.us-east-1.elasticbeanstalk.com"),
				CNAME:          aws.String("acme-prod-web.us-east-1.elasticbeanstalk.com"),
				DateCreated:    aws.Time(mustParseTime("2025-03-01T08:00:00Z")),
				DateUpdated:    aws.Time(mustParseTime("2026-03-21T09:30:00Z")),
				EnvironmentArn: aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-webapp/acme-prod-web"),
				AbortableOperationInProgress: aws.Bool(true),
			},
		},
		{
			ID:     "e-acmelegacy",
			Name:   "acme-legacy-worker",
			Status: "Terminating",
			Fields: map[string]string{
				"environment_name": "acme-legacy-worker",
				"environment_id":   "e-acmelegacy",
				"application_name": "acme-legacy",
				"status":           "Terminating",
				"health":           "Grey",
				"version_label":    "v1.0.0",
				"solution_stack":   "64bit Amazon Linux 2 v3.5.0 running Python 3.8",
				"platform_arn":     "arn:aws:elasticbeanstalk:us-east-1::platform/Python 3.8 running on 64bit Amazon Linux 2/3.5.0",
				"endpoint_url":     "",
				"date_created":     "2024-06-15 16:00:00",
				"environment_arn":  "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-legacy/acme-legacy-worker",
			},
			RawStruct: ebtypes.EnvironmentDescription{
				EnvironmentName: aws.String("acme-legacy-worker"),
				EnvironmentId:   aws.String("e-acmelegacy"),
				ApplicationName: aws.String("acme-legacy"),
				Status:          ebtypes.EnvironmentStatusTerminating,
				Health:          ebtypes.EnvironmentHealthGrey,
				HealthStatus:    ebtypes.EnvironmentHealthStatusNoData,
				VersionLabel:    aws.String("v1.0.0"),
				SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.0 running Python 3.8"),
				PlatformArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1::platform/Python 3.8 running on 64bit Amazon Linux 2/3.5.0"),
				DateCreated:    aws.Time(mustParseTime("2024-06-15T16:00:00Z")),
				DateUpdated:    aws.Time(mustParseTime("2026-03-21T08:00:00Z")),
				EnvironmentArn: aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/acme-legacy/acme-legacy-worker"),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ECS Service Events (child of ECS Services)
// ---------------------------------------------------------------------------

func ecsSvcEventFixtures(serviceName string) []resource.Resource {
	ts1 := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 22, 9, 55, 0, 0, time.UTC)
	ts3 := time.Date(2026, 3, 22, 9, 50, 0, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "evt-demo-001",
			Name:   "2026-03-22 10:00:00",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2026-03-22 10:00:00",
				"message":   "(service " + serviceName + ") has reached a steady state.",
			},
			RawStruct: ecstypes.ServiceEvent{
				Id:        aws.String("evt-demo-001"),
				CreatedAt: &ts1,
				Message:   aws.String("(service " + serviceName + ") has reached a steady state."),
			},
		},
		{
			ID:     "evt-demo-002",
			Name:   "2026-03-22 09:55:00",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2026-03-22 09:55:00",
				"message":   "(service " + serviceName + ") has started 2 tasks: (task abc123).",
			},
			RawStruct: ecstypes.ServiceEvent{
				Id:        aws.String("evt-demo-002"),
				CreatedAt: &ts2,
				Message:   aws.String("(service " + serviceName + ") has started 2 tasks: (task abc123)."),
			},
		},
		{
			ID:     "evt-demo-003",
			Name:   "2026-03-22 09:50:00",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2026-03-22 09:50:00",
				"message":   "(service " + serviceName + ") registered 1 targets in (target-group my-tg).",
			},
			RawStruct: ecstypes.ServiceEvent{
				Id:        aws.String("evt-demo-003"),
				CreatedAt: &ts3,
				Message:   aws.String("(service " + serviceName + ") registered 1 targets in (target-group my-tg)."),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ECS Service Tasks (child of ECS Services)
// ---------------------------------------------------------------------------

func ecsSvcTaskFixtures(serviceName string) []resource.Resource {
	startedAt := time.Date(2026, 3, 22, 8, 0, 0, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "a1b2c3d4e5f6",
			Name:   "a1b2c3d4e5f6",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id_short":  "a1b2c3d4e5f6",
				"status":         "RUNNING",
				"health":         "HEALTHY",
				"task_def_short": serviceName + ":5",
				"started_at":     "2026-03-22 08:00:00",
				"stopped_reason": "",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/a1b2c3d4e5f6"),
				LastStatus:        aws.String("RUNNING"),
				DesiredStatus:     aws.String("RUNNING"),
				HealthStatus:      ecstypes.HealthStatusHealthy,
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + serviceName + ":5"),
				StartedAt:         &startedAt,
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
			},
		},
		{
			ID:     "f6e5d4c3b2a1",
			Name:   "f6e5d4c3b2a1",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id_short":  "f6e5d4c3b2a1",
				"status":         "RUNNING",
				"health":         "HEALTHY",
				"task_def_short": serviceName + ":5",
				"started_at":     "2026-03-22 08:00:00",
				"stopped_reason": "",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/f6e5d4c3b2a1"),
				LastStatus:        aws.String("RUNNING"),
				DesiredStatus:     aws.String("RUNNING"),
				HealthStatus:      ecstypes.HealthStatusHealthy,
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + serviceName + ":5"),
				StartedAt:         &startedAt,
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ECS Service Logs (child of ECS Services)
// ---------------------------------------------------------------------------

func ecsSvcLogFixtures(serviceName string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-svc-log-demo-001",
			Name:   "INFO Starting application server on port 8080",
			Status: "",
			Fields: map[string]string{
				"timestamp":    "2026-03-22 10:00",
				"stream_short": "web/a1b2c3d4",
				"message":      "INFO Starting application server on port 8080",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp:     aws.Int64(1774278000000),
				Message:       aws.String("INFO Starting application server on port 8080"),
				LogStreamName: aws.String("ecs/web/a1b2c3d4e5f6"),
				EventId:       aws.String("evt-svc-log-demo-001"),
			},
		},
		{
			ID:     "evt-svc-log-demo-002",
			Name:   "INFO Health check passed",
			Status: "",
			Fields: map[string]string{
				"timestamp":    "2026-03-22 10:01",
				"stream_short": "web/a1b2c3d4",
				"message":      "INFO Health check passed",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp:     aws.Int64(1774278060000),
				Message:       aws.String("INFO Health check passed"),
				LogStreamName: aws.String("ecs/web/a1b2c3d4e5f6"),
				EventId:       aws.String("evt-svc-log-demo-002"),
			},
		},
		{
			ID:     "evt-svc-log-demo-003",
			Name:   "ERROR Connection refused to database",
			Status: "",
			Fields: map[string]string{
				"timestamp":    "2026-03-22 10:02",
				"stream_short": "web/f6e5d4c3",
				"message":      "ERROR Connection refused to database",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp:     aws.Int64(1774278120000),
				Message:       aws.String("ERROR Connection refused to database"),
				LogStreamName: aws.String("ecs/web/f6e5d4c3b2a1"),
				EventId:       aws.String("evt-svc-log-demo-003"),
			},
		},
	}
}
