package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["secrets"] = secretsManagerSecrets
	demoData["ssm"] = ssmParameters
	demoData["kms"] = kmsKeys
}

// secretsManagerSecrets returns demo Secrets Manager fixtures.
func secretsManagerSecrets() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "prod/database/primary",
			Name:   "prod/database/primary",
			Status: "",
			Fields: map[string]string{				"secret_name":      "prod/database/primary",
				"description":      "Aurora PostgreSQL primary connection string",
				"last_accessed":    "2026-03-21",
				"last_changed":     "2026-02-15",
				"rotation_enabled": "Yes",
			},
			RawStruct: smtypes.SecretListEntry{
				Name:             aws.String("prod/database/primary"),
				ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"),
				Description:      aws.String("Aurora PostgreSQL primary connection string"),
				LastAccessedDate: aws.Time(time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)),
				LastChangedDate:  aws.Time(time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)),
				RotationEnabled:  aws.Bool(true),
				CreatedDate:      aws.Time(time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "prod/api/stripe-key",
			Name:   "prod/api/stripe-key",
			Status: "",
			Fields: map[string]string{				"secret_name":      "prod/api/stripe-key",
				"description":      "Stripe API secret key for payment processing",
				"last_accessed":    "2026-03-20",
				"last_changed":     "2026-01-05",
				"rotation_enabled": "No",
			},
			RawStruct: smtypes.SecretListEntry{
				Name:             aws.String("prod/api/stripe-key"),
				ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key-GhIjKl"),
				Description:      aws.String("Stripe API secret key for payment processing"),
				LastAccessedDate: aws.Time(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)),
				LastChangedDate:  aws.Time(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)),
				RotationEnabled:  aws.Bool(false),
				CreatedDate:      aws.Time(time.Date(2025, 4, 22, 14, 30, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "prod/redis/auth-token",
			Name:   "prod/redis/auth-token",
			Status: "",
			Fields: map[string]string{				"secret_name":      "prod/redis/auth-token",
				"description":      "ElastiCache Redis AUTH token",
				"last_accessed":    "2026-03-19",
				"last_changed":     "2026-03-01",
				"rotation_enabled": "Yes",
			},
			RawStruct: smtypes.SecretListEntry{
				Name:             aws.String("prod/redis/auth-token"),
				ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/redis/auth-token-MnOpQr"),
				Description:      aws.String("ElastiCache Redis AUTH token"),
				LastAccessedDate: aws.Time(time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)),
				LastChangedDate:  aws.Time(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
				RotationEnabled:  aws.Bool(true),
				CreatedDate:      aws.Time(time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "staging/database/mysql",
			Name:   "staging/database/mysql",
			Status: "",
			Fields: map[string]string{
				"secret_name":      "staging/database/mysql",
				"description":      "Staging MySQL connection credentials",
				"last_accessed":    "2026-03-18",
				"last_changed":     "2025-12-10",
				"rotation_enabled": "No",
			},
			RawStruct: smtypes.SecretListEntry{
				Name:             aws.String("staging/database/mysql"),
				ARN:              aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:staging/database/mysql-StUvWx"),
				Description:      aws.String("Staging MySQL connection credentials"),
				LastAccessedDate: aws.Time(time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)),
				LastChangedDate:  aws.Time(time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)),
				RotationEnabled:  aws.Bool(false),
				CreatedDate:      aws.Time(time.Date(2025, 3, 15, 9, 0, 0, 0, time.UTC)),
			},
		},
	}
}

// ssmParameters returns demo SSM Parameter Store fixtures.
func ssmParameters() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/acme/prod/app/config",
			Name:   "/acme/prod/app/config",
			Status: "",
			Fields: map[string]string{
				"name":          "/acme/prod/app/config",
				"type":          "String",
				"version":       "12",
				"last_modified": "2026-03-15T14:30:00Z",
				"description":   "Production application configuration",
			},
			RawStruct: ssmtypes.ParameterMetadata{
				Name:             aws.String("/acme/prod/app/config"),
				ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/app/config"),
				Type:             ssmtypes.ParameterTypeString,
				Version:          12,
				LastModifiedDate: aws.Time(mustParseTime("2026-03-15T14:30:00+00:00")),
				Description:      aws.String("Production application configuration"),
				DataType:         aws.String("text"),
			},
		},
		{
			ID:     "/acme/prod/db/connection-string",
			Name:   "/acme/prod/db/connection-string",
			Status: "",
			Fields: map[string]string{
				"name":          "/acme/prod/db/connection-string",
				"type":          "SecureString",
				"version":       "5",
				"last_modified": "2026-02-20T09:15:00Z",
				"description":   "Encrypted database connection string",
			},
			RawStruct: ssmtypes.ParameterMetadata{
				Name:             aws.String("/acme/prod/db/connection-string"),
				ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/db/connection-string"),
				Type:             ssmtypes.ParameterTypeSecureString,
				Version:          5,
				LastModifiedDate: aws.Time(mustParseTime("2026-02-20T09:15:00+00:00")),
				Description:      aws.String("Encrypted database connection string"),
				KeyId:            aws.String("alias/acme-prod-key"),
				DataType:         aws.String("text"),
			},
		},
		{
			ID:     "/acme/prod/feature-flags",
			Name:   "/acme/prod/feature-flags",
			Status: "",
			Fields: map[string]string{
				"name":          "/acme/prod/feature-flags",
				"type":          "StringList",
				"version":       "28",
				"last_modified": "2026-03-20T11:45:00Z",
				"description":   "Feature flag list for production",
			},
			RawStruct: ssmtypes.ParameterMetadata{
				Name:             aws.String("/acme/prod/feature-flags"),
				ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/prod/feature-flags"),
				Type:             ssmtypes.ParameterTypeStringList,
				Version:          28,
				LastModifiedDate: aws.Time(mustParseTime("2026-03-20T11:45:00+00:00")),
				Description:      aws.String("Feature flag list for production"),
				DataType:         aws.String("text"),
			},
		},
		{
			ID:     "/acme/staging/ami-id",
			Name:   "/acme/staging/ami-id",
			Status: "",
			Fields: map[string]string{
				"name":          "/acme/staging/ami-id",
				"type":          "String",
				"version":       "3",
				"last_modified": "2026-01-10T16:00:00Z",
				"description":   "Latest approved AMI for staging",
			},
			RawStruct: ssmtypes.ParameterMetadata{
				Name:             aws.String("/acme/staging/ami-id"),
				ARN:              aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/acme/staging/ami-id"),
				Type:             ssmtypes.ParameterTypeString,
				Version:          3,
				LastModifiedDate: aws.Time(mustParseTime("2026-01-10T16:00:00+00:00")),
				Description:      aws.String("Latest approved AMI for staging"),
				DataType:         aws.String("aws:ec2:image"),
			},
		},
	}
}

// kmsKeys returns demo KMS key fixtures.
// KMS RawStruct is a POINTER (*kmstypes.KeyMetadata), matching the production
// fetcher behavior in internal/aws/kms.go.
func kmsKeys() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "a1b2c3d4-5678-90ab-cdef-111111111111",
			Name:   "alias/acme-prod-key",
			Status: "Enabled",
			Fields: map[string]string{
				"key_id":      "a1b2c3d4-5678-90ab-cdef-111111111111",
				"alias":       "alias/acme-prod-key",
				"status":      "Enabled",
				"description": "Primary encryption key for production workloads",
			},
			RawStruct: &kmstypes.KeyMetadata{
				KeyId:        aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Description:  aws.String("Primary encryption key for production workloads"),
				KeyState:     kmstypes.KeyStateEnabled,
				KeyManager:   kmstypes.KeyManagerTypeCustomer,
				KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
				CreationDate: aws.Time(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)),
				Enabled:      true,
			},
		},
		{
			ID:     "b2c3d4e5-6789-01ab-cdef-222222222222",
			Name:   "alias/acme-secrets-key",
			Status: "Enabled",
			Fields: map[string]string{
				"key_id":      "b2c3d4e5-6789-01ab-cdef-222222222222",
				"alias":       "alias/acme-secrets-key",
				"status":      "Enabled",
				"description": "Secrets Manager encryption key",
			},
			RawStruct: &kmstypes.KeyMetadata{
				KeyId:        aws.String("b2c3d4e5-6789-01ab-cdef-222222222222"),
				Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/b2c3d4e5-6789-01ab-cdef-222222222222"),
				Description:  aws.String("Secrets Manager encryption key"),
				KeyState:     kmstypes.KeyStateEnabled,
				KeyManager:   kmstypes.KeyManagerTypeCustomer,
				KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
				CreationDate: aws.Time(time.Date(2025, 3, 22, 14, 0, 0, 0, time.UTC)),
				Enabled:      true,
			},
		},
		{
			ID:     "c3d4e5f6-7890-12ab-cdef-333333333333",
			Name:   "alias/acme-s3-key",
			Status: "Disabled",
			Fields: map[string]string{
				"key_id":      "c3d4e5f6-7890-12ab-cdef-333333333333",
				"alias":       "alias/acme-s3-key",
				"status":      "Disabled",
				"description": "Legacy S3 bucket encryption key (deprecated)",
			},
			RawStruct: &kmstypes.KeyMetadata{
				KeyId:        aws.String("c3d4e5f6-7890-12ab-cdef-333333333333"),
				Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/c3d4e5f6-7890-12ab-cdef-333333333333"),
				Description:  aws.String("Legacy S3 bucket encryption key (deprecated)"),
				KeyState:     kmstypes.KeyStateDisabled,
				KeyManager:   kmstypes.KeyManagerTypeCustomer,
				KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
				CreationDate: aws.Time(time.Date(2024, 8, 1, 9, 0, 0, 0, time.UTC)),
				Enabled:      false,
			},
		},
	}
}
