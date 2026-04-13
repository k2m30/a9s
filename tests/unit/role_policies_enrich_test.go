package unit

import (
	"context"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Narrow mocks for IAMGetPolicyAPI, IAMGetPolicyVersionAPI, IAMGetRolePolicyAPI
// (local to enrich tests — distinct names from aws_role_policies_test.go mocks)
// ---------------------------------------------------------------------------

type enrichGetPolicyClient struct {
	output *iam.GetPolicyOutput
	err    error
}

func (m *enrichGetPolicyClient) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return m.output, m.err
}

type enrichGetPolicyVersionClient struct {
	output *iam.GetPolicyVersionOutput
	err    error
}

func (m *enrichGetPolicyVersionClient) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return m.output, m.err
}

type enrichGetRolePolicyClient struct {
	output *iam.GetRolePolicyOutput
	err    error
}

func (m *enrichGetRolePolicyClient) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return m.output, m.err
}

type enrichListAttachedRolePoliciesClient struct {
	output *iam.ListAttachedRolePoliciesOutput
}

func (m *enrichListAttachedRolePoliciesClient) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return m.output, nil
}

type enrichListRolePoliciesClient struct {
	output *iam.ListRolePoliciesOutput
}

func (m *enrichListRolePoliciesClient) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return m.output, nil
}

type countingEnrichGetPolicyVersionClient struct {
	output *iam.GetPolicyVersionOutput
	count  *int
}

func (m *countingEnrichGetPolicyVersionClient) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	*m.count++
	return m.output, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestFetchManagedPolicyDocument_ReturnsParsedDocument(t *testing.T) {
	docJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::my-bucket/*"}]}`
	encoded := url.PathEscape(docJSON)

	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{
			Policy: &iamtypes.Policy{
				DefaultVersionId: aws.String("v3"),
			},
		},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{
				Document: aws.String(encoded),
			},
		},
	}

	doc, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/test-policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", doc)
	}
	if m["Version"] != "2012-10-17" {
		t.Errorf("expected Version 2012-10-17, got %v", m["Version"])
	}
	stmts, ok := m["Statement"].([]any)
	if !ok || len(stmts) == 0 {
		t.Fatal("expected non-empty Statement array")
	}
	stmt := stmts[0].(map[string]any)
	if stmt["Effect"] != "Allow" {
		t.Errorf("expected Effect Allow, got %v", stmt["Effect"])
	}
}

func TestFetchManagedPolicyDocument_GetPolicyError_ReturnsError(t *testing.T) {
	getPolicyMock := &enrichGetPolicyClient{
		err: errFake("GetPolicy: access denied"),
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{},
	}

	_, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/test-policy")
	if err == nil {
		t.Fatal("expected error from GetPolicy failure, got nil")
	}
}

func TestFetchManagedPolicyDocument_GetPolicyVersionError_ReturnsError(t *testing.T) {
	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{
			Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")},
		},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		err: errFake("GetPolicyVersion: throttled"),
	}

	_, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/test-policy")
	if err == nil {
		t.Fatal("expected error from GetPolicyVersion failure, got nil")
	}
}

func TestFetchInlinePolicyDocument_ReturnsParsedDocument(t *testing.T) {
	docJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Action":"*","Resource":"*"}]}`
	encoded := url.PathEscape(docJSON)

	mock := &enrichGetRolePolicyClient{
		output: &iam.GetRolePolicyOutput{
			PolicyDocument: aws.String(encoded),
		},
	}

	doc, err := awsclient.FetchInlinePolicyDocument(context.Background(), mock, "test-role", "deny-all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", doc)
	}
	stmts, ok := m["Statement"].([]any)
	if !ok || len(stmts) == 0 {
		t.Fatal("expected non-empty Statement array")
	}
	stmt := stmts[0].(map[string]any)
	if stmt["Effect"] != "Deny" {
		t.Errorf("expected Effect Deny, got %v", stmt["Effect"])
	}
}

func TestFetchInlinePolicyDocument_APIError_ReturnsError(t *testing.T) {
	mock := &enrichGetRolePolicyClient{
		err: errFake("GetRolePolicy: no such policy"),
	}

	_, err := awsclient.FetchInlinePolicyDocument(context.Background(), mock, "my-role", "missing-policy")
	if err == nil {
		t.Fatal("expected error from GetRolePolicy failure, got nil")
	}
}

func TestFetchRolePolicies_IncludesRoleNameInFields(t *testing.T) {
	attachedMock := &enrichListAttachedRolePoliciesClient{
		output: &iam.ListAttachedRolePoliciesOutput{
			AttachedPolicies: []iamtypes.AttachedPolicy{
				{PolicyName: aws.String("test-policy"), PolicyArn: aws.String("arn:aws:iam::123456789012:policy/test-policy")},
			},
		},
	}
	inlineMock := &enrichListRolePoliciesClient{
		output: &iam.ListRolePoliciesOutput{PolicyNames: []string{"inline-test"}},
	}

	result, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, map[string]string{"role_name": "my-role"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) < 2 {
		t.Fatalf("expected at least 2 resources, got %d", len(result.Resources))
	}
	// Check managed policy has role_name
	managedRes := result.Resources[0]
	if managedRes.Fields["role_name"] != "my-role" {
		t.Errorf("managed policy: expected role_name=my-role, got %q", managedRes.Fields["role_name"])
	}
	// Check inline policy has role_name
	inlineRes := result.Resources[1]
	if inlineRes.Fields["role_name"] != "my-role" {
		t.Errorf("inline policy: expected role_name=my-role, got %q", inlineRes.Fields["role_name"])
	}
}

func TestFetchManagedPolicyDocument_NoCache_EachCallHitsAPI(t *testing.T) {
	// FetchManagedPolicyDocument is a pure fetch function with no internal cache.
	// Each call should hit the API independently.
	docJSON := `{"Version":"2012-10-17","Statement":[]}`
	encoded := url.PathEscape(docJSON)

	callCount := 0
	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{
			Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")},
		},
	}
	getVersionMock := &countingEnrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)},
		},
		count: &callCount,
	}

	// First call
	_, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/no-cache-test")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 API call after first call, got %d", callCount)
	}

	// Second call — FetchManagedPolicyDocument has no cache, so it hits the API again
	_, err = awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/no-cache-test")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls (no cache at fetch level), got %d", callCount)
	}
}

func TestEnricherRegistry_RolePolicies_EnricherIsNonNil(t *testing.T) {
	// Verify the registered enricher is the real one (not a stub).
	e := resource.GetEnricher("role_policies")
	if e == nil {
		t.Fatal("role_policies enricher must not be nil")
	}
}

func TestDecodePolicyDocument_PlusSignPreserved(t *testing.T) {
	// IAM uses RFC 3986 percent-encoding, not query-string encoding.
	// A literal + in a policy value (e.g., external ID) must be preserved,
	// not converted to a space.
	docJSON := `{"Version":"2012-10-17","Statement":[{"Condition":{"StringEquals":{"sts:ExternalId":"abc+def"}}}]}`
	// Percent-encode the + as %2B (as IAM would)
	encoded := url.PathEscape(docJSON) // PathEscape encodes + as %2B

	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{
			Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")},
		},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)},
		},
	}

	doc, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/plus-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", doc)
	}
	stmt := m["Statement"].([]any)[0].(map[string]any)
	cond := stmt["Condition"].(map[string]any)
	se := cond["StringEquals"].(map[string]any)
	extID, ok := se["sts:ExternalId"].(string)
	if !ok {
		t.Fatal("expected sts:ExternalId to be a string")
	}
	if extID != "abc+def" {
		t.Errorf("expected plus sign preserved as 'abc+def', got %q", extID)
	}
}

func TestDecodePolicyDocument_LiteralPlusInDocument_NotConvertedToSpace(t *testing.T) {
	// When IAM returns a document where the original JSON contains a literal +,
	// the + in the encoded string should be treated as a literal + (not space).
	// This tests the case where the encoded document contains a raw + character.
	encoded := `%7B%22key%22%3A%22a+b%22%7D` // {"key":"a+b"} with literal +
	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{
			Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")},
		},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)},
		},
	}

	doc, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPolicyMock, getVersionMock, "arn:aws:iam::123456789012:policy/literal-plus-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", doc)
	}
	val := m["key"].(string)
	if val != "a+b" {
		t.Errorf("expected literal plus preserved as 'a+b', got %q (space would mean QueryUnescape was used)", val)
	}
}

// errFake is a simple error type used to construct errors in tests.
type errFake string

func (e errFake) Error() string { return string(e) }
