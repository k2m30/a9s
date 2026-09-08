// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// KMSFake implements aws.KMSAPI against fixture data loaded at construction time.
type KMSFake struct {
	fix *fixtures.KMSFixtures
}

// NewKMS constructs a KMSFake backed by fixture data from the fixtures package.
func NewKMS() *KMSFake {
	return &KMSFake{fix: fixtures.NewKMSFixtures()}
}

func (f *KMSFake) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	keys := make([]kmstypes.KeyListEntry, 0, len(f.fix.KeyList))
	for _, meta := range f.fix.KeyList {
		keys = append(keys, kmstypes.KeyListEntry{
			KeyId:  meta.KeyId,
			KeyArn: meta.Arn,
		})
	}
	return &kms.ListKeysOutput{Keys: keys}, nil
}

func (f *KMSFake) DescribeKey(_ context.Context, input *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if input.KeyId == nil {
		return nil, fmt.Errorf("DescribeKey: KeyId is required")
	}
	if *input.KeyId == fixtures.KMSAccessDeniedKeyID {
		return nil, &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User is not authorized to perform kms:DescribeKey",
			Fault:   smithy.FaultClient,
		}
	}
	meta, ok := f.fix.Keys[*input.KeyId]
	if !ok {
		return nil, &kmstypes.NotFoundException{Message: notFoundMessage("Key", *input.KeyId)}
	}
	return &kms.DescribeKeyOutput{KeyMetadata: meta}, nil
}

func (f *KMSFake) ListAliases(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{Aliases: f.fix.Aliases}, nil
}

func (f *KMSFake) GetKeyRotationStatus(_ context.Context, input *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	if input.KeyId == nil {
		return nil, fmt.Errorf("GetKeyRotationStatus: KeyId is required")
	}
	if _, ok := f.fix.Keys[*input.KeyId]; !ok {
		return nil, fmt.Errorf("GetKeyRotationStatus: key %q not found", *input.KeyId)
	}
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: f.fix.RotationEnabled[*input.KeyId]}, nil
}

// ListGrants is a no-op stub satisfying KMSListGrantsAPI.
// Demo mode does not model KMS grants.
func (f *KMSFake) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}

// GetKeyPolicy returns the named key's default policy document from fixture
// data. Backs the kms:role related-panel pivot (checkKMSRole).
func (f *KMSFake) GetKeyPolicy(_ context.Context, input *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	if input == nil || input.KeyId == nil {
		return &kms.GetKeyPolicyOutput{}, nil
	}
	policy, ok := f.fix.KeyPolicies[*input.KeyId]
	if !ok {
		return &kms.GetKeyPolicyOutput{}, nil
	}
	return &kms.GetKeyPolicyOutput{Policy: &policy}, nil
}
