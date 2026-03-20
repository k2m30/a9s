package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

// ---------------------------------------------------------------------------
// T-KMS01 - Test KMS multi-step fetch (ListKeys -> DescribeKey, filter CUSTOMER)
// ---------------------------------------------------------------------------

func TestFetchKMSKeys_ParsesCustomerManagedKeys(t *testing.T) {
	listKeysMock := &mockKMSListKeysClient{
		output: &kms.ListKeysOutput{
			Keys: []kmstypes.KeyListEntry{
				{KeyId: aws.String("key-111-aaa"), KeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/key-111-aaa")},
				{KeyId: aws.String("key-222-bbb"), KeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/key-222-bbb")},
				{KeyId: aws.String("key-333-ccc"), KeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/key-333-ccc")},
			},
		},
	}

	describeKeyMock := &mockKMSDescribeKeyClient{
		outputs: map[string]*kms.DescribeKeyOutput{
			"key-111-aaa": {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String("key-111-aaa"),
					Arn:         aws.String("arn:aws:kms:us-east-1:123456789012:key/key-111-aaa"),
					Description: aws.String("Customer encryption key"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeCustomer,
					KeyUsage:    kmstypes.KeyUsageTypeEncryptDecrypt,
					Enabled:     true,
				},
			},
			"key-222-bbb": {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String("key-222-bbb"),
					Arn:         aws.String("arn:aws:kms:us-east-1:123456789012:key/key-222-bbb"),
					Description: aws.String("AWS managed key"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeAws,
					KeyUsage:    kmstypes.KeyUsageTypeEncryptDecrypt,
					Enabled:     true,
				},
			},
			"key-333-ccc": {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String("key-333-ccc"),
					Arn:         aws.String("arn:aws:kms:us-east-1:123456789012:key/key-333-ccc"),
					Description: aws.String("Signing key"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeCustomer,
					KeyUsage:    kmstypes.KeyUsageTypeSignVerify,
					Enabled:     true,
				},
			},
		},
	}

	listAliasesMock := &mockKMSListAliasesClient{
		output: &kms.ListAliasesOutput{
			Aliases: []kmstypes.AliasListEntry{
				{AliasName: aws.String("alias/my-encryption-key"), TargetKeyId: aws.String("key-111-aaa")},
				{AliasName: aws.String("alias/aws/s3"), TargetKeyId: aws.String("key-222-bbb")},
				{AliasName: aws.String("alias/my-signing-key"), TargetKeyId: aws.String("key-333-ccc")},
			},
		},
	}

	resources, err := awsclient.FetchKMSKeys(context.Background(), listKeysMock, describeKeyMock, listAliasesMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should only return CUSTOMER-managed keys (2 out of 3)
	if len(resources) != 2 {
		t.Fatalf("expected 2 customer-managed resources, got %d", len(resources))
	}

	// Verify required fields
	requiredFields := []string{"key_id", "alias", "status", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first key (customer)
	r0 := resources[0]
	if r0.ID != "key-111-aaa" {
		t.Errorf("resource[0].ID: expected %q, got %q", "key-111-aaa", r0.ID)
	}
	if r0.Fields["alias"] != "alias/my-encryption-key" {
		t.Errorf("resource[0].Fields[\"alias\"]: expected %q, got %q", "alias/my-encryption-key", r0.Fields["alias"])
	}
	if r0.Fields["status"] != "Enabled" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "Enabled", r0.Fields["status"])
	}

	// Verify second key (also customer)
	r1 := resources[1]
	if r1.ID != "key-333-ccc" {
		t.Errorf("resource[1].ID: expected %q, got %q", "key-333-ccc", r1.ID)
	}
	if r1.Fields["alias"] != "alias/my-signing-key" {
		t.Errorf("resource[1].Fields[\"alias\"]: expected %q, got %q", "alias/my-signing-key", r1.Fields["alias"])
	}

	// Verify RawStruct is set
	if r0.RawStruct == nil {
		t.Error("resource[0].RawStruct should not be nil")
	}

	// Verify RawJSON is non-empty
	if r0.RawJSON == "" {
		t.Error("resource[0].RawJSON should not be empty")
	}
}

func TestFetchKMSKeys_ListKeysError(t *testing.T) {
	listKeysMock := &mockKMSListKeysClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}
	describeKeyMock := &mockKMSDescribeKeyClient{}
	listAliasesMock := &mockKMSListAliasesClient{}

	resources, err := awsclient.FetchKMSKeys(context.Background(), listKeysMock, describeKeyMock, listAliasesMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchKMSKeys_EmptyResponse(t *testing.T) {
	listKeysMock := &mockKMSListKeysClient{
		output: &kms.ListKeysOutput{
			Keys: []kmstypes.KeyListEntry{},
		},
	}
	describeKeyMock := &mockKMSDescribeKeyClient{}
	listAliasesMock := &mockKMSListAliasesClient{}

	resources, err := awsclient.FetchKMSKeys(context.Background(), listKeysMock, describeKeyMock, listAliasesMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchKMSKeys_NoAliasForKey(t *testing.T) {
	listKeysMock := &mockKMSListKeysClient{
		output: &kms.ListKeysOutput{
			Keys: []kmstypes.KeyListEntry{
				{KeyId: aws.String("key-no-alias"), KeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/key-no-alias")},
			},
		},
	}

	describeKeyMock := &mockKMSDescribeKeyClient{
		outputs: map[string]*kms.DescribeKeyOutput{
			"key-no-alias": {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:       aws.String("key-no-alias"),
					Arn:         aws.String("arn:aws:kms:us-east-1:123456789012:key/key-no-alias"),
					Description: aws.String("Key without alias"),
					KeyState:    kmstypes.KeyStateEnabled,
					KeyManager:  kmstypes.KeyManagerTypeCustomer,
					Enabled:     true,
				},
			},
		},
	}

	listAliasesMock := &mockKMSListAliasesClient{
		output: &kms.ListAliasesOutput{
			Aliases: []kmstypes.AliasListEntry{},
		},
	}

	resources, err := awsclient.FetchKMSKeys(context.Background(), listKeysMock, describeKeyMock, listAliasesMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	// Key without alias should have empty alias field
	if resources[0].Fields["alias"] != "" {
		t.Errorf("expected empty alias, got %q", resources[0].Fields["alias"])
	}
}

// ---------------------------------------------------------------------------
// T-KMS02 - Resource type definition
// ---------------------------------------------------------------------------

func TestKMS_ResourceTypeDef(t *testing.T) {
	rt := resource.FindResourceType("kms")
	if rt == nil {
		t.Fatal("resource type 'kms' not found")
	}

	if rt.Name != "KMS Keys" {
		t.Errorf("expected name %q, got %q", "KMS Keys", rt.Name)
	}

	expected := []struct {
		title string
		key   string
		width int
	}{
		{"Alias", "alias", 32},
		{"Key ID", "key_id", 38},
		{"Status", "status", 12},
		{"Description", "description", 36},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
	}
}

func TestKMS_Aliases(t *testing.T) {
	aliases := []string{"kms", "keys"}
	for _, alias := range aliases {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("expected resource type for alias %q, got nil", alias)
		}
	}
}
