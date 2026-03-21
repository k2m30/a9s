package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["eks"] = eksClusterFixtures
	demoData["ng"] = nodegroupFixtures
}

// ---------------------------------------------------------------------------
// EKS Clusters
// ---------------------------------------------------------------------------

// eksClusterFixtures returns demo EKS cluster fixtures.
// Note: EKS fetcher sets RawStruct to *ekstypes.Cluster (pointer).
func eksClusterFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-prod",
			Name:   "acme-prod",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":     "acme-prod",
				"version":          "1.29",
				"status":           "ACTIVE",
				"endpoint":         "https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com",
				"platform_version": "eks.8",
			},
			RawStruct: &ekstypes.Cluster{
				Name:            aws.String("acme-prod"),
				Arn:             aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-prod"),
				Version:         aws.String("1.29"),
				Status:          ekstypes.ClusterStatusActive,
				Endpoint:        aws.String("https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com"),
				PlatformVersion: aws.String("eks.8"),
				RoleArn:         aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
				CreatedAt:       aws.Time(mustParseTime("2025-02-15T10:00:00Z")),
				KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
					ServiceIpv4Cidr: aws.String("172.20.0.0/16"),
					IpFamily:        ekstypes.IpFamilyIpv4,
				},
				ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
					VpcId:                  aws.String("vpc-0abc123def456789a"),
					SubnetIds:              []string{"subnet-0aaa111111111111a", "subnet-0bbb222222222222b", "subnet-0ccc333333333333c"},
					SecurityGroupIds:       []string{"sg-0aaa111111111111a"},
					ClusterSecurityGroupId: aws.String("sg-0cluster11111111a"),
					EndpointPublicAccess:   true,
					EndpointPrivateAccess:  true,
				},
				Logging: &ekstypes.Logging{
					ClusterLogging: []ekstypes.LogSetup{
						{
							Enabled: aws.Bool(true),
							Types:   []ekstypes.LogType{ekstypes.LogTypeApi, ekstypes.LogTypeAudit, ekstypes.LogTypeAuthenticator},
						},
					},
				},
				Identity: &ekstypes.Identity{
					Oidc: &ekstypes.OIDC{
						Issuer: aws.String("https://oidc.eks.us-east-1.amazonaws.com/id/ABCDEF1234567890"),
					},
				},
				Tags: map[string]string{
					"Environment": "prod",
					"Team":        "platform",
				},
			},
		},
		{
			ID:     "acme-staging",
			Name:   "acme-staging",
			Status: "ACTIVE",
			Fields: map[string]string{
				"cluster_name":     "acme-staging",
				"version":          "1.29",
				"status":           "ACTIVE",
				"endpoint":         "https://FEDCBA0987654321.gr7.us-east-1.eks.amazonaws.com",
				"platform_version": "eks.8",
			},
			RawStruct: &ekstypes.Cluster{
				Name:            aws.String("acme-staging"),
				Arn:             aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-staging"),
				Version:         aws.String("1.29"),
				Status:          ekstypes.ClusterStatusActive,
				Endpoint:        aws.String("https://FEDCBA0987654321.gr7.us-east-1.eks.amazonaws.com"),
				PlatformVersion: aws.String("eks.8"),
				RoleArn:         aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
				CreatedAt:       aws.Time(mustParseTime("2025-06-10T14:00:00Z")),
				KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
					ServiceIpv4Cidr: aws.String("172.20.0.0/16"),
					IpFamily:        ekstypes.IpFamilyIpv4,
				},
				ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
					VpcId:                  aws.String("vpc-0abc123def456789a"),
					SubnetIds:              []string{"subnet-0bbb222222222222b"},
					SecurityGroupIds:       []string{"sg-0bbb222222222222b"},
					ClusterSecurityGroupId: aws.String("sg-0cluster22222222b"),
					EndpointPublicAccess:   true,
					EndpointPrivateAccess:  false,
				},
				Tags: map[string]string{
					"Environment": "staging",
				},
			},
		},
		{
			ID:     "acme-dev",
			Name:   "acme-dev",
			Status: "CREATING",
			Fields: map[string]string{
				"cluster_name":     "acme-dev",
				"version":          "1.30",
				"status":           "CREATING",
				"endpoint":         "",
				"platform_version": "",
			},
			RawStruct: &ekstypes.Cluster{
				Name:      aws.String("acme-dev"),
				Arn:       aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-dev"),
				Version:   aws.String("1.30"),
				Status:    ekstypes.ClusterStatusCreating,
				RoleArn:   aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
				CreatedAt: aws.Time(mustParseTime("2026-03-21T08:00:00Z")),
				ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
					VpcId:                 aws.String("vpc-0abc123def456789a"),
					SubnetIds:             []string{"subnet-0bbb222222222222b"},
					SecurityGroupIds:      []string{"sg-0bbb222222222222b"},
					EndpointPublicAccess:  true,
					EndpointPrivateAccess: false,
				},
				Tags: map[string]string{
					"Environment": "dev",
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// EKS Node Groups
// ---------------------------------------------------------------------------

// nodegroupFixtures returns demo EKS node group fixtures.
// Node groups reference the "acme-prod" EKS cluster.
// Note: The fetcher sets RawStruct to ekstypes.Nodegroup (value, not pointer).
func nodegroupFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "general-pool",
			Name:   "general-pool",
			Status: "ACTIVE",
			Fields: map[string]string{
				"nodegroup_name": "general-pool",
				"cluster_name":   "acme-prod",
				"status":         "ACTIVE",
				"instance_types": "m5.xlarge, m5.2xlarge",
				"desired_size":   "3",
			},
			RawStruct: ekstypes.Nodegroup{
				NodegroupName: aws.String("general-pool"),
				ClusterName:   aws.String("acme-prod"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/general-pool/12345678-1234-1234-1234-123456789012"),
				Status:        ekstypes.NodegroupStatusActive,
				InstanceTypes: []string{"m5.xlarge", "m5.2xlarge"},
				AmiType:       ekstypes.AMITypesAl2023X8664Standard,
				CapacityType:  ekstypes.CapacityTypesOnDemand,
				DiskSize:      aws.Int32(100),
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(2),
					MaxSize:     aws.Int32(8),
					DesiredSize: aws.Int32(3),
				},
				NodeRole:       aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
				Subnets:        []string{"subnet-0aaa111111111111a", "subnet-0bbb222222222222b"},
				ReleaseVersion: aws.String("1.29.0-20260301"),
				Version:        aws.String("1.29"),
				CreatedAt:      aws.Time(mustParseTime("2025-02-20T12:00:00Z")),
				Labels: map[string]string{
					"workload-type": "general",
					"team":          "platform",
				},
				Tags: map[string]string{
					"Environment": "prod",
					"Team":        "platform",
				},
			},
		},
		{
			ID:     "gpu-pool",
			Name:   "gpu-pool",
			Status: "ACTIVE",
			Fields: map[string]string{
				"nodegroup_name": "gpu-pool",
				"cluster_name":   "acme-prod",
				"status":         "ACTIVE",
				"instance_types": "g4dn.xlarge",
				"desired_size":   "2",
			},
			RawStruct: ekstypes.Nodegroup{
				NodegroupName: aws.String("gpu-pool"),
				ClusterName:   aws.String("acme-prod"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/gpu-pool/22345678-1234-1234-1234-123456789012"),
				Status:        ekstypes.NodegroupStatusActive,
				InstanceTypes: []string{"g4dn.xlarge"},
				AmiType:       ekstypes.AMITypesAl2X8664Gpu,
				CapacityType:  ekstypes.CapacityTypesOnDemand,
				DiskSize:      aws.Int32(200),
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(0),
					MaxSize:     aws.Int32(4),
					DesiredSize: aws.Int32(2),
				},
				NodeRole:       aws.String("arn:aws:iam::123456789012:role/eks-gpu-node-role"),
				Subnets:        []string{"subnet-0aaa111111111111a"},
				ReleaseVersion: aws.String("1.29.0-20260301"),
				Version:        aws.String("1.29"),
				CreatedAt:      aws.Time(mustParseTime("2025-04-05T09:30:00Z")),
				Labels: map[string]string{
					"workload-type":              "gpu",
					"nvidia.com/gpu.accelerator": "tesla-t4",
				},
				Taints: []ekstypes.Taint{
					{
						Key:    aws.String("nvidia.com/gpu"),
						Value:  aws.String("true"),
						Effect: ekstypes.TaintEffectNoSchedule,
					},
				},
				Tags: map[string]string{
					"Environment": "prod",
					"Team":        "ml",
				},
			},
		},
		{
			ID:     "spot-pool",
			Name:   "spot-pool",
			Status: "UPDATING",
			Fields: map[string]string{
				"nodegroup_name": "spot-pool",
				"cluster_name":   "acme-prod",
				"status":         "UPDATING",
				"instance_types": "m5.large, m5a.large, m4.large",
				"desired_size":   "5",
			},
			RawStruct: ekstypes.Nodegroup{
				NodegroupName: aws.String("spot-pool"),
				ClusterName:   aws.String("acme-prod"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/spot-pool/32345678-1234-1234-1234-123456789012"),
				Status:        ekstypes.NodegroupStatusUpdating,
				InstanceTypes: []string{"m5.large", "m5a.large", "m4.large"},
				AmiType:       ekstypes.AMITypesAl2023X8664Standard,
				CapacityType:  ekstypes.CapacityTypesSpot,
				DiskSize:      aws.Int32(50),
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(2),
					MaxSize:     aws.Int32(10),
					DesiredSize: aws.Int32(5),
				},
				NodeRole:       aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
				Subnets:        []string{"subnet-0aaa111111111111a", "subnet-0bbb222222222222b", "subnet-0ccc333333333333c"},
				ReleaseVersion: aws.String("1.29.0-20260215"),
				Version:        aws.String("1.29"),
				CreatedAt:      aws.Time(mustParseTime("2025-09-01T15:00:00Z")),
				Labels: map[string]string{
					"workload-type": "spot-burst",
				},
				Tags: map[string]string{
					"Environment": "prod",
					"CostCenter":  "batch-processing",
				},
			},
		},
	}
}
