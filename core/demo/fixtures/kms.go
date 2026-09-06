// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// KMSFixtures holds typed fixture data for KMS.
type KMSFixtures struct {
	// KeyList is the account's keys in declaration order, one entry per key —
	// what ListKeys returns. Keys is a lookup index that deliberately holds
	// each key twice, so listing from it would emit every key twice and in
	// map order.
	KeyList []*kmstypes.KeyMetadata
	// Keys maps both the bare key ID and the full key ARN to KeyMetadata
	// (used by DescribeKey, which accepts either form).
	Keys map[string]*kmstypes.KeyMetadata
	// Aliases is the full list of key aliases (returned by ListAliases).
	Aliases []kmstypes.AliasListEntry
	// KeyPolicies maps key ID to its default key-policy JSON document — backs
	// kms:GetKeyPolicy for the kms:role related-panel pivot (checkKMSRole).
	KeyPolicies map[string]string
	// RotationEnabled maps a key's bare KeyId to the KeyRotationEnabled value
	// GetKeyRotationStatus reports for it. Keys with no entry report false
	// (the fake's zero-value default), matching real CMKs that ship with
	// rotation off. Backs EnrichKMSRotation's kms.rotation-disabled check.
	RotationEnabled map[string]bool
}

// KMSAccessDeniedKeyID names the ListKeys entry whose DescribeKey call the
// KMSFake denies with AccessDeniedException — witnesses
// FetchKMSKeysPage's kms.access-denied finding (real key state unreadable,
// key still shown).
const KMSAccessDeniedKeyID = "b8c9d0e1-f2a3-5678-90bc-eeffaabbccdd"

// KMSPublicPolicy is the witness key for kms.public-policy: its default key
// policy grants kms:Decrypt to a wildcard principal. Every other demo key
// either has no policy fixture or names concrete principals.
const KMSPublicPolicy = "c9d0e1f2-a3b4-6789-01cd-ffaabbccddee"

// NewKMSFixtures constructs KMSFixtures from the canonical demo data.
var sharedKMSFixtures = sync.OnceValue(func() *KMSFixtures {
	keyMetadata := []*kmstypes.KeyMetadata{
		// Rotation enabled (see RotationEnabled below) → the only demo CMK for
		// which EnrichKMSRotation raises no kms.rotation-disabled finding,
		// letting colorKMS fall through to its Enabled->Healthy branch.
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
		// Access-denied key — DescribeKey is denied for this ID (see
		// KMSFake.DescribeKey), witnessing kms.access-denied. Real key state
		// is unreadable; the row is synthesized from ListKeys alone.
		{
			KeyId:      aws.String(KMSAccessDeniedKeyID),
			Arn:        aws.String("arn:aws:kms:us-east-1:123456789012:key/" + KMSAccessDeniedKeyID),
			KeyManager: kmstypes.KeyManagerTypeCustomer,
		},
		// Backup prod-vault encryption key (checkBackupKMS pivot).
		// DescribeBackupVault("acme-prod-vault").EncryptionKeyArn points here.
		// ID must match BackupProdVaultKMSKeyID in backup.go.
		{
			KeyId:                aws.String(BackupProdVaultKMSKeyID),
			Arn:                  aws.String("arn:aws:kms:us-east-1:123456789012:key/" + BackupProdVaultKMSKeyID),
			Description:          aws.String("Encryption key for acme-prod-vault (AWS Backup)"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 4, 10, 9, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
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
		// orders-prod-cmk-0001 — DDB→kms pivot: matched by checkDdbKMS stripping the ARN
		// suffix from SSEDescription.KMSMasterKeyArn on the orders-prod table.
		{
			KeyId:                aws.String(OrdersProdKMSKeyID),
			Arn:                  aws.String(OrdersProdKMSKeyARN),
			Description:          aws.String("Customer-managed CMK for orders-prod DynamoDB table encryption"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// OpenSearch graph-root encryption key — required for opensearch→kms related-panel pivot.
		// The acme-logs domain sets EncryptionAtRestOptions.KmsKeyId = OpenSearchKMSKeyARN;
		// checkOpenSearchKMS strips the ARN to bare key ID = OpenSearchKMSKeyID and looks it up here.
		{
			KeyId:                aws.String(OpenSearchKMSKeyID),
			Arn:                  aws.String(OpenSearchKMSKeyARN),
			Description:          aws.String("Encryption key for acme-logs OpenSearch domain (at-rest encryption)"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// Redshift acme-warehouse encryption key — required for redshift→kms related-panel pivot.
		// The acme-warehouse cluster sets KmsKeyId = RedshiftKMSKeyARN1; the
		// checker strips the ARN to the bare key ID (RedshiftKMSKeyID1) and looks it up here.
		{
			KeyId:                aws.String(RedshiftKMSKeyID1),
			Arn:                  aws.String(RedshiftKMSKeyARN1),
			Description:          aws.String("Encryption key for acme-warehouse Redshift analytics cluster"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// Redshift acme-reporting encryption key — required for redshift→kms related-panel pivot (second graph-root).
		// The acme-reporting cluster sets KmsKeyId = RedshiftKMSKeyARN2; the
		// checker strips the ARN to the bare key ID (RedshiftKMSKeyID2) and looks it up here.
		{
			KeyId:                aws.String(RedshiftKMSKeyID2),
			Arn:                  aws.String(RedshiftKMSKeyARN2),
			Description:          aws.String("Encryption key for acme-reporting Redshift reporting cluster"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 7, 1, 10, 0, 0, 0, time.UTC)),
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
		// Witness: key policy open to any AWS principal.
		{
			KeyId:                aws.String(KMSPublicPolicy),
			Arn:                  aws.String("arn:aws:kms:us-east-1:123456789012:key/" + KMSPublicPolicy),
			Description:          aws.String("Shared analytics export key — key policy allows any AWS principal to decrypt"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 3, 18, 9, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// EFS prod-app-data encryption key — required for efs→kms related-panel pivot.
		// The prod-efs-app-data filesystem sets KmsKeyId = ProdEFSKmsKeyARN; the
		// checker strips the ARN to the bare key ID and looks it up here.
		{
			KeyId:                aws.String(ProdEFSKmsKeyID),
			Arn:                  aws.String(ProdEFSKmsKeyARN),
			Description:          aws.String("Encryption key for prod-efs-app-data EFS filesystem"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 2, 10, 9, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// AWS-managed default S3 key (checkS3KMS pivot, ManagedKeyBucketName
		// in s3.go). GetBucketEncryption reports the full alias ARN
		// "arn:aws:kms:...:alias/aws/s3" — the exact shape that caused the
		// pre-fix truncation bug (the naive last-"/" split used to chop it
		// down to "s3", the bucket's own resource type, instead of passing
		// the alias through whole). checkS3KMS returns the alias-style ID
		// "alias/aws/s3" (AWSManagedS3KeyID) as the navigation ID; real
		// DescribeKey accepts that as KeyId directly and the KeyMetadata it
		// returns always carries the true KeyId, so this fixture entry uses
		// the alias string as its KeyId to keep the fake's DescribeKey
		// response self-consistent with what was looked up (AWS-managed
		// keys report a stable, well-known KeyId across all callers; there is
		// no separate real UUID to reconcile with in this fixture set).
		{
			KeyId:                aws.String(AWSManagedS3KeyID),
			Arn:                  aws.String("arn:aws:kms:us-east-1:123456789012:key/" + AWSManagedS3KeyID),
			Description:          aws.String("Default master key that protects my S3 objects when no other key is defined"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeAws,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// AMI EBS boot-volume encryption key — required for ami→kms related-panel
		// pivot. checkAMIKMS returns the AMI's BlockDeviceMappings[].Ebs.KmsKeyId
		// verbatim (full ARN, not stripped to a bare ID), so this entry's KeyId
		// is the ARN string itself (AMIEBSKmsKeyARN in ec2.go) rather than a bare
		// UUID — see the AMIEBSKmsKeyID doc comment in ec2.go for why this key
		// cannot share the widely-reused "primary" KMS key.
		{
			KeyId:                aws.String(AMIEBSKmsKeyARN),
			Arn:                  aws.String(AMIEBSKmsKeyARN),
			Description:          aws.String("Boot volume encryption key for acme-app-server AMI"),
			KeyState:             kmstypes.KeyStateEnabled,
			KeyManager:           kmstypes.KeyManagerTypeCustomer,
			KeyUsage:             kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate:         aws.Time(time.Date(2025, 2, 15, 10, 0, 0, 0, time.UTC)),
			Enabled:              true,
			EncryptionAlgorithms: []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
			MultiRegion:          aws.Bool(false),
			Origin:               kmstypes.OriginTypeAwsKms,
		},
		// legacy-prod-cmk-deleted — required for ddb→kms related-panel pivot on
		// legacy-kms-lost (INACCESSIBLE_ENCRYPTION_CREDENTIALS). Modeled as
		// PendingDeletion rather than fully absent: a real deleted CMK stops
		// resolving via DescribeKey entirely once deletion completes (which
		// would make the related panel unable to offer a drill target at all,
		// same as any other 404), whereas a CMK scheduled for deletion still
		// resolves but is unusable for encryption — the same real AWS failure
		// mode DynamoDB reports as INACCESSIBLE_ENCRYPTION_CREDENTIALS, and it
		// keeps the pivot drillable while preserving the "lost key" scenario.
		{
			KeyId:        aws.String("legacy-prod-cmk-deleted"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/legacy-prod-cmk-deleted"),
			Description:  aws.String("Former production CMK for legacy-kms-lost — scheduled for deletion, table lost access"),
			KeyState:     kmstypes.KeyStatePendingDeletion,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2022, 6, 1, 8, 0, 0, 0, time.UTC)),
			Enabled:      false,
		},
		// legacy-archived-cmk-lost — required for ddb→kms related-panel pivot on
		// legacy-archived (ARCHIVED via INACCESSIBLE_ENCRYPTION_CREDENTIALS).
		// Same PendingDeletion modeling rationale as legacy-prod-cmk-deleted above.
		{
			KeyId:        aws.String("legacy-archived-cmk-lost"),
			Arn:          aws.String("arn:aws:kms:us-east-1:123456789012:key/legacy-archived-cmk-lost"),
			Description:  aws.String("Former CMK for legacy-archived — scheduled for deletion, triggered table archival"),
			KeyState:     kmstypes.KeyStatePendingDeletion,
			KeyManager:   kmstypes.KeyManagerTypeCustomer,
			KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
			CreationDate: aws.Time(time.Date(2022, 5, 1, 8, 0, 0, 0, time.UTC)),
			Enabled:      false,
		},
	}

	// Indexed by both bare KeyId and full key ARN, mirroring real DescribeKey,
	// which accepts either form (plus alias name/ARN, added below) as KeyId.
	keys := make(map[string]*kmstypes.KeyMetadata, len(keyMetadata)*2)
	for _, k := range keyMetadata {
		keys[*k.KeyId] = k
		if k.Arn != nil {
			keys[*k.Arn] = k
		}
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
			AliasName:   aws.String("alias/shared-analytics-export"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/shared-analytics-export"),
			TargetKeyId: aws.String(KMSPublicPolicy),
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
		// AWS-managed default S3 key alias (ManagedKeyBucketName in s3.go).
		{
			AliasName:   aws.String(AWSManagedS3KeyID),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:" + AWSManagedS3KeyID),
			TargetKeyId: aws.String(AWSManagedS3KeyID),
		},
		// Redis prod KMS key alias.
		{
			AliasName:   aws.String("alias/acme-redis-prod-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-redis-prod-key"),
			TargetKeyId: aws.String(ProdRedisKMSKeyID),
		},
		// orders-prod DynamoDB CMK alias.
		{
			AliasName:   aws.String("alias/orders-prod-cmk"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/orders-prod-cmk"),
			TargetKeyId: aws.String(OrdersProdKMSKeyID),
		},
		// Redshift acme-warehouse KMS key alias.
		{
			AliasName:   aws.String("alias/acme-redshift-warehouse-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-redshift-warehouse-key"),
			TargetKeyId: aws.String(RedshiftKMSKeyID1),
		},
		// Redshift acme-reporting KMS key alias.
		{
			AliasName:   aws.String("alias/acme-redshift-reporting-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-redshift-reporting-key"),
			TargetKeyId: aws.String(RedshiftKMSKeyID2),
		},
		// Backup prod-vault KMS key alias.
		{
			AliasName:   aws.String("alias/acme-backup-prod-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-backup-prod-key"),
			TargetKeyId: aws.String(BackupProdVaultKMSKeyID),
		},
		// EFS prod-app-data KMS key alias.
		{
			AliasName:   aws.String("alias/acme-efs-prod-app-data-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-efs-prod-app-data-key"),
			TargetKeyId: aws.String(ProdEFSKmsKeyID),
		},
		// OpenSearch acme-logs at-rest encryption key alias.
		{
			AliasName:   aws.String("alias/acme-opensearch-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-opensearch-key"),
			TargetKeyId: aws.String(OpenSearchKMSKeyID),
		},
		// AMI EBS boot-volume encryption key alias.
		{
			AliasName:   aws.String("alias/acme-ami-ebs-boot-key"),
			AliasArn:    aws.String("arn:aws:kms:us-east-1:123456789012:alias/acme-ami-ebs-boot-key"),
			TargetKeyId: aws.String(AMIEBSKmsKeyARN),
		},
	}

	// KeyPolicies — required for the kms:role related-panel pivot
	// (checkKMSRole → kms:GetKeyPolicy). Grants the primary production key's
	// default policy to acme-ec2-instance-profile (iam.go), matching a
	// realistic default key policy shape.
	keyPolicies := map[string]string{
		"a1b2c3d4-5678-90ab-cdef-111111111111": `{"Version":"2012-10-17","Statement":[{"Sid":"EnableRootAccess","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"},{"Sid":"AllowKeyUseByEC2InstanceRole","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-ec2-instance-profile"},"Action":["kms:Decrypt","kms:GenerateDataKey"],"Resource":"*"}]}`,
		KMSPublicPolicy:                        `{"Version":"2012-10-17","Statement":[{"Sid":"EnableRootAccess","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"},{"Sid":"AllowAnyoneToDecrypt","Effect":"Allow","Principal":"*","Action":["kms:Decrypt","kms:DescribeKey"],"Resource":"*"}]}`,
	}

	// RotationEnabled — the primary production key is the sole demo CMK with
	// rotation on; every other key defaults to false via the map zero value.
	rotationEnabled := map[string]bool{
		"a1b2c3d4-5678-90ab-cdef-111111111111": true,
	}

	return &KMSFixtures{KeyList: keyMetadata, Keys: keys, Aliases: aliases, KeyPolicies: keyPolicies, RotationEnabled: rotationEnabled}
})

func NewKMSFixtures() *KMSFixtures {
	return sharedKMSFixtures()
}

func init() {
	// dim: colorKMS (core/aws/catalog_secrets.go) has exactly three branches —
	// Enabled→Healthy, Disabled→Warning, Pending*/Unavailable→Broken — and
	// docs/resources/kms.md §3.1/§3.2 documents no Dim-producing signal.
	Register(Pin{ShortName: "kms", Rows: 22, Issues: 8, CoverageGaps: []string{"dim"}})
}
