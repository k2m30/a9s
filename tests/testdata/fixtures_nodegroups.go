package testdata

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// RealNodeGroups returns sanitized EKS node group data based on real AWS structure.
// Account: 123456789012 (sanitized)
// Region: us-east-1
// Cluster: test-cluster-1
// Total node groups: 3
func RealNodeGroups() []ekstypes.Nodegroup {
	// gpu node group — CREATE_FAILED due to vCPU limit on g4dn.xlarge
	gpuCreatedAt := time.Date(2025, 5, 29, 12, 55, 39, 984000000, time.UTC)
	gpuModifiedAt := time.Date(2025, 5, 29, 13, 28, 52, 149000000, time.UTC)
	gpuDesiredSize := int32(2)
	gpuMinSize := int32(1)
	gpuMaxSize := int32(3)
	gpuMaxUnavailablePct := int32(33)

	// kafka node group — ACTIVE, fixed size 3x t3.large with NO_SCHEDULE taint
	kafkaCreatedAt := time.Date(2025, 6, 6, 10, 33, 52, 285000000, time.UTC)
	kafkaModifiedAt := time.Date(2026, 3, 18, 16, 14, 57, 316000000, time.UTC)
	kafkaDesiredSize := int32(3)
	kafkaMinSize := int32(3)
	kafkaMaxSize := int32(3)
	kafkaMaxUnavailablePct := int32(33)

	// system node group — ACTIVE, 2-3x t3.large with karpenter controller label
	kubeCreatedAt := time.Date(2025, 6, 6, 8, 4, 11, 211000000, time.UTC)
	kubeModifiedAt := time.Date(2026, 3, 18, 16, 16, 14, 801000000, time.UTC)
	kubeDesiredSize := int32(2)
	kubeMinSize := int32(2)
	kubeMaxSize := int32(3)
	kubeMaxUnavailablePct := int32(33)

	return []ekstypes.Nodegroup{
		{
			NodegroupName: aws.String("gpu-20250101120000000000000001"),
			NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/test-cluster-1/gpu-20250101120000000000000001/78cb8e0e-6400-fea1-9939-803bc27e4134"),
			ClusterName:   aws.String("test-cluster-1"),
			Version:       aws.String("1.31"),
			ReleaseVersion: aws.String("1.31.7-20250519"),
			CreatedAt:     &gpuCreatedAt,
			ModifiedAt:    &gpuModifiedAt,
			Status:        ekstypes.NodegroupStatusCreateFailed,
			CapacityType:  ekstypes.CapacityTypesOnDemand,
			ScalingConfig: &ekstypes.NodegroupScalingConfig{
				MinSize:     &gpuMinSize,
				MaxSize:     &gpuMaxSize,
				DesiredSize: &gpuDesiredSize,
			},
			InstanceTypes: []string{"g4dn.xlarge"},
			Subnets: []string{
				"subnet-0aaa111111111111a",
				"subnet-0bbb222222222222b",
				"subnet-0ccc333333333333c",
			},
			AmiType:  ekstypes.AMITypesAl2X8664Gpu,
			NodeRole: aws.String("arn:aws:iam::123456789012:role/gpu-eks-node-group-role"),
			Labels:   map[string]string{"group": "gpu"},
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-gpu-20250101120000000000000001-78cb8e0e-6400-fea1-9939-803bc27e4134")},
				},
			},
			Health: &ekstypes.NodegroupHealth{
				Issues: []ekstypes.Issue{
					{
						Code:    ekstypes.NodegroupIssueCodeAsgInstanceLaunchFailures,
						Message: aws.String("Could not launch On-Demand Instances. VcpuLimitExceeded - You have requested more vCPU capacity than your current vCPU limit of 4 allows for the instance bucket that the specified instance type belongs to. Please visit http://aws.amazon.com/contact-us/ec2-request to request an adjustment to this limit. Launching EC2 instance failed."),
						ResourceIds: []string{
							"eks-gpu-20250101120000000000000001-78cb8e0e-6400-fea1-9939-803bc27e4134",
						},
					},
				},
			},
			UpdateConfig: &ekstypes.NodegroupUpdateConfig{
				MaxUnavailablePercentage: &gpuMaxUnavailablePct,
			},
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Name:    aws.String("gpu-20250101120000000000000001"),
				Version: aws.String("1"),
				Id:      aws.String("lt-0aaa111111111111a"),
			},
			Tags: map[string]string{
				"name": "test-cluster-1",
				"Name": "gpu",
			},
		},
		{
			NodegroupName:  aws.String("kafka-20250101120000000000000002"),
			NodegroupArn:   aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/test-cluster-1/kafka-20250101120000000000000002/22cba266-eb5d-e56c-8752-f68ba653ada0"),
			ClusterName:    aws.String("test-cluster-1"),
			Version:        aws.String("1.31"),
			ReleaseVersion: aws.String("1.31.7-20250519"),
			CreatedAt:      &kafkaCreatedAt,
			ModifiedAt:     &kafkaModifiedAt,
			Status:         ekstypes.NodegroupStatusActive,
			CapacityType:   ekstypes.CapacityTypesOnDemand,
			ScalingConfig: &ekstypes.NodegroupScalingConfig{
				MinSize:     &kafkaMinSize,
				MaxSize:     &kafkaMaxSize,
				DesiredSize: &kafkaDesiredSize,
			},
			InstanceTypes: []string{"t3.large"},
			Subnets: []string{
				"subnet-0aaa111111111111a",
				"subnet-0bbb222222222222b",
				"subnet-0ccc333333333333c",
			},
			AmiType:  ekstypes.AMITypesAl2023X8664Standard,
			NodeRole: aws.String("arn:aws:iam::123456789012:role/kafka-eks-node-group-role"),
			Labels:   map[string]string{"group": "kafka"},
			Taints: []ekstypes.Taint{
				{
					Key:    aws.String("kafka"),
					Value:  aws.String("true"),
					Effect: ekstypes.TaintEffectNoSchedule,
				},
			},
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-kafka-20250101120000000000000002-22cba266-eb5d-e56c-8752-f68ba653ada0")},
				},
			},
			Health: &ekstypes.NodegroupHealth{
				Issues: []ekstypes.Issue{},
			},
			UpdateConfig: &ekstypes.NodegroupUpdateConfig{
				MaxUnavailablePercentage: &kafkaMaxUnavailablePct,
			},
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Name:    aws.String("kafka-20250101120000000000000002"),
				Version: aws.String("2"),
				Id:      aws.String("lt-0bbb222222222222b"),
			},
			Tags: map[string]string{
				"name": "test-cluster-1",
				"Name": "kafka",
			},
		},
		{
			NodegroupName:  aws.String("system-20250101120000000000000003"),
			NodegroupArn:   aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/test-cluster-1/system-20250101120000000000000003/18cba222-6638-8d89-0e8e-3460df9a17cf"),
			ClusterName:    aws.String("test-cluster-1"),
			Version:        aws.String("1.31"),
			ReleaseVersion: aws.String("1.31.7-20250519"),
			CreatedAt:      &kubeCreatedAt,
			ModifiedAt:     &kubeModifiedAt,
			Status:         ekstypes.NodegroupStatusActive,
			CapacityType:   ekstypes.CapacityTypesOnDemand,
			ScalingConfig: &ekstypes.NodegroupScalingConfig{
				MinSize:     &kubeMinSize,
				MaxSize:     &kubeMaxSize,
				DesiredSize: &kubeDesiredSize,
			},
			InstanceTypes: []string{"t3.large"},
			Subnets: []string{
				"subnet-0aaa111111111111a",
				"subnet-0bbb222222222222b",
				"subnet-0ccc333333333333c",
			},
			AmiType:  ekstypes.AMITypesAl2023X8664Standard,
			NodeRole: aws.String("arn:aws:iam::123456789012:role/system-eks-node-group-role"),
			Labels: map[string]string{
				"karpenter.sh/controller": "true",
				"group":                  "system",
			},
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-system-20250101120000000000000003-18cba222-6638-8d89-0e8e-3460df9a17cf")},
				},
			},
			Health: &ekstypes.NodegroupHealth{
				Issues: []ekstypes.Issue{},
			},
			UpdateConfig: &ekstypes.NodegroupUpdateConfig{
				MaxUnavailablePercentage: &kubeMaxUnavailablePct,
			},
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Name:    aws.String("system-20250101120000000000000003"),
				Version: aws.String("3"),
				Id:      aws.String("lt-0ccc333333333333c"),
			},
			Tags: map[string]string{
				"name": "test-cluster-1",
				"Name": "system",
			},
		},
	}
}
