// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides ECS fixture data for the ECS fake.
package fixtures

import (
	"fmt"
	"sync"

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
var sharedECSFixtures = sync.OnceValue(func() *ECSFixtures {
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
})

func NewECSFixtures() *ECSFixtures {
	return sharedECSFixtures()
}

// ECS posture witnesses.
const (
	// ECSServicePublicIP — the only service whose awsvpc configuration
	// assigns public IPs; every other service leaves AssignPublicIp unset.
	ECSServicePublicIP = "api-gateway"
	// The five task-definition signals each get their own RUNNING task on its
	// own revision. A task carries exactly one issue-severity finding that way:
	// a transitional or stopped task would add its lifecycle finding on top,
	// and a stopped one emits no posture finding at all (see ecsTaskGone).
	//
	// ECSTaskPrivileged runs web-frontend:7, the only definition with a
	// privileged container.
	ECSTaskPrivileged = "0a1b2c3d4e5f60010001000100010001"
	// ECSTaskHostNamespace runs order-worker:4, the only definition on the
	// host network and process namespace.
	ECSTaskHostNamespace = "0a1b2c3d4e5f60010001000100010002"
	// ECSTaskWritableRoot runs web-frontend:6, the only definition leaving a
	// container's root filesystem writable.
	ECSTaskWritableRoot = "0a1b2c3d4e5f60010001000100010003"
	// ECSTaskNoLogging runs batch-etl-runner:2, the only definition with a
	// container that has no log driver.
	ECSTaskNoLogging = "0a1b2c3d4e5f60010001000100010004"
	// ECSTaskEnvSecret runs order-worker:3, the only definition with a
	// plaintext credential in a container environment.
	ECSTaskEnvSecret = "0a1b2c3d4e5f60010001000100010005"
)

// Superseded revisions the witness tasks above still run.
const (
	ecsDefOrderWorkerOld    = "arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:4"
	ecsDefWebFrontendOld    = "arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:6"
	ecsDefBatchETLOld       = "arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:2"
	ecsDefOrderWorkerSecret = "arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:3"
	ecsDefWebFrontendPriv   = "arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:7"
)

const (
	// ECSServiceNoTasksRunning is the witness for ecs-svc.tasks.none-running,
	// and for the placement branch of the service event scan: it has no tasks
	// running because none can be placed, and its event says so.
	ECSServiceNoTasksRunning = "acme-svc-stalled"

	// ECSServiceBelowDesiredCount runs under its desired count, and is the
	// witness for the load-balancer branch of the event scan: the tasks that
	// did start are failing their health checks.
	ECSServiceBelowDesiredCount = "acme-svc-degraded"

	// Neither witness dates its own event. The enricher reads a ten-minute
	// window and these fixtures are built once per process, so a stored date
	// ages out of a session; the fake stamps them against the call instead
	// (core/demo/fakes/ecs.go, stampEvents).

	ecsClusterArnServices = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"
	ecsClusterArnBatch    = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch"
	ecsClusterArnStaging  = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-staging"
	ecsClusterArnFailed   = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster-failed"
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
			// aws:cloudformation:stack-name tag — required for ecs→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture (cfn.go).
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
		// Issue: Status=FAILED → Broken
		{
			ClusterName:                       aws.String("acme-cluster-failed"),
			ClusterArn:                        aws.String(ecsClusterArnFailed),
			Status:                            aws.String("FAILED"),
			RunningTasksCount:                 0,
			PendingTasksCount:                 0,
			ActiveServicesCount:               0,
			RegisteredContainerInstancesCount: 0,
			CapacityProviders:                 []string{"FARGATE"},
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		// Status=PROVISIONING → Warning (colorECSCluster)
		{
			ClusterName:                       aws.String("acme-cluster-provisioning"),
			ClusterArn:                        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster-provisioning"),
			Status:                            aws.String("PROVISIONING"),
			RunningTasksCount:                 0,
			PendingTasksCount:                 0,
			ActiveServicesCount:               0,
			RegisteredContainerInstancesCount: 0,
			CapacityProviders:                 []string{"FARGATE"},
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// Status=DEPROVISIONING → wave1 finding (CodeECSStateDeprovisioning, SevWarn) → Warning.
		{
			ClusterName:                       aws.String("acme-cluster-deprovisioning"),
			ClusterArn:                        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster-deprovisioning"),
			Status:                            aws.String("DEPROVISIONING"),
			RunningTasksCount:                 0,
			PendingTasksCount:                 0,
			ActiveServicesCount:               0,
			RegisteredContainerInstancesCount: 0,
			CapacityProviders:                 []string{"FARGATE"},
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		// Status=INACTIVE → wave1 finding (CodeECSStateInactive, SevBroken) → Broken.
		{
			ClusterName:                       aws.String("acme-cluster-inactive"),
			ClusterArn:                        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster-inactive"),
			Status:                            aws.String("INACTIVE"),
			RunningTasksCount:                 0,
			PendingTasksCount:                 0,
			ActiveServicesCount:               0,
			RegisteredContainerInstancesCount: 0,
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("dev")},
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
			// RoleArn — required for ecs-svc→role related-panel pivot.
			RoleArn: aws.String("arn:aws:iam::123456789012:role/acme-ecs-service-role"),
			// LoadBalancers — required for ecs-svc→tg and ecs-svc→elb related-panel
			// pivots. fixtProdWebTGARN is a real TG fixture (elb.go) attached to a
			// real ELB (fixtProdELBARN).
			LoadBalancers: []ecstypes.LoadBalancer{
				{TargetGroupArn: aws.String(fixtProdWebTGARN), ContainerName: aws.String("api"), ContainerPort: aws.Int32(8080)},
			},
			// NetworkConfiguration — required for ecs-svc→sg, ecs-svc→subnet, and
			// ecs-svc→vpc related-panel pivots. SG and subnet are real ec2.go fixtures.
			NetworkConfiguration: &ecstypes.NetworkConfiguration{
				AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
					SecurityGroups: []string{"sg-0bbb222222222222b"},
					Subnets:        []string{"subnet-0aaa111111111111a"},
					// The ecs-svc.public-ip witness — the only demo service
					// that hands its tasks routable addresses.
					AssignPublicIp: ecstypes.AssignPublicIpEnabled,
				},
			},
			// aws:cloudformation:stack-name tag — required for ecs-svc→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture (cfn.go).
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-eks-cluster")},
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
		{
			ServiceName:        aws.String("canary-sidecar"),
			ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-staging/canary-sidecar"),
			ClusterArn:         aws.String(ecsClusterArnStaging),
			Status:             aws.String("DRAINING"),
			DesiredCount:       0,
			RunningCount:       2,
			PendingCount:       0,
			LaunchType:         ecstypes.LaunchTypeFargate,
			TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/canary-sidecar:2"),
			SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
			CreatedAt:          aws.Time(mustTime("2025-11-10T09:00:00Z")),
			Tags: []ecstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("staging")},
				{Key: aws.String("Team"), Value: aws.String("reliability")},
			},
		},
	}

	// Issue: Status=INACTIVE → Broken (service deregistered / stopped)
	named = append(named, ecstypes.Service{
		ServiceName:        aws.String("acme-svc-inactive"),
		ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/acme-svc-inactive"),
		ClusterArn:         aws.String(ecsClusterArnServices),
		Status:             aws.String("INACTIVE"),
		DesiredCount:       0,
		RunningCount:       0,
		PendingCount:       0,
		LaunchType:         ecstypes.LaunchTypeFargate,
		TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-svc-inactive:1"),
		SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
		CreatedAt:          aws.Time(mustTime("2024-09-01T08:00:00Z")),
		Tags: []ecstypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})

	// Issue: ACTIVE but RunningCount < DesiredCount → Warning (underprovisioned)
	named = append(named, ecstypes.Service{
		ServiceName:        aws.String(ECSServiceBelowDesiredCount),
		ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/acme-svc-degraded"),
		ClusterArn:         aws.String(ecsClusterArnServices),
		Status:             aws.String("ACTIVE"),
		DesiredCount:       5,
		RunningCount:       2,
		PendingCount:       0,
		LaunchType:         ecstypes.LaunchTypeFargate,
		TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-svc-degraded:3"),
		SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
		CreatedAt:          aws.Time(mustTime("2025-01-12T10:00:00Z")),
		Events: []ecstypes.ServiceEvent{{
			Message: aws.String("(service " + ECSServiceBelowDesiredCount + ") (instance i-0a1b2c3d4e5f60001) " +
				"(port 8080) is unhealthy in (target-group acme-web-tg) due to " +
				"(reason Health checks failed with these codes: [502])."),
		}},
		Tags: []ecstypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
			{Key: aws.String("Team"), Value: aws.String("platform")},
		},
	})

	// Issue: ACTIVE, wants tasks, none running → Broken. The only demo service
	// with RunningCount 0 against a non-zero DesiredCount; every other service
	// is either at its desired count, deliberately scaled to zero, or the
	// below-desired witness above.
	named = append(named, ecstypes.Service{
		ServiceName:        aws.String(ECSServiceNoTasksRunning),
		ServiceArn:         aws.String("arn:aws:ecs:us-east-1:123456789012:service/acme-services/" + ECSServiceNoTasksRunning),
		ClusterArn:         aws.String(ecsClusterArnServices),
		Status:             aws.String("ACTIVE"),
		DesiredCount:       2,
		RunningCount:       0,
		PendingCount:       0,
		LaunchType:         ecstypes.LaunchTypeFargate,
		TaskDefinition:     aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + ECSServiceNoTasksRunning + ":1"),
		SchedulingStrategy: ecstypes.SchedulingStrategyReplica,
		CreatedAt:          aws.Time(mustTime("2025-06-04T11:00:00Z")),
		Events: []ecstypes.ServiceEvent{{
			Message: aws.String("(service " + ECSServiceNoTasksRunning + ") was unable to place a task " +
				"because no container instance met all of its requirements. The closest matching " +
				"container-instance 0a1b2c3d has insufficient memory available. For more information, " +
				"see the Troubleshooting section of the Amazon ECS Developer Guide."),
		}},
		Tags: []ecstypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		},
	})

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
			// Attachments — required for ecs-task→eni and ecs-task→subnet
			// related-panel pivots (awsvpc networking mode).
			Attachments: []ecstypes.Attachment{
				{
					Type: aws.String("ElasticNetworkInterface"),
					Details: []ecstypes.KeyValuePair{
						{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-0aaa111111111111a")},
						{Name: aws.String("subnetId"), Value: aws.String("subnet-0aaa111111111111a")},
					},
				},
			},
			// Containers — required for ecs-task→ecr related-panel pivot.
			Containers: []ecstypes.Container{
				{Name: aws.String("api"), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest")},
			},
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
			// ContainerInstanceArn — required for ecs-task→ec2 related-panel
			// pivot (EC2 launch-type task).
			ContainerInstanceArn: aws.String("arn:aws:ecs:us-east-1:123456789012:container-instance/acme-batch/e1f2a3b4c5d6e1f2a3b4c5d6"),
			Cpu:                  aws.String("2048"),
			Memory:               aws.String("4096"),
			Group:                aws.String("service:batch-etl-runner"),
			StartedAt:            aws.Time(mustTime("2026-03-21T02:00:00Z")),
			HealthStatus:         ecstypes.HealthStatusHealthy,
			Connectivity:         ecstypes.ConnectivityConnected,
			AvailabilityZone:     aws.String("us-east-1c"),
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
		// Issue: lastStatus=STOPPED, StopCode=TaskFailedToStart → Broken
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/f6a1b2c3d4e5f60102030405"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("STOPPED"),
			DesiredStatus:     aws.String("STOPPED"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-svc-degraded:3"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			Group:             aws.String("service:acme-svc-degraded"),
			CreatedAt:         aws.Time(mustTime("2026-04-18T07:00:00Z")),
			StoppedAt:         aws.Time(mustTime("2026-04-18T07:02:00Z")),
			StoppedReason:     aws.String("Task failed to start: container runtime error"),
			StopCode:          ecstypes.TaskStopCodeTaskFailedToStart,
			HealthStatus:      ecstypes.HealthStatusUnknown,
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		// Issue: lastStatus=RUNNING, healthStatus=UNHEALTHY → Broken
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/a7b8c9d0e1f2a7b8c9d0e1f2"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-gateway:12"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			Group:             aws.String("service:api-gateway"),
			StartedAt:         aws.Time(mustTime("2026-04-17T20:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusUnhealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1c"),
		},
		// LastStatus=ACTIVATING → wave1 finding (CodeECSTaskStateActivating, SevWarn) → Warning.
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/b8c9d0e1f2a7b8c9d0e1f2a8"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("ACTIVATING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			Group:             aws.String("service:web-frontend"),
			CreatedAt:         aws.Time(mustTime("2026-04-27T10:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusUnknown,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		// LastStatus=DEACTIVATING → wave1 finding (CodeECSTaskStateDeactivating, SevWarn) → Warning.
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/c9d0e1f2a7b8c9d0e1f2a8b9"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("DEACTIVATING"),
			DesiredStatus:     aws.String("STOPPED"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/order-worker:5"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("1024"),
			Memory:            aws.String("2048"),
			Group:             aws.String("service:order-worker"),
			StartedAt:         aws.Time(mustTime("2026-04-27T08:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		// LastStatus=STOPPING → wave1 finding (CodeECSTaskStateStopping, SevWarn) → Warning.
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/d0e1f2a7b8c9d0e1f2a8b9c0"),
			ClusterArn:        aws.String(ecsClusterArnBatch),
			LastStatus:        aws.String("STOPPING"),
			DesiredStatus:     aws.String("STOPPED"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
			LaunchType:        ecstypes.LaunchTypeEc2,
			Cpu:               aws.String("2048"),
			Memory:            aws.String("4096"),
			Group:             aws.String("service:batch-etl-runner"),
			StartedAt:         aws.Time(mustTime("2026-04-27T02:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			AvailabilityZone:  aws.String("us-east-1c"),
		},
		// LastStatus=PROVISIONING → wave1 finding (CodeECSTaskStateProvisioning, SevWarn) → Warning.
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/e1f2a7b8c9d0e1f2a8b9c0d1"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("PROVISIONING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-frontend:8"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			Group:             aws.String("service:web-frontend"),
			CreatedAt:         aws.Time(mustTime("2026-05-04T11:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusUnknown,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1b"),
		},
		// LastStatus=DEPROVISIONING → wave1 finding (CodeECSTaskStateDeprovisioning, SevWarn) → Warning.
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/f2a7b8c9d0e1f2a8b9c0d1e2"),
			ClusterArn:        aws.String(ecsClusterArnBatch),
			LastStatus:        aws.String("DEPROVISIONING"),
			DesiredStatus:     aws.String("STOPPED"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/batch-etl-runner:3"),
			LaunchType:        ecstypes.LaunchTypeEc2,
			Cpu:               aws.String("2048"),
			Memory:            aws.String("4096"),
			Group:             aws.String("service:batch-etl-runner"),
			StartedAt:         aws.Time(mustTime("2026-05-04T03:00:00Z")),
			HealthStatus:      ecstypes.HealthStatusUnknown,
			Connectivity:      ecstypes.ConnectivityConnected,
			AvailabilityZone:  aws.String("us-east-1c"),
		},
		// Healthy, running tasks whose only signal is their definition's own
		// posture — one per task-definition rule, so no posture finding ever
		// shares a row with a lifecycle finding.
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/0a1b2c3d4e5f60010001000100010001"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String(ecsDefWebFrontendPriv),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			Group:             aws.String("service:web-frontend"),
			CreatedAt:         aws.Time(mustTime("2026-04-26T09:00:00Z")),
			StartedAt:         aws.Time(mustTime("2026-04-26T09:01:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/0a1b2c3d4e5f60010001000100010002"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String(ecsDefOrderWorkerOld),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("1024"),
			Memory:            aws.String("2048"),
			Group:             aws.String("service:order-worker"),
			CreatedAt:         aws.Time(mustTime("2026-04-26T09:00:00Z")),
			StartedAt:         aws.Time(mustTime("2026-04-26T09:01:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1b"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/0a1b2c3d4e5f60010001000100010003"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String(ecsDefWebFrontendOld),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			Group:             aws.String("service:web-frontend"),
			CreatedAt:         aws.Time(mustTime("2026-04-26T09:00:00Z")),
			StartedAt:         aws.Time(mustTime("2026-04-26T09:01:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1c"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-batch/0a1b2c3d4e5f60010001000100010004"),
			ClusterArn:        aws.String(ecsClusterArnBatch),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String(ecsDefBatchETLOld),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("2048"),
			Memory:            aws.String("4096"),
			Group:             aws.String("service:batch-etl-runner"),
			CreatedAt:         aws.Time(mustTime("2026-04-26T09:00:00Z")),
			StartedAt:         aws.Time(mustTime("2026-04-26T09:01:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
			AvailabilityZone:  aws.String("us-east-1a"),
		},
		{
			TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/0a1b2c3d4e5f60010001000100010005"),
			ClusterArn:        aws.String(ecsClusterArnServices),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String(ecsDefOrderWorkerSecret),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("1024"),
			Memory:            aws.String("2048"),
			Group:             aws.String("service:order-worker"),
			CreatedAt:         aws.Time(mustTime("2026-04-26T09:00:00Z")),
			StartedAt:         aws.Time(mustTime("2026-04-26T09:01:00Z")),
			HealthStatus:      ecstypes.HealthStatusHealthy,
			Connectivity:      ecstypes.ConnectivityConnected,
			PlatformVersion:   aws.String("1.4.0"),
			PlatformFamily:    aws.String("Linux"),
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
			// TaskRoleArn/ExecutionRoleArn — required for the ecs-task:role
			// related-panel pivot witness (checkECSTaskRole, Count:2).
			// acme-lambda-execution and acme-ci-deploy-role are real iam.go
			// role fixtures.
			TaskRoleArn:      aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
			ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String("api"),
					Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest"),
					Cpu:   512,
					PortMappings: []ecstypes.PortMapping{
						{ContainerPort: aws.Int32(8080), Protocol: ecstypes.TransportProtocolTcp},
					},
					// Secrets — required for the ecs-task:secrets and
					// ecs-task:ssm related-panel pivot witnesses. DB_PASSWORD
					// and API_KEY are Secrets Manager ARNs (secrets.go
					// fixtures prod/database/primary and prod/api/gateway-key);
					// CONFIG_PARAM is an SSM parameter ARN (ssm.go fixture
					// /acme/prod/app/config).
					Secrets: []ecstypes.Secret{
						{
							Name:      aws.String("DB_PASSWORD"),
							ValueFrom: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"),
						},
						{
							Name:      aws.String("API_KEY"),
							ValueFrom: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/gateway-key-XyZ123"),
						},
						{
							Name:      aws.String("CONFIG_PARAM"),
							ValueFrom: aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/app/config"),
						},
					},
				},
			},
			// EFS volume — required for efs→ecs-task related-panel pivot.
			// checkEFSECSTask scans Volumes[].EfsVolumeConfiguration.FileSystemId == ProdEFSID.
			Volumes: []ecstypes.Volume{
				{
					Name: aws.String("efs-app-data"),
					EfsVolumeConfiguration: &ecstypes.EFSVolumeConfiguration{
						FileSystemId: aws.String(ProdEFSID),
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
			// EFS volume — required for efs→ecs-task related-panel pivot.
			// checkEFSECSTask scans Volumes[].EfsVolumeConfiguration.FileSystemId == ProdEFSID.
			Volumes: []ecstypes.Volume{
				{
					Name: aws.String("efs-app-data"),
					EfsVolumeConfiguration: &ecstypes.EFSVolumeConfiguration{
						FileSystemId: aws.String(ProdEFSID),
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

	// The four superseded revisions the posture witness tasks still run —
	// each carries exactly one of the task-definition signals, so no signal
	// ever lands on more than the one task pinned to that revision.
	defs[ecsDefOrderWorkerSecret] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(ecsDefOrderWorkerSecret),
		Family:            aws.String("order-worker"),
		Revision:          3,
		Status:            ecstypes.TaskDefinitionStatusActive,
		NetworkMode:       ecstypes.NetworkModeAwsvpc,
		Cpu:               aws.String("1024"),
		Memory:            aws.String("2048"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("worker"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/order-worker:1.3.0"),
				Cpu:   1024,
				// The ecs-task.env-secret witness: revision 5 moved this
				// token into the Secrets block, revision 3 still pastes it.
				Environment: []ecstypes.KeyValuePair{
					{Name: aws.String("LOG_LEVEL"), Value: aws.String("info")},
					{Name: aws.String("LEGACY_API_TOKEN"), Value: aws.String("t0kEn-9f3a71c4bb2e5d80")},
				},
			},
		},
	}
	defs[ecsDefWebFrontendPriv] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(ecsDefWebFrontendPriv),
		Family:            aws.String("web-frontend"),
		Revision:          7,
		Status:            ecstypes.TaskDefinitionStatusActive,
		NetworkMode:       ecstypes.NetworkModeAwsvpc,
		Cpu:               aws.String("256"),
		Memory:            aws.String("512"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("web"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/web-frontend:1.7.0"),
				Cpu:   256,
				// The ecs-task.privileged witness: revision 7 was rolled back
				// to a privileged sidecar-debugging image.
				Privileged: aws.Bool(true),
			},
		},
	}
	defs[ecsDefWebFrontendOld] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(ecsDefWebFrontendOld),
		Family:            aws.String("web-frontend"),
		Revision:          6,
		Status:            ecstypes.TaskDefinitionStatusActive,
		NetworkMode:       ecstypes.NetworkModeAwsvpc,
		Cpu:               aws.String("256"),
		Memory:            aws.String("512"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("web"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/web-frontend:1.6.0"),
				Cpu:   256,
			},
		},
	}
	defs[ecsDefOrderWorkerOld] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(ecsDefOrderWorkerOld),
		Family:            aws.String("order-worker"),
		Revision:          4,
		Status:            ecstypes.TaskDefinitionStatusActive,
		// The ecs-task.host-namespace witness.
		NetworkMode: ecstypes.NetworkModeHost,
		PidMode:     ecstypes.PidModeHost,
		Cpu:         aws.String("1024"),
		Memory:      aws.String("2048"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("worker"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/order-worker:1.4.0"),
				Cpu:   1024,
			},
		},
	}
	defs[ecsDefBatchETLOld] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(ecsDefBatchETLOld),
		Family:            aws.String("batch-etl-runner"),
		Revision:          2,
		Status:            ecstypes.TaskDefinitionStatusActive,
		NetworkMode:       ecstypes.NetworkModeBridge,
		Cpu:               aws.String("2048"),
		Memory:            aws.String("4096"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("etl"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-etl:1.2.0"),
				Cpu:   2048,
			},
		},
	}

	// acme-svc-degraded:3 backs the TaskFailedToStart task above. Registered
	// for the same reason as the filler definitions below: a demo graph never
	// points a task at a definition the fake cannot describe.
	defs["arn:aws:ecs:us-east-1:123456789012:task-definition/acme-svc-degraded:3"] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-svc-degraded:3"),
		Family:            aws.String("acme-svc-degraded"),
		Revision:          3,
		Status:            ecstypes.TaskDefinitionStatusActive,
		NetworkMode:       ecstypes.NetworkModeAwsvpc,
		Cpu:               aws.String("512"),
		Memory:            aws.String("1024"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("degraded"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/degraded:latest"),
				Cpu:   512,
			},
		},
	}

	defs["arn:aws:ecs:us-east-1:123456789012:task-definition/"+ECSServiceNoTasksRunning+":1"] = &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + ECSServiceNoTasksRunning + ":1"),
		Family:            aws.String(ECSServiceNoTasksRunning),
		Revision:          1,
		Status:            ecstypes.TaskDefinitionStatusActive,
		NetworkMode:       ecstypes.NetworkModeAwsvpc,
		Cpu:               aws.String("256"),
		Memory:            aws.String("512"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name:  aws.String("stalled"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/stalled:latest"),
				Cpu:   256,
			},
		},
	}

	// The generated filler services (ecsServiceNamePool) reference task-def
	// "<name>:<i+1>" (matching buildECSServices' tdVersion = i+1). Register a
	// minimal task-def for each so DescribeTaskDefinition resolves them instead
	// of returning a ClientException — a self-consistent demo graph never points
	// a service at a task definition the fake does not have.
	for i, name := range ecsServiceNamePool {
		arn := fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task-definition/%s:%d", name, i+1)
		defs[arn] = &ecstypes.TaskDefinition{
			TaskDefinitionArn: aws.String(arn),
			Family:            aws.String(name),
			Status:            ecstypes.TaskDefinitionStatusActive,
			NetworkMode:       ecstypes.NetworkModeAwsvpc,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{
					Name:  aws.String(name),
					Image: aws.String("alpine:latest"),
					Cpu:   256,
				},
			},
		}
	}
	applyECSContainerDefaults(defs)
	return defs
}

// applyECSContainerDefaults gives every container a read-only root filesystem
// and an awslogs driver, leaving exactly one witness definition without each:
// web-frontend:6 keeps a writable root, batch-etl-runner:2 keeps a container
// with no log driver. Applied here rather than inline so the healthy default
// can never be forgotten on a definition added later.
func applyECSContainerDefaults(defs map[string]*ecstypes.TaskDefinition) {
	const (
		writableRootDef = ecsDefWebFrontendOld
		noLoggingDef    = ecsDefBatchETLOld
	)
	for arn, def := range defs {
		for i := range def.ContainerDefinitions {
			c := &def.ContainerDefinitions[i]
			if arn != writableRootDef {
				c.ReadonlyRootFilesystem = aws.Bool(true)
			}
			if arn != noLoggingDef {
				c.LogConfiguration = &ecstypes.LogConfiguration{
					LogDriver: ecstypes.LogDriverAwslogs,
					Options: map[string]string{
						"awslogs-group":         "/ecs/" + aws.ToString(def.Family),
						"awslogs-region":        "us-east-1",
						"awslogs-stream-prefix": "ecs",
					},
				}
			}
		}
	}
}

func init() {
	Register(Pin{ShortName: "ecs", Rows: 7, Issues: 4, CoverageGaps: []string{"dim"}})
	// acme-svc-stalled is the witness for a service that wants tasks and runs
	// none; it carries one of the 26 rows and one of the 7 Broken badges.
	Register(Pin{ShortName: "ecs-svc", Rows: 26, Issues: 7, CoverageGaps: []string{"dim"}})
	Register(Pin{ShortName: "ecs-task", Rows: 17, Issues: 9})
}
