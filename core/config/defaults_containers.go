// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func containersDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"eks": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "Name", Width: 28},
				{Title: "Version", Path: "Version", Width: 10},
				{Title: "Status", Key: "status", Path: "Status", Width: 14},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
				{Title: "Platform Version", Path: "PlatformVersion", Width: 18},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Version"}, {Path: "Status"}, {Path: "Endpoint"},
				{Path: "PlatformVersion"}, {Path: "Arn"}, {Path: "RoleArn"}, {Path: "KubernetesNetworkConfig"},
				{Path: "ResourcesVpcConfig"}, {Path: "Logging"}, {Path: "Identity"}, {Path: "CreatedAt"}, {Path: "Tags"},
				{Key: "health_issues", Label: "Health Issues"},
			},
		},
		"ng": {
			List: []ListColumn{
				{Title: "Node Group", Path: "NodegroupName", Width: 28},
				// Keyed as well as Path-based, unlike the columns beside
				// it: on a row with no RawStruct — a warm cache replay, or a
				// node group whose describe was denied — the path cannot
				// answer and Fields carries the real cluster.
				{Title: "Cluster", Key: "cluster_name", Path: "ClusterName", Width: 24},
				{Title: "Status", Key: "status", Path: "Status", Width: 14},
				{Title: "Instance Types", Path: "InstanceTypes", Width: 20},
				{Title: "Desired", Path: "ScalingConfig.DesiredSize", Width: 9},
			},
			Detail: []DetailField{
				{Path: "NodegroupName"}, {Path: "ClusterName"}, {Path: "Status"}, {Path: "InstanceTypes"},
				{Path: "AmiType"}, {Path: "CapacityType"}, {Path: "DiskSize"}, {Path: "ScalingConfig"},
				{Path: "NodeRole"}, {Path: "NodegroupArn"}, {Path: "ReleaseVersion"}, {Path: "Version"},
				{Path: "Subnets"}, {Path: "LaunchTemplate"}, {Path: "Labels"}, {Path: "Taints"},
				{Path: "Tags"}, {Path: "Health"}, {Path: "CreatedAt"},
				{Key: "health_issues", Label: "Health Issues"},
			},
		},
	}
}
