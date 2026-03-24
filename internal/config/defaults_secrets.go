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
			Detail: []string{
				"Name", "Description", "LastAccessedDate", "LastChangedDate",
				"RotationEnabled", "ARN", "KmsKeyId",
				"CreatedDate", "LastRotatedDate", "RotationLambdaARN",
				"RotationRules", "PrimaryRegion", "Tags",
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
			Detail: []string{
				"Name", "Type", "Version", "LastModifiedDate",
				"LastModifiedUser", "Description", "KeyId",
				"Tier", "DataType", "AllowedPattern",
			},
		},
		"kms": {
			List: []ListColumn{
				{Title: "Alias", Path: "AliasName", Width: 32},
				{Title: "Key ID", Path: "KeyId", Width: 38},
				{Title: "Status", Path: "KeyState", Width: 12},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []string{
				"KeyId", "Arn", "Description", "KeyState",
				"KeyUsage", "KeySpec", "KeyManager", "Enabled",
				"CreationDate", "Origin", "MultiRegion",
				"EncryptionAlgorithms", "SigningAlgorithms",
			},
		},
	}
}
