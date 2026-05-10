// Package fixtures provides EKS fixture data for the EKS fake.
package fixtures

import (
	"sync"
	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// EKSFixtures holds all EKS domain objects served by the fake.
type EKSFixtures struct {
	// Clusters is the full list returned by ListClusters / DescribeCluster.
	// RawStructs are *ekstypes.Cluster (pointers, matching production fetcher).
	Clusters []*ekstypes.Cluster
	// Nodegroups maps cluster name → []Nodegroup.
	// RawStructs are ekstypes.Nodegroup (values, matching production fetcher).
	Nodegroups map[string][]ekstypes.Nodegroup
}

// NewEKSFixtures builds and returns a fully-populated EKSFixtures struct.
var sharedEKSFixtures = sync.OnceValue(func() *EKSFixtures {
	clusters := buildEKSClusters()
	ngs := buildEKSNodegroups()
	return &EKSFixtures{
		Clusters:   clusters,
		Nodegroups: ngs,
	}
})

func NewEKSFixtures() *EKSFixtures {
	return sharedEKSFixtures()
}

const (
	eksVPCID          = "vpc-0abc123def456789a"
	eksSubnetA        = "subnet-0aaa111111111111a"
	eksSubnetB        = "subnet-0bbb222222222222b"
	eksSubnetC        = "subnet-0ccc333333333333c"
	eksNodeRoleARN    = "arn:aws:iam::123456789012:role/acme-eks-node-role"
	eksClusterRoleARN = "arn:aws:iam::123456789012:role/acme-eks-cluster-role"
	eksKMSKeyARN      = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
)

func buildEKSClusters() []*ekstypes.Cluster {
	return []*ekstypes.Cluster{
		{
			Name:     aws.String("acme-prod"),
			Arn:      aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-prod"),
			Version:  aws.String("1.29"),
			Status:   ekstypes.ClusterStatusActive,
			Endpoint: aws.String("https://ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com"),
			RoleArn:  aws.String(eksClusterRoleARN),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:                 aws.String(eksVPCID),
				SubnetIds:             []string{eksSubnetA, eksSubnetB, eksSubnetC},
				SecurityGroupIds:      []string{"sg-0eks111111111111e"},
				EndpointPublicAccess:  true,
				EndpointPrivateAccess: true,
				PublicAccessCidrs:     []string{"0.0.0.0/0"},
			},
			KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
				ServiceIpv4Cidr: aws.String("172.20.0.0/16"),
				IpFamily:        ekstypes.IpFamilyIpv4,
			},
			Logging: &ekstypes.Logging{
				ClusterLogging: []ekstypes.LogSetup{
					{
						Types:   []ekstypes.LogType{ekstypes.LogTypeApi, ekstypes.LogTypeAudit},
						Enabled: aws.Bool(true),
					},
				},
			},
			EncryptionConfig: []ekstypes.EncryptionConfig{
				{
					Resources: []string{"secrets"},
					Provider:  &ekstypes.Provider{KeyArn: aws.String(eksKMSKeyARN)},
				},
			},
			CreatedAt:       aws.Time(mustTime("2025-03-01T10:00:00Z")),
			PlatformVersion: aws.String("eks.5"),
			Tags: map[string]string{
				"Environment": "prod",
				"Team":        "platform",
			},
		},
		{
			Name:     aws.String("acme-staging"),
			Arn:      aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-staging"),
			Version:  aws.String("1.29"),
			Status:   ekstypes.ClusterStatusActive,
			Endpoint: aws.String("https://STAGING0123456789.gr7.us-east-1.eks.amazonaws.com"),
			RoleArn:  aws.String(eksClusterRoleARN),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:                 aws.String(eksVPCID),
				SubnetIds:             []string{eksSubnetA, eksSubnetB},
				EndpointPublicAccess:  true,
				EndpointPrivateAccess: false,
				PublicAccessCidrs:     []string{"0.0.0.0/0"},
			},
			KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
				ServiceIpv4Cidr: aws.String("172.20.0.0/16"),
				IpFamily:        ekstypes.IpFamilyIpv4,
			},
			CreatedAt:       aws.Time(mustTime("2025-06-15T14:00:00Z")),
			PlatformVersion: aws.String("eks.5"),
			Tags: map[string]string{
				"Environment": "staging",
			},
		},
		{
			Name:    aws.String("acme-dev"),
			Arn:     aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-dev"),
			Version: aws.String("1.30"),
			Status:  ekstypes.ClusterStatusCreating,
			RoleArn: aws.String(eksClusterRoleARN),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:     aws.String(eksVPCID),
				SubnetIds: []string{eksSubnetA},
			},
			CreatedAt: aws.Time(mustTime("2026-03-21T09:00:00Z")),
			Tags: map[string]string{
				"Environment": "dev",
			},
		},
		// Status=FAILED → Color:Broken
		{
			Name:    aws.String("acme-staging-failed"),
			Arn:     aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-staging-failed"),
			Version: aws.String("1.29"),
			Status:  ekstypes.ClusterStatusFailed,
			RoleArn: aws.String(eksClusterRoleARN),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:     aws.String(eksVPCID),
				SubnetIds: []string{eksSubnetA, eksSubnetB},
			},
			CreatedAt: aws.Time(mustTime("2026-04-01T11:00:00Z")),
			Tags: map[string]string{
				"Environment": "staging",
			},
		},
		// Status=ACTIVE with Health.Issues → health_issues_count field triggers Broken
		{
			Name:    aws.String("acme-degraded-prod"),
			Arn:     aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-degraded-prod"),
			Version: aws.String("1.28"),
			Status:  ekstypes.ClusterStatusActive,
			RoleArn: aws.String(eksClusterRoleARN),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:     aws.String(eksVPCID),
				SubnetIds: []string{eksSubnetA, eksSubnetB, eksSubnetC},
			},
			Health: &ekstypes.ClusterHealth{
				Issues: []ekstypes.ClusterIssue{
					{
						Code:    ekstypes.ClusterIssueCodeConfigurationConflict,
						Message: aws.String("Subnet subnet-0aaa111111111111a not found"),
					},
					{
						Code:    ekstypes.ClusterIssueCodeAccessDenied,
						Message: aws.String("Cluster role missing eks:DescribeCluster"),
					},
				},
			},
			CreatedAt: aws.Time(mustTime("2025-01-10T08:00:00Z")),
			Tags: map[string]string{
				"Environment": "prod",
				"Team":        "platform",
			},
		},
	}
}

func buildEKSNodegroups() map[string][]ekstypes.Nodegroup {
	return map[string][]ekstypes.Nodegroup{
		"acme-prod": {
			{
				NodegroupName: aws.String("general-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/general-pool/abc12345"),
				ClusterName:   aws.String("acme-prod"),
				Status:        ekstypes.NodegroupStatusActive,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(50),
				InstanceTypes: []string{"m5.xlarge"},
				Subnets:       []string{eksSubnetA, eksSubnetB, eksSubnetC},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(2),
					MaxSize:     aws.Int32(8),
					DesiredSize: aws.Int32(3),
				},
				Resources: &ekstypes.NodegroupResources{
					AutoScalingGroups: []ekstypes.AutoScalingGroup{
						{Name: aws.String("eks-acme-prod-ng-general")},
					},
				},
				LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
					Id:      aws.String("lt-0eks111111111111a"),
					Version: aws.String("1"),
				},
				CreatedAt:      aws.Time(mustTime("2025-03-05T12:00:00Z")),
				ModifiedAt:     aws.Time(mustTime("2026-02-10T08:30:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment":                       "prod",
					"k8s.io/cluster-autoscaler/enabled": "true",
				},
			},
		},
		"acme-staging": {
			{
				NodegroupName: aws.String("staging-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-staging/staging-pool/def67890"),
				ClusterName:   aws.String("acme-staging"),
				Status:        ekstypes.NodegroupStatusActive,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(30),
				InstanceTypes: []string{"t3.medium"},
				Subnets:       []string{eksSubnetA, eksSubnetB},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(1),
					MaxSize:     aws.Int32(4),
					DesiredSize: aws.Int32(2),
				},
				CreatedAt:      aws.Time(mustTime("2025-06-20T10:00:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment": "staging",
				},
			},
			// Status=UPDATING → Warning state (no health issues)
			{
				NodegroupName: aws.String("acme-staging-updating"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-staging/acme-staging-updating/fed09876"),
				ClusterName:   aws.String("acme-staging"),
				Status:        ekstypes.NodegroupStatusUpdating,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(30),
				InstanceTypes: []string{"t3.medium"},
				Subnets:       []string{eksSubnetA, eksSubnetB},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(1),
					MaxSize:     aws.Int32(4),
					DesiredSize: aws.Int32(2),
				},
				CreatedAt:      aws.Time(mustTime("2025-06-20T10:00:00Z")),
				ReleaseVersion: aws.String("1.30.0-20250101"),
				Version:        aws.String("1.30"),
				Tags: map[string]string{
					"Environment": "staging",
				},
			},
		},
		// Degraded + Failed node groups under acme-prod
		"acme-prod-issue-ngs": {
			// Status=DEGRADED with InsufficientFreeAddresses issue → Broken
			{
				NodegroupName: aws.String("acme-prod-degraded-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/acme-prod-degraded-pool/aaa11111"),
				ClusterName:   aws.String("acme-prod"),
				Status:        ekstypes.NodegroupStatusDegraded,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(50),
				InstanceTypes: []string{"m5.xlarge"},
				Subnets:       []string{eksSubnetA, eksSubnetB},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(2),
					MaxSize:     aws.Int32(6),
					DesiredSize: aws.Int32(4),
				},
				Health: &ekstypes.NodegroupHealth{
					Issues: []ekstypes.Issue{
						{
							Code:    ekstypes.NodegroupIssueCodeInsufficientFreeAddresses,
							Message: aws.String("Subnet has run out of addresses"),
						},
					},
				},
				CreatedAt:      aws.Time(mustTime("2025-03-05T12:00:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
			// Status=CREATE_FAILED with two issues → Broken
			{
				NodegroupName: aws.String("acme-prod-failed-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/acme-prod-failed-pool/bbb22222"),
				ClusterName:   aws.String("acme-prod"),
				Status:        ekstypes.NodegroupStatusCreateFailed,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(50),
				InstanceTypes: []string{"m5.2xlarge"},
				Subnets:       []string{eksSubnetA, eksSubnetB, eksSubnetC},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(3),
					MaxSize:     aws.Int32(10),
					DesiredSize: aws.Int32(5),
				},
				Health: &ekstypes.NodegroupHealth{
					Issues: []ekstypes.Issue{
						{
							Code:    ekstypes.NodegroupIssueCodeEc2LaunchTemplateVersionMismatch,
							Message: aws.String("Launch template version mismatch detected"),
						},
						{
							Code:    ekstypes.NodegroupIssueCodeAutoScalingGroupInvalidConfiguration,
							Message: aws.String("Auto Scaling group configuration is invalid"),
						},
					},
				},
				CreatedAt:      aws.Time(mustTime("2026-04-10T08:00:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
		},
	}
}
