package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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
	keys := make([]kmstypes.KeyListEntry, 0, len(f.fix.Keys))
	for id, meta := range f.fix.Keys {
		keys = append(keys, kmstypes.KeyListEntry{
			KeyId:  &id,
			KeyArn: meta.Arn,
		})
	}
	return &kms.ListKeysOutput{Keys: keys}, nil
}

func (f *KMSFake) DescribeKey(_ context.Context, input *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if input.KeyId == nil {
		return nil, fmt.Errorf("DescribeKey: KeyId is required")
	}
	meta, ok := f.fix.Keys[*input.KeyId]
	if !ok {
		return nil, fmt.Errorf("DescribeKey: key %q not found", *input.KeyId)
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
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: false}, nil
}
