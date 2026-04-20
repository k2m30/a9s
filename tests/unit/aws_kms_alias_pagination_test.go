package unit

// aws_kms_alias_pagination_test.go — Failing tests for KMS alias full-pagination.
//
// CODER CHECKLIST — new export required from internal/aws/kms.go:
//
//   func FetchKMSKeysPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error)
//
// This helper must be extracted from the init() registered closure at kms.go:17-123.
// The current init() closure calls ListAliases only once (lines 51-59), which misses
// aliases on subsequent pages. The new FetchKMSKeysPage must fully paginate
// ListAliases before building the alias map (matching the behavior of FetchKMSKeys
// at kms.go:169-191, which already paginates correctly).
//
// After FetchKMSKeysPage is added, the init() registration should call it:
//
//   resource.RegisterPaginated("kms", func(ctx context.Context, clients any, tok string) (resource.FetchResult, error) {
//       c, _ := clients.(*ServiceClients)
//       return FetchKMSKeysPage(ctx, c, tok)
//   })

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Fake: KMSAPI — supports paginated ListAliases, paginated ListKeys,
// and per-key DescribeKey.
// ---------------------------------------------------------------------------

// kmsAliasPaginationFake implements awsclient.KMSAPI with programmable
// paginated responses for ListAliases and single-page ListKeys + DescribeKey.
type kmsAliasPaginationFake struct {
	// listKeysOutput is the single-page response for ListKeys.
	listKeysOutput *kms.ListKeysOutput

	// aliasPages is an ordered slice of ListAliases responses to serve in sequence.
	// aliasCallIdx tracks which page to serve next.
	aliasPages   []*kms.ListAliasesOutput
	aliasCallIdx int

	// describeOutputs maps keyID → DescribeKey response.
	describeOutputs map[string]*kms.DescribeKeyOutput
}

func (f *kmsAliasPaginationFake) ListKeys(
	_ context.Context,
	_ *kms.ListKeysInput,
	_ ...func(*kms.Options),
) (*kms.ListKeysOutput, error) {
	if f.listKeysOutput == nil {
		return &kms.ListKeysOutput{}, nil
	}
	return f.listKeysOutput, nil
}

func (f *kmsAliasPaginationFake) ListAliases(
	_ context.Context,
	_ *kms.ListAliasesInput,
	_ ...func(*kms.Options),
) (*kms.ListAliasesOutput, error) {
	if f.aliasCallIdx >= len(f.aliasPages) {
		return &kms.ListAliasesOutput{Truncated: false}, nil
	}
	out := f.aliasPages[f.aliasCallIdx]
	f.aliasCallIdx++
	return out, nil
}

func (f *kmsAliasPaginationFake) DescribeKey(
	_ context.Context,
	input *kms.DescribeKeyInput,
	_ ...func(*kms.Options),
) (*kms.DescribeKeyOutput, error) {
	if input.KeyId == nil {
		return nil, fmt.Errorf("DescribeKey: nil KeyId")
	}
	if out, ok := f.describeOutputs[*input.KeyId]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("key %q not found", *input.KeyId)
}

// Stubs for additional KMSAPI methods not under test here.

func (f *kmsAliasPaginationFake) GetKeyRotationStatus(
	_ context.Context,
	_ *kms.GetKeyRotationStatusInput,
	_ ...func(*kms.Options),
) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func (f *kmsAliasPaginationFake) ListGrants(
	_ context.Context,
	_ *kms.ListGrantsInput,
	_ ...func(*kms.Options),
) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}

func (f *kmsAliasPaginationFake) GetKeyPolicy(
	_ context.Context,
	_ *kms.GetKeyPolicyInput,
	_ ...func(*kms.Options),
) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{}, nil
}

// Compile-time check: kmsAliasPaginationFake satisfies awsclient.KMSAPI.
var _ awsclient.KMSAPI = (*kmsAliasPaginationFake)(nil)

// ---------------------------------------------------------------------------
// TestFetchKMSKeysPage_FullyPaginatesListAliases
//
// Verifies that FetchKMSKeysPage fully paginates ListAliases across three pages
// before building the alias map. All three customer-managed keys must have their
// correct alias populated (not left blank for keys on page 2 or 3).
//
// Setup:
//   - ListKeys returns [key-a, key-b, key-c], not truncated.
//   - ListAliases page 1: alias for key-a only, Truncated=true, NextMarker="m1".
//   - ListAliases page 2: alias for key-b only, Truncated=true, NextMarker="m2".
//   - ListAliases page 3: alias for key-c only, Truncated=false.
//   - DescribeKey returns all three as CUSTOMER-managed.
//
// Expected: all three resources have their expected aliases populated.
// ---------------------------------------------------------------------------

func TestFetchKMSKeysPage_FullyPaginatesListAliases(t *testing.T) {
	// CODER NOTE: This test will fail to compile until FetchKMSKeysPage is
	// exported from internal/aws/kms.go. That is intentional — TDD red phase.

	const (
		keyA   = "aaaa0000-0000-0000-0000-000000000001"
		keyB   = "bbbb0000-0000-0000-0000-000000000002"
		keyC   = "cccc0000-0000-0000-0000-000000000003"
		aliasA = "alias/key-alpha"
		aliasB = "alias/key-beta"
		aliasC = "alias/key-gamma"
	)

	fake := &kmsAliasPaginationFake{
		listKeysOutput: &kms.ListKeysOutput{
			Keys: []kmstypes.KeyListEntry{
				{KeyId: aws.String(keyA)},
				{KeyId: aws.String(keyB)},
				{KeyId: aws.String(keyC)},
			},
			Truncated: false,
		},
		aliasPages: []*kms.ListAliasesOutput{
			{
				// Page 1: only key-a alias
				Aliases: []kmstypes.AliasListEntry{
					{TargetKeyId: aws.String(keyA), AliasName: aws.String(aliasA)},
				},
				Truncated:  true,
				NextMarker: aws.String("m1"),
			},
			{
				// Page 2: only key-b alias
				Aliases: []kmstypes.AliasListEntry{
					{TargetKeyId: aws.String(keyB), AliasName: aws.String(aliasB)},
				},
				Truncated:  true,
				NextMarker: aws.String("m2"),
			},
			{
				// Page 3: only key-c alias, last page
				Aliases: []kmstypes.AliasListEntry{
					{TargetKeyId: aws.String(keyC), AliasName: aws.String(aliasC)},
				},
				Truncated: false,
			},
		},
		describeOutputs: map[string]*kms.DescribeKeyOutput{
			keyA: {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String(keyA),
					Description: aws.String("Alpha key"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeCustomer,
				},
			},
			keyB: {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String(keyB),
					Description: aws.String("Beta key"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeCustomer,
				},
			},
			keyC: {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String(keyC),
					Description: aws.String("Gamma key"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeCustomer,
				},
			},
		},
	}

	clients := &awsclient.ServiceClients{KMS: fake}

	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("len(resources) = %d, want 3", len(result.Resources))
	}

	// Build alias-by-ID map from result
	gotAlias := make(map[string]string, 3)
	for _, r := range result.Resources {
		gotAlias[r.ID] = r.Fields["alias"]
	}

	// All three keys must have their alias from the correct page.
	type wantAlias struct {
		keyID string
		alias string
	}
	wants := []wantAlias{
		{keyA, aliasA},
		{keyB, aliasB},
		{keyC, aliasC},
	}
	for _, w := range wants {
		got, ok := gotAlias[w.keyID]
		if !ok {
			t.Errorf("resource with ID %q not found in results", w.keyID)
			continue
		}
		if got != w.alias {
			t.Errorf("resource %q Fields[\"alias\"] = %q, want %q (ListAliases pagination not fully followed)", w.keyID, got, w.alias)
		}
	}

	// Verify ListAliases was called 3 times (all pages consumed).
	if fake.aliasCallIdx != 3 {
		t.Errorf("ListAliases called %d times, want 3 (full pagination)", fake.aliasCallIdx)
	}
}
