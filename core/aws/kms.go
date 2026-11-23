// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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

	// Build alias map by fully paginating ListAliases. Failures here are
	// soft-fallback (aliases become empty) but must surface to the operator
	// via the composite error — silently stopping would hide a permissions
	// issue or throttling that's actively degrading the view.
	aliasMap, aliasFailures := buildKMSAliasMap(ctx, c)
	var tally describeTally

	var resources []resource.Resource
	for _, key := range listOutput.Keys {
		listedID := aws.ToString(key.KeyId)
		rows := tally.one("DescribeKey", listedID, listedID, func() (*resource.Resource, error) {
			descOutput, descErr := c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(listedID)})
			// A key policy that keeps the browsing role out of another team's key
			// is routine, not a failed read: the row says so itself, and the
			// page reports no failure for it.
			if IsAccessDenied(descErr) {
				r := kmsAccessDeniedResource(listedID, aws.ToString(key.KeyArn))
				return &r, nil
			}
			if descErr != nil {
				return nil, descErr
			}
			if descOutput.KeyMetadata == nil {
				return nil, UnusableAnswerErr{Call: "DescribeKey", Field: "key metadata"}
			}
			r := kmsKeyResource(descOutput.KeyMetadata, aliasMap)
			if kmsKind(r) == kmsKindAWSManaged {
				return nil, errNotInList
			}
			return &r, nil
		}, func(err error) resource.Resource {
			r := DegradedDetails("kms", listedID, err)
			r.Fields["key_id"] = listedID
			r.Fields["arn"] = aws.ToString(key.KeyArn)
			return r
		})
		resources = append(resources, rows...)
	}

	isTruncated := listOutput.Truncated
	var nextToken string
	if listOutput.NextMarker != nil {
		nextToken = *listOutput.NextMarker
	}

	err = errors.Join(
		kmsAliasFailure(aliasFailures),
		tally.err("kms: DescribeKey"),
	)
	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, err
}

// buildKMSAliasMap fully paginates ListAliases and returns the KeyId→
// AliasNames map (a key may carry several aliases, in ListAliases order)
// plus any failures encountered (a ListAliases error, or
// a Truncated=true response with no NextMarker — which would otherwise
// restart the page-1 fetch forever). Shared by FetchKMSKeysPage and
// FetchKMSKeysByIDs so both apply the same no-marker guard.
func buildKMSAliasMap(ctx context.Context, c *ServiceClients) (map[string][]string, []Failure) {
	var failures []Failure
	stuck := false
	// ListAliases pages by Truncated and NextMarker, sent back as Marker
	// (https://docs.aws.amazon.com/kms/latest/APIReference/API_ListAliases.html).
	aliases, complete, aliasErr := PageAll(ctx, EnrichmentCap, func(ctx context.Context, marker *string) ([]kmstypes.AliasListEntry, *string, error) {
		out, err := c.KMS.ListAliases(ctx, &kms.ListAliasesInput{Limit: aws.Int32(DefaultPageSize), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		if out.Truncated && out.NextMarker == nil {
			stuck = true
		}
		if !out.Truncated {
			return out.Aliases, nil, nil
		}
		return out.Aliases, out.NextMarker, nil
	})
	switch {
	case aliasErr != nil:
		// Soft-fallback: aliases become empty strings, but record the failure
		// so operators know aliases may be missing.
		failures = append(failures, FailedCall("ListAliases", aliasErr))
	case stuck:
		failures = append(failures, UnusableAnswer("ListAliases", "truncated response with no NextMarker"))
	case !complete:
		failures = append(failures, UnusableAnswer("ListAliases", fmt.Sprintf("more than %d pages", EnrichmentCap)))
	}
	aliasMap := make(map[string][]string)
	for _, alias := range aliases {
		if alias.TargetKeyId != nil && alias.AliasName != nil {
			aliasMap[*alias.TargetKeyId] = append(aliasMap[*alias.TargetKeyId], *alias.AliasName)
		}
	}
	return aliasMap, failures
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
// Per-ID failures (after throttle retries run out) are collected into a
// composite error returned alongside the partial success list. The caller must
// check both return values: non-nil error indicates some IDs could not be
// resolved but the slice may still contain valid results.
func FetchKMSKeysByIDs(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Soft-fallback: aliases become empty strings, but any ListAliases
	// failure is recorded so operators know aliases may be missing.
	aliasMap, aliasFailures := buildKMSAliasMap(ctx, c)
	var failures []Failure

	var resources []resource.Resource
	for _, id := range ids {
		if id == "" {
			continue
		}
		out, err := c.KMS.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(id)})
		if err != nil {
			if IsAccessDenied(err) {
				resources = append(resources, kmsAccessDeniedResource(id, ""))
				continue
			}
			if _, isAlias := kmsAliasName(id); isAlias && IsNotFoundErr(err) {
				// A deleted alias names no key; that is an answer, not a failure.
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
		known := aliasMap[keyID]
		if requested, isAlias := kmsAliasName(id); isAlias && !slices.Contains(known, requested) {
			// DescribeKey answered for this alias, so it is one of the key's,
			// even when ListAliases does not report it yet.
			known = append(slices.Clone(known), requested)
		}
		alias, aliases := kmsAliasFields(known)
		resources = append(resources, withWave1("kms", resource.Resource{
			ID:   keyID,
			Name: alias,
			Fields: map[string]string{
				"key_id":      keyID,
				"arn":         aws.ToString(meta.Arn),
				"alias":       alias,
				"aliases":     aliases,
				"status":      status,
				"description": description,
			},
			RawStruct: meta,
		}, withState("kms", status, nil)...))
	}

	return resources, errors.Join(
		kmsAliasFailure(aliasFailures),
		AggregateFailures("kms FetchByIDs", failures, len(ids)),
	)
}

// kmsAliasFailure keeps the ListAliases walk out of the per-key failure
// count: one alias-map error is not one key that could not be read.
func kmsAliasFailure(failures []Failure) error {
	return AggregateFailures("kms aliases", failures, len(failures))
}

// kmsAliasFields returns the alias a row displays, the last one ListAliases
// reported, and every alias of the key, comma-joined for kmsRefToID.
func kmsAliasFields(aliases []string) (display, all string) {
	if len(aliases) > 0 {
		display = aliases[len(aliases)-1]
	}
	return display, strings.Join(aliases, ",")
}

// kmsAliasName returns the alias name a DescribeKey KeyId names, bare or as
// an alias ARN.
func kmsAliasName(keyID string) (string, bool) {
	res, _, ok := localARN(keyID, domain.RefContext{}, "kms")
	return res, ok && strings.HasPrefix(res, "alias/")
}

// The kinds of KMS key, by who can change it and how it rotates. AWS owns an
// AWS-managed key's policy and rotation. "Automatic key rotation is supported
// only on symmetric encryption KMS keys with key material that AWS KMS
// generates" (AWS KMS Developer Guide, "Rotate AWS KMS keys"), which excludes
// asymmetric and HMAC keys, imported key material and keys in a custom key
// store; those rotate by hand. "You can enable and disable automatic key
// rotation only on a primary key" (AWS KMS Developer Guide, "Multi-Region keys
// in AWS KMS"), so a multi-Region replica rotates with its primary.
const (
	kmsKindAutomaticRotation = "automatic rotation"
	kmsKindManualRotation    = "manual rotation"
	kmsKindAWSManaged        = "AWS managed"
	kmsKindReplica           = "multi-Region replica"
	// kmsKindMetadataUnread is a listed key whose DescribeKey was refused or
	// came back empty, so nothing says which kind it is.
	kmsKindMetadataUnread = "metadata not read"
)

// kmsListedKinds are the kinds the kms list holds: FetchKMSKeysPage leaves
// AWS-managed keys out, and one reaches a row only as a drill target.
var kmsListedKinds = []string{kmsKindAutomaticRotation, kmsKindManualRotation, kmsKindReplica, kmsKindMetadataUnread} //nolint:gochecknoglobals // static value set

func kmsKind(r domain.Resource) string {
	meta, ok := assertStruct[kmstypes.KeyMetadata](r.RawStruct)
	switch {
	case !ok:
		return kmsKindMetadataUnread
	case meta.KeyManager == kmstypes.KeyManagerTypeAws:
		return kmsKindAWSManaged
	case meta.MultiRegionConfiguration != nil && meta.MultiRegionConfiguration.MultiRegionKeyType == kmstypes.MultiRegionKeyTypeReplica:
		return kmsKindReplica
	case meta.KeySpec == kmstypes.KeySpecSymmetricDefault && meta.Origin == kmstypes.OriginTypeAwsKms && aws.ToString(meta.CustomKeyStoreId) == "":
		return kmsKindAutomaticRotation
	}
	return kmsKindManualRotation
}

func kmsKeyResource(meta *kmstypes.KeyMetadata, aliasMap map[string][]string) resource.Resource {
	keyID := aws.ToString(meta.KeyId)
	status := string(meta.KeyState)
	alias, aliases := kmsAliasFields(aliasMap[keyID])
	return withWave1("kms", resource.Resource{
		ID:   keyID,
		Name: alias,
		Fields: map[string]string{
			"key_id":      keyID,
			"arn":         aws.ToString(meta.Arn),
			"alias":       alias,
			"aliases":     aliases,
			"status":      status,
			"description": aws.ToString(meta.Description),
		},
		RawStruct: meta,
	}, withState("kms", status, nil)...)
}

// kmsAccessDeniedResource synthesizes a row for a key whose DescribeKey call
// was denied: the real key state is unknowable, but the key's existence
// (from ListKeys) is not in question, so it stays visible with a finding
// distinguishing it from a key whose state was read.
func kmsAccessDeniedResource(keyID, keyARN string) resource.Resource {
	return withWave1("kms", resource.Resource{
		ID: keyID,
		Fields: map[string]string{
			"key_id": keyID,
			"arn":    keyARN,
		},
	}, []domain.Finding{
		wave1Finding(CodeKMSAccessDenied),
	}...)
}
