package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// KMSFixtures holds typed fixture data for KMS.
type KMSFixtures struct {
	// Keys maps key ID to KeyMetadata (used by DescribeKey).
	Keys map[string]*kmstypes.KeyMetadata
	// Aliases is the full list of key aliases (returned by ListAliases).
	Aliases []kmstypes.AliasListEntry
}

// NewKMSFixtures constructs KMSFixtures from the canonical demo data.
func NewKMSFixtures() *KMSFixtures {
	keyMetadata := []*kmstypes.KeyMetadata{
		{
			KeyId:                aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
			Arn:                  aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			Description:          aws.String("Primary encryption key for production workloads"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			SigningAlgorithms:    []kmstypes.SigningAlgorithmSpec{kmstypes.SigningAlgorithmSpecEcdsaSha256},
			KeySpec:              kmstypes.KeySpecSymmetricDefault,
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		{
			KeyId:        aws.String("b2c3d4e5-6789-01ab-cdef-222222222222"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/b2c3d4e5-6789-01ab-cdef-222222222222"),
			Description:  aws.String("Secrets Manager encryption key"),
			KeyState:     kmstypes.KeyStateEnabled,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2025, 3, 22, 14, 0, 0, 0, time.UTC)),
			Enabled:      true,
		},
		{
			KeyId:        aws.String("c3d4e5f6-7890-12ab-cdef-333333333333"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/c3d4e5f6-7890-12ab-cdef-333333333333"),
			Description:  aws.String("Legacy S3 bucket encryption key (deprecated)"),
			KeyState:     kmstypes.KeyStateDisabled,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2024, 8, 1, 9, 0, 0, 0, time.UTC)),
			Enabled:      false,
		},
		// CT-event cross-reference key required by ctdetail nav tests (T029).
		{
			KeyId:        aws.String("2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b"),
			Description:  aws.String("Auto-rotation key for production secrets (ct-events case D cross-ref)"),
			KeyState:     kmstypes.KeyStateEnabled,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)),
			Enabled:      true,
		},
		// Disabled → Warning
		{
			KeyId:        aws.String("d4e5f6a7-bcde-1234-5678-aabbccddeeff"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/d4e5f6a7-bcde-1234-5678-aabbccddeeff"),
			Description:  aws.String("Disabled encryption key — suspended pending audit"),
			KeyState:     kmstypes.KeyStateDisabled,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC)),
			Enabled:      false,
		},
		// PendingDeletion → Broken
		{
			KeyId:        aws.String("e5f6a7b8-cdef-2345-6789-bbccddeeffe0"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/e5f6a7b8-cdef-2345-6789-bbccddeeffe0"),
			Description:  aws.String("Key scheduled for deletion in 14 days"),
			KeyState:     kmstypes.KeyStatePendingDeletion,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2024, 6, 20, 10, 0, 0, 0, time.UTC)),
			Enabled:      false,
		},
		// Customer-managed key with no rotation (rotation status is a separate Wave-2 call)
		{
			KeyId:        aws.String("f6a7b8c9-def0-3456-789a-ccddeeff0011"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/f6a7b8c9-def0-3456-789a-ccddeeff0011"),
			Description:  aws.String("Customer-managed CMK with rotation disabled"),
			KeyState:     kmstypes.KeyStateEnabled,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2023, 11, 1, 12, 0, 0, 0, time.UTC)),
			Enabled:      true,
		},
		// Unavailable → Broken
		{
			KeyId:        aws.String("a7b8c9d0-ef01-4567-89ab-ddeeff001122"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/a7b8c9d0-ef01-4567-89ab-ddeeff001122"),
			Description:  aws.String("Key unavailable — custom key store connection lost"),
			KeyState:     kmstypes.KeyStateUnavailable,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2024, 9, 5, 6, 0, 0, 0, time.UTC)),
			Enabled:      false,
		},
		// S3 healthy-bucket SSE-KMS key (checkS3KMS pivot).
		// The checker strips everything up to the last "/" from the KMS key ARN,
		// leaving the bare key ID. This must match S3BucketKMSKeyID in s3.go.
		{
			KeyId:        aws.String(S3BucketKMSKeyID),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/" + S3BucketKMSKeyID),
			Description:  aws.String("Server-side encryption key for a9s-demo-healthy S3 bucket"),
			KeyState:     kmstypes.KeyStateEnabled,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)),
			Enabled:      true,
		},
		// Redis prod encryption key — required for redis→kms related-panel pivot.
		// The prod-redis-sessions RG sets KmsKeyId = ProdRedisKMSKeyARN; the
		// checker strips the ARN to the bare key ID and looks it up here.
		{
			KeyId:                aws.String(ProdRedisKMSKeyID),
			Arn:                  aws.String(ProdRedisKMSKeyARN),
			Description:          aws.String("Encryption key for production ElastiCache Redis sessions cluster"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 3, 15, 8, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// PendingDeletion key — referenced by broken-dbi-encryption-locked fixture (KmsKeyId).
		// This key was deleted while still in use by an RDS instance, causing
		// the instance to enter the inaccessible-encryption-credentials state.
		{
			KeyId:                aws.String("deadbeef-0000-0000-0000-000000000000"),
			Arn:                  aws.String("arn:aws:kms:us-east-1:123456789012:key/deadbeef-0000-0000-0000-000000000000"),
			Description:          aws.String("Deleted RDS encryption key — caused DB instance access failure"),
			KeyState:             kmstypes.KeyStatePendingDeletion,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2024, 1, 10, 8, 0, 0, 0, time.UTC)),
			Enabled:              false,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
	}

	keys := make(map[string]*kmstypes.KeyMetadata, len(keyMetadata))
	for _, k := range keyMetadata {
		keys[*k.KeyId] = k
	}

	aliases := []kmstypes.AliasListEntry{
		{
			AliasName:   aws.String("alias/acme-prod-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-prod-key"),
			TargetKeyId: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
		{
			AliasName:   aws.String("alias/acme-secrets-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-secrets-key"),
			TargetKeyId: aws.String("b2c3d4e5-6789-01ab-cdef-222222222222"),
		},
		{
			AliasName:   aws.String("alias/acme-s3-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-s3-key"),
			TargetKeyId: aws.String("c3d4e5f6-7890-12ab-cdef-333333333333"),
		},
		{
			AliasName:   aws.String("alias/acme-rotation-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-rotation-key"),
			TargetKeyId: aws.String("2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b"),
		},
		// Issue-state aliases
		{
			AliasName:   aws.String("alias/aws/disabled-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/aws/disabled-key"),
			TargetKeyId: aws.String("d4e5f6a7-bcde-1234-5678-aabbccddeeff"),
		},
		{
			AliasName:   aws.String("alias/pending-deletion-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/pending-deletion-key"),
			TargetKeyId: aws.String("e5f6a7b8-cdef-2345-6789-bbccddeeffe0"),
		},
		{
			AliasName:   aws.String("alias/no-rotation-cmk"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/no-rotation-cmk"),
			TargetKeyId: aws.String("f6a7b8c9-def0-3456-789a-ccddeeff0011"),
		},
		{
			AliasName:   aws.String("alias/unavailable-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/unavailable-key"),
			TargetKeyId: aws.String("a7b8c9d0-ef01-4567-89ab-ddeeff001122"),
		},
		{
			AliasName:   aws.String("alias/deleted-rds-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/deleted-rds-key"),
			TargetKeyId: aws.String("deadbeef-0000-0000-0000-000000000000"),
		},
		// S3 healthy-bucket KMS key alias.
		{
			AliasName:   aws.String("alias/" + S3BucketKMSKeyID),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/" + S3BucketKMSKeyID),
			TargetKeyId: aws.String(S3BucketKMSKeyID),
		},
		// Redis prod KMS key alias.
		{
			AliasName:   aws.String("alias/acme-redis-prod-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-redis-prod-key"),
			TargetKeyId: aws.String(ProdRedisKMSKeyID),
		},
	}

	return &KMSFixtures{Keys: keys, Aliases: aliases}
}
