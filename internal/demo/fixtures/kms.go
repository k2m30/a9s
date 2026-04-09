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
	}

	return &KMSFixtures{Keys: keys, Aliases: aliases}
}
