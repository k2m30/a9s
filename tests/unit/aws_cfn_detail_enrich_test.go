package unit

// aws_cfn_detail_enrich_test.go — coverage for enrichCfn (core/aws/cfn_detail_enrichment.go),
// the on-demand detail enricher registered for the "cfn" resource type (#261).
//
// Covers:
//   - wrong clients type / nil DetailEnrichmentCtx / nil Clients / nil DetailDocs → error
//   - wrong RawStruct type → error
//   - missing stack identifier (both StackId and StackName empty) → error
//   - StackId preferred over StackName when both are present
//   - cache miss → calls GetTemplate; JSON template body parsed to structured
//     data; YAML template body kept raw
//   - cache key is version-aware (LastUpdatedTime, falling back to
//     CreationTime) — verified behaviorally (call counts + returned
//     TemplateBody), not by asserting the literal key string: a stack update
//     within the same session changes the row's LastUpdatedTime, so the next
//     open naturally misses the pre-update template instead of serving it
//     stale; an unchanged version hits; LastUpdatedTime nil falls back to
//     CreationTime without colliding across two stacks that only differ by
//     CreationTime
//   - StackEnriched re-enrichment path accepted as RawStruct
//   - API error propagated
//   - registry sanity: GetDetailEnricher("cfn") non-nil

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// enrichCfnFake — narrow CFNAPI + CFNGetTemplateAPI fake with a call counter
// ---------------------------------------------------------------------------

type enrichCfnFake struct {
	getTemplateFn    func(*cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error)
	getTemplateCalls int
}

func (f *enrichCfnFake) GetTemplate(_ context.Context, in *cloudformation.GetTemplateInput, _ ...func(*cloudformation.Options)) (*cloudformation.GetTemplateOutput, error) {
	f.getTemplateCalls++
	if f.getTemplateFn != nil {
		return f.getTemplateFn(in)
	}
	return &cloudformation.GetTemplateOutput{}, nil
}

func (f *enrichCfnFake) DescribeStacks(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return &cloudformation.DescribeStacksOutput{}, nil
}
func (f *enrichCfnFake) DescribeStackEvents(_ context.Context, _ *cloudformation.DescribeStackEventsInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	return &cloudformation.DescribeStackEventsOutput{}, nil
}
func (f *enrichCfnFake) ListStackResources(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	return &cloudformation.ListStackResourcesOutput{}, nil
}

var _ awsclient.CFNAPI = (*enrichCfnFake)(nil)
var _ awsclient.CFNGetTemplateAPI = (*enrichCfnFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func cfnEnricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("cfn")
	if e == nil {
		t.Fatal("cfn detail enricher not registered")
	}
	return e
}

func makeCfnCtx(client awsclient.CFNAPI, docs *awsclient.DetailDocCache) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{CloudFormation: client},
		DetailDocs: docs,
	}
}

const cfnTestStackID = "arn:aws:cloudformation:us-east-1:123456789012:stack/prod-vpc-network/8f2c9b40-abc1-11ee-9c1a-0abcdef12345"
const cfnTestStackName = "prod-vpc-network"

const cfnTestTemplateJSON = `{"AWSTemplateFormatVersion":"2010-09-09","Resources":{"VPC":{"Type":"AWS::EC2::VPC","Properties":{"CidrBlock":"10.0.0.0/16"}}}}`

const cfnTestTemplateYAML = "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  VPC:\n    Type: AWS::EC2::VPC\n    Properties:\n      CidrBlock: 10.0.0.0/16\n"

func makeCfnStack(stackID, stackName string) cfntypes.Stack {
	var idPtr, namePtr *string
	if stackID != "" {
		idPtr = aws.String(stackID)
	}
	if stackName != "" {
		namePtr = aws.String(stackName)
	}
	return cfntypes.Stack{
		StackId:      idPtr,
		StackName:    namePtr,
		StackStatus:  cfntypes.StackStatusUpdateComplete,
		CreationTime: aws.Time(time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)),
	}
}

func makeCfnRes(stackID, stackName string) resource.Resource {
	return resource.Resource{ID: stackID, RawStruct: makeCfnStack(stackID, stackName)}
}

// ---------------------------------------------------------------------------
// Tests: invalid context
// ---------------------------------------------------------------------------

func TestEnrichCfn_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichCfn_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichCfn_NilClients_ReturnsError(t *testing.T) {
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)
	ctx := &awsclient.DetailEnrichmentCtx{Clients: nil, DetailDocs: &awsclient.DetailDocCache{}}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

func TestEnrichCfn_NilDetailDocs_ReturnsError(t *testing.T) {
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)
	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{CloudFormation: &enrichCfnFake{}},
		DetailDocs: nil,
	}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil DetailDocs, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: bad RawStruct / missing identifier
// ---------------------------------------------------------------------------

func TestEnrichCfn_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := cfnEnricher(t)
	res := resource.Resource{ID: cfnTestStackID, RawStruct: "not-a-stack"}

	_, err := enricher(context.Background(), makeCfnCtx(&enrichCfnFake{}, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichCfn_NoIdentifier_ReturnsError(t *testing.T) {
	enricher := cfnEnricher(t)
	res := makeCfnRes("", "")

	_, err := enricher(context.Background(), makeCfnCtx(&enrichCfnFake{}, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error for stack with no StackId or StackName, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: cache miss, JSON vs YAML template body
// ---------------------------------------------------------------------------

func TestEnrichCfn_CacheMiss_JSONTemplate_Parsed(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTestTemplateJSON)}, nil
		},
	}

	cache := &awsclient.DetailDocCache{}
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	got, err := enricher(context.Background(), makeCfnCtx(fake, cache), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.getTemplateCalls != 1 {
		t.Errorf("GetTemplate called %d times, want 1", fake.getTemplateCalls)
	}

	enriched := got.RawStruct.(awsclient.StackEnriched)
	body, ok := enriched.TemplateBody.(map[string]any)
	if !ok {
		t.Fatalf("enriched.TemplateBody = %T, want map[string]any (parsed JSON)", enriched.TemplateBody)
	}
	if body["AWSTemplateFormatVersion"] != "2010-09-09" {
		t.Errorf("enriched.TemplateBody[AWSTemplateFormatVersion] = %v, want 2010-09-09", body["AWSTemplateFormatVersion"])
	}
}

// ---------------------------------------------------------------------------
// Tests: cache key is version-aware (LastUpdatedTime, falling back to
// CreationTime) — exercised behaviorally (call counts + returned
// TemplateBody), never by asserting the literal key string, so these don't
// couple to enrichCfn's internal key format.
// ---------------------------------------------------------------------------

const cfnTemplateV1 = "template-v1-body"
const cfnTemplateV2 = "template-v2-body"

var cfnTestCreationTime = time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

func makeCfnStackVersioned(stackID string, creationTime, lastUpdatedTime *time.Time) cfntypes.Stack {
	return cfntypes.Stack{
		StackId:         aws.String(stackID),
		StackName:       aws.String(cfnTestStackName),
		StackStatus:     cfntypes.StackStatusUpdateComplete,
		CreationTime:    creationTime,
		LastUpdatedTime: lastUpdatedTime,
	}
}

func TestEnrichCfn_CacheKey_LastUpdatedTimeChanges_MissesAndRefetches(t *testing.T) {
	var calls int
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			calls++
			body := cfnTemplateV1
			if calls > 1 {
				body = cfnTemplateV2
			}
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(body)}, nil
		},
	}
	cache := &awsclient.DetailDocCache{}
	ctx := makeCfnCtx(fake, cache)
	enricher := cfnEnricher(t)

	before := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), aws.Time(cfnTestCreationTime))
	got1, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: before})
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if body := got1.RawStruct.(awsclient.StackEnriched).TemplateBody; body != cfnTemplateV1 {
		t.Fatalf("sanity check failed: first call TemplateBody = %v, want %q", body, cfnTemplateV1)
	}

	// Simulates a stack UPDATE_COMPLETE observed in-session: same identifier,
	// a newer LastUpdatedTime on the refreshed list row.
	after := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), aws.Time(cfnTestCreationTime.Add(24*time.Hour)))
	got2, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: after})
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if calls != 2 {
		t.Fatalf("GetTemplate called %d times, want 2 (a changed LastUpdatedTime must miss the cache, not serve the pre-update template)", calls)
	}
	if body := got2.RawStruct.(awsclient.StackEnriched).TemplateBody; body != cfnTemplateV2 {
		t.Errorf("second call TemplateBody = %v, want %q (fresh fetch, not the stale cached %q)", body, cfnTemplateV2, cfnTemplateV1)
	}
}

func TestEnrichCfn_CacheKey_SameLastUpdatedTime_Hits(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTemplateV1)}, nil
		},
	}
	cache := &awsclient.DetailDocCache{}
	ctx := makeCfnCtx(fake, cache)
	enricher := cfnEnricher(t)
	lastUpdated := cfnTestCreationTime.Add(time.Hour)

	first := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), aws.Time(lastUpdated))
	if _, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: first}); err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// A distinct row value with an identical LastUpdatedTime (e.g. a list
	// refresh that observed no change) must still hit the cache.
	second := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), aws.Time(lastUpdated))
	if _, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: second}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if fake.getTemplateCalls != 1 {
		t.Errorf("GetTemplate called %d times across two enrichments with an unchanged LastUpdatedTime, want 1 (cache hit)", fake.getTemplateCalls)
	}
}

func TestEnrichCfn_CacheKey_NilLastUpdatedTime_FallsBackToCreationTime_NoCollision(t *testing.T) {
	var calls int
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			calls++
			body := cfnTemplateV1
			if calls > 1 {
				body = cfnTemplateV2
			}
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(body)}, nil
		},
	}
	cache := &awsclient.DetailDocCache{}
	ctx := makeCfnCtx(fake, cache)
	enricher := cfnEnricher(t)

	// Same StackId, LastUpdatedTime nil on both — isolates the fallback: if
	// CreationTime were dropped from the key instead of folded in, these two
	// would wrongly share a cache entry despite differing CreationTime.
	first := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), nil)
	if _, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: first}); err != nil {
		t.Fatalf("first call error: %v", err)
	}

	second := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime.Add(24*time.Hour)), nil)
	got2, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: second})
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if calls != 2 {
		t.Fatalf("GetTemplate called %d times, want 2 (a different CreationTime with nil LastUpdatedTime must not collide)", calls)
	}
	if body := got2.RawStruct.(awsclient.StackEnriched).TemplateBody; body != cfnTemplateV2 {
		t.Errorf("second call TemplateBody = %v, want %q (own fetch, not the first stack's cached %q)", body, cfnTemplateV2, cfnTemplateV1)
	}
}

// ---------------------------------------------------------------------------
// Tests: DetailEnrichmentCtx.SkipCache (explicit detail refresh) bypasses the
// cache READ only — the fetch result is still WRITTEN, so a subsequent
// non-refresh open benefits from the freshly-refreshed entry
// (core/aws/detail_enrich_engine.go).
// ---------------------------------------------------------------------------

func TestEnrichCfn_SkipCacheTrue_BypassesReadButStillWrites(t *testing.T) {
	var calls int
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			calls++
			body := cfnTemplateV1
			if calls > 1 {
				body = cfnTemplateV2
			}
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(body)}, nil
		},
	}
	cache := &awsclient.DetailDocCache{}
	stack := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), aws.Time(cfnTestCreationTime))

	// Pre-populate the cache under the current versioned key via a normal
	// (non-refresh) enrichment.
	normalCtx := makeCfnCtx(fake, cache)
	if _, err := cfnEnricher(t)(context.Background(), normalCtx, resource.Resource{ID: cfnTestStackID, RawStruct: stack}); err != nil {
		t.Fatalf("seed call error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("sanity check failed: seed call GetTemplate calls = %d, want 1", calls)
	}

	// An explicit refresh (SkipCache: true) must still call the API despite
	// the warm cache entry, and attach the fresh payload.
	refreshCtx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{CloudFormation: fake},
		DetailDocs: cache,
		SkipCache:  true,
	}
	got, err := cfnEnricher(t)(context.Background(), refreshCtx, resource.Resource{ID: cfnTestStackID, RawStruct: stack})
	if err != nil {
		t.Fatalf("refresh call error: %v", err)
	}
	if calls != 2 {
		t.Errorf("GetTemplate called %d times, want 2 (SkipCache:true must bypass the cache read)", calls)
	}
	if body := got.RawStruct.(awsclient.StackEnriched).TemplateBody; body != cfnTemplateV2 {
		t.Errorf("refresh call TemplateBody = %v, want %q (fresh fetch, not the cached %q)", body, cfnTemplateV2, cfnTemplateV1)
	}

	// The refreshed entry must have overwritten the cache: a subsequent
	// non-refresh open sees the fresh payload, not the original seed.
	got3, err := cfnEnricher(t)(context.Background(), normalCtx, resource.Resource{ID: cfnTestStackID, RawStruct: stack})
	if err != nil {
		t.Fatalf("post-refresh call error: %v", err)
	}
	if calls != 2 {
		t.Errorf("GetTemplate called %d times after the post-refresh open, want 2 (the refresh must have written the cache)", calls)
	}
	if body := got3.RawStruct.(awsclient.StackEnriched).TemplateBody; body != cfnTemplateV2 {
		t.Errorf("post-refresh call TemplateBody = %v, want %q (the refresh's write, not the original seed %q)", body, cfnTemplateV2, cfnTemplateV1)
	}
}

func TestEnrichCfn_SkipCacheFalse_KeepsHitBehavior(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTemplateV1)}, nil
		},
	}
	cache := &awsclient.DetailDocCache{}
	stack := makeCfnStackVersioned(cfnTestStackID, aws.Time(cfnTestCreationTime), aws.Time(cfnTestCreationTime))

	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{CloudFormation: fake},
		DetailDocs: cache,
		SkipCache:  false,
	}
	enricher := cfnEnricher(t)
	if _, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: stack}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := enricher(context.Background(), ctx, resource.Resource{ID: cfnTestStackID, RawStruct: stack}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if fake.getTemplateCalls != 1 {
		t.Errorf("GetTemplate called %d times across two SkipCache:false enrichments, want 1 (cache hit behavior unchanged)", fake.getTemplateCalls)
	}
}

func TestEnrichCfn_YAMLTemplate_KeptAsRawString(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTestTemplateYAML)}, nil
		},
	}

	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	got, err := enricher(context.Background(), makeCfnCtx(fake, &awsclient.DetailDocCache{}), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.StackEnriched)
	if enriched.TemplateBody != cfnTestTemplateYAML {
		t.Errorf("enriched.TemplateBody = %v, want raw YAML string", enriched.TemplateBody)
	}
}

// ---------------------------------------------------------------------------
// Tests: StackId preferred over StackName; second call uses cache
// ---------------------------------------------------------------------------

func TestEnrichCfn_StackIdPreferredOverStackName(t *testing.T) {
	var seenStackName *string
	fake := &enrichCfnFake{
		getTemplateFn: func(in *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			seenStackName = in.StackName
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTestTemplateJSON)}, nil
		},
	}

	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	if _, err := enricher(context.Background(), makeCfnCtx(fake, &awsclient.DetailDocCache{}), res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenStackName == nil || *seenStackName != cfnTestStackID {
		t.Errorf("GetTemplateInput.StackName = %v, want the StackId %q (StackId takes precedence)", seenStackName, cfnTestStackID)
	}
}

func TestEnrichCfn_FallsBackToStackName_WhenStackIdEmpty(t *testing.T) {
	var seenStackName *string
	fake := &enrichCfnFake{
		getTemplateFn: func(in *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			seenStackName = in.StackName
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTestTemplateJSON)}, nil
		},
	}

	enricher := cfnEnricher(t)
	res := makeCfnRes("", cfnTestStackName)

	if _, err := enricher(context.Background(), makeCfnCtx(fake, &awsclient.DetailDocCache{}), res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenStackName == nil || *seenStackName != cfnTestStackName {
		t.Errorf("GetTemplateInput.StackName = %v, want fallback StackName %q", seenStackName, cfnTestStackName)
	}
}

func TestEnrichCfn_SecondCall_UsesCacheNotAPI(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTestTemplateJSON)}, nil
		},
	}
	cache := &awsclient.DetailDocCache{}
	ctx := makeCfnCtx(fake, cache)
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	if _, err := enricher(context.Background(), ctx, res); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := enricher(context.Background(), ctx, res); err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if fake.getTemplateCalls != 1 {
		t.Errorf("GetTemplate called %d times across two enrichments, want 1 (second should use cache)", fake.getTemplateCalls)
	}
}

// ---------------------------------------------------------------------------
// Tests: re-enrichment path
// ---------------------------------------------------------------------------

func TestEnrichCfn_StackEnrichedRawStruct_Accepted(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return &cloudformation.GetTemplateOutput{TemplateBody: aws.String(cfnTestTemplateJSON)}, nil
		},
	}
	enricher := cfnEnricher(t)

	res := resource.Resource{
		ID: cfnTestStackID,
		RawStruct: awsclient.StackEnriched{
			Stack:        makeCfnStack(cfnTestStackID, cfnTestStackName),
			TemplateBody: "stale-previous-body",
		},
	}

	got, err := enricher(context.Background(), makeCfnCtx(fake, &awsclient.DetailDocCache{}), res)
	if err != nil {
		t.Fatalf("unexpected error on StackEnriched re-enrichment: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.StackEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want StackEnriched", got.RawStruct)
	}
	if enriched.StackId == nil || *enriched.StackId != cfnTestStackID {
		t.Errorf("enriched.StackId = %v, want %q", enriched.StackId, cfnTestStackID)
	}
}

// ---------------------------------------------------------------------------
// Tests: API error propagation
// ---------------------------------------------------------------------------

func TestEnrichCfn_APIError_Propagated(t *testing.T) {
	fake := &enrichCfnFake{
		getTemplateFn: func(_ *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error) {
			return nil, errFake("GetTemplate: access denied")
		},
	}
	enricher := cfnEnricher(t)
	res := makeCfnRes(cfnTestStackID, cfnTestStackName)

	_, err := enricher(context.Background(), makeCfnCtx(fake, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: registry sanity
// ---------------------------------------------------------------------------

func TestDetailEnricherRegistry_Cfn_IsNonNil(t *testing.T) {
	e := resource.GetDetailEnricher("cfn")
	if e == nil {
		t.Fatal("cfn detail enricher must be registered and non-nil")
	}
}
