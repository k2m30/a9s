// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func secretsDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"secrets": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Description"}, {Path: "LastAccessedDate"}, {Path: "LastChangedDate"},
				{Path: "RotationEnabled"}, {Path: "ARN"}, {Path: "KmsKeyId"},
				{Path: "CreatedDate"}, {Path: "LastRotatedDate"}, {Path: "RotationLambdaARN"},
				{Path: "RotationRules"}, {Path: "PrimaryRegion"}, {Path: "Tags"},
			},
		},
		"ssm": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Type"}, {Path: "Version"}, {Path: "LastModifiedDate"},
				{Path: "LastModifiedUser"}, {Path: "Description"}, {Path: "KeyId"},
				{Path: "Tier"}, {Path: "DataType"}, {Path: "AllowedPattern"},
			},
		},
		"kms": {
			Detail: []DetailField{
				{Path: "KeyId"}, {Path: "Arn"}, {Path: "Description"}, {Path: "KeyState"},
				{Path: "KeyUsage"}, {Path: "KeySpec"}, {Path: "KeyManager"}, {Path: "Enabled"},
				{Path: "CreationDate"}, {Path: "Origin"}, {Path: "MultiRegion"},
				{Path: "EncryptionAlgorithms"}, {Path: "SigningAlgorithms"},
			},
		},
	}
}
