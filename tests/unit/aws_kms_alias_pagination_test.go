package unit

// aws_kms_alias_pagination_test.go — KMS alias full-pagination.
//
// FetchKMSKeysPage (core/aws/kms.go) fully paginates ListAliases before
// building the alias map, matching FetchKMSKeys; a single ListAliases call
// misses aliases on subsequent pages.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
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

// ---------------------------------------------------------------------------
// TestFetchKMSKeysPage_TruncatedAliasesWithNilMarkerTerminates
//
// A ListAliases response with Truncated=true and a nil NextMarker must end
// pagination, not reset the marker to nil and restart from page 1.
// kmsRunawayAliasFake always returns that exact malformed response regardless
// of the Marker it was called with, and bounds the risk itself: it fails the
// test the instant ListAliases is called more times than the guard could ever
// need, instead of letting a runaway loop spin the suite for real. The
// nil-marker guard breaks out on the very first call.
// ---------------------------------------------------------------------------

// kmsRunawayAliasFake implements awsclient.KMSAPI. Its ListAliases always
// reports Truncated=true with a nil NextMarker — the malformed response
// FetchKMSKeysPage's nil-marker guard must treat as "stop, record a
// failure", never "retry from page 1". callCap is the hang-prevention
// backstop described above, not the assertion itself.
type kmsRunawayAliasFake struct {
	t              *testing.T
	calls          int
	callCap        int
	listKeysOutput *kms.ListKeysOutput
	describeOutput *kms.DescribeKeyOutput
}

func (f *kmsRunawayAliasFake) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return f.listKeysOutput, nil
}

func (f *kmsRunawayAliasFake) ListAliases(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	f.calls++
	if f.calls > f.callCap {
		f.t.Fatalf("ListAliases called %d times (cap %d) — a reverted nil-NextMarker guard would loop here forever; refusing to spin the suite for real", f.calls, f.callCap)
	}
	return &kms.ListAliasesOutput{Truncated: true, NextMarker: nil}, nil
}

func (f *kmsRunawayAliasFake) DescribeKey(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	return f.describeOutput, nil
}

func (f *kmsRunawayAliasFake) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func (f *kmsRunawayAliasFake) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}

func (f *kmsRunawayAliasFake) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{}, nil
}

// Compile-time check: kmsRunawayAliasFake satisfies awsclient.KMSAPI.
var _ awsclient.KMSAPI = (*kmsRunawayAliasFake)(nil)

func TestFetchKMSKeysPage_TruncatedAliasesWithNilMarkerTerminates(t *testing.T) {
	const keyID = "aaaa0000-0000-0000-0000-000000000099"

	fake := &kmsRunawayAliasFake{
		t:       t,
		callCap: 5,
		listKeysOutput: &kms.ListKeysOutput{
			Keys: []kmstypes.KeyListEntry{{KeyId: aws.String(keyID)}},
		},
		describeOutput: &kms.DescribeKeyOutput{
			KeyMetadata: &kmstypes.KeyMetadata{
				KeyId:      aws.String(keyID),
				KeyState:   kmstypes.KeyStateEnabled,
				KeyManager: kmstypes.KeyManagerTypeCustomer,
			},
		},
	}

	clients := &awsclient.ServiceClients{KMS: fake}
	result, err := awsclient.FetchKMSKeysPage(context.Background(), clients, "")

	if fake.calls != 1 {
		t.Errorf("ListAliases called %d times, want exactly 1 (terminate on the first Truncated=true/nil-NextMarker response, not retry)", fake.calls)
	}
	if err == nil {
		t.Fatal("expected the malformed ListAliases response to surface as an error via AggregateFailures")
	}
	if !strings.Contains(err.Error(), "NextMarker") {
		t.Errorf("error %q does not mention the NextMarker contract violation", err.Error())
	}
	if len(result.Resources) != 1 {
		t.Errorf("expected the already-fetched key to remain visible despite the alias failure, got %d resources", len(result.Resources))
	}
}
