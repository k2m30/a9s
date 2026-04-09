// Package fixtures provides ECS fixture data for the ECS fake.
package fixtures

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECSFixtures holds all ECS domain objects served by the fake.
type ECSFixtures struct {
	// Clusters is the full list returned by ListClusters / DescribeClusters.
	Clusters []ecstypes.Cluster
	// Services is the full list returned by ListServices / DescribeServices.
	Services []ecstypes.Service
	// Tasks is the full list returned by ListTasks / DescribeTasks.
	Tasks []ecstypes.Task
	// TaskDefinitions maps task definition ARN → TaskDefinition.
	TaskDefinitions map[string]*ecstypes.TaskDefinition
}

// NewECSFixtures builds and returns a fully-populated ECSFixtures struct.
func NewECSFixtures() *ECSFixtures {
	clusters := buildECSClusters()
	services := buildECSServices()
	tasks := buildECSTasks()
	tdefs := buildECSTaskDefinitions()
	return &ECSFixtures{
		Clusters:        clusters,
		Services:        services,
		Tasks:           tasks,
		TaskDefinitions: tdefs,
	}
}

const (
	ecsClusterArnServices = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"
	ecsClusterArnBatch    = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch"
	ecsClusterArnStaging  = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-staging"
)

var ecsServiceNamePool = []string{
	"metrics-collector", "user-auth-svc", "product-catalog", "search-svc",
	"notification-dispatcher", "email-service", "file-upload-svc", "payments-svc",
	"recommendation-engine", "analytics-collector", "session-manager",
	"report-builder", "data-importer", "audit-trail-svc", "config-manager",
	"rate-limiter-svc", "feature-flag-svc",
}

func buildECSClusters() []ecstypes.Cluster {
	return []ecstypes.Cluster{
		{
			ClusterName:                       aws.String("acme-services"),
			ClusterArn:                        aws.String(ecsClusterArnServices),
			Status:                            aws.String("ACTIVE"),
			RunningTasksCount:                 10,
			PendingTasksCount:                 1,
			ActiveServicesCount:               3,
			RegisteredContainerInstancesCount: 0,
			CapacityProviders:                 []string{"FARGATE", "FARGATE_SPOT"},
			DefaultCapacityProviderStrategy: []ecstypes.CapacityProviderStrategyItem{
				{CapacityProvider: aws.String("FARGATE"), Weight: 1, Base: 0},
			},
			Settings: []ecstypes.ClusterSetting{
				{Name: ecstypes.ClusterSettingNameContainerInsights, Value: aws.String("enabled")},
			},
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				},
			},
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
			},
		},
		{
			ClusterName:                       aws.String("acme-batch"),
			ClusterArn:                        aws.String(ecsClusterArnBatch),
			Status:                            aws.String("ACTIVE"),
			RunningTasksCount:                 3,
			PendingTasksCount:                 0,
			ActiveServicesCount:               2,
			RegisteredContainerInstancesCount: 4,
			CapacityProviders:                 []string{"FARGATE"},
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				},
			},
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Team"), Value: aws.String("data")},
			},
		},
		{
			ClusterName:                       aws.String("acme-staging"),
			ClusterArn:                        aws.String(ecsClusterArnStaging),
			Status:                            aws.String("ACTIVE"),
			RunningTasksCount:                 2,
			PendingTasksCount:                 0,
			ActiveServicesCount:               1,
			RegisteredContainerInstancesCount: 0,
			CapacityProviders:                 []string{"FARGATE"},
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				},
			},
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
	}
}

func buildECSServices() []ecstypes.Service {
	clusterArns := []string{ecsClusterArnServices, ecsClusterArnBatch, ecsClusterArnStaging}
	clusterNames := []string{"acme-services", "acme-batch", "acme-staging"}
	launchTypes := []ecstypes.LaunchType{ecstypes.LaunchTypeFargate, ecstypes.LaunchTypeEc2}

	named := []ecstypes.Service{
		{
			ServiceName:        aws.String("api-gateway"),
			ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/api-gateway"),
			ClusterArn:         aws.String(ecsClusterArnServices),
			Status:             aws.String("ACTIVE"),
			DesiredCount:       4,
			RunningCount:       4,
			PendingCount:       0,
			LaunchType:         ecstypes.LaunchTypeFargate,
			TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
			SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
			CreatedAt:          aws.Time(mustTime("2025-06-15T10:30:00Z")),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
			},
		},
		{
			ServiceName:        aws.String("web-frontend"),
			ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/web-frontend"),
			ClusterArn:         aws.String(ecsClusterArnServices),
			Status:             aws.String("ACTIVE"),
			DesiredCount:       3,
			RunningCount:       3,
			PendingCount:       0,
			LaunchType:         ecstypes.LaunchTypeFargate,
			TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
			SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
			CreatedAt:          aws.Time(mustTime("2025-08-20T14:00:00Z")),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			ServiceName:        aws.String("order-worker"),
			ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/order-worker"),
			ClusterArn:         aws.String(ecsClusterArnServices),
			Status:             aws.String("ACTIVE"),
			DesiredCount:       2,
			RunningCount:       1,
			PendingCount:       1,
			LaunchType:         ecstypes.LaunchTypeFargate,
			TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5"),
			SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
			CreatedAt:          aws.Time(mustTime("2025-10-01T09:15:00Z")),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			ServiceName:        aws.String("batch-etl-runner"),
			ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-batch/batch-etl-runner"),
			ClusterArn:         aws.String(ecsClusterArnBatch),
			Status:             aws.String("ACTIVE"),
			DesiredCount:       1,
			RunningCount:       1,
			PendingCount:       0,
			LaunchType:         ecstypes.LaunchTypeEc2,
			TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
			SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
			CreatedAt:          aws.Time(mustTime("2025-07-10T08:00:00Z")),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Team"), Value: aws.String("data")},
			},
		},
		{
			ServiceName:        aws.String("log-aggregator"),
			ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-batch/log-aggregator"),
			ClusterArn:         aws.String(ecsClusterArnBatch),
			Status:             aws.String("DRAINING"),
			DesiredCount:       0,
			RunningCount:       1,
			PendingCount:       0,
			LaunchType:         ecstypes.LaunchTypeEc2,
			TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7"),
			SchedulingStrategy: ecstypes.SchedulingStrategyDaemon,
			CreatedAt:          aws.Time(mustTime("2025-03-01T12:00:00Z")),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
	}

	// Generate 17 more services to reach 22 total.
	for i := range 17 {
		name := ecsServiceNamePool[i]
		clusterIdx := i % len(clusterArns)
		clusterArn := clusterArns[clusterIdx]
		clusterName := clusterNames[clusterIdx]
		launchType := launchTypes[i%len(launchTypes)]
		desired := int32(1 + i%4)
		running := desired
		if i == 8 {
			running = desired - 1
		}
		pending := desired - running
		createdAt := fmt.Sprintf("2025-%02d-%02dT%02d:00:00Z", 3+(i%10), 1+i, 8+(i%12))
		tdVersion := i + 1
		named = append(named, ecstypes.Service{
			ServiceName:        aws.String(name),
			ServiceArn:         aws.String(fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:service/%s/%s", clusterName, name)),
			ClusterArn:         aws.String(clusterArn),
			Status:             aws.String("ACTIVE"),
			DesiredCount:       desired,
			RunningCount:       running,
			PendingCount:       pending,
			LaunchType:         launchType,
			TaskDefinition:     aws.String(fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task-definition/%s:%d", name, tdVersion)),
			SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
			CreatedAt:          aws.Time(mustTime(createdAt)),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		})
	}

	return named
}

func buildECSTasks() []ecstypes.Task {
	return []ecstypes.Task{
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/a1b2c3d4e5f6a1b2c3d4e5f6"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("STOPPED"),
			DesiredStatus:     aws.String("STOPPED"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			Group:             aws.String("service:api-gateway"),
			StartedAt:         aws.Time(mustTime("2026-03-20T08:15:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/b2c3d4e5f6a1b2c3d4e5f601"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			Group:             aws.String("service:web-frontend"),
			StartedAt:         aws.Time(mustTime("2026-03-19T16:30:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1b"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/c3d4e5f6a1b2c3d4e5f60102"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("PENDING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("1024"),
			Memory:            aws.String("2048"),
			Group:             aws.String("service:order-worker"),
			CreatedAt:         aws.Time(mustTime("2026-03-21T09:45:00Z")),
			HealthStatus:      ecstypes.HealthStatusUnknown,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/d4e5f6a1b2c3d4e5f6010203"),
			ClusterArn:        aws.String(ecsClusterArnBatch),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
			LaunchType:        ecstypes.LaunchTypeEc2,
			Cpu:               aws.String("2048"),
			Memory:            aws.String("4096"),
			Group:             aws.String("service:batch-etl-runner"),
			StartedAt:         aws.Time(mustTime("2026-03-21T02:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			AvailabilityZone:  aws.String("us-east-1c"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/e5f6a1b2c3d4e5f601020304"),
			ClusterArn:        aws.String(ecsClusterArnBatch),
			LastStatus:        aws.String("STOPPED"),
			DesiredStatus:     aws.String("STOPPED"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7"),
			LaunchType:        ecstypes.LaunchTypeEc2,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			Group:             aws.String("service:log-aggregator"),
			StartedAt:         aws.Time(mustTime("2026-03-20T06:00:00Z")),
			StoppedAt:         aws.Time(mustTime("2026-03-21T08:30:00Z")),
			StoppedReason:     aws.String("Service draining"),
			StopCode:          ecstypes.TaskStopCodeServiceSchedulerInitiated,
			AvailabilityZone:  aws.String("us-east-1b"),
		},
	}
}

func buildECSTaskDefinitions() map[string]*ecstypes.TaskDefinition {
	defs := map[string]*ecstypes.TaskDefinition{
		"arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12": {
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
			Family:            aws.String("api-gateway"),
			Revision:          12,
			Status:            ecstypes.TaskDefinitionStatusActive,
			NetworkMode:       ecstypes.NetworkModeAwsvpc,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String("api"),
					Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest"),
					Cpu:   512,
					PortMappings: []ecstypes.PortMapping{
						{ContainerPort: aws.Int32(8080), Protocol: ecstypes.TransportProtocolTcp},
					},
				},
			},
		},
		"arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8": {
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
			Family:            aws.String("web-frontend"),
			Revision:          8,
			Status:            ecstypes.TaskDefinitionStatusActive,
			NetworkMode:       ecstypes.NetworkModeAwsvpc,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String("web"),
					Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/web-frontend:latest"),
					Cpu:   256,
					PortMappings: []ecstypes.PortMapping{
						{ContainerPort: aws.Int32(3000), Protocol: ecstypes.TransportProtocolTcp},
					},
				},
			},
		},
		"arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5": {
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5"),
			Family:            aws.String("order-worker"),
			Revision:          5,
			Status:            ecstypes.TaskDefinitionStatusActive,
			NetworkMode:       ecstypes.NetworkModeAwsvpc,
			Cpu:               aws.String("1024"),
			Memory:            aws.String("2048"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String("worker"),
					Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/order-worker:latest"),
					Cpu:   1024,
				},
			},
		},
		"arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3": {
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
			Family:            aws.String("batch-etl-runner"),
			Revision:          3,
			Status:            ecstypes.TaskDefinitionStatusActive,
			NetworkMode:       ecstypes.NetworkModeBridge,
			Cpu:               aws.String("2048"),
			Memory:            aws.String("4096"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String("etl"),
					Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-etl:latest"),
					Cpu:   2048,
				},
			},
		},
		"arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7": {
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/log-aggregator:7"),
			Family:            aws.String("log-aggregator"),
			Revision:          7,
			Status:            ecstypes.TaskDefinitionStatusActive,
			NetworkMode:       ecstypes.NetworkModeBridge,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String("fluent"),
					Image: aws.String("amazon/aws-for-fluent-bit:latest"),
					Cpu:   512,
				},
			},
		},
	}
	return defs
}
