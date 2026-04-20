package unit

// aws_iam_policies_enrich_test.go — coverage for enrichPolicy in iam_policies_enrich.go.
//
// Covers:
//   - wrong clients type → error "invalid detail-enrichment context"
//   - nil DetailEnrichmentCtx → error
//   - nil Clients inside ctx → error
//   - nil PolicyDocs inside ctx → error
//   - wrong RawStruct type → error
//   - nil policy ARN → error
//   - cache hit → returns enriched without calling API
//   - cache miss → calls FetchManagedPolicyDocument, stores result
//   - PolicyEnriched re-enrichment path → accepts PolicyEnriched as input
//   - API error propagated to caller

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
// enrichPolicyIAM — a full IAMAPI fake with controllable GetPolicy/GetPolicyVersion
// ---------------------------------------------------------------------------

type enrichPolicyIAM struct {
	getPolicyFn        func(*iam.GetPolicyInput) (*iam.GetPolicyOutput, error)
	getPolicyVersionFn func(*iam.GetPolicyVersionInput) (*iam.GetPolicyVersionOutput, error)
}

// GetPolicy delegates to getPolicyFn if set, otherwise returns empty output.
func (f *enrichPolicyIAM) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	if f.getPolicyFn != nil {
		return f.getPolicyFn(in)
	}
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")}}, nil
}

// GetPolicyVersion delegates to getPolicyVersionFn if set.
func (f *enrichPolicyIAM) GetPolicyVersion(_ context.Context, in *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	if f.getPolicyVersionFn != nil {
		return f.getPolicyVersionFn(in)
	}
	emptyDoc := url.PathEscape(`{"Version":"2012-10-17","Statement":[]}`)
	return &iam.GetPolicyVersionOutput{
		PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(emptyDoc)},
	}, nil
}

// --- Stubs for the rest of IAMAPI ---

func (f *enrichPolicyIAM) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{}, nil
}
func (f *enrichPolicyIAM) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{}, nil
}
func (f *enrichPolicyIAM) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{}, nil
}
func (f *enrichPolicyIAM) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}
func (f *enrichPolicyIAM) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{}, nil
}
func (f *enrichPolicyIAM) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}
func (f *enrichPolicyIAM) ListAttachedUserPolicies(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}
func (f *enrichPolicyIAM) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}
func (f *enrichPolicyIAM) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return &iam.ListGroupsForUserOutput{}, nil
}
func (f *enrichPolicyIAM) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	return &iam.ListEntitiesForPolicyOutput{}, nil
}
func (f *enrichPolicyIAM) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{}, nil
}
func (f *enrichPolicyIAM) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	return &iam.GetGroupOutput{Group: &iamtypes.Group{}}, nil
}
func (f *enrichPolicyIAM) ListGroupPolicies(_ context.Context, _ *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{}, nil
}
func (f *enrichPolicyIAM) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return &iam.GetRolePolicyOutput{}, nil
}
func (f *enrichPolicyIAM) GetLoginProfile(_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	return &iam.GetLoginProfileOutput{}, nil
}
func (f *enrichPolicyIAM) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}
func (f *enrichPolicyIAM) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{}, nil
}
func (f *enrichPolicyIAM) GetInstanceProfile(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	return &iam.GetInstanceProfileOutput{InstanceProfile: &iamtypes.InstanceProfile{}}, nil
}

// Compile-time check: enrichPolicyIAM satisfies IAMAPI.
var _ awsclient.IAMAPI = (*enrichPolicyIAM)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// enrichPolicyEnricher returns the registered detail enricher for "policy".
// Fails immediately if not found.
func enrichPolicyEnricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("policy")
	if e == nil {
		t.Fatal("policy detail enricher not registered")
	}
	return e
}

// makePolicyCtx returns a *DetailEnrichmentCtx with a fresh PolicyDocumentCache.
func makePolicyCtx(iamClient awsclient.IAMAPI) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: iamClient},
		PolicyDocs: &awsclient.PolicyDocumentCache{},
	}
}

// makePolicyRes returns a Resource with an iamtypes.Policy RawStruct.
func makePolicyRes(arn string) resource.Resource {
	return resource.Resource{
		ID: arn,
		RawStruct: iamtypes.Policy{
			Arn:              aws.String(arn),
			PolicyName:       aws.String("test-policy"),
			DefaultVersionId: aws.String("v1"),
		},
	}
}

// buildVersionDoc returns a URL-encoded policy document JSON.
func buildVersionDoc(docJSON string) string {
	return url.PathEscape(docJSON)
}

// ---------------------------------------------------------------------------
// Tests: invalid clients
// ---------------------------------------------------------------------------

func TestEnrichPolicy_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes("arn:aws:iam::123456789012:policy/test-policy")

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichPolicy_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes("arn:aws:iam::123456789012:policy/test-policy")

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichPolicy_NilClients_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes("arn:aws:iam::123456789012:policy/test-policy")
	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    nil,
		PolicyDocs: &awsclient.PolicyDocumentCache{},
	}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

func TestEnrichPolicy_NilPolicyDocs_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes("arn:aws:iam::123456789012:policy/test-policy")
	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: &enrichPolicyIAM{}},
		PolicyDocs: nil,
	}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil PolicyDocs, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: bad RawStruct
// ---------------------------------------------------------------------------

func TestEnrichPolicy_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := resource.Resource{
		ID:        "arn:aws:iam::123456789012:policy/test-policy",
		RawStruct: "not-a-policy",
	}

	_, err := enricher(context.Background(), makePolicyCtx(&enrichPolicyIAM{}), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichPolicy_NilPolicyARN_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := resource.Resource{
		ID:        "arn:aws:iam::123456789012:policy/test-policy",
		RawStruct: iamtypes.Policy{Arn: nil},
	}

	_, err := enricher(context.Background(), makePolicyCtx(&enrichPolicyIAM{}), res)
	if err == nil {
		t.Fatal("expected error for nil policy ARN, got nil")
	}
}

func TestEnrichPolicy_EmptyPolicyARN_ReturnsError(t *testing.T) {
	enricher := enrichPolicyEnricher(t)
	res := resource.Resource{
		ID:        "",
		RawStruct: iamtypes.Policy{Arn: aws.String("")},
	}

	_, err := enricher(context.Background(), makePolicyCtx(&enrichPolicyIAM{}), res)
	if err == nil {
		t.Fatal("expected error for empty policy ARN, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: cache hit
// ---------------------------------------------------------------------------

func TestEnrichPolicy_CacheHit_ReturnsEnrichedWithoutAPICall(t *testing.T) {
	const policyArn = "arn:aws:iam::123456789012:policy/cached-policy"

	// Track if GetPolicyVersion is ever called.
	apiCalled := false
	iamFake := &enrichPolicyIAM{
		getPolicyVersionFn: func(_ *iam.GetPolicyVersionInput) (*iam.GetPolicyVersionOutput, error) {
			apiCalled = true
			return nil, errFake("should not be called")
		},
	}

	// Pre-populate the cache with a document.
	policyDocs := &awsclient.PolicyDocumentCache{}
	cachedDoc := map[string]any{"Version": "2012-10-17", "Statement": []any{}}
	policyDocs.Set(awsclient.ManagedKey(policyArn), cachedDoc)

	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: iamFake},
		PolicyDocs: policyDocs,
	}

	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes(policyArn)

	got, err := enricher(context.Background(), ctx, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalled {
		t.Error("API was called despite cache hit — cache short-circuit not working")
	}

	enriched, ok := got.RawStruct.(awsclient.PolicyEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want PolicyEnriched", got.RawStruct)
	}
	if enriched.Arn == nil || *enriched.Arn != policyArn {
		t.Errorf("enriched.Arn = %v, want %q", enriched.Arn, policyArn)
	}
	if enriched.Document == nil {
		t.Error("enriched.Document must not be nil on cache hit")
	}
}

// ---------------------------------------------------------------------------
// Tests: cache miss
// ---------------------------------------------------------------------------

func TestEnrichPolicy_CacheMiss_CallsAPIAndStoresResult(t *testing.T) {
	const policyArn = "arn:aws:iam::123456789012:policy/new-policy"
	docJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"*"}]}`

	apiCallCount := 0
	iamFake := &enrichPolicyIAM{
		getPolicyFn: func(_ *iam.GetPolicyInput) (*iam.GetPolicyOutput, error) {
			return &iam.GetPolicyOutput{
				Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v2")},
			}, nil
		},
		getPolicyVersionFn: func(_ *iam.GetPolicyVersionInput) (*iam.GetPolicyVersionOutput, error) {
			apiCallCount++
			return &iam.GetPolicyVersionOutput{
				PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(buildVersionDoc(docJSON))},
			}, nil
		},
	}

	policyDocs := &awsclient.PolicyDocumentCache{}
	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: iamFake},
		PolicyDocs: policyDocs,
	}

	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes(policyArn)

	got, err := enricher(context.Background(), ctx, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCallCount != 1 {
		t.Errorf("GetPolicyVersion called %d times, want 1", apiCallCount)
	}

	enriched, ok := got.RawStruct.(awsclient.PolicyEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want PolicyEnriched", got.RawStruct)
	}
	if enriched.Arn == nil || *enriched.Arn != policyArn {
		t.Errorf("enriched.Arn = %v, want %q", enriched.Arn, policyArn)
	}
	if enriched.Document == nil {
		t.Error("enriched.Document must not be nil after API call")
	}

	// Verify the result was stored in cache.
	cached := policyDocs.Get(awsclient.ManagedKey(policyArn))
	if cached == nil {
		t.Error("document should be stored in cache after fetch, but cache.Get returned nil")
	}
}

func TestEnrichPolicy_CacheMiss_SecondCallUsesCacheNotAPI(t *testing.T) {
	const policyArn = "arn:aws:iam::123456789012:policy/two-call-policy"
	docJSON := `{"Version":"2012-10-17","Statement":[]}`

	apiCallCount := 0
	iamFake := &enrichPolicyIAM{
		getPolicyFn: func(_ *iam.GetPolicyInput) (*iam.GetPolicyOutput, error) {
			return &iam.GetPolicyOutput{
				Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")},
			}, nil
		},
		getPolicyVersionFn: func(_ *iam.GetPolicyVersionInput) (*iam.GetPolicyVersionOutput, error) {
			apiCallCount++
			return &iam.GetPolicyVersionOutput{
				PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(buildVersionDoc(docJSON))},
			}, nil
		},
	}

	policyDocs := &awsclient.PolicyDocumentCache{}
	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{IAM: iamFake},
		PolicyDocs: policyDocs,
	}
	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes(policyArn)

	// First call — hits API.
	if _, err := enricher(context.Background(), ctx, res); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	// Second call — should use cache.
	if _, err := enricher(context.Background(), ctx, res); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if apiCallCount != 1 {
		t.Errorf("GetPolicyVersion called %d times across two enrichments, want 1 (second should use cache)", apiCallCount)
	}
}

// ---------------------------------------------------------------------------
// Tests: PolicyEnriched re-enrichment path
// ---------------------------------------------------------------------------

func TestEnrichPolicy_PolicyEnrichedRawStruct_Accepted(t *testing.T) {
	const policyArn = "arn:aws:iam::123456789012:policy/already-enriched"
	docJSON := `{"Version":"2012-10-17","Statement":[]}`

	iamFake := &enrichPolicyIAM{
		getPolicyFn: func(_ *iam.GetPolicyInput) (*iam.GetPolicyOutput, error) {
			return &iam.GetPolicyOutput{
				Policy: &iamtypes.Policy{DefaultVersionId: aws.String("v1")},
			}, nil
		},
		getPolicyVersionFn: func(_ *iam.GetPolicyVersionInput) (*iam.GetPolicyVersionOutput, error) {
			return &iam.GetPolicyVersionOutput{
				PolicyVersion: &iamtypes.PolicyVersion{Document: aws.String(buildVersionDoc(docJSON))},
			}, nil
		},
	}

	ctx := makePolicyCtx(iamFake)
	enricher := enrichPolicyEnricher(t)

	// Input is already a PolicyEnriched (re-enrichment scenario).
	res := resource.Resource{
		ID: policyArn,
		RawStruct: awsclient.PolicyEnriched{
			Policy: iamtypes.Policy{
				Arn:              aws.String(policyArn),
				PolicyName:       aws.String("already-enriched"),
				DefaultVersionId: aws.String("v1"),
			},
		},
	}

	got, err := enricher(context.Background(), ctx, res)
	if err != nil {
		t.Fatalf("unexpected error on PolicyEnriched re-enrichment: %v", err)
	}

	enriched, ok := got.RawStruct.(awsclient.PolicyEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want PolicyEnriched", got.RawStruct)
	}
	if enriched.Arn == nil || *enriched.Arn != policyArn {
		t.Errorf("enriched.Arn = %v, want %q", enriched.Arn, policyArn)
	}
}

// ---------------------------------------------------------------------------
// Tests: API error propagation
// ---------------------------------------------------------------------------

func TestEnrichPolicy_APIError_Propagated(t *testing.T) {
	const policyArn = "arn:aws:iam::123456789012:policy/error-policy"

	iamFake := &enrichPolicyIAM{
		getPolicyFn: func(_ *iam.GetPolicyInput) (*iam.GetPolicyOutput, error) {
			return nil, errFake("GetPolicy: access denied")
		},
	}

	ctx := makePolicyCtx(iamFake)
	enricher := enrichPolicyEnricher(t)
	res := makePolicyRes(policyArn)

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: PolicyDocumentCache registry
// ---------------------------------------------------------------------------

func TestDetailEnricherRegistry_Policy_IsNonNil(t *testing.T) {
	e := resource.GetDetailEnricher("policy")
	if e == nil {
		t.Fatal("policy detail enricher must be registered and non-nil")
	}
}
