// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
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

// ngsForCluster returns all nodegroups in the fixture whose ClusterName
// field matches clusterName, regardless of which map-key they are nested
// under. Fixture authors may group variant-scenario nodegroups under
// descriptive pseudo-keys like "acme-prod-issue-ngs" while keeping the
// ClusterName set to the real cluster; filtering by ClusterName uniformly
// across every bucket surfaces them correctly without trusting map keys.
func (f *EKSFake) ngsForCluster(clusterName string) []ekstypes.Nodegroup {
	var out []ekstypes.Nodegroup
	for _, ngs := range f.fix.Nodegroups {
		for _, ng := range ngs {
			if aws.ToString(ng.ClusterName) == clusterName {
				out = append(out, ng)
			}
		}
	}
	return out
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
	ngs := f.ngsForCluster(clusterName)
	names := make([]string, 0, len(ngs))
	for _, ng := range ngs {
		names = append(names, aws.ToString(ng.NodegroupName))
	}
	return &eks.ListNodegroupsOutput{Nodegroups: names}, nil
}

func (f *EKSFake) DescribeNodegroup(_ context.Context, input *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	clusterName := aws.ToString(input.ClusterName)
	ngName := aws.ToString(input.NodegroupName)
	ngs := f.ngsForCluster(clusterName)
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
