package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["ec2"] = ec2Instances
	demoData["ecs-svc"] = ecsServiceFixtures
	demoData["ecs"] = ecsClusterFixtures
	demoData["ecs-task"] = ecsTaskFixtures
	demoData["asg"] = asgFixtures
	demoData["eb"] = ebFixtures
}

// ---------------------------------------------------------------------------
// EC2 Instances
// ---------------------------------------------------------------------------

// ec2Instances returns demo EC2 instance fixtures with populated RawStruct.
// Includes a mix of running/stopped/pending states and realistic naming for
// the demo scenario (filter /web must show results).
func ec2Instances() []resource.Resource {
	return []resource.Resource{
		makeEC2Instance(
			"i-0a1b2c3d4e5f60001", "web-prod-01", "running",
			ec2types.InstanceTypeT3Large, "10.0.1.10", "54.210.33.112",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 11, 15, 8, 30, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60002", "web-prod-02", "running",
			ec2types.InstanceTypeT3Large, "10.0.1.11", "54.210.33.113",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 11, 15, 8, 32, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60003", "api-staging-01", "running",
			ec2types.InstanceTypeM5Xlarge, "10.0.2.50", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60004", "worker-batch-03", "stopped",
			ec2types.InstanceTypeC5Xlarge, "10.0.3.100", "",
			"vpc-0abc123def456789a", "subnet-0ccc333333333333c",
			time.Date(2025, 9, 5, 11, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60005", "bastion-prod", "running",
			ec2types.InstanceTypeT3Micro, "10.0.0.5", "52.87.221.44",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60006", "db-proxy-01", "running",
			ec2types.InstanceTypeR5Large, "10.0.4.200", "",
			"vpc-0abc123def456789a", "subnet-0ddd444444444444d",
			time.Date(2025, 12, 10, 18, 45, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60007", "web-staging-01", "pending",
			ec2types.InstanceTypeT3Medium, "10.0.2.70", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60008", "ml-trainer-gpu", "stopping",
			ec2types.InstanceTypeG4dnXlarge, "10.0.5.30", "",
			"vpc-0abc123def456789a", "subnet-0eee555555555555e",
			time.Date(2026, 2, 14, 22, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60009", "temp-load-test", "shutting-down",
			ec2types.InstanceTypeC5Large, "10.0.3.55", "",
			"vpc-0abc123def456789a", "subnet-0ccc333333333333c",
			time.Date(2026, 3, 20, 16, 30, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60010", "old-migration-worker", "terminated",
			ec2types.InstanceTypeT3Small, "", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2025, 8, 1, 12, 0, 0, 0, time.UTC),
		),
	}
}

// makeEC2Instance constructs a resource.Resource with a fully populated
// ec2types.Instance as RawStruct. This enables both detail and YAML views
// in demo mode.
func makeEC2Instance(
	instanceID, name, state string,
	instanceType ec2types.InstanceType,
	privateIP, publicIP string,
	vpcID, subnetID string,
	launchTime time.Time,
) resource.Resource {
	stateName := ec2types.InstanceStateName(state)
	stateCode := stateNameToCode(stateName)

	inst := ec2types.Instance{
		InstanceId:       aws.String(instanceID),
		InstanceType:     instanceType,
		PrivateIpAddress: aws.String(privateIP),
		State: &ec2types.InstanceState{
			Name: stateName,
			Code: aws.Int32(stateCode),
		},
		VpcId:    aws.String(vpcID),
		SubnetId: aws.String(subnetID),
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String(name)},
			{Key: aws.String("Environment"), Value: aws.String(envFromName(name))},
		},
		LaunchTime: aws.Time(launchTime),
	}

	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}

	launchTimeStr := launchTime.Format("2006-01-02T15:04:05Z07:00")

	return resource.Resource{
		ID:     instanceID,
		Name:   name,
		Status: state,
		Fields: map[string]string{
			"instance_id": instanceID,
			"name":        name,
			"state":       state,
			"type":        string(instanceType),
			"private_ip":  privateIP,
			"public_ip":   publicIP,
			"launch_time": launchTimeStr,
		},
		RawStruct: inst,
	}
}

// stateNameToCode maps EC2 instance state names to their numeric codes.
func stateNameToCode(name ec2types.InstanceStateName) int32 {
	switch name {
	case ec2types.InstanceStateNamePending:
		return 0
	case ec2types.InstanceStateNameRunning:
		return 16
	case ec2types.InstanceStateNameShuttingDown:
		return 32
	case ec2types.InstanceStateNameTerminated:
		return 48
	case ec2types.InstanceStateNameStopping:
		return 64
	case ec2types.InstanceStateNameStopped:
		return 80
	default:
		return -1
	}
}

// envFromName infers an environment tag from the instance name.
func envFromName(name string) string {
	for _, prefix := range []string{"prod", "staging", "dev"} {
		for i := 0; i <= len(name)-len(prefix); i++ {
			if name[i:i+len(prefix)] == prefix {
				return prefix
			}
		}
	}
	return "prod"
}

// ---------------------------------------------------------------------------
// ECS Services
// ---------------------------------------------------------------------------

const (
	ecsClusterArnServices = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"
	ecsClusterArnBatch    = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch"
)

// ecsServiceFixtures returns demo ECS service fixtures.
// Services reference the two ECS clusters "acme-services" and "acme-batch".
func ecsServiceFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "api-gateway",
			Name:   "api-gateway",
			Status: "ACTIVE",
			Fields: map[string]string{
				"service_name":  "api-gateway",
				"cluster":       ecsClusterArnServices,
				"status":        "ACTIVE",
				"desired_count": "4",
				"running_count": "4",
				"launch_type":   "FARGATE",
			},
			RawStruct: ecstypes.Service{
				ServiceName:   aws.String("api-gateway"),
				ServiceArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/api-gateway"),
				ClusterArn:    aws.String(ecsClusterArnServices),
				Status:        aws.String("ACTIVE"),
				DesiredCount:  4,
				RunningCount:  4,
				PendingCount:  0,
				LaunchType:    ecstypes.LaunchTypeFargate,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
				SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
				CreatedAt:     aws.Time(mustParseTime("2025-06-15T10:30:00Z")),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Team"), Value: aws.String("platform")},
				},
			},
		},
		{
			ID:     "web-frontend",
			Name:   "web-frontend",
			Status: "ACTIVE",
			Fields: map[string]string{
				"service_name":  "web-frontend",
				"cluster":       ecsClusterArnServices,
				"status":        "ACTIVE",
				"desired_count": "3",
				"running_count": "3",
				"launch_type":   "FARGATE",
			},
			RawStruct: ecstypes.Service{
				ServiceName:   aws.String("web-frontend"),
				ServiceArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/web-frontend"),
				ClusterArn:    aws.String(ecsClusterArnServices),
				Status:        aws.String("ACTIVE"),
				DesiredCount:  3,
				RunningCount:  3,
				PendingCount:  0,
				LaunchType:    ecstypes.LaunchTypeFargate,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
				SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
				CreatedAt:     aws.Time(mustParseTime("2025-08-20T14:00:00Z")),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "order-worker",
			Name:   "order-worker",
			Status: "ACTIVE",
			Fields: map[string]string{
				"service_name":  "order-worker",
				"cluster":       ecsClusterArnServices,
				"status":        "ACTIVE",
				"desired_count": "2",
				"running_count": "1",
				"launch_type":   "FARGATE",
			},
			RawStruct: ecstypes.Service{
				ServiceName:   aws.String("order-worker"),
				ServiceArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/order-worker"),
				ClusterArn:    aws.String(ecsClusterArnServices),
				Status:        aws.String("ACTIVE"),
				DesiredCount:  2,
				RunningCount:  1,
				PendingCount:  1,
				LaunchType:    ecstypes.LaunchTypeFargate,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5"),
				SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
				CreatedAt:     aws.Time(mustParseTime("2025-10-01T09:15:00Z")),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "batch-etl-runner",
			Name:   "batch-etl-runner",
			Status: "ACTIVE",
			Fields: map[string]string{
				"service_name":  "batch-etl-runner",
				"cluster":       ecsClusterArnBatch,
				"status":        "ACTIVE",
				"desired_count": "1",
				"running_count": "1",
				"launch_type":   "EC2",
			},
			RawStruct: ecstypes.Service{
				ServiceName:   aws.String("batch-etl-runner"),
				ServiceArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-batch/batch-etl-runner"),
				ClusterArn:    aws.String(ecsClusterArnBatch),
				Status:        aws.String("ACTIVE"),
				DesiredCount:  1,
				RunningCount:  1,
				PendingCount:  0,
				LaunchType:    ecstypes.LaunchTypeEc2,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
				SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
				CreatedAt:     aws.Time(mustParseTime("2025-07-10T08:00:00Z")),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Team"), Value: aws.String("data")},
				},
			},
		},
		{
			ID:     "log-aggregator",
			Name:   "log-aggregator",
			Status: "DRAINING",
			Fields: map[string]string{
				"service_name":  "log-aggregator",
				"cluster":       ecsClusterArnBatch,
				"status":        "DRAINING",
				"desired_count": "0",
				"running_count": "1",
				"launch_type":   "EC2",
			},
			RawStruct: ecstypes.Service{
				ServiceName:   aws.String("log-aggregator"),
				ServiceArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-batch/log-aggregator"),
				ClusterArn:    aws.String(ecsClusterArnBatch),
				Status:        aws.String("DRAINING"),
				DesiredCount:  0,
				RunningCount:  1,
				PendingCount:  0,
				LaunchType:    ecstypes.LaunchTypeEc2,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7"),
				SchedulingStrategy: ecstypes.SchedulingStrategyDaemon,
				CreatedAt:     aws.Time(mustParseTime("2025-03-01T12:00:00Z")),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ECS Clusters
// ---------------------------------------------------------------------------

// ecsClusterFixtures returns demo ECS cluster fixtures.
// Two clusters: "acme-services" (main workloads) and "acme-batch" (batch jobs).
func ecsClusterFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-services",
			Name:   "acme-services",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":   "acme-services",
				"status":         "ACTIVE",
				"running_tasks":  "10",
				"pending_tasks":  "1",
				"services_count": "3",
			},
			RawStruct: ecstypes.Cluster{
				ClusterName:                      aws.String("acme-services"),
				ClusterArn:                       aws.String(ecsClusterArnServices),
				Status:                           aws.String("ACTIVE"),
				RunningTasksCount:                10,
				PendingTasksCount:                1,
				ActiveServicesCount:              3,
				RegisteredContainerInstancesCount: 0,
				CapacityProviders:                []string{"FARGATE", "FARGATE_SPOT"},
				Settings: []ecstypes.ClusterSetting{
					{Name: ecstypes.ClusterSettingNameContainerInsights, Value: aws.String("enabled")},
				},
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Team"), Value: aws.String("platform")},
				},
			},
		},
		{
			ID:     "acme-batch",
			Name:   "acme-batch",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":   "acme-batch",
				"status":         "ACTIVE",
				"running_tasks":  "3",
				"pending_tasks":  "0",
				"services_count": "2",
			},
			RawStruct: ecstypes.Cluster{
				ClusterName:                      aws.String("acme-batch"),
				ClusterArn:                       aws.String(ecsClusterArnBatch),
				Status:                           aws.String("ACTIVE"),
				RunningTasksCount:                3,
				PendingTasksCount:                0,
				ActiveServicesCount:              2,
				RegisteredContainerInstancesCount: 4,
				CapacityProviders:                []string{"FARGATE"},
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Team"), Value: aws.String("data")},
				},
			},
		},
		{
			ID:     "acme-staging",
			Name:   "acme-staging",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":   "acme-staging",
				"status":         "ACTIVE",
				"running_tasks":  "2",
				"pending_tasks":  "0",
				"services_count": "1",
			},
			RawStruct: ecstypes.Cluster{
				ClusterName:       aws.String("acme-staging"),
				ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-staging"),
				Status:            aws.String("ACTIVE"),
				RunningTasksCount: 2,
				PendingTasksCount: 0,
				ActiveServicesCount: 1,
				RegisteredContainerInstancesCount: 0,
				CapacityProviders: []string{"FARGATE"},
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ECS Tasks
// ---------------------------------------------------------------------------

// ecsTaskFixtures returns demo ECS task fixtures.
// Tasks reference clusters "acme-services" and "acme-batch".
func ecsTaskFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "a1b2c3d4e5f6a1b2c3d4e5f6",
			Name:   "a1b2c3d4e5f6a1b2c3d4e5f6",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id":         "a1b2c3d4e5f6a1b2c3d4e5f6",
				"cluster":         ecsClusterArnServices,
				"status":          "RUNNING",
				"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12",
				"launch_type":     "FARGATE",
				"cpu":             "512",
				"memory":          "1024",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/a1b2c3d4e5f6a1b2c3d4e5f6"),
				ClusterArn:        aws.String(ecsClusterArnServices),
				LastStatus:        aws.String("RUNNING"),
				DesiredStatus:     aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("512"),
				Memory:            aws.String("1024"),
				Group:             aws.String("service:api-gateway"),
				StartedAt:         aws.Time(mustParseTime("2026-03-20T08:15:00Z")),
				HealthStatus:      ecstypes.HealthStatusHealthy,
				Connectivity:      ecstypes.ConnectivityConnected,
				PlatformVersion:   aws.String("1.4.0"),
				PlatformFamily:    aws.String("Linux"),
				AvailabilityZone:  aws.String("us-east-1a"),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "b2c3d4e5f6a1b2c3d4e5f601",
			Name:   "b2c3d4e5f6a1b2c3d4e5f601",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id":         "b2c3d4e5f6a1b2c3d4e5f601",
				"cluster":         ecsClusterArnServices,
				"status":          "RUNNING",
				"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8",
				"launch_type":     "FARGATE",
				"cpu":             "256",
				"memory":          "512",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/b2c3d4e5f6a1b2c3d4e5f601"),
				ClusterArn:        aws.String(ecsClusterArnServices),
				LastStatus:        aws.String("RUNNING"),
				DesiredStatus:     aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
				Group:             aws.String("service:web-frontend"),
				StartedAt:         aws.Time(mustParseTime("2026-03-19T16:30:00Z")),
				HealthStatus:      ecstypes.HealthStatusHealthy,
				Connectivity:      ecstypes.ConnectivityConnected,
				PlatformVersion:   aws.String("1.4.0"),
				PlatformFamily:    aws.String("Linux"),
				AvailabilityZone:  aws.String("us-east-1b"),
			},
		},
		{
			ID:     "c3d4e5f6a1b2c3d4e5f60102",
			Name:   "c3d4e5f6a1b2c3d4e5f60102",
			Status: "PENDING",
			Fields: map[string]string{
				"task_id":         "c3d4e5f6a1b2c3d4e5f60102",
				"cluster":         ecsClusterArnServices,
				"status":          "PENDING",
				"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5",
				"launch_type":     "FARGATE",
				"cpu":             "1024",
				"memory":          "2048",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/c3d4e5f6a1b2c3d4e5f60102"),
				ClusterArn:        aws.String(ecsClusterArnServices),
				LastStatus:        aws.String("PENDING"),
				DesiredStatus:     aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5"),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("1024"),
				Memory:            aws.String("2048"),
				Group:             aws.String("service:order-worker"),
				CreatedAt:         aws.Time(mustParseTime("2026-03-21T09:45:00Z")),
				HealthStatus:      ecstypes.HealthStatusUnknown,
				PlatformVersion:   aws.String("1.4.0"),
				PlatformFamily:    aws.String("Linux"),
				AvailabilityZone:  aws.String("us-east-1a"),
			},
		},
		{
			ID:     "d4e5f6a1b2c3d4e5f6010203",
			Name:   "d4e5f6a1b2c3d4e5f6010203",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id":         "d4e5f6a1b2c3d4e5f6010203",
				"cluster":         ecsClusterArnBatch,
				"status":          "RUNNING",
				"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3",
				"launch_type":     "EC2",
				"cpu":             "2048",
				"memory":          "4096",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/d4e5f6a1b2c3d4e5f6010203"),
				ClusterArn:        aws.String(ecsClusterArnBatch),
				LastStatus:        aws.String("RUNNING"),
				DesiredStatus:     aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
				LaunchType:        ecstypes.LaunchTypeEc2,
				Cpu:               aws.String("2048"),
				Memory:            aws.String("4096"),
				Group:             aws.String("service:batch-etl-runner"),
				StartedAt:         aws.Time(mustParseTime("2026-03-21T02:00:00Z")),
				HealthStatus:      ecstypes.HealthStatusHealthy,
				Connectivity:      ecstypes.ConnectivityConnected,
				AvailabilityZone:  aws.String("us-east-1c"),
			},
		},
		{
			ID:     "e5f6a1b2c3d4e5f601020304",
			Name:   "e5f6a1b2c3d4e5f601020304",
			Status: "STOPPED",
			Fields: map[string]string{
				"task_id":         "e5f6a1b2c3d4e5f601020304",
				"cluster":         ecsClusterArnBatch,
				"status":          "STOPPED",
				"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7",
				"launch_type":     "EC2",
				"cpu":             "512",
				"memory":          "1024",
			},
			RawStruct: ecstypes.Task{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/e5f6a1b2c3d4e5f601020304"),
				ClusterArn:        aws.String(ecsClusterArnBatch),
				LastStatus:        aws.String("STOPPED"),
				DesiredStatus:     aws.String("STOPPED"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7"),
				LaunchType:        ecstypes.LaunchTypeEc2,
				Cpu:               aws.String("512"),
				Memory:            aws.String("1024"),
				Group:             aws.String("service:log-aggregator"),
				StartedAt:         aws.Time(mustParseTime("2026-03-20T06:00:00Z")),
				StoppedAt:         aws.Time(mustParseTime("2026-03-21T08:30:00Z")),
				StoppedReason:     aws.String("Service draining"),
				StopCode:          ecstypes.TaskStopCodeServiceSchedulerInitiated,
				AvailabilityZone:  aws.String("us-east-1b"),
			},
		},
	}
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

