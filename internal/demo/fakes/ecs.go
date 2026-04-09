// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ECSFake implements aws.ECSAPI against fixture data loaded at construction time.
type ECSFake struct {
	fix *fixtures.ECSFixtures
}

// NewECS constructs an ECSFake backed by fixture data from the fixtures package.
func NewECS() *ECSFake {
	return &ECSFake{fix: fixtures.NewECSFixtures()}
}

func (f *ECSFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	arns := make([]string, 0, len(f.fix.Clusters))
	for _, c := range f.fix.Clusters {
		arns = append(arns, aws.ToString(c.ClusterArn))
	}
	return &ecs.ListClustersOutput{ClusterArns: arns}, nil
}

func (f *ECSFake) DescribeClusters(_ context.Context, input *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	if len(input.Clusters) == 0 {
		return &ecs.DescribeClustersOutput{Clusters: f.fix.Clusters}, nil
	}
	wanted := toSet(input.Clusters)
	var result []ecstypes.Cluster
	for _, c := range f.fix.Clusters {
		arn := aws.ToString(c.ClusterArn)
		name := aws.ToString(c.ClusterName)
		if wanted[arn] || wanted[name] {
			result = append(result, c)
		}
	}
	return &ecs.DescribeClustersOutput{Clusters: result}, nil
}

func (f *ECSFake) ListServices(_ context.Context, input *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	clusterFilter := aws.ToString(input.Cluster)
	var arns []string
	for _, svc := range f.fix.Services {
		clusterArn := aws.ToString(svc.ClusterArn)
		if clusterFilter == "" || clusterArn == clusterFilter {
			arns = append(arns, aws.ToString(svc.ServiceArn))
		}
	}
	return &ecs.ListServicesOutput{ServiceArns: arns}, nil
}

func (f *ECSFake) DescribeServices(_ context.Context, input *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if len(input.Services) == 0 {
		return &ecs.DescribeServicesOutput{Services: f.fix.Services}, nil
	}
	wanted := toSet(input.Services)
	var result []ecstypes.Service
	for _, svc := range f.fix.Services {
		arn := aws.ToString(svc.ServiceArn)
		name := aws.ToString(svc.ServiceName)
		if wanted[arn] || wanted[name] {
			result = append(result, svc)
		}
	}
	return &ecs.DescribeServicesOutput{Services: result}, nil
}

func (f *ECSFake) ListTasks(_ context.Context, input *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	clusterFilter := aws.ToString(input.Cluster)
	var arns []string
	for _, t := range f.fix.Tasks {
		clusterArn := aws.ToString(t.ClusterArn)
		if clusterFilter == "" || clusterArn == clusterFilter {
			arns = append(arns, aws.ToString(t.TaskArn))
		}
	}
	return &ecs.ListTasksOutput{TaskArns: arns}, nil
}

func (f *ECSFake) DescribeTasks(_ context.Context, input *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if len(input.Tasks) == 0 {
		return &ecs.DescribeTasksOutput{Tasks: f.fix.Tasks}, nil
	}
	wanted := toSet(input.Tasks)
	var result []ecstypes.Task
	for _, t := range f.fix.Tasks {
		if wanted[aws.ToString(t.TaskArn)] {
			result = append(result, t)
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: result}, nil
}

func (f *ECSFake) DescribeTaskDefinition(_ context.Context, input *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	arn := aws.ToString(input.TaskDefinition)
	tdef, ok := f.fix.TaskDefinitions[arn]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "ClientException",
			Message: "Unable to describe task definition: " + arn,
		}
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: tdef}, nil
}
