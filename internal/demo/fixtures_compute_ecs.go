package demo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["ecs-svc"] = ecsServiceFixtures
	demoData["ecs"] = ecsClusterFixtures
	demoData["ecs-task"] = ecsTaskFixtures
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
	services := []resource.Resource{
		{
			ID:     "api-gateway",
			Name:   "api-gateway",
			Status: "ACTIVE",
			Fields: map[string]string{
				"service_name":  "api-gateway",
				"cluster":       "acme-services",
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
				"cluster":       "acme-services",
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
				"cluster":       "acme-services",
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
				"cluster":       "acme-batch",
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
				"cluster":       "acme-batch",
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

	// Generate 17 more ECS services to reach 22 total
	ecsLaunchTypes := []ecstypes.LaunchType{ecstypes.LaunchTypeFargate, ecstypes.LaunchTypeEc2}
	ecsClusters := []string{"acme-services", "acme-batch", "acme-staging"}
	ecsClusterArns := []string{ecsClusterArnServices, ecsClusterArnBatch, "arn:aws:ecs:us-east-1:123456789012:cluster/acme-staging"}
	ecsStatuses := []string{"ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE", "ACTIVE"}
	for i := 0; i < 17; i++ {
		name := ecsServiceNamePool[i]
		clusterIdx := i % len(ecsClusters)
		cluster := ecsClusters[clusterIdx]
		clusterArn := ecsClusterArns[clusterIdx]
		launchType := ecsLaunchTypes[i%len(ecsLaunchTypes)]
		desired := int32(1 + i%4)
		running := desired
		if i == 8 {
			running = desired - 1 // one service scaling up
		}
		pending := desired - running
		createdAt := fmt.Sprintf("2025-%02d-%02dT%02d:00:00Z", 3+(i%10), 1+i, 8+(i%12))
		tdVersion := i + 1

		services = append(services, resource.Resource{
			ID:     name,
			Name:   name,
			Status: ecsStatuses[i],
			Fields: map[string]string{
				"service_name":  name,
				"cluster":       cluster,
				"status":        ecsStatuses[i],
				"desired_count": fmt.Sprintf("%d", desired),
				"running_count": fmt.Sprintf("%d", running),
				"launch_type":   string(launchType),
			},
			RawStruct: ecstypes.Service{
				ServiceName:        aws.String(name),
				ServiceArn:         aws.String(fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:service/%s/%s", cluster, name)),
				ClusterArn:         aws.String(clusterArn),
				Status:             aws.String(ecsStatuses[i]),
				DesiredCount:       desired,
				RunningCount:       running,
				PendingCount:       pending,
				LaunchType:         launchType,
				TaskDefinition:     aws.String(fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task-definition/%s:%d", name, tdVersion)),
				SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
				CreatedAt:          aws.Time(mustParseTime(createdAt)),
				Tags: []ecstypes.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		})
	}

	return services
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
