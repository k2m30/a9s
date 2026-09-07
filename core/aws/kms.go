// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.ListKeysOutput, error) {
		return c.KMS.ListKeys(ctx, input)
	})
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

	// Build alias map by fully paginating ListAliases. Failures here are
	// soft-fallback (aliases become empty) but must surface to the operator
	// via the composite error — silently stopping would hide a permissions
	// issue or throttling that's actively degrading the view.
	var failures []Failure
	aliasMap := make(map[string]string)
	var aliasMarker *string
	for {
		aliasOutput, aliasErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.ListAliasesOutput, error) {
			return c.KMS.ListAliases(ctx, &kms.ListAliasesInput{
				Limit:  aws.Int32(DefaultPageSize),
				Marker: aliasMarker,
			})
		})
		if aliasErr != nil {
			failures = append(failures, FailedCall("ListAliases", aliasErr))
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
		if aliasOutput.NextMarker == nil {
			// Truncated=true with no marker violates the ListAliases contract.
			// Restarting from aliasMarker=nil would page 1 forever, so treat
			// it the same as any other alias-fetch failure: stop and surface
			// it via the composite error, keeping the (already-fetched) keys.
			failures = append(failures, UnusableAnswer("ListAliases", "truncated response with no NextMarker"))
			break
		}
		aliasMarker = aliasOutput.NextMarker
	}

	var resources []resource.Resource
	for _, key := range listOutput.Keys {
		if key.KeyId == nil {
			continue
		}
		descOutput, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.DescribeKeyOutput, error) {
			return c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{
				KeyId: aws.String(*key.KeyId),
			})
		})
		if descErr != nil {
			if IsAccessDenied(descErr) {
				resources = append(resources, kmsAccessDeniedResource(*key.KeyId))
				continue
			}
			failures = append(failures, FailedCall(*key.KeyId, descErr))
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
			ID:       keyID,
			Name:     alias,
			Findings: kmsStateFindings(meta.KeyState, status),
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
	}, AggregateFailures("kms: FetchKMSKeysPage", failures, len(listOutput.Keys))
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
//
// Per-ID failures (after RetryOnThrottle exhaustion) are collected into a
// composite error returned alongside the partial success list. The caller must
// check both return values: non-nil error indicates some IDs could not be
// resolved but the slice may still contain valid results.
func FetchKMSKeysByIDs(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var failures []Failure

	aliasMap := make(map[string]string)
	var aliasMarker *string
	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.ListAliasesOutput, error) {
			return c.KMS.ListAliases(ctx, &kms.ListAliasesInput{
				Limit:  aws.Int32(DefaultPageSize),
				Marker: aliasMarker,
			})
		})
		if err != nil {
			// Soft-fallback: aliases become empty strings, but record the failure
			// so operators know aliases may be missing.
			failures = append(failures, FailedCall("ListAliases", err))
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
		if out.NextMarker == nil {
			// See the identical guard in FetchKMSKeysPage: Truncated=true with
			// no marker would otherwise restart the page-1 fetch forever.
			failures = append(failures, UnusableAnswer("ListAliases", "truncated response with no NextMarker"))
			break
		}
		aliasMarker = out.NextMarker
	}

	var resources []resource.Resource
	for _, id := range ids {
		if id == "" {
			continue
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.DescribeKeyOutput, error) {
			return c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(id)})
		})
		if err != nil {
			if IsAccessDenied(err) {
				resources = append(resources, kmsAccessDeniedResource(id))
				continue
			}
			failures = append(failures, FailedCall(id, err))
			continue
		}
		if out == nil || out.KeyMetadata == nil {
			failures = append(failures, UnusableAnswer(id, "no metadata"))
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
			ID:       keyID,
			Name:     alias,
			Findings: kmsStateFindings(meta.KeyState, status),
			Fields: map[string]string{
				"key_id":      keyID,
				"alias":       alias,
				"status":      status,
				"description": description,
			},
			RawStruct: meta,
		})
	}

	return resources, AggregateFailures("kms FetchByIDs", failures, len(ids))
}

// kmsAccessDeniedResource synthesizes a row for a key whose DescribeKey call
// was denied: the real key state is unknowable, but the key's existence
// (from ListKeys) is not in question, so it stays visible with a finding
// distinguishing it from the generic "unavailable" bucket in kmsStateFindings.
func kmsAccessDeniedResource(keyID string) resource.Resource {
	return resource.Resource{
		ID: keyID,
		Findings: []domain.Finding{
			{Code: CodeKMSAccessDenied, Phrase: "access denied (kms:DescribeKey)", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{
			"key_id": keyID,
			"status": "AccessDenied",
		},
	}
}

// kmsStateFindings maps a KMS key state to the canonical Finding slice.
// Enabled keys produce no findings (healthy state). All other states produce
// a single finding so callers can drive row coloring and attention aggregation
// without re-parsing the raw status string.
func kmsStateFindings(state kmstypes.KeyState, stateStr string) []domain.Finding {
	switch state {
	case "":
		// A row with no state at all is one built outside the fetcher; there
		// is nothing to report, and the default arm below would call it
		// unavailable.
		return nil
	case kmstypes.KeyStateEnabled:
		return nil
	case kmstypes.KeyStatePendingDeletion:
		return []domain.Finding{{Code: CodeKMSStatePendingDeletion, Phrase: "pending deletion", Severity: domain.SevBroken, Source: "wave1"}}
	case kmstypes.KeyStateDisabled:
		return []domain.Finding{{Code: CodeKMSStateDisabled, Phrase: "disabled", Severity: domain.SevWarn, Source: "wave1"}}
	default:
		return []domain.Finding{{Code: CodeKMSStateUnavailable, Phrase: stateStr, Severity: domain.SevBroken, Source: "wave1"}}
	}
}
