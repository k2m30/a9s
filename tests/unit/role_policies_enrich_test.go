package unit

import (
	"context"
	"net/url"
	"strings"
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

func TestDetailEnricherRegistry_RolePolicies_EnricherIsNonNil(t *testing.T) {
	// Verify the registered detail enricher is the real one (not a stub).
	e := resource.GetDetailEnricher("role_policies")
	if e == nil {
		t.Fatal("role_policies detail enricher must not be nil")
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

// ---------------------------------------------------------------------------
// fakeIAMBase embeds awsclient.IAMAPI so the struct satisfies that interface
// at compile time (via embedding), but provides no concrete method
// implementations. When enrichRolePolicy does c.IAM.(IAMGetRolePolicyAPI) or
// c.IAM.(IAMGetPolicyAPI), the assertion fails at runtime — which is exactly
// what TestEnrichRolePolicy_InlineIAMTypeAssertionFails and
// TestEnrichRolePolicy_ManagedIAMTypeAssertionFails need to verify.
// ---------------------------------------------------------------------------

type fakeIAMBase struct {
	awsclient.IAMAPI
}

// ---------------------------------------------------------------------------
// combinedIAMMock satisfies IAMGetPolicyAPI + IAMGetPolicyVersionAPI via
// concrete method implementations, and delegates all other IAMAPI methods
// to the embedded awsclient.IAMAPI. Used for managed-policy enricher tests.
// ---------------------------------------------------------------------------

type combinedIAMMock struct {
	awsclient.IAMAPI
	getPolicyOut    *iam.GetPolicyOutput
	getPolicyErr    error
	getPolicyVerOut *iam.GetPolicyVersionOutput
	getPolicyVerErr error
}

func (m *combinedIAMMock) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return m.getPolicyOut, m.getPolicyErr
}

func (m *combinedIAMMock) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return m.getPolicyVerOut, m.getPolicyVerErr
}

// ---------------------------------------------------------------------------
// inlineIAMMock wraps *enrichGetRolePolicyClient so it satisfies the full
// awsclient.IAMAPI interface. All methods other than GetRolePolicy are
// delegated to the embedded fakeIAMBase (which will panic if called —
// that is intentional: the enricher should only call GetRolePolicy for inline
// policy tests, and any unexpected call to another method indicates a bug).
// ---------------------------------------------------------------------------

type inlineIAMMock struct {
	awsclient.IAMAPI
	inner *enrichGetRolePolicyClient
}

func (m *inlineIAMMock) GetRolePolicy(ctx context.Context, in *iam.GetRolePolicyInput, opts ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return m.inner.GetRolePolicy(ctx, in, opts...)
}

// ---------------------------------------------------------------------------
// rolePoliciesEnricher retrieves the registered detail enricher for
// "role_policies" and fails the test immediately if it is not found.
// ---------------------------------------------------------------------------

func rolePoliciesEnricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("role_policies")
	if e == nil {
		t.Fatal("role_policies detail enricher not registered")
	}
	return e
}

// makeRolePoliciesCtx builds a *DetailEnrichmentCtx with a fresh cache.
func makeRolePoliciesCtx(iamClient awsclient.IAMAPI) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: iamClient},
		PolicyDocs: &awsclient.PolicyDocumentCache{},
	}
}

// makeRolePoliciesCtxWithCache builds a *DetailEnrichmentCtx with the given cache.
func makeRolePoliciesCtxWithCache(iamClient awsclient.IAMAPI, cache *awsclient.PolicyDocumentCache) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: iamClient},
		PolicyDocs: cache,
	}
}

// makeInlineRes returns a Resource with an Inline RolePolicyRow RawStruct.
func makeInlineRes(roleName, policyName string) resource.Resource {
	return resource.Resource{
		Fields: map[string]string{
			"role_name":   roleName,
			"policy_name": policyName,
		},
		RawStruct: awsclient.RolePolicyRow{
			PolicyName: policyName,
			PolicyType: "Inline",
		},
	}
}

// makeManagedRes returns a Resource with a Managed RolePolicyRow RawStruct.
func makeManagedRes(policyName, policyArn string) resource.Resource {
	return resource.Resource{
		Fields: map[string]string{"policy_name": policyName},
		RawStruct: awsclient.RolePolicyRow{
			PolicyName: policyName,
			PolicyArn:  policyArn,
			PolicyType: "Managed",
		},
	}
}

// validEnrichedDoc asserts RawStruct is a RolePolicyRow with a map Document.
func validEnrichedDoc(t *testing.T, res resource.Resource) map[string]any {
	t.Helper()
	row, ok := res.RawStruct.(awsclient.RolePolicyRow)
	if !ok {
		t.Fatalf("RawStruct is not RolePolicyRow: %T", res.RawStruct)
	}
	doc, ok := row.Document.(map[string]any)
	if !ok {
		t.Fatalf("Document is not map[string]any: %T", row.Document)
	}
	return doc
}

// ---------------------------------------------------------------------------
// TestEnrichRolePolicy_* — enrichRolePolicy behavioral tests (via registry)
// ---------------------------------------------------------------------------

func TestEnrichRolePolicy_InlineCacheHit(t *testing.T) {
	// Cache pre-populated — enricher must not call any IAM API.
	cachedDoc := map[string]any{"Version": "2012-10-17", "cached": true}
	cache := &awsclient.PolicyDocumentCache{}
	cache.Set(awsclient.InlineKey("my-role", "my-inline-policy"), cachedDoc)

	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtxWithCache(&fakeIAMBase{}, cache)
	res := makeInlineRes("my-role", "my-inline-policy")

	enriched, err := enricher(context.Background(), dctx, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc := validEnrichedDoc(t, enriched)
	if doc["cached"] != true {
		t.Errorf("expected cached doc to be returned, got %v", doc)
	}
}

func TestEnrichRolePolicy_InlineCacheMiss_FetchSuccess(t *testing.T) {
	docJSON := `{"Version":"2012-10-17","Statement":[]}`
	encoded := url.PathEscape(docJSON)
	mock := &inlineIAMMock{inner: &enrichGetRolePolicyClient{
		output: &iam.GetRolePolicyOutput{PolicyDocument: aws.String(encoded)},
	}}
	cache := &awsclient.PolicyDocumentCache{}
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtxWithCache(mock, cache)
	res := makeInlineRes("my-role", "fetch-policy")

	enriched, err := enricher(context.Background(), dctx, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc := validEnrichedDoc(t, enriched)
	if doc["Version"] != "2012-10-17" {
		t.Errorf("expected Version 2012-10-17, got %v", doc["Version"])
	}
	if cache.Get(awsclient.InlineKey("my-role", "fetch-policy")) == nil {
		t.Error("cache should be populated after inline fetch")
	}
}

func TestEnrichRolePolicy_InlineCacheMiss_FetchError(t *testing.T) {
	mock := &inlineIAMMock{inner: &enrichGetRolePolicyClient{err: errFake("GetRolePolicy: access denied")}}
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(mock)
	res := makeInlineRes("my-role", "failing-policy")

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error from GetRolePolicy failure, got nil")
	}
}

func TestEnrichRolePolicy_InlineMissingRoleName(t *testing.T) {
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(&fakeIAMBase{})
	res := resource.Resource{
		Fields:    map[string]string{}, // no role_name key
		RawStruct: awsclient.RolePolicyRow{PolicyName: "some-policy", PolicyType: "Inline"},
	}

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for missing role_name, got nil")
	}
	if !strings.Contains(err.Error(), "role_name") {
		t.Errorf("error should mention role_name, got: %v", err)
	}
}

func TestEnrichRolePolicy_InlineUnknownPolicyType_FallsToManagedMissingARN(t *testing.T) {
	// PolicyType != "Inline" falls to the else (managed) branch.
	// An empty PolicyArn triggers the missing-ARN guard.
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(&fakeIAMBase{})
	res := resource.Resource{
		Fields: map[string]string{},
		RawStruct: awsclient.RolePolicyRow{
			PolicyName: "some-policy",
			PolicyArn:  "",
			PolicyType: "Unknown",
		},
	}

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for unknown PolicyType with empty ARN, got nil")
	}
	if !strings.Contains(err.Error(), "policy ARN") {
		t.Errorf("error should mention policy ARN, got: %v", err)
	}
}

func TestEnrichRolePolicy_ManagedCacheHit(t *testing.T) {
	policyArn := "arn:aws:iam::123456789012:policy/cached-policy"
	cachedDoc := map[string]any{"Version": "2012-10-17", "cached": true}
	cache := &awsclient.PolicyDocumentCache{}
	cache.Set(awsclient.ManagedKey(policyArn), cachedDoc)

	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtxWithCache(&fakeIAMBase{}, cache)
	res := makeManagedRes("cached-policy", policyArn)

	enriched, err := enricher(context.Background(), dctx, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc := validEnrichedDoc(t, enriched)
	if doc["cached"] != true {
		t.Errorf("expected cached doc to be returned, got %v", doc)
	}
}

func TestEnrichRolePolicy_ManagedCacheMiss_FetchSuccess(t *testing.T) {
	docJSON := `{"Version":"2012-10-17","Statement":[]}`
	encoded := url.PathEscape(docJSON)
	policyArn := "arn:aws:iam::123456789012:policy/fetched-policy"

	mock := &combinedIAMMock{
		getPolicyOut:    &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}},
		getPolicyVerOut: &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)}},
	}
	cache := &awsclient.PolicyDocumentCache{}
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtxWithCache(mock, cache)
	res := makeManagedRes("fetched-policy", policyArn)

	enriched, err := enricher(context.Background(), dctx, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc := validEnrichedDoc(t, enriched)
	if doc["Version"] != "2012-10-17" {
		t.Errorf("expected Version 2012-10-17, got %v", doc["Version"])
	}
	if cache.Get(awsclient.ManagedKey(policyArn)) == nil {
		t.Error("cache should be populated after managed fetch")
	}
}

func TestEnrichRolePolicy_ManagedCacheMiss_FetchError(t *testing.T) {
	policyArn := "arn:aws:iam::123456789012:policy/failing-policy"
	mock := &combinedIAMMock{getPolicyErr: errFake("GetPolicy: denied")}
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(mock)
	res := makeManagedRes("failing-policy", policyArn)

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error from GetPolicy failure, got nil")
	}
}

func TestEnrichRolePolicy_ManagedMissingPolicyArn(t *testing.T) {
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(&fakeIAMBase{})
	res := resource.Resource{
		Fields: map[string]string{},
		RawStruct: awsclient.RolePolicyRow{
			PolicyName: "no-arn-policy",
			PolicyArn:  "",
			PolicyType: "Managed",
		},
	}

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for missing policy ARN, got nil")
	}
	if !strings.Contains(err.Error(), "policy ARN") {
		t.Errorf("error should mention policy ARN, got: %v", err)
	}
}

func TestEnrichRolePolicy_ManagedGetPolicyVersionError(t *testing.T) {
	// Cache miss + GetPolicy succeeds but GetPolicyVersion returns an error.
	policyArn := "arn:aws:iam::123456789012:policy/ver-error-policy"
	mock := &combinedIAMMock{
		getPolicyOut:    &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}},
		getPolicyVerErr: errFake("GetPolicyVersion: throttled"),
	}
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(mock)
	res := makeManagedRes("ver-error-policy", policyArn)

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error from GetPolicyVersion failure, got nil")
	}
	if !strings.Contains(err.Error(), "GetPolicyVersion") {
		t.Errorf("error should mention GetPolicyVersion, got: %v", err)
	}
}

func TestEnrichRolePolicy_WrongRawStructType(t *testing.T) {
	enricher := rolePoliciesEnricher(t)
	dctx := makeRolePoliciesCtx(&fakeIAMBase{})
	res := resource.Resource{
		Fields:    map[string]string{},
		RawStruct: "not-a-RolePolicyRow",
	}

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
	if !strings.Contains(err.Error(), "RawStruct") {
		t.Errorf("error should mention RawStruct, got: %v", err)
	}
}

func TestEnrichRolePolicy_NonDetailEnrichmentCtxClients(t *testing.T) {
	// Pass a *ServiceClients directly — type assertion to *DetailEnrichmentCtx fails.
	enricher := rolePoliciesEnricher(t)
	clients := &awsclient.ServiceClients{IAM: &fakeIAMBase{}}
	res := makeInlineRes("my-role", "my-policy")

	_, err := enricher(context.Background(), clients, res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
	if !strings.Contains(err.Error(), "invalid detail-enrichment context") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEnrichRolePolicy_NilDetailEnrichmentCtx(t *testing.T) {
	// Typed nil — ok-cast to *DetailEnrichmentCtx succeeds but nil guard catches it.
	enricher := rolePoliciesEnricher(t)
	var dctx *awsclient.DetailEnrichmentCtx
	res := makeInlineRes("my-role", "my-policy")

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for nil *DetailEnrichmentCtx, got nil")
	}
	if !strings.Contains(err.Error(), "invalid detail-enrichment context") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEnrichRolePolicy_DetailEnrichmentCtx_NilClients(t *testing.T) {
	enricher := rolePoliciesEnricher(t)
	dctx := &awsclient.DetailEnrichmentCtx{
		Clients:    nil,
		PolicyDocs: &awsclient.PolicyDocumentCache{},
	}
	res := makeInlineRes("my-role", "my-policy")

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
	if !strings.Contains(err.Error(), "invalid detail-enrichment context") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEnrichRolePolicy_DetailEnrichmentCtx_NilPolicyDocs(t *testing.T) {
	enricher := rolePoliciesEnricher(t)
	dctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: &fakeIAMBase{}},
		PolicyDocs: nil,
	}
	res := makeInlineRes("my-role", "my-policy")

	_, err := enricher(context.Background(), dctx, res)
	if err == nil {
		t.Fatal("expected error for nil PolicyDocs, got nil")
	}
	if !strings.Contains(err.Error(), "invalid detail-enrichment context") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// decodePolicyDocument additional tests (exercised via FetchManagedPolicyDocument)
// ---------------------------------------------------------------------------

func TestDecodePolicyDocument_QueryUnescapeFallback(t *testing.T) {
	// PathUnescape("%7B%22a%22%3A+1%7D") → {"a":+1}  — INVALID JSON (+ before number)
	// QueryUnescape("%7B%22a%22%3A+1%7D") → {"a": 1} — VALID JSON   (+ → space)
	// The decodePolicyDocument fallback branch should parse {"a":1} successfully.
	encoded := "%7B%22a%22%3A+1%7D"

	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)},
		},
	}

	doc, err := awsclient.FetchManagedPolicyDocument(
		context.Background(), getPolicyMock, getVersionMock,
		"arn:aws:iam::123456789012:policy/fallback-test",
	)
	if err != nil {
		t.Fatalf("expected QueryUnescape fallback to succeed, got error: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", doc)
	}
	val, exists := m["a"]
	if !exists {
		t.Fatal("expected key 'a' in decoded document")
	}
	if val != float64(1) {
		t.Errorf("expected float64(1) for key 'a', got %v (%T)", val, val)
	}
}

func TestDecodePolicyDocument_PathUnescapeError(t *testing.T) {
	// %ZZ is not valid percent-encoding — url.PathUnescape returns an error.
	encoded := "%ZZ"

	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)},
		},
	}

	_, err := awsclient.FetchManagedPolicyDocument(
		context.Background(), getPolicyMock, getVersionMock,
		"arn:aws:iam::123456789012:policy/bad-encoding-test",
	)
	if err == nil {
		t.Fatal("expected error for malformed percent-encoding, got nil")
	}
	if !strings.Contains(err.Error(), "URL decode") {
		t.Errorf("expected 'URL decode' in error, got: %v", err)
	}
}

func TestDecodePolicyDocument_BothFail_ReturnsJSONParseError(t *testing.T) {
	// "malformed" contains no percent-encoding; both PathUnescape and QueryUnescape
	// return it unchanged, and json.Unmarshal fails for both.
	encoded := "malformed"

	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(encoded)},
		},
	}

	_, err := awsclient.FetchManagedPolicyDocument(
		context.Background(), getPolicyMock, getVersionMock,
		"arn:aws:iam::123456789012:policy/both-fail-test",
	)
	if err == nil {
		t.Fatal("expected JSON parse error when both decode strategies fail, got nil")
	}
	if !strings.Contains(err.Error(), "JSON parse") {
		t.Errorf("expected 'JSON parse' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FetchManagedPolicyDocument nil-guard tests
// ---------------------------------------------------------------------------

func TestFetchManagedPolicyDocument_NilPolicy(t *testing.T) {
	getPolicyMock := &enrichGetPolicyClient{output: &iam.GetPolicyOutput{Policy: nil}}
	getVersionMock := &enrichGetPolicyVersionClient{output: &iam.GetPolicyVersionOutput{}}

	_, err := awsclient.FetchManagedPolicyDocument(
		context.Background(), getPolicyMock, getVersionMock,
		"arn:aws:iam::123456789012:policy/nil-policy-test",
	)
	if err == nil {
		t.Fatal("expected error for nil Policy, got nil")
	}
	if !strings.Contains(err.Error(), "nil policy or version ID") {
		t.Errorf("expected 'nil policy or version ID' in error, got: %v", err)
	}
}

func TestFetchManagedPolicyDocument_NilDefaultVersionId(t *testing.T) {
	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: nil}},
	}
	getVersionMock := &enrichGetPolicyVersionClient{output: &iam.GetPolicyVersionOutput{}}

	_, err := awsclient.FetchManagedPolicyDocument(
		context.Background(), getPolicyMock, getVersionMock,
		"arn:aws:iam::123456789012:policy/nil-version-id-test",
	)
	if err == nil {
		t.Fatal("expected error for nil DefaultVersionId, got nil")
	}
	if !strings.Contains(err.Error(), "nil policy or version ID") {
		t.Errorf("expected 'nil policy or version ID' in error, got: %v", err)
	}
}

func TestFetchManagedPolicyDocument_NilPolicyVersionDocument(t *testing.T) {
	getPolicyMock := &enrichGetPolicyClient{
		output: &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}},
	}
	getVersionMock := &enrichGetPolicyVersionClient{
		output: &iam.GetPolicyVersionOutput{
			PolicyVersion: &iamtypes.PolicyVersion{Document: nil},
		},
	}

	_, err := awsclient.FetchManagedPolicyDocument(
		context.Background(), getPolicyMock, getVersionMock,
		"arn:aws:iam::123456789012:policy/nil-doc-test",
	)
	if err == nil {
		t.Fatal("expected error for nil PolicyVersion.Document, got nil")
	}
	if !strings.Contains(err.Error(), "nil document") {
		t.Errorf("expected 'nil document' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FetchInlinePolicyDocument nil-guard test
// ---------------------------------------------------------------------------

func TestFetchInlinePolicyDocument_NilDocument(t *testing.T) {
	mock := &enrichGetRolePolicyClient{
		output: &iam.GetRolePolicyOutput{PolicyDocument: nil},
	}

	_, err := awsclient.FetchInlinePolicyDocument(context.Background(), mock, "my-role", "nil-doc-policy")
	if err == nil {
		t.Fatal("expected error for nil PolicyDocument, got nil")
	}
	if !strings.Contains(err.Error(), "nil document") {
		t.Errorf("expected 'nil document' in error, got: %v", err)
	}
}
