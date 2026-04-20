package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ECSListClustersAPI defines the interface for the ECS ListClusters operation.
type ECSListClustersAPI interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
}

// ECSDescribeClustersAPI defines the interface for the ECS DescribeClusters operation.
type ECSDescribeClustersAPI interface {
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
}

// ECSListServicesAPI defines the interface for the ECS ListServices operation.
type ECSListServicesAPI interface {
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
}

// ECSDescribeServicesAPI defines the interface for the ECS DescribeServices operation.
type ECSDescribeServicesAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

// ECSListTasksAPI defines the interface for the ECS ListTasks operation.
type ECSListTasksAPI interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

// ECSDescribeTasksAPI defines the interface for the ECS DescribeTasks operation.
type ECSDescribeTasksAPI interface {
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

// ECSDescribeTaskDefinitionAPI defines the interface for the ECS DescribeTaskDefinition operation.
type ECSDescribeTaskDefinitionAPI interface {
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
}

// ECSAPI is the aggregate interface covering all ECS operations used by a9s fetchers.
// *ecs.Client structurally satisfies this interface.
type ECSAPI interface {
	ECSListClustersAPI
	ECSDescribeClustersAPI
	ECSListServicesAPI
	ECSDescribeServicesAPI
	ECSListTasksAPI
	ECSDescribeTasksAPI
	ECSDescribeTaskDefinitionAPI
}
