package unit

// aws_kms_fetch_by_ids_test.go — pin tests for FetchKMSKeysByIDs
// (internal/aws/kms.go:166). Production code is already correct;
// these tests prevent regressions in the bypass-filter, alias-lookup,
// and per-ID error-swallow behaviour.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// fakeKMSByIDs implements awsclient.KMSAPI.
// Only ListAliases and DescribeKey carry real test behaviour;
// the remaining four methods return zero-value outputs and nil errors.
type fakeKMSByIDs struct {
	listAliasesFunc func(ctx context.Context, params *kms.ListAliasesInput, optFns ...func(*kms.Options)) (*kms.ListAliasesOutput, error)
	describeKeyFunc func(ctx context.Context, params *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	listAliasesCalls int
}

func (f *fakeKMSByIDs) ListKeys(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{}, nil
}

func (f *fakeKMSByIDs) DescribeKey(ctx context.Context, params *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if f.describeKeyFunc != nil {
		return f.describeKeyFunc(ctx, params, optFns...)
	}
	return &kms.DescribeKeyOutput{}, nil
}

func (f *fakeKMSByIDs) ListAliases(ctx context.Context, params *kms.ListAliasesInput, optFns ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	f.listAliasesCalls++
	if f.listAliasesFunc != nil {
		return f.listAliasesFunc(ctx, params, optFns...)
	}
	return &kms.ListAliasesOutput{}, nil
}

func (f *fakeKMSByIDs) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func (f *fakeKMSByIDs) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}

func (f *fakeKMSByIDs) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{}, nil
}

// ---------------------------------------------------------------------------
// Test A — empty / nil input makes no API calls
// ---------------------------------------------------------------------------

func TestFetchKMSKeysByIDs_EmptyInput_NoAPICall(t *testing.T) {
	fake := &fakeKMSByIDs{}
	c := &awsclient.ServiceClients{KMS: fake}
	ctx := context.Background()

	for _, ids := range [][]string{nil, {}} {
		result, err := awsclient.FetchKMSKeysByIDs(ctx, c, ids)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result for empty input, got %v", result)
		}
		if fake.listAliasesCalls != 0 {
			t.Errorf("ListAliases should not be called for empty input; called %d times", fake.listAliasesCalls)
		}
	}
}

// ---------------------------------------------------------------------------
// Test B — two keys returned with correct alias and field shape
// ---------------------------------------------------------------------------

func TestFetchKMSKeysByIDs_ReturnsResourcesWithAliasAndFieldShape(t *testing.T) {
	aliasMap := map[string]string{
		"key-aws-managed-001": "alias/aws/elasticfilesystem",
		"key-customer-002":    "alias/my-customer-key",
	}
	metaMap := map[string]*kmstypes.KeyMetadata{
		"key-aws-managed-001": {
			KeyId:       aws.String("key-aws-managed-001"),
			Description: aws.String("EFS managed key"),
			KeyState:    kmstypes.KeyStateEnabled,
		},
		"key-customer-002": {
			KeyId:       aws.String("key-customer-002"),
			Description: aws.String("Customer master key"),
			KeyState:    kmstypes.KeyStateEnabled,
		},
	}

	fake := &fakeKMSByIDs{
		listAliasesFunc: func(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
			var aliases []kmstypes.AliasListEntry
			for id, name := range aliasMap {
				id2, name2 := id, name
				aliases = append(aliases, kmstypes.AliasListEntry{
					TargetKeyId: aws.String(id2),
					AliasName:   aws.String(name2),
				})
			}
			return &kms.ListAliasesOutput{Aliases: aliases}, nil
		},
		describeKeyFunc: func(_ context.Context, params *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			id := aws.ToString(params.KeyId)
			meta, ok := metaMap[id]
			if !ok {
				return nil, errors.New("key not found: " + id)
			}
			return &kms.DescribeKeyOutput{KeyMetadata: meta}, nil
		},
	}

	c := &awsclient.ServiceClients{KMS: fake}
	// Include an empty string — it must be skipped.
	result, err := awsclient.FetchKMSKeysByIDs(context.Background(), c, []string{"key-aws-managed-001", "", "key-customer-002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result))
	}

	byID := make(map[string]struct{})
	for i := range result {
		r := result[i]
		byID[r.ID] = struct{}{}
		wantAlias := aliasMap[r.ID]
		if r.Fields["alias"] != wantAlias {
			t.Errorf("resource %q: Fields[alias]=%q, want %q", r.ID, r.Fields["alias"], wantAlias)
		}
		if r.Name != r.Fields["alias"] {
			t.Errorf("resource %q: Name=%q should equal Fields[alias]=%q", r.ID, r.Name, r.Fields["alias"])
		}
		if r.Fields["key_id"] != r.ID {
			t.Errorf("resource %q: Fields[key_id]=%q, want %q", r.ID, r.Fields["key_id"], r.ID)
		}
		wantStatus := string(metaMap[r.ID].KeyState)
		if r.Fields["status"] != wantStatus {
			t.Errorf("resource %q: Fields[status]=%q, want %q", r.ID, r.Fields["status"], wantStatus)
		}
		wantDesc := aws.ToString(metaMap[r.ID].Description)
		if r.Fields["description"] != wantDesc {
			t.Errorf("resource %q: Fields[description]=%q, want %q", r.ID, r.Fields["description"], wantDesc)
		}
	}
	for _, wantID := range []string{"key-aws-managed-001", "key-customer-002"} {
		if _, ok := byID[wantID]; !ok {
			t.Errorf("result missing expected resource ID %q", wantID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test C — DescribeKey failure for one key surfaces composite error; partial
//          results (the good key) are still returned.
// ---------------------------------------------------------------------------

func TestFetchKMSKeysByIDs_DescribeKeyFailure_SurfacesComposite(t *testing.T) {
	fake := &fakeKMSByIDs{
		listAliasesFunc: func(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
			return &kms.ListAliasesOutput{}, nil // no aliases
		},
		describeKeyFunc: func(_ context.Context, params *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			if aws.ToString(params.KeyId) == "key-bad-002" {
				return nil, errors.New("access denied")
			}
			return &kms.DescribeKeyOutput{
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:    params.KeyId,
					KeyState: kmstypes.KeyStateEnabled,
				},
			}, nil
		},
	}

	c := &awsclient.ServiceClients{KMS: fake}
	result, err := awsclient.FetchKMSKeysByIDs(context.Background(), c, []string{"key-ok-001", "key-bad-002"})

	// Partial success: the good key must be present.
	if len(result) != 1 {
		t.Fatalf("expected 1 partial result (good key preserved), got %d", len(result))
	}
	if result[0].ID != "key-ok-001" {
		t.Errorf("expected resource ID %q, got %q", "key-ok-001", result[0].ID)
	}

	// Error must be non-nil and contain the N-of-M count, the failed ID, and the
	// injected error substring.
	if err == nil {
		t.Fatal("expected non-nil composite error for per-ID DescribeKey failure")
	}
	errStr := err.Error()
	for _, want := range []string{"kms FetchByIDs failed for 1 of 2 IDs", "key-bad-002", "access denied"} {
		if !strings.Contains(errStr, want) {
			t.Errorf("error %q does not contain expected substring %q", errStr, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Test D — ListAliases failure is a soft fallback; key data still returned,
//          but error is surfaced (operator informed of missing aliases).
// ---------------------------------------------------------------------------

func TestFetchKMSKeysByIDs_ListAliasesFailure_SoftFallbackButSurfaced(t *testing.T) {
	fake := &fakeKMSByIDs{
		listAliasesFunc: func(_ context.Context, _ *kms.ListAliasesInput, _ ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
			return nil, errors.New("throttled ListAliases error")
		},
		describeKeyFunc: func(_ context.Context, params *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			return &kms.DescribeKeyOutput{
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       params.KeyId,
					Description: aws.String("fallback key"),
					KeyState:    kmstypes.KeyStateEnabled,
				},
			}, nil
		},
	}

	c := &awsclient.ServiceClients{KMS: fake}
	result, err := awsclient.FetchKMSKeysByIDs(context.Background(), c, []string{"key-no-alias-001"})

	// Soft fallback: the key is still returned even though alias lookup failed.
	if len(result) != 1 {
		t.Fatalf("expected 1 resource (soft fallback preserves key data), got %d", len(result))
	}
	if result[0].ID != "key-no-alias-001" {
		t.Errorf("expected resource ID %q, got %q", "key-no-alias-001", result[0].ID)
	}
	// Alias column must be empty (no alias map built).
	if result[0].Fields["alias"] != "" {
		t.Errorf("Fields[alias] should be empty when ListAliases fails; got %q", result[0].Fields["alias"])
	}

	// Error must be non-nil and name both "ListAliases" and the injected error text.
	if err == nil {
		t.Fatal("expected non-nil error surfacing the ListAliases failure")
	}
	errStr := err.Error()
	for _, want := range []string{"ListAliases", "throttled ListAliases error"} {
		if !strings.Contains(errStr, want) {
			t.Errorf("error %q does not contain expected substring %q", errStr, want)
		}
	}
}
