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

	resource.RegisterPaginated("kms", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}

		input := &kms.ListKeysInput{
			Limit: aws.Int32(DefaultPageSize),
		}
		if continuationToken != "" {
			input.Marker = aws.String(continuationToken)
		}

		listOutput, err := c.KMS.ListKeys(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing KMS keys: %w", err)
		}

		if len(listOutput.Keys) == 0 {
			return resource.FetchResult{
				Resources: []resource.Resource{},
				Pagination: &resource.PaginationMeta{
					IsTruncated: false,
					NextToken:   "",
					PageSize:    0,
					TotalHint:   -1,
				},
			}, nil
		}

		// Build alias map — single page only to bound work.
		// Aliases not found on this page will show as empty; subsequent
		// pages (when the user scrolls) will fill them in.
		aliasMap := make(map[string]string)
		aliasOutput, aliasErr := c.KMS.ListAliases(ctx, &kms.ListAliasesInput{
			Limit: aws.Int32(DefaultPageSize),
		})
		if aliasErr == nil {
			for _, alias := range aliasOutput.Aliases {
				if alias.TargetKeyId != nil && alias.AliasName != nil {
					aliasMap[*alias.TargetKeyId] = *alias.AliasName
				}
			}
		}

		var resources []resource.Resource
		for _, key := range listOutput.Keys {
			if key.KeyId == nil {
				continue
			}
			descOutput, err := c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{
				KeyId: aws.String(*key.KeyId),
			})
			if err != nil {
				continue
			}
			meta := descOutput.KeyMetadata
			if meta == nil {
				continue
			}
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

			resources = append(resources, resource.Resource{
				ID:     keyID,
				Name:   alias,
				Status: status,
				Fields: map[string]string{
					"key_id":      keyID,
					"alias":       alias,
					"status":      status,
					"description": description,
				},
				RawStruct: meta,
			})
		}

		isTruncated := listOutput.Truncated
		var nextToken string
		if listOutput.NextMarker != nil {
			nextToken = *listOutput.NextMarker
		}

		return resource.FetchResult{
			Resources: resources,
			Pagination: &resource.PaginationMeta{
				IsTruncated: isTruncated,
				NextToken:   nextToken,
				PageSize:    len(resources),
				TotalHint:   -1,
			},
		}, nil
	})

	resource.RegisterRelated("kms", []resource.RelatedDef{
		{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkKMSEBS, NeedsTargetCache: true},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkKMSRDS, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkKMSSecrets, NeedsTargetCache: true},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkKMSS3, NeedsTargetCache: true},
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
	// Step 1: List all keys with pagination
	var allKeys []kmstypes.KeyListEntry
	var keysMarker *string

	for {
		listOutput, err := listKeysAPI.ListKeys(ctx, &kms.ListKeysInput{
			Marker: keysMarker,
		})
		if err != nil {
			return nil, fmt.Errorf("listing KMS keys: %w", err)
		}

		allKeys = append(allKeys, listOutput.Keys...)

		if !listOutput.Truncated {
			break
		}
		keysMarker = listOutput.NextMarker
	}

	if len(allKeys) == 0 {
		return []resource.Resource{}, nil
	}

	// Step 2: List all aliases with pagination to build a map of keyID -> alias
	aliasMap := make(map[string]string)
	var aliasMarker *string

	for {
		aliasOutput, err := listAliasesAPI.ListAliases(ctx, &kms.ListAliasesInput{
			Marker: aliasMarker,
		})
		if err != nil {
			return nil, fmt.Errorf("listing KMS aliases: %w", err)
		}

		for _, alias := range aliasOutput.Aliases {
			if alias.TargetKeyId != nil && alias.AliasName != nil {
				aliasMap[*alias.TargetKeyId] = *alias.AliasName
			}
		}

		if !aliasOutput.Truncated {
			break
		}
		aliasMarker = aliasOutput.NextMarker
	}

	// Step 3: Describe each key and filter to CUSTOMER-managed
	var resources []resource.Resource

	for _, key := range allKeys {
		if key.KeyId == nil {
			continue
		}

		descOutput, err := describeKeyAPI.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(*key.KeyId),
		})
		if err != nil {
			continue // skip keys we can't describe (e.g. permission denied)
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
