// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// EKSFake implements aws.EKSAPI against fixture data loaded at construction time.
type EKSFake struct {
	fix *fixtures.EKSFixtures
}

// NewEKS constructs an EKSFake backed by fixture data from the fixtures package.
func NewEKS() *EKSFake {
	return &EKSFake{fix: fixtures.NewEKSFixtures()}
}

func (f *EKSFake) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	names := make([]string, 0, len(f.fix.Clusters))
	for _, c := range f.fix.Clusters {
		names = append(names, aws.ToString(c.Name))
	}
	return &eks.ListClustersOutput{Clusters: names}, nil
}

func (f *EKSFake) DescribeCluster(_ context.Context, input *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	name := aws.ToString(input.Name)
	for _, c := range f.fix.Clusters {
		if aws.ToString(c.Name) == name {
			return &eks.DescribeClusterOutput{Cluster: c}, nil
		}
	}
	return nil, &smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "No cluster found for name: " + name,
	}
}

func (f *EKSFake) ListNodegroups(_ context.Context, input *eks.ListNodegroupsInput, _ ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	clusterName := aws.ToString(input.ClusterName)
	found := false
	for _, c := range f.fix.Clusters {
		if aws.ToString(c.Name) == clusterName {
			found = true
			break
		}
	}
	if !found {
		return nil, &smithy.GenericAPIError{
			Code:    "ResourceNotFoundException",
			Message: "No cluster found for name: " + clusterName,
		}
	}
	ngs := f.fix.Nodegroups[clusterName]
	names := make([]string, 0, len(ngs))
	for _, ng := range ngs {
		names = append(names, aws.ToString(ng.NodegroupName))
	}
	return &eks.ListNodegroupsOutput{Nodegroups: names}, nil
}

func (f *EKSFake) DescribeNodegroup(_ context.Context, input *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	clusterName := aws.ToString(input.ClusterName)
	ngName := aws.ToString(input.NodegroupName)
	ngs, ok := f.fix.Nodegroups[clusterName]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "ResourceNotFoundException",
			Message: "No nodegroup found for name: " + ngName,
		}
	}
	for i := range ngs {
		if aws.ToString(ngs[i].NodegroupName) == ngName {
			ng := ngs[i]
			return &eks.DescribeNodegroupOutput{Nodegroup: &ng}, nil
		}
	}
	return nil, &smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "No nodegroup found for name: " + ngName,
	}
}
