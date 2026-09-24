// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// ECSFake implements aws.ECSAPI against fixture data loaded at construction time.
type ECSFake struct {
	fix *fixtures.ECSFixtures
	// Now is the clock service events are stamped against; nil means
	// time.Now.
	Now func() time.Time
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
	if clusterFilter != "" && !f.hasCluster(clusterFilter) {
		return nil, &ecstypes.ClusterNotFoundException{
			Message: notFoundMessage("Cluster", clusterFilter),
		}
	}
	var arns []string
	for _, svc := range f.fix.Services {
		clusterArn := aws.ToString(svc.ClusterArn)
		if clusterFilter == "" || clusterArn == clusterFilter {
			arns = append(arns, aws.ToString(svc.ServiceArn))
		}
	}
	return &ecs.ListServicesOutput{ServiceArns: arns}, nil
}

// stampEvents dates a service's events against the call rather than against
// the moment the fixtures were built. The enricher reads a ten-minute window,
// so a stored timestamp ages out of a session left open longer than that and
// the finding disappears with the wall clock. Thirty seconds is what a service
// retrying a placement looks like.
//
// The events are copied, never restamped in place: the fixtures are shared by
// every fake in the process.
func (f *ECSFake) stampEvents(svc ecstypes.Service) ecstypes.Service {
	if len(svc.Events) == 0 {
		return svc
	}
	now := time.Now
	if f.Now != nil {
		now = f.Now
	}
	at := now().Add(-30 * time.Second)
	events := make([]ecstypes.ServiceEvent, len(svc.Events))
	copy(events, svc.Events)
	for i := range events {
		events[i].CreatedAt = aws.Time(at)
	}
	svc.Events = events
	return svc
}

func (f *ECSFake) DescribeServices(_ context.Context, input *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	wanted := toSet(input.Services)
	result := make([]ecstypes.Service, 0, len(f.fix.Services))
	for _, svc := range f.fix.Services {
		arn := aws.ToString(svc.ServiceArn)
		name := aws.ToString(svc.ServiceName)
		if len(input.Services) == 0 || wanted[arn] || wanted[name] {
			result = append(result, f.stampEvents(svc))
		}
	}
	return &ecs.DescribeServicesOutput{Services: result}, nil
}

func (f *ECSFake) ListTasks(_ context.Context, input *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	clusterFilter := aws.ToString(input.Cluster)
	serviceFilter := aws.ToString(input.ServiceName)
	var arns []string
	for _, t := range f.fix.Tasks {
		// The real ListTasks accepts either the cluster's short name or its
		// full ARN; a ecs-svc row carries the name, so matching the ARN alone
		// returns nothing for every demo service.
		clusterArn := aws.ToString(t.ClusterArn)
		if clusterFilter != "" && clusterArn != clusterFilter && arnResourceName(clusterArn) != clusterFilter {
			continue
		}
		if serviceFilter != "" && aws.ToString(t.Group) != "service:"+serviceFilter {
			continue
		}
		if input.DesiredStatus != "" && aws.ToString(t.DesiredStatus) != string(input.DesiredStatus) {
			continue
		}
		arns = append(arns, aws.ToString(t.TaskArn))
	}
	return &ecs.ListTasksOutput{TaskArns: arns}, nil
}

// arnResourceName returns what follows the last "/" of an ARN's resource
// part, or ref itself when ref is not an ARN.
func arnResourceName(ref string) string {
	a, err := arn.Parse(ref)
	if err != nil {
		return ref
	}
	return a.Resource[strings.LastIndex(a.Resource, "/")+1:]
}

func (f *ECSFake) DescribeTasks(_ context.Context, input *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if len(input.Tasks) == 0 {
		return &ecs.DescribeTasksOutput{Tasks: f.fix.Tasks}, nil
	}
	wanted := toSet(input.Tasks)
	var result []ecstypes.Task
	for _, t := range f.fix.Tasks {
		taskARN := aws.ToString(t.TaskArn)
		if wanted[taskARN] || wanted[arnResourceName(taskARN)] {
			result = append(result, t)
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: result}, nil
}

func (f *ECSFake) DescribeContainerInstances(_ context.Context, input *ecs.DescribeContainerInstancesInput, _ ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error) {
	out := &ecs.DescribeContainerInstancesOutput{}
	for _, arn := range input.ContainerInstances {
		i := slices.IndexFunc(f.fix.ContainerInstances, func(ci ecstypes.ContainerInstance) bool {
			return aws.ToString(ci.ContainerInstanceArn) == arn
		})
		if i < 0 {
			out.Failures = append(out.Failures, ecstypes.Failure{Arn: aws.String(arn), Reason: aws.String("MISSING")})
			continue
		}
		out.ContainerInstances = append(out.ContainerInstances, f.fix.ContainerInstances[i])
	}
	return out, nil
}

// ListContainerInstances lists the container instances registered with the
// cluster, named by its short name or ARN; a container instance ARN is
// "container-instance/<cluster>/<id>".
func (f *ECSFake) ListContainerInstances(_ context.Context, input *ecs.ListContainerInstancesInput, _ ...func(*ecs.Options)) (*ecs.ListContainerInstancesOutput, error) {
	cluster := aws.ToString(input.Cluster)
	if !f.hasCluster(cluster) {
		return nil, &ecstypes.ClusterNotFoundException{Message: notFoundMessage("Cluster", cluster)}
	}
	cluster = arnResourceName(cluster)
	out := &ecs.ListContainerInstancesOutput{ContainerInstanceArns: []string{}}
	for _, ci := range f.fix.ContainerInstances {
		a, err := arn.Parse(aws.ToString(ci.ContainerInstanceArn))
		if err == nil && strings.Split(a.Resource, "/")[1] == cluster {
			out.ContainerInstanceArns = append(out.ContainerInstanceArns, aws.ToString(ci.ContainerInstanceArn))
		}
	}
	return out, nil
}

// DescribeCapacityProviders answers for the named capacity providers, by
// name or ARN, and a MISSING failure for any other.
func (f *ECSFake) DescribeCapacityProviders(_ context.Context, input *ecs.DescribeCapacityProvidersInput, _ ...func(*ecs.Options)) (*ecs.DescribeCapacityProvidersOutput, error) {
	out := &ecs.DescribeCapacityProvidersOutput{}
	for _, ref := range input.CapacityProviders {
		i := slices.IndexFunc(f.fix.CapacityProviders, func(p ecstypes.CapacityProvider) bool {
			return aws.ToString(p.Name) == ref || aws.ToString(p.CapacityProviderArn) == ref
		})
		if i < 0 {
			out.Failures = append(out.Failures, ecstypes.Failure{Arn: aws.String(ref), Reason: aws.String("MISSING")})
			continue
		}
		out.CapacityProviders = append(out.CapacityProviders, f.fix.CapacityProviders[i])
	}
	return out, nil
}

func (f *ECSFake) DescribeTaskDefinition(_ context.Context, input *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	arn := aws.ToString(input.TaskDefinition)
	if f.fix.DeniedTaskDefinitions[arn] {
		return nil, &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User is not authorized to perform: ecs:DescribeTaskDefinition on " + arn,
		}
	}
	tdef, ok := f.fix.TaskDefinitions[arn]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "ClientException",
			Message: "Unable to describe task definition: " + arn,
		}
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: tdef}, nil
}

// hasCluster reports whether the fixtures register this cluster, under either
// spelling ListServices accepts: the cluster's short name or its ARN.
func (f *ECSFake) hasCluster(nameOrARN string) bool {
	return slices.ContainsFunc(f.fix.Clusters, func(c ecstypes.Cluster) bool {
		return aws.ToString(c.ClusterName) == nameOrARN || aws.ToString(c.ClusterArn) == nameOrARN
	})
}
