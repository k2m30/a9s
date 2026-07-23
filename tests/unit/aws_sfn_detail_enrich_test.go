package unit

// aws_sfn_detail_enrich_test.go — coverage for enrichSfn (core/aws/sfn_detail_enrichment.go),
// the on-demand detail enricher registered for the "sfn" resource type (#261).
//
// Covers:
//   - wrong clients type / nil DetailEnrichmentCtx / nil Clients → error
//   - nil DetailDocs no longer errors (uncached, mirrors enrichLambda's contract)
//   - wrong RawStruct type → error
//   - nil/empty state-machine ARN → error
//   - DescribeStateMachine result parsed correctly, ASL definition parsed to structured data
//   - unparsable definition falls back to the raw string
//   - two consecutive enrichments of the same ARN both call the API (uncached:
//     UpdateStateMachine keeps the ARN, so a session cache would go stale
//     without ever missing — see enrichSfn's doc comment)
//   - StateMachineEnriched re-enrichment path accepted as RawStruct
//   - API error propagated
//   - registry sanity: GetDetailEnricher("sfn") non-nil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// enrichSfnFake — narrow SFNAPI fake with a DescribeStateMachine call counter
// ---------------------------------------------------------------------------

type enrichSfnFake struct {
	describeFn    func(*sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error)
	describeCalls int
}

func (f *enrichSfnFake) DescribeStateMachine(_ context.Context, in *sfn.DescribeStateMachineInput, _ ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	f.describeCalls++
	if f.describeFn != nil {
		return f.describeFn(in)
	}
	return &sfn.DescribeStateMachineOutput{
		Status:  sfntypes.StateMachineStatusActive,
		RoleArn: aws.String("arn:aws:iam::123456789012:role/order-processing-role"),
	}, nil
}

func (f *enrichSfnFake) ListStateMachines(_ context.Context, _ *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	return &sfn.ListStateMachinesOutput{}, nil
}
func (f *enrichSfnFake) ListExecutions(_ context.Context, _ *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	return &sfn.ListExecutionsOutput{}, nil
}
func (f *enrichSfnFake) GetExecutionHistory(_ context.Context, _ *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	return &sfn.GetExecutionHistoryOutput{}, nil
}
func (f *enrichSfnFake) ListTagsForResource(_ context.Context, _ *sfn.ListTagsForResourceInput, _ ...func(*sfn.Options)) (*sfn.ListTagsForResourceOutput, error) {
	return &sfn.ListTagsForResourceOutput{}, nil
}

var _ awsclient.SFNAPI = (*enrichSfnFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sfnEnricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("sfn")
	if e == nil {
		t.Fatal("sfn detail enricher not registered")
	}
	return e
}

func makeSfnCtx(client awsclient.SFNAPI, docs *awsclient.DetailDocCache) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{SFN: client},
		DetailDocs: docs,
	}
}

const sfnTestArn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"

const sfnTestASL = `{"Comment":"Order processing workflow","StartAt":"ValidateOrder","States":{"ValidateOrder":{"Type":"Task","Resource":"arn:aws:lambda:us-east-1:123456789012:function:validate-order","Next":"ProcessPayment"},"ProcessPayment":{"Type":"Task","Resource":"arn:aws:lambda:us-east-1:123456789012:function:process-payment","End":true}}}`

func makeSfnItem(arn string) sfntypes.StateMachineListItem {
	return sfntypes.StateMachineListItem{
		StateMachineArn: aws.String(arn),
		Name:            aws.String("order-processing"),
		Type:            sfntypes.StateMachineTypeStandard,
		CreationDate:    aws.Time(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)),
	}
}

func makeSfnRes(arn string) resource.Resource {
	return resource.Resource{ID: arn, RawStruct: makeSfnItem(arn)}
}

// ---------------------------------------------------------------------------
// Tests: invalid context
// ---------------------------------------------------------------------------

func TestEnrichSfn_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichSfn_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichSfn_NilClients_ReturnsError(t *testing.T) {
	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)
	ctx := &awsclient.DetailEnrichmentCtx{Clients: nil, DetailDocs: &awsclient.DetailDocCache{}}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

func TestEnrichSfn_NilDetailDocs_NoLongerErrors(t *testing.T) {
	// sfn is uncached (see enrichSfn's doc comment): DetailDocs is
	// consequently not a required dependency, mirroring enrichLambda's
	// uncached contract (aws_lambda_detail_enrich_test.go).
	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)
	ctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{SFN: &enrichSfnFake{}},
		DetailDocs: nil,
	}

	if _, err := enricher(context.Background(), ctx, res); err != nil {
		t.Fatalf("unexpected error with nil DetailDocs (sfn is uncached): %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: bad RawStruct / missing ARN
// ---------------------------------------------------------------------------

func TestEnrichSfn_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := sfnEnricher(t)
	res := resource.Resource{ID: sfnTestArn, RawStruct: "not-a-state-machine"}

	_, err := enricher(context.Background(), makeSfnCtx(&enrichSfnFake{}, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichSfn_NilARN_ReturnsError(t *testing.T) {
	enricher := sfnEnricher(t)
	res := resource.Resource{
		ID:        sfnTestArn,
		RawStruct: sfntypes.StateMachineListItem{StateMachineArn: nil},
	}

	_, err := enricher(context.Background(), makeSfnCtx(&enrichSfnFake{}, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error for nil state machine ARN, got nil")
	}
}

func TestEnrichSfn_EmptyARN_ReturnsError(t *testing.T) {
	enricher := sfnEnricher(t)
	res := resource.Resource{
		ID:        "",
		RawStruct: sfntypes.StateMachineListItem{StateMachineArn: aws.String("")},
	}

	_, err := enricher(context.Background(), makeSfnCtx(&enrichSfnFake{}, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error for empty state machine ARN, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: every enrichment calls the API (uncached)
// ---------------------------------------------------------------------------

func TestEnrichSfn_ConsecutiveCalls_BothHitAPIWithFreshPayload(t *testing.T) {
	fake := &enrichSfnFake{
		describeFn: func(_ *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
			return &sfn.DescribeStateMachineOutput{
				Status:     sfntypes.StateMachineStatusActive,
				RoleArn:    aws.String("arn:aws:iam::123456789012:role/order-processing-role"),
				Definition: aws.String(sfnTestASL),
			}, nil
		},
	}

	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)
	ctx := makeSfnCtx(fake, nil)

	for call := 1; call <= 2; call++ {
		got, err := enricher(context.Background(), ctx, res)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", call, err)
		}

		enriched, ok := got.RawStruct.(awsclient.StateMachineEnriched)
		if !ok {
			t.Fatalf("call %d: RawStruct = %T, want StateMachineEnriched", call, got.RawStruct)
		}
		if enriched.StateMachineArn == nil || *enriched.StateMachineArn != sfnTestArn {
			t.Errorf("call %d: enriched.StateMachineArn = %v, want %q", call, enriched.StateMachineArn, sfnTestArn)
		}
		if enriched.Status != sfntypes.StateMachineStatusActive {
			t.Errorf("call %d: enriched.Status = %v, want ACTIVE", call, enriched.Status)
		}
		if enriched.RoleArn == nil || *enriched.RoleArn != "arn:aws:iam::123456789012:role/order-processing-role" {
			t.Errorf("call %d: enriched.RoleArn = %v, want the order-processing role", call, enriched.RoleArn)
		}
		def, ok := enriched.Definition.(map[string]any)
		if !ok {
			t.Fatalf("call %d: enriched.Definition = %T, want map[string]any (parsed ASL)", call, enriched.Definition)
		}
		if def["Comment"] != "Order processing workflow" {
			t.Errorf("call %d: enriched.Definition[Comment] = %v, want %q", call, def["Comment"], "Order processing workflow")
		}
	}

	if fake.describeCalls != 2 {
		t.Errorf("DescribeStateMachine called %d times across two enrichments, want 2 (uncached: "+
			"UpdateStateMachine keeps the ARN, so a session cache would go stale without ever missing)", fake.describeCalls)
	}
}

func TestEnrichSfn_UnparsableDefinition_KeptAsRawString(t *testing.T) {
	const rawDef = "not valid ASL json {{{"
	fake := &enrichSfnFake{
		describeFn: func(_ *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
			return &sfn.DescribeStateMachineOutput{Definition: aws.String(rawDef)}, nil
		},
	}

	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)

	got, err := enricher(context.Background(), makeSfnCtx(fake, &awsclient.DetailDocCache{}), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.StateMachineEnriched)
	if enriched.Definition != rawDef {
		t.Errorf("enriched.Definition = %v, want raw string %q", enriched.Definition, rawDef)
	}
}

// TestEnrichSfn_BigIntInDefinition_RoundTripsLosslessly pins F2: parseJSONOrRaw
// (core/aws/detail_enrich_engine.go) must decode with json.Decoder.UseNumber
// so an ASL definition's integer fields above 2^53 survive as json.Number,
// not a plain json.Unmarshal-into-any float64 that silently rounds
// 9007199254740993 to 9007199254740992 — the same corruption
// core/fieldpath's tryParseJSON was fixed for (extract.go's doc comment).
// TestEnrichSfn_BigIntInDefinition_RoundTripsLosslessly pins F2's tiered
// normalization (jsonyaml.NormalizeJSONNumbers, called by parseJSONOrRaw):
// a json.Number that fits int64 promotes to a bare int64 — 9007199254740993
// fits (int64 max is ~9.22e18), so it must come back as int64(9007199254740993),
// not the json.Number wrapper (which normalizeJSONNumbers only keeps for a
// value past uint64 range — see the sibling test below) and never the
// float64 a plain json.Unmarshal-into-any would silently round to
// (9007199254740992).
func TestEnrichSfn_BigIntInDefinition_RoundTripsLosslessly(t *testing.T) {
	const bigInt = "9007199254740993"
	rawDef := `{"Comment":"order processing","Timeout":` + bigInt + `}`
	fake := &enrichSfnFake{
		describeFn: func(_ *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
			return &sfn.DescribeStateMachineOutput{Definition: aws.String(rawDef)}, nil
		},
	}

	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)

	got, err := enricher(context.Background(), makeSfnCtx(fake, nil), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.StateMachineEnriched)
	def, ok := enriched.Definition.(map[string]any)
	if !ok {
		t.Fatalf("enriched.Definition = %T, want map[string]any (parsed ASL)", enriched.Definition)
	}
	timeout, ok := def["Timeout"].(int64)
	if !ok {
		t.Fatalf("Definition[Timeout] = %T (%v), want int64 (fits, so normalization promotes it) — a plain json.Unmarshal into any would instead silently round it to float64", def["Timeout"], def["Timeout"])
	}
	if timeout != 9007199254740993 {
		t.Errorf("Definition[Timeout] = %d, want the full-precision value %d (a float64 would read 9007199254740992)", timeout, int64(9007199254740993))
	}
}

// TestEnrichSfn_IntBeyondUint64InDefinition_StaysJSONNumber verifies the
// other tier: a value past uint64's range (2^64) fits none of
// int64/uint64/exact-round-trip-float64, so normalization must leave it as
// json.Number with its full digits intact rather than losing precision to
// float64 (whose shortest %g form for 2^64 is "1.8446744073709552e+19",
// not the original digit string).
func TestEnrichSfn_IntBeyondUint64InDefinition_StaysJSONNumber(t *testing.T) {
	const beyondUint64 = "18446744073709551616" // 2^64, one past uint64 max
	rawDef := `{"Comment":"order processing","Timeout":` + beyondUint64 + `}`
	fake := &enrichSfnFake{
		describeFn: func(_ *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
			return &sfn.DescribeStateMachineOutput{Definition: aws.String(rawDef)}, nil
		},
	}

	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)

	got, err := enricher(context.Background(), makeSfnCtx(fake, nil), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.StateMachineEnriched)
	def, ok := enriched.Definition.(map[string]any)
	if !ok {
		t.Fatalf("enriched.Definition = %T, want map[string]any (parsed ASL)", enriched.Definition)
	}
	num, ok := def["Timeout"].(json.Number)
	if !ok {
		t.Fatalf("Definition[Timeout] = %T (%v), want json.Number — a value past uint64 range fits no numeric tier, so it must be retained as-is", def["Timeout"], def["Timeout"])
	}
	if num.String() != beyondUint64 {
		t.Errorf("Definition[Timeout] = %q, want the full-precision digits %q", num.String(), beyondUint64)
	}
}

// ---------------------------------------------------------------------------
// Tests: re-enrichment path
// ---------------------------------------------------------------------------

func TestEnrichSfn_StateMachineEnrichedRawStruct_Accepted(t *testing.T) {
	fake := &enrichSfnFake{
		describeFn: func(_ *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
			return &sfn.DescribeStateMachineOutput{Status: sfntypes.StateMachineStatusActive}, nil
		},
	}
	enricher := sfnEnricher(t)

	res := resource.Resource{
		ID: sfnTestArn,
		RawStruct: awsclient.StateMachineEnriched{
			StateMachineListItem: makeSfnItem(sfnTestArn),
			Status:               sfntypes.StateMachineStatusDeleting,
		},
	}

	got, err := enricher(context.Background(), makeSfnCtx(fake, &awsclient.DetailDocCache{}), res)
	if err != nil {
		t.Fatalf("unexpected error on StateMachineEnriched re-enrichment: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.StateMachineEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want StateMachineEnriched", got.RawStruct)
	}
	if enriched.StateMachineArn == nil || *enriched.StateMachineArn != sfnTestArn {
		t.Errorf("enriched.StateMachineArn = %v, want %q", enriched.StateMachineArn, sfnTestArn)
	}
}

// ---------------------------------------------------------------------------
// Tests: API error propagation
// ---------------------------------------------------------------------------

func TestEnrichSfn_APIError_Propagated(t *testing.T) {
	fake := &enrichSfnFake{
		describeFn: func(_ *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
			return nil, errFake("DescribeStateMachine: access denied")
		},
	}
	enricher := sfnEnricher(t)
	res := makeSfnRes(sfnTestArn)

	_, err := enricher(context.Background(), makeSfnCtx(fake, &awsclient.DetailDocCache{}), res)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: registry sanity
// ---------------------------------------------------------------------------

func TestDetailEnricherRegistry_Sfn_IsNonNil(t *testing.T) {
	e := resource.GetDetailEnricher("sfn")
	if e == nil {
		t.Fatal("sfn detail enricher must be registered and non-nil")
	}
}
