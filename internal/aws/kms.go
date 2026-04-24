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

	resource.RegisterPaginated("kms", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchKMSKeysPage(ctx, c, continuationToken)
	})

	resource.RegisterFetchByIDs("kms", func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchKMSKeysByIDs(ctx, c, ids)
	})

	resource.RegisterRelated("kms", []resource.RelatedDef{
		{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkKMSEBS, NeedsTargetCache: true},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkKMSRDS, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkKMSSecrets, NeedsTargetCache: true},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkKMSS3, NeedsTargetCache: false},
		{TargetType: "role", DisplayName: "IAM Roles (grants)", Checker: checkKMSRole, NeedsTargetCache: false},
	})
}

// FetchKMSKeysPage fetches a single page of KMS keys using the registered
// paginated fetcher pattern.
//
// It fully paginates ListAliases before iterating keys, ensuring all aliases
// are available regardless of how many pages they span.
func FetchKMSKeysPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
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

	// Build alias map by fully paginating ListAliases.
	aliasMap := make(map[string]string)
	var aliasMarker *string
	for {
		aliasOutput, aliasErr := c.KMS.ListAliases(ctx, &kms.ListAliasesInput{
			Limit:  aws.Int32(DefaultPageSize),
			Marker: aliasMarker,
		})
		if aliasErr != nil {
			break
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

	var resources []resource.Resource
	for _, key := range listOutput.Keys {
		if key.KeyId == nil {
			continue
		}
		descOutput, descErr := c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(*key.KeyId),
		})
		if descErr != nil {
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
}

// FetchKMSKeysByIDs fetches specific KMS keys by their key IDs, bypassing the
// KeyManager=CUSTOMER filter the paginated fetcher applies. Used by the
// related-panel lazy-add path so checkers referencing AWS-managed keys
// (`aws/elasticfilesystem`, `aws/rds`, `aws/s3`, etc.) still drill into a
// real entry instead of landing on an empty list.
//
// Each ID may be a bare UUID or a full ARN — DescribeKey accepts both. The
// returned Resources use the bare KeyId as the ID (matching FetchKMSKeysPage)
// and populate an alias when one is known.
func FetchKMSKeysByIDs(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	aliasMap := make(map[string]string)
	var aliasMarker *string
	for {
		out, err := c.KMS.ListAliases(ctx, &kms.ListAliasesInput{
			Limit:  aws.Int32(DefaultPageSize),
			Marker: aliasMarker,
		})
		if err != nil {
			break
		}
		for _, a := range out.Aliases {
			if a.TargetKeyId != nil && a.AliasName != nil {
				aliasMap[*a.TargetKeyId] = *a.AliasName
			}
		}
		if !out.Truncated {
			break
		}
		aliasMarker = out.NextMarker
	}

	var resources []resource.Resource
	for _, id := range ids {
		if id == "" {
			continue
		}
		out, err := c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(id)})
		if err != nil || out == nil || out.KeyMetadata == nil {
			continue
		}
		meta := out.KeyMetadata
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
	return resources, nil
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
			RawStruct: meta,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
