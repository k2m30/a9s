package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/k2m30/a9s/internal/resource"
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
		return nil, err
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

		arn := ""
		if meta.Arn != nil {
			arn = *meta.Arn
		}

		description := ""
		if meta.Description != nil {
			description = *meta.Description
		}

		status := string(meta.KeyState)

		keyUsage := string(meta.KeyUsage)

		alias := aliasMap[keyID]

		creationDate := ""
		if meta.CreationDate != nil {
			creationDate = meta.CreationDate.Format("2006-01-02T15:04:05Z07:00")
		}

		// Build DetailData
		detail := map[string]string{
			"Key ID":        keyID,
			"ARN":           arn,
			"Alias":         alias,
			"Description":   description,
			"Status":        status,
			"Key Usage":     keyUsage,
			"Key Manager":   string(meta.KeyManager),
			"Enabled":       fmt.Sprintf("%v", meta.Enabled),
			"Creation Date": creationDate,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(meta, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  meta,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
