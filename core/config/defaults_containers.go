// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func containersDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"eks": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Version"}, {Path: "Status"}, {Path: "Endpoint"},
				{Path: "PlatformVersion"}, {Path: "Arn"}, {Path: "RoleArn"}, {Path: "KubernetesNetworkConfig"},
				{Path: "ResourcesVpcConfig"}, {Path: "Logging"}, {Path: "Identity"}, {Path: "CreatedAt"}, {Path: "Tags"},
				{Key: "health_issues", Label: "Health Issues"},
			},
		},
		"ng": {
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
