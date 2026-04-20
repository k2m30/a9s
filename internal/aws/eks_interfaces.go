package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
)

// EKSListClustersAPI defines the interface for the EKS ListClusters operation.
type EKSListClustersAPI interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
}

// EKSDescribeClusterAPI defines the interface for the EKS DescribeCluster operation.
type EKSDescribeClusterAPI interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

// EKSListNodegroupsAPI defines the interface for the EKS ListNodegroups operation.
type EKSListNodegroupsAPI interface {
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
}

// EKSDescribeNodegroupAPI defines the interface for the EKS DescribeNodegroup operation.
type EKSDescribeNodegroupAPI interface {
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
}

// EKSAPI is the aggregate interface covering all EKS operations used by a9s fetchers.
// *eks.Client structurally satisfies this interface.
type EKSAPI interface {
	EKSListClustersAPI
	EKSDescribeClusterAPI
	EKSListNodegroupsAPI
	EKSDescribeNodegroupAPI
}
