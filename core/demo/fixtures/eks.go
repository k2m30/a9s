// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides EKS fixture data for the EKS fake.
package fixtures

import (
	"sync"
	"time"

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
	// DeniedClusters are listed by ListClusters but DescribeCluster answers
	// AccessDeniedException; UnavailableClusters are listed but not found —
	// the two degraded rows (eks.warn.details_denied / details_unavailable).
	DeniedClusters      []string
	UnavailableClusters []string
	// DeniedNodegroups / UnavailableNodegroups map cluster name → node group
	// names that ListNodegroups returns but DescribeNodegroup denies / cannot
	// find (ng.warn.details_denied / details_unavailable).
	DeniedNodegroups      map[string][]string
	UnavailableNodegroups map[string][]string
}

// Degraded-row witnesses. The node groups hang off the graph-root cluster.
const (
	WarnEKSDetailsDeniedID      = "warn-eks-details-denied"
	WarnEKSDetailsUnavailableID = "warn-eks-details-unavailable"
	WarnNGDetailsDeniedID       = "warn-ng-details-denied"
	WarnNGDetailsUnavailableID  = "warn-ng-details-unavailable"
)

// NewEKSFixtures builds and returns a fully-populated EKSFixtures struct.
var sharedEKSFixtures = sync.OnceValue(func() *EKSFixtures {
	clusters := buildEKSClusters()
	ngs := buildEKSNodegroups()
	return &EKSFixtures{
		Clusters:              clusters,
		Nodegroups:            ngs,
		DeniedClusters:        []string{WarnEKSDetailsDeniedID},
		UnavailableClusters:   []string{WarnEKSDetailsUnavailableID},
		DeniedNodegroups:      map[string][]string{"acme-prod": {WarnNGDetailsDeniedID}},
		UnavailableNodegroups: map[string][]string{"acme-prod": {WarnNGDetailsUnavailableID}},
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
				EndpointPublicAccess:  false,
				EndpointPrivateAccess: true,
			},
			KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
				ServiceIpv4Cidr: aws.String("172.20.0.0/16"),
				IpFamily:        ekstypes.IpFamilyIpv4,
			},
			Logging:          eksFullControlPlaneLogging(),
			EncryptionConfig: eksSecretsEncryption(),
			CreatedAt:        aws.Time(mustTime("2025-03-01T10:00:00Z")),
			PlatformVersion:  aws.String("eks.5"),
			// aws:cloudformation:stack-name tag — required for eks→cfn
			// related-panel pivot. acme-eks-cluster is a real stack fixture (cfn.go).
			Tags: map[string]string{
				"Environment":                   "prod",
				"Team":                          "platform",
				"aws:cloudformation:stack-name": "acme-eks-cluster",
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
				EndpointPublicAccess:  false,
				EndpointPrivateAccess: true,
			},
			Logging:          eksFullControlPlaneLogging(),
			EncryptionConfig: eksSecretsEncryption(),
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
			// EKSPublicEndpoint witness: the Kubernetes endpoint answers from
			// anywhere on the internet.
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:                aws.String(eksVPCID),
				SubnetIds:            []string{eksSubnetA},
				EndpointPublicAccess: true,
				PublicAccessCidrs:    []string{"0.0.0.0/0"},
			},
			Logging:          eksFullControlPlaneLogging(),
			EncryptionConfig: eksSecretsEncryption(),
			CreatedAt:        aws.Time(mustTime("2026-03-21T09:00:00Z")),
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
			// EKSLoggingIncomplete witness: audit and scheduler output never
			// reaches CloudWatch.
			Logging: &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{
				{Enabled: aws.Bool(true), Types: []ekstypes.LogType{
					ekstypes.LogTypeApi, ekstypes.LogTypeAuthenticator, ekstypes.LogTypeControllerManager,
				}},
				{Enabled: aws.Bool(false), Types: []ekstypes.LogType{
					ekstypes.LogTypeAudit, ekstypes.LogTypeScheduler,
				}},
			}},
			EncryptionConfig: eksSecretsEncryption(),
			CreatedAt:        aws.Time(mustTime("2026-04-01T11:00:00Z")),
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
			// EKSVersionUnsupported witness: 1.28 is the one demo minor the
			// registry reports as out of standard support.
			Logging:          eksFullControlPlaneLogging(),
			EncryptionConfig: eksSecretsEncryption(),
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
		// Status=UPDATING → wave1 finding (CodeEKSStateUpdating, SevWarn) → Warning.
		{
			Name:    aws.String("acme-prod-updating"),
			Arn:     aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-prod-updating"),
			Version: aws.String("1.30"),
			Status:  ekstypes.ClusterStatusUpdating,
			RoleArn: aws.String(eksClusterRoleARN),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:     aws.String(eksVPCID),
				SubnetIds: []string{eksSubnetA, eksSubnetB},
			},
			// EKSSecretsNoKMS witness: EncryptionConfig is deliberately absent.
			Logging:   eksFullControlPlaneLogging(),
			CreatedAt: aws.Time(mustTime("2025-08-01T10:00:00Z")),
			Tags: map[string]string{
				"Environment": "prod",
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
				// RemoteAccessSecurityGroup — required for ng→sg related-panel pivot.
				Resources: &ekstypes.NodegroupResources{
					AutoScalingGroups: []ekstypes.AutoScalingGroup{
						{Name: aws.String("eks-acme-prod-ng-general")},
					},
					RemoteAccessSecurityGroup: aws.String("sg-0eks111111111111e"),
				},
				// LaunchTemplate.Id — required for the lt->ng related-panel
				// pivot. References eks-node-lt (lt.go); the id is the same
				// symbol the EC2 fake's DescribeLaunchTemplateVersions
				// resolves to acme-eks-worker's pinned AMI.
				LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
					Id:      aws.String(EKSNodeLTID),
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
			// Status=CREATING → wave1 finding (CodeNGStateCreating, SevWarn) → Warning.
			{
				NodegroupName: aws.String("acme-prod-creating-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/acme-prod-creating-pool/ccc33333"),
				ClusterName:   aws.String("acme-prod"),
				Status:        ekstypes.NodegroupStatusCreating,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(50),
				InstanceTypes: []string{"m5.xlarge"},
				Subnets:       []string{eksSubnetA, eksSubnetB},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(2),
					MaxSize:     aws.Int32(6),
					DesiredSize: aws.Int32(2),
				},
				CreatedAt:      aws.Time(mustTime("2026-04-25T09:00:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
			// Status=DELETING → wave1 finding (CodeNGStateDeleting, SevWarn) → Warning.
			{
				NodegroupName: aws.String("acme-prod-deleting-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/acme-prod-deleting-pool/ddd44444"),
				ClusterName:   aws.String("acme-prod"),
				Status:        ekstypes.NodegroupStatusDeleting,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(50),
				InstanceTypes: []string{"m5.xlarge"},
				Subnets:       []string{eksSubnetA, eksSubnetB},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(0),
					MaxSize:     aws.Int32(4),
					DesiredSize: aws.Int32(0),
				},
				CreatedAt:      aws.Time(mustTime("2025-05-01T09:00:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
			// Status=DELETE_FAILED → wave1 finding (CodeNGStateDeleteFailed, SevBroken) → Broken.
			{
				NodegroupName: aws.String("acme-prod-delete-failed-pool"),
				NodegroupArn:  aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/acme-prod-delete-failed-pool/eee55555"),
				ClusterName:   aws.String("acme-prod"),
				Status:        ekstypes.NodegroupStatusDeleteFailed,
				NodeRole:      aws.String(eksNodeRoleARN),
				AmiType:       ekstypes.AMITypesAl2X8664,
				DiskSize:      aws.Int32(50),
				InstanceTypes: []string{"m5.xlarge"},
				Subnets:       []string{eksSubnetA, eksSubnetB},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize:     aws.Int32(1),
					MaxSize:     aws.Int32(4),
					DesiredSize: aws.Int32(1),
				},
				CreatedAt:      aws.Time(mustTime("2025-06-10T09:00:00Z")),
				ReleaseVersion: aws.String("1.29.3-20240322"),
				Version:        aws.String("1.29"),
				Tags: map[string]string{
					"Environment": "prod",
				},
			},
		},
	}
}

// Witness clusters for the eks posture findings. Each names the ONE demo
// cluster that carries its finding; every other cluster is set to the
// healthy value for that condition.
// The scoped-CIDR variant of the public endpoint has no witness of its own:
// the demo bench requires exactly one row per finding code, and both the open
// and the scoped case carry eks.public-endpoint. The severity split is pinned
// by the unit tests instead.
const (
	// EKSPublicEndpoint — the Kubernetes endpoint is open to 0.0.0.0/0.
	EKSPublicEndpoint = "acme-dev"
	// EKSLoggingIncomplete — not all control-plane log types are enabled.
	EKSLoggingIncomplete = "acme-staging-failed"
	// EKSSecretsNoKMS — no encryption configuration covers secrets.
	EKSSecretsNoKMS = "acme-prod-updating"
	// EKSVersionUnsupported — Kubernetes minor past standard support. This is
	// the 1.28 cluster: it is the only demo minor that is not also worn by
	// another cluster, and the ruling is that no cluster's Version changes to
	// make room for this witness.
	EKSVersionUnsupported = "acme-degraded-prod"
)

// eksFullControlPlaneLogging is the healthy control-plane logging setup: all
// five types AWS emits, enabled. Every cluster but EKSLoggingIncomplete uses
// it so exactly one demo row trips the incomplete-logging finding.
func eksFullControlPlaneLogging() *ekstypes.Logging {
	return &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{{
		Enabled: aws.Bool(true),
		Types: []ekstypes.LogType{
			ekstypes.LogTypeApi, ekstypes.LogTypeAudit, ekstypes.LogTypeAuthenticator,
			ekstypes.LogTypeControllerManager, ekstypes.LogTypeScheduler,
		},
	}}}
}

// eksSecretsEncryption is the healthy secrets-encryption setup. Every cluster
// but EKSSecretsNoKMS uses it.
func eksSecretsEncryption() []ekstypes.EncryptionConfig {
	return []ekstypes.EncryptionConfig{{
		Resources: []string{"secrets"},
		Provider:  &ekstypes.Provider{KeyArn: aws.String(eksKMSKeyARN)},
	}}
}

// EKSVersionSupport is what the demo registry reports for each Kubernetes
// minor the fixtures use, backing DescribeClusterVersions. Only 1.28 is out of
// standard support, which is what makes acme-degraded-prod the sole witness
// without any cluster's Version being changed to suit the test.
var EKSVersionSupport = map[string]ekstypes.VersionStatus{ //nolint:gochecknoglobals // static demo data
	"1.28": ekstypes.VersionStatusExtendedSupport,
	"1.29": ekstypes.VersionStatusStandardSupport,
	"1.30": ekstypes.VersionStatusStandardSupport,
}

// EKSEndOfStandardSupport is the date the demo registry reports for the one
// minor that has left standard support.
var EKSEndOfStandardSupport = time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // static demo data
