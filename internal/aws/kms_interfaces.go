package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// KMSListKeysAPI defines the interface for the KMS ListKeys operation.
type KMSListKeysAPI interface {
	ListKeys(ctx context.Context, params *kms.ListKeysInput, optFns ...func(*kms.Options)) (*kms.ListKeysOutput, error)
}

// KMSDescribeKeyAPI defines the interface for the KMS DescribeKey operation.
type KMSDescribeKeyAPI interface {
	DescribeKey(ctx context.Context, params *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
}

// KMSListAliasesAPI defines the interface for the KMS ListAliases operation.
type KMSListAliasesAPI interface {
	ListAliases(ctx context.Context, params *kms.ListAliasesInput, optFns ...func(*kms.Options)) (*kms.ListAliasesOutput, error)
}

// KMSGetKeyRotationStatusAPI defines the interface for the KMS GetKeyRotationStatus operation.
type KMSGetKeyRotationStatusAPI interface {
	GetKeyRotationStatus(ctx context.Context, params *kms.GetKeyRotationStatusInput, optFns ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error)
}

// KMSListGrantsAPI defines the interface for the KMS ListGrants operation.
type KMSListGrantsAPI interface {
	ListGrants(ctx context.Context, params *kms.ListGrantsInput, optFns ...func(*kms.Options)) (*kms.ListGrantsOutput, error)
}

// KMSGetKeyPolicyAPI defines the interface for the KMS GetKeyPolicy operation.
type KMSGetKeyPolicyAPI interface {
	GetKeyPolicy(ctx context.Context, params *kms.GetKeyPolicyInput, optFns ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error)
}

// KMSAPI is the aggregate interface covering all KMS operations used by a9s fetchers.
// *kms.Client structurally satisfies this interface.
type KMSAPI interface {
	KMSListKeysAPI
	KMSDescribeKeyAPI
	KMSListAliasesAPI
	KMSGetKeyRotationStatusAPI
	KMSListGrantsAPI
	KMSGetKeyPolicyAPI
}
