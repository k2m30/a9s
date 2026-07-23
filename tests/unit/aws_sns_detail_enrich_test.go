package unit

// aws_sns_detail_enrich_test.go — coverage for enrichSns
// (core/aws/sns_detail_enrichment.go), the on-demand detail enricher
// registered for the "sns" resource type (#261).
//
// Covers:
//   - wrong clients type / nil DetailEnrichmentCtx / nil Clients → error
//     (sns has no DetailDocs dependency — uncached, per the contract)
//   - wrong RawStruct type → error
//   - missing TopicArn → error
//   - success: JSON-object attribute values (Policy) parsed to structured data;
//     plain string attributes (DisplayName) kept as-is
//   - a JSON-looking but malformed attribute value falls back to the raw string
//   - TopicEnriched re-enrichment path accepted as RawStruct
//   - API error propagated
//   - registry sanity: GetDetailEnricher("sns") non-nil

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// enrichSnsFake — narrow SNSAPI fake with a call counter
// ---------------------------------------------------------------------------

type enrichSnsFake struct {
	getAttrsFn    func(*sns.GetTopicAttributesInput) (*sns.GetTopicAttributesOutput, error)
	getAttrsCalls int
}

func (f *enrichSnsFake) GetTopicAttributes(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	f.getAttrsCalls++
	if f.getAttrsFn != nil {
		return f.getAttrsFn(in)
	}
	return &sns.GetTopicAttributesOutput{}, nil
}

func (f *enrichSnsFake) ListSubscriptionsByTopic(_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	return &sns.ListSubscriptionsByTopicOutput{}, nil
}
func (f *enrichSnsFake) ListTagsForResource(_ context.Context, _ *sns.ListTagsForResourceInput, _ ...func(*sns.Options)) (*sns.ListTagsForResourceOutput, error) {
	return &sns.ListTagsForResourceOutput{}, nil
}
func (f *enrichSnsFake) GetSubscriptionAttributes(_ context.Context, _ *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	return &sns.GetSubscriptionAttributesOutput{}, nil
}

var _ awsclient.SNSAPI = (*enrichSnsFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func snsEnricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("sns")
	if e == nil {
		t.Fatal("sns detail enricher not registered")
	}
	return e
}

func makeSnsCtx(client awsclient.SNSAPI) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{SNS: client}}
}

const snsTestArn = "arn:aws:sns:us-east-1:123456789012:order-events"

const snsTestPolicyJSON = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowPublish","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/order-service"},"Action":"SNS:Publish","Resource":"arn:aws:sns:us-east-1:123456789012:order-events"}]}`

func makeSnsTopic(arn string) snstypes.Topic {
	var arnPtr *string
	if arn != "" {
		arnPtr = aws.String(arn)
	}
	return snstypes.Topic{TopicArn: arnPtr}
}

func makeSnsRes(arn string) resource.Resource {
	return resource.Resource{ID: arn, RawStruct: makeSnsTopic(arn)}
}

// ---------------------------------------------------------------------------
// Tests: invalid context
// ---------------------------------------------------------------------------

func TestEnrichSns_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := snsEnricher(t)
	res := makeSnsRes(snsTestArn)

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichSns_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := snsEnricher(t)
	res := makeSnsRes(snsTestArn)

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichSns_NilClients_ReturnsError(t *testing.T) {
	enricher := snsEnricher(t)
	res := makeSnsRes(snsTestArn)
	ctx := &awsclient.DetailEnrichmentCtx{Clients: nil}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: bad RawStruct / missing ARN
// ---------------------------------------------------------------------------

func TestEnrichSns_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := snsEnricher(t)
	res := resource.Resource{ID: snsTestArn, RawStruct: "not-a-topic"}

	_, err := enricher(context.Background(), makeSnsCtx(&enrichSnsFake{}), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichSns_EmptyTopicArn_ReturnsError(t *testing.T) {
	enricher := snsEnricher(t)
	res := makeSnsRes("")

	_, err := enricher(context.Background(), makeSnsCtx(&enrichSnsFake{}), res)
	if err == nil {
		t.Fatal("expected error for topic with no ARN, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: success — JSON-object attribute parsed, plain attribute kept as string
// ---------------------------------------------------------------------------

func TestEnrichSns_Success_JSONAttributeParsedPlainAttributeKept(t *testing.T) {
	fake := &enrichSnsFake{
		getAttrsFn: func(_ *sns.GetTopicAttributesInput) (*sns.GetTopicAttributesOutput, error) {
			return &sns.GetTopicAttributesOutput{
				Attributes: map[string]string{
					"TopicArn":                snsTestArn,
					"DisplayName":             "Order Events",
					"SubscriptionsConfirmed":  "3",
					"SubscriptionsPending":    "0",
					"Owner":                   "123456789012",
					"Policy":                  snsTestPolicyJSON,
					"EffectiveDeliveryPolicy": `{"http":{"defaultHealthyRetryPolicy":{"numRetries":3}}}`,
				},
			}, nil
		},
	}

	enricher := snsEnricher(t)
	res := makeSnsRes(snsTestArn)

	got, err := enricher(context.Background(), makeSnsCtx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.getAttrsCalls != 1 {
		t.Errorf("GetTopicAttributes called %d times, want 1", fake.getAttrsCalls)
	}

	enriched, ok := got.RawStruct.(awsclient.TopicEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want TopicEnriched", got.RawStruct)
	}

	displayName, ok := enriched.Attributes["DisplayName"].(string)
	if !ok || displayName != "Order Events" {
		t.Errorf("Attributes[DisplayName] = %v (%T), want plain string %q", enriched.Attributes["DisplayName"], enriched.Attributes["DisplayName"], "Order Events")
	}

	policy, ok := enriched.Attributes["Policy"].(map[string]any)
	if !ok {
		t.Fatalf("Attributes[Policy] = %T, want map[string]any (parsed JSON)", enriched.Attributes["Policy"])
	}
	if policy["Version"] != "2012-10-17" {
		t.Errorf("Attributes[Policy][Version] = %v, want 2012-10-17", policy["Version"])
	}

	effectiveDeliveryPolicy, ok := enriched.Attributes["EffectiveDeliveryPolicy"].(map[string]any)
	if !ok {
		t.Fatalf("Attributes[EffectiveDeliveryPolicy] = %T, want map[string]any (parsed JSON)", enriched.Attributes["EffectiveDeliveryPolicy"])
	}
	if _, ok := effectiveDeliveryPolicy["http"]; !ok {
		t.Errorf("Attributes[EffectiveDeliveryPolicy] missing http key: %v", effectiveDeliveryPolicy)
	}
}

func TestEnrichSns_MalformedJSONLookingAttribute_KeptAsRawString(t *testing.T) {
	const malformed = `{not valid json`
	fake := &enrichSnsFake{
		getAttrsFn: func(_ *sns.GetTopicAttributesInput) (*sns.GetTopicAttributesOutput, error) {
			return &sns.GetTopicAttributesOutput{
				Attributes: map[string]string{"Policy": malformed},
			}, nil
		},
	}

	enricher := snsEnricher(t)
	res := makeSnsRes(snsTestArn)

	got, err := enricher(context.Background(), makeSnsCtx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.TopicEnriched)
	if enriched.Attributes["Policy"] != malformed {
		t.Errorf("Attributes[Policy] = %v, want raw fallback %q", enriched.Attributes["Policy"], malformed)
	}
}

// ---------------------------------------------------------------------------
// Tests: re-enrichment path
// ---------------------------------------------------------------------------

func TestEnrichSns_TopicEnrichedRawStruct_Accepted(t *testing.T) {
	fake := &enrichSnsFake{
		getAttrsFn: func(_ *sns.GetTopicAttributesInput) (*sns.GetTopicAttributesOutput, error) {
			return &sns.GetTopicAttributesOutput{
				Attributes: map[string]string{"DisplayName": "Order Events"},
			}, nil
		},
	}
	enricher := snsEnricher(t)

	res := resource.Resource{
		ID: snsTestArn,
		RawStruct: awsclient.TopicEnriched{
			Topic:      makeSnsTopic(snsTestArn),
			Attributes: map[string]any{"DisplayName": "stale-previous-name"},
		},
	}

	got, err := enricher(context.Background(), makeSnsCtx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error on TopicEnriched re-enrichment: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.TopicEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want TopicEnriched", got.RawStruct)
	}
	if enriched.Attributes["DisplayName"] != "Order Events" {
		t.Errorf("Attributes[DisplayName] = %v, want refreshed %q", enriched.Attributes["DisplayName"], "Order Events")
	}
}

// ---------------------------------------------------------------------------
// Tests: API error propagation
// ---------------------------------------------------------------------------

func TestEnrichSns_APIError_Propagated(t *testing.T) {
	fake := &enrichSnsFake{
		getAttrsFn: func(_ *sns.GetTopicAttributesInput) (*sns.GetTopicAttributesOutput, error) {
			return nil, errFake("GetTopicAttributes: access denied")
		},
	}
	enricher := snsEnricher(t)
	res := makeSnsRes(snsTestArn)

	_, err := enricher(context.Background(), makeSnsCtx(fake), res)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: registry sanity
// ---------------------------------------------------------------------------

func TestDetailEnricherRegistry_Sns_IsNonNil(t *testing.T) {
	e := resource.GetDetailEnricher("sns")
	if e == nil {
		t.Fatal("sns detail enricher must be registered and non-nil")
	}
}
