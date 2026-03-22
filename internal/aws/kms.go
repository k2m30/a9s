package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("kms", []string{"alias", "key_id", "status", "description"})
	resource.Register("kms", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchKMSKeys(ctx, c.KMS, c.KMS, c.KMS)
	})
}

// FetchKMSKeys performs a multi-step fetch:
// 1. ListKeys to get key IDs
// 2. DescribeKey for each key to get metadata
// 3. ListAliases to build alias map
// Only returns customer-managed keys (KeyManager == "CUSTOMER").
func FetchKMSKeys(
	ctx context.Context,
	listKeysAPI KMSListKeysAPI,
	describeKeyAPI KMSDescribeKeyAPI,
	listAliasesAPI KMSListAliasesAPI,
) ([]resource.Resource, error) {
	// Step 1: List all keys
	listOutput, err := listKeysAPI.ListKeys(ctx, &kms.ListKeysInput{})
	if err != nil {
		return nil, fmt.Errorf("listing KMS keys: %w", err)
	}

	if len(listOutput.Keys) == 0 {
		return []resource.Resource{}, nil
	}

	// Step 2: List all aliases to build a map of keyID -> alias
	aliasOutput, err := listAliasesAPI.ListAliases(ctx, &kms.ListAliasesInput{})
	if err != nil {
		return nil, err
	}

	aliasMap := make(map[string]string)
	for _, alias := range aliasOutput.Aliases {
		if alias.TargetKeyId != nil && alias.AliasName != nil {
			aliasMap[*alias.TargetKeyId] = *alias.AliasName
		}
	}

	// Step 3: Describe each key and filter to CUSTOMER-managed
	var resources []resource.Resource

	for _, key := range listOutput.Keys {
		if key.KeyId == nil {
			continue
		}

		descOutput, err := describeKeyAPI.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(*key.KeyId),
		})
		if err != nil {
			return nil, err
		}

		meta := descOutput.KeyMetadata
		if meta == nil {
			continue
		}

		// Filter: only customer-managed keys
		if meta.KeyManager != kmstypes.KeyManagerTypeCustomer {
			continue
		}

		keyID := ""
		if meta.KeyId != nil {
			keyID = *meta.KeyId
		}

		description := ""
		if meta.Description != nil {
			description = *meta.Description
		}

		status := string(meta.KeyState)

		alias := aliasMap[keyID]

		r := resource.Resource{
			ID:     keyID,
			Name:   alias,
			Status: status,
			Fields: map[string]string{
				"key_id":      keyID,
				"alias":       alias,
				"status":      status,
				"description": description,
			},
			RawStruct:  meta,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
