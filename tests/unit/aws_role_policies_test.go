package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Local mock: IAMListAttachedRolePoliciesAPI
// ---------------------------------------------------------------------------

type mockIAMListAttachedRolePoliciesClient struct {
	outputs []*iam.ListAttachedRolePoliciesOutput
	err     error
	callIdx int
}

func (m *mockIAMListAttachedRolePoliciesClient) ListAttachedRolePolicies(
	ctx context.Context,
	params *iam.ListAttachedRolePoliciesInput,
	optFns ...func(*iam.Options),
) (*iam.ListAttachedRolePoliciesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.ListAttachedRolePoliciesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// ---------------------------------------------------------------------------
// Local mock: IAMListRolePoliciesAPI
// ---------------------------------------------------------------------------

type mockIAMListRolePoliciesClient struct {
	outputs []*iam.ListRolePoliciesOutput
	err     error
	callIdx int
}

func (m *mockIAMListRolePoliciesClient) ListRolePolicies(
	ctx context.Context,
	params *iam.ListRolePoliciesInput,
	optFns ...func(*iam.Options),
) (*iam.ListRolePoliciesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.ListRolePoliciesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestFetchRolePolicies_Basic verifies merging 3 managed + 2 inline policies
// with correct order (managed first), correct ID/Name/Status/Fields.
func TestFetchRolePolicies_Basic(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{
						PolicyName: aws.String("ReadOnlyAccess"),
						PolicyArn:  aws.String("arn:aws:iam::aws:policy/ReadOnlyAccess"),
					},
					{
						PolicyName: aws.String("CloudWatchFullAccess"),
						PolicyArn:  aws.String("arn:aws:iam::aws:policy/CloudWatchFullAccess"),
					},
					{
						PolicyName: aws.String("AmazonS3ReadOnlyAccess"),
						PolicyArn:  aws.String("arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"),
					},
				},
				IsTruncated: false,
			},
		},
	}

	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{
				PolicyNames: []string{"trust-policy", "logging-policy"},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"role_name": "my-service-role"}

	resources, err := awsclient.FetchRolePolicies(
		context.Background(),
		attachedMock,
		inlineMock,
		parentCtx,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 3 managed + 2 inline = 5 total
	if len(resources) != 5 {
		t.Fatalf("expected 5 resources, got %d", len(resources))
	}

	// Managed policies come first
	r0 := resources[0]
	t.Run("managed_policy_name", func(t *testing.T) {
		if r0.Fields["policy_name"] != "ReadOnlyAccess" {
			t.Errorf("Fields[policy_name]: expected %q, got %q", "ReadOnlyAccess", r0.Fields["policy_name"])
		}
	})
	t.Run("managed_policy_arn", func(t *testing.T) {
		if r0.Fields["policy_arn"] != "arn:aws:iam::aws:policy/ReadOnlyAccess" {
			t.Errorf("Fields[policy_arn]: expected ARN, got %q", r0.Fields["policy_arn"])
		}
	})
	t.Run("managed_policy_type", func(t *testing.T) {
		if r0.Fields["policy_type"] != "Managed" {
			t.Errorf("Fields[policy_type]: expected %q, got %q", "Managed", r0.Fields["policy_type"])
		}
	})
	t.Run("managed_ID_is_ARN", func(t *testing.T) {
		if r0.ID != "arn:aws:iam::aws:policy/ReadOnlyAccess" {
			t.Errorf("ID: expected ARN, got %q", r0.ID)
		}
	})
	t.Run("managed_Name_is_policy_name", func(t *testing.T) {
		if r0.Name != "ReadOnlyAccess" {
			t.Errorf("Name: expected %q, got %q", "ReadOnlyAccess", r0.Name)
		}
	})

	// Inline policies come after managed
	r3 := resources[3]
	t.Run("inline_policy_name", func(t *testing.T) {
		if r3.Fields["policy_name"] != "trust-policy" {
			t.Errorf("Fields[policy_name]: expected %q, got %q", "trust-policy", r3.Fields["policy_name"])
		}
	})
	t.Run("inline_policy_arn_empty", func(t *testing.T) {
		if r3.Fields["policy_arn"] != "" {
			t.Errorf("Fields[policy_arn]: expected empty for inline, got %q", r3.Fields["policy_arn"])
		}
	})
	t.Run("inline_policy_type", func(t *testing.T) {
		if r3.Fields["policy_type"] != "Inline" {
			t.Errorf("Fields[policy_type]: expected %q, got %q", "Inline", r3.Fields["policy_type"])
		}
	})
	t.Run("inline_ID_is_name", func(t *testing.T) {
		if r3.ID != "trust-policy" {
			t.Errorf("ID: expected %q, got %q", "trust-policy", r3.ID)
		}
	})

	t.Run("required_fields_present_on_all_rows", func(t *testing.T) {
		requiredFields := []string{"policy_name", "policy_arn", "policy_type"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("Row %d Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchRolePolicies_ManagedOnly verifies behavior when a role has only
// managed policies and no inline policies.
func TestFetchRolePolicies_ManagedOnly(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: aws.String("ReadOnlyAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/ReadOnlyAccess")},
				},
				IsTruncated: false,
			},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "managed-only-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Fields["policy_type"] != "Managed" {
		t.Errorf("expected Managed, got %q", resources[0].Fields["policy_type"])
	}
}

// TestFetchRolePolicies_InlineOnly verifies behavior when a role has only
// inline policies and no managed policies.
func TestFetchRolePolicies_InlineOnly(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{AttachedPolicies: []iamtypes.AttachedPolicy{}, IsTruncated: false},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{"inline-policy-1"}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "inline-only-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Fields["policy_type"] != "Inline" {
		t.Errorf("expected Inline, got %q", resources[0].Fields["policy_type"])
	}
}

// TestFetchRolePolicies_Empty verifies that a role with no policies at all
// returns an empty slice and no error.
func TestFetchRolePolicies_Empty(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{AttachedPolicies: []iamtypes.AttachedPolicy{}, IsTruncated: false},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "empty-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchRolePolicies_AttachedAPIError verifies that errors from the
// ListAttachedRolePolicies API are propagated.
func TestFetchRolePolicies_AttachedAPIError(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		err: fmt.Errorf("access denied for attached policies"),
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "error-role"}
	_, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain 'access denied', got %q", err.Error())
	}
}

// TestFetchRolePolicies_InlineAPIError verifies that errors from the
// ListRolePolicies API are propagated.
func TestFetchRolePolicies_InlineAPIError(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{AttachedPolicies: []iamtypes.AttachedPolicy{}, IsTruncated: false},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		err: fmt.Errorf("access denied for inline policies"),
	}

	parentCtx := map[string]string{"role_name": "error-role"}
	_, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain 'access denied', got %q", err.Error())
	}
}

// TestFetchRolePolicies_NilFields verifies that nil PolicyName/PolicyArn
// on an AttachedPolicy does not cause a panic.
func TestFetchRolePolicies_NilFields(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: nil, PolicyArn: nil},
				},
				IsTruncated: false,
			},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "nil-fields-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
}

// TestFetchRolePolicies_AdminHighlight verifies that AdministratorAccess
// policy gets Status="failed" for status coloring.
func TestFetchRolePolicies_AdminHighlight(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{
						PolicyName: aws.String("AdministratorAccess"),
						PolicyArn:  aws.String("arn:aws:iam::aws:policy/AdministratorAccess"),
					},
				},
				IsTruncated: false,
			},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "admin-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Status != "failed" {
		t.Errorf("AdministratorAccess Status: expected %q, got %q", "failed", resources[0].Status)
	}
}

// TestFetchRolePolicies_PowerUserHighlight verifies that PowerUserAccess
// policy gets Status="failed" for status coloring.
func TestFetchRolePolicies_PowerUserHighlight(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{
						PolicyName: aws.String("PowerUserAccess"),
						PolicyArn:  aws.String("arn:aws:iam::aws:policy/PowerUserAccess"),
					},
				},
				IsTruncated: false,
			},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "power-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Status != "failed" {
		t.Errorf("PowerUserAccess Status: expected %q, got %q", "failed", resources[0].Status)
	}
}

// TestFetchRolePolicies_InlineDim verifies that inline policies get
// Status="terminated" for dimming.
func TestFetchRolePolicies_InlineDim(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{AttachedPolicies: []iamtypes.AttachedPolicy{}, IsTruncated: false},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{"my-inline-policy"}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "inline-dim-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Status != "terminated" {
		t.Errorf("Inline policy Status: expected %q, got %q", "terminated", resources[0].Status)
	}
}

// TestFetchRolePolicies_RawStruct verifies that RawStruct is a RolePolicyRow.
func TestFetchRolePolicies_RawStruct(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{
						PolicyName: aws.String("ReadOnlyAccess"),
						PolicyArn:  aws.String("arn:aws:iam::aws:policy/ReadOnlyAccess"),
					},
				},
				IsTruncated: false,
			},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{PolicyNames: []string{}, IsTruncated: false},
		},
	}

	parentCtx := map[string]string{"role_name": "rawstruct-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	row, ok := r.RawStruct.(awsclient.RolePolicyRow)
	if !ok {
		t.Fatalf("RawStruct should be awsclient.RolePolicyRow, got %T", r.RawStruct)
	}
	if row.PolicyName != "ReadOnlyAccess" {
		t.Errorf("RolePolicyRow.PolicyName: expected %q, got %q", "ReadOnlyAccess", row.PolicyName)
	}
	if row.PolicyArn != "arn:aws:iam::aws:policy/ReadOnlyAccess" {
		t.Errorf("RolePolicyRow.PolicyArn: expected ARN, got %q", row.PolicyArn)
	}
	if row.PolicyType != "Managed" {
		t.Errorf("RolePolicyRow.PolicyType: expected %q, got %q", "Managed", row.PolicyType)
	}
}

// TestFetchRolePolicies_Pagination verifies that the fetcher handles
// paginated responses from both APIs.
func TestFetchRolePolicies_Pagination(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesClient{
		outputs: []*iam.ListAttachedRolePoliciesOutput{
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: aws.String("Policy1"), PolicyArn: aws.String("arn:aws:iam::aws:policy/Policy1")},
				},
				IsTruncated: true,
				Marker:      aws.String("marker1"),
			},
			{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: aws.String("Policy2"), PolicyArn: aws.String("arn:aws:iam::aws:policy/Policy2")},
				},
				IsTruncated: false,
			},
		},
	}
	inlineMock := &mockIAMListRolePoliciesClient{
		outputs: []*iam.ListRolePoliciesOutput{
			{
				PolicyNames: []string{"InlineA"},
				IsTruncated: true,
				Marker:      aws.String("marker2"),
			},
			{
				PolicyNames: []string{"InlineB"},
				IsTruncated: false,
			},
		},
	}

	parentCtx := map[string]string{"role_name": "paginated-role"}
	resources, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 4 {
		t.Fatalf("expected 4 resources (2 managed + 2 inline), got %d", len(resources))
	}

	// Verify managed come first
	if resources[0].Fields["policy_type"] != "Managed" {
		t.Errorf("resources[0] should be Managed, got %q", resources[0].Fields["policy_type"])
	}
	if resources[1].Fields["policy_type"] != "Managed" {
		t.Errorf("resources[1] should be Managed, got %q", resources[1].Fields["policy_type"])
	}
	// Verify inline come second
	if resources[2].Fields["policy_type"] != "Inline" {
		t.Errorf("resources[2] should be Inline, got %q", resources[2].Fields["policy_type"])
	}
	if resources[3].Fields["policy_type"] != "Inline" {
		t.Errorf("resources[3] should be Inline, got %q", resources[3].Fields["policy_type"])
	}
}

// TestRolePolicyColumns verifies the column count, keys, titles, and widths.
func TestRolePolicyColumns(t *testing.T) {
	cols := resource.RolePolicyColumns()

	if len(cols) != 3 {
		t.Fatalf("RolePolicyColumns() returned %d columns, expected 3", len(cols))
	}

	wantCols := []struct {
		key   string
		title string
		width int
	}{
		{"policy_name", "Policy Name", 40},
		{"policy_arn", "Policy ARN", 56},
		{"policy_type", "Type", 10},
	}

	for i, want := range wantCols {
		if i >= len(cols) {
			t.Errorf("Missing column at index %d", i)
			continue
		}
		if cols[i].Key != want.key {
			t.Errorf("Column %d Key: expected %q, got %q", i, want.key, cols[i].Key)
		}
		if cols[i].Title != want.title {
			t.Errorf("Column %d Title: expected %q, got %q", i, want.title, cols[i].Title)
		}
		if cols[i].Width != want.width {
			t.Errorf("Column %d Width: expected %d, got %d", i, want.width, cols[i].Width)
		}
	}
}

// TestRolePolicies_ChildFetcherRegistered verifies that the child fetcher
// is registered under the correct short name.
func TestRolePolicies_ChildFetcherRegistered(t *testing.T) {
	f := resource.GetChildFetcher("role_policies")
	if f == nil {
		t.Fatal("role_policies child fetcher not registered")
	}
}

// TestRolePolicies_ParentHasChildDef verifies that the role parent resource
// type has a Children entry for role_policies.
func TestRolePolicies_ParentHasChildDef(t *testing.T) {
	var roleType *resource.ResourceTypeDef
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName == "role" {
			roleType = &rt
			break
		}
	}
	if roleType == nil {
		t.Fatal("role resource type not found")
	}

	found := false
	for _, child := range roleType.Children {
		if child.ChildType == "role_policies" {
			found = true
			if child.Key != "enter" {
				t.Errorf("role_policies child def Key: expected %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["role_name"] != "ID" {
				t.Errorf("role_policies ContextKeys[role_name]: expected %q, got %q",
					"ID", child.ContextKeys["role_name"])
			}
			break
		}
	}
	if !found {
		t.Error("role resource type missing Children entry for role_policies")
	}
}
