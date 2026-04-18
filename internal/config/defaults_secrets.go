package config

func secretsDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"secrets": {
			List: []ListColumn{
				{Title: "Secret Name", Path: "Name", Width: 36},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Last Accessed", Path: "LastAccessedDate", Width: 18},
				{Title: "Last Changed", Path: "LastChangedDate", Width: 18},
				{Title: "Rotation", Path: "RotationEnabled", Width: 10},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Description"}, {Path: "LastAccessedDate"}, {Path: "LastChangedDate"},
				{Path: "RotationEnabled"}, {Path: "ARN"}, {Path: "KmsKeyId"},
				{Path: "CreatedDate"}, {Path: "LastRotatedDate"}, {Path: "RotationLambdaARN"},
				{Path: "RotationRules"}, {Path: "PrimaryRegion"}, {Path: "Tags"},
			},
		},
		"ssm": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 40},
				{Title: "Type", Path: "Type", Width: 14},
				{Title: "Version", Path: "Version", Width: 8},
				{Title: "Last Modified", Path: "LastModifiedDate", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Type"}, {Path: "Version"}, {Path: "LastModifiedDate"},
				{Path: "LastModifiedUser"}, {Path: "Description"}, {Path: "KeyId"},
				{Path: "Tier"}, {Path: "DataType"}, {Path: "AllowedPattern"},
			},
		},
		"kms": {
			List: []ListColumn{
				{Title: "Alias", Path: "AliasName", Width: 32},
				{Title: "Key ID", Path: "KeyId", Width: 38},
				{Title: "Status", Path: "KeyState", Width: 12},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []DetailField{
				{Path: "KeyId"}, {Path: "Arn"}, {Path: "Description"}, {Path: "KeyState"},
				{Path: "KeyUsage"}, {Path: "KeySpec"}, {Path: "KeyManager"}, {Path: "Enabled"},
				{Path: "CreationDate"}, {Path: "Origin"}, {Path: "MultiRegion"},
				{Path: "EncryptionAlgorithms"}, {Path: "SigningAlgorithms"},
			},
		},
	}
}
