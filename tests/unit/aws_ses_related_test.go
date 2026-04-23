// aws_ses_related_fixture_test.go — Fixture-based related-panel checker tests for SES.
//
// These tests use NewSESFixtures() canonical constants to verify that the SES
// related-panel checkers produce correct counts when driven by the demo graph-root
// identity "acme-corp.com" and its wired event destinations.
//
// Contract assertions:
//   - checkSESEbRule with graph-root identity (config set = SESConfigSetName) →
//     Count=1 (one EventBridgeDestination in fixture event destinations).
//   - checkSESKinesis with graph-root identity → Count=1 (one Firehose destination).
//     Returned ID contains SESFirehoseStreamARN.
//   - checkSESSns with graph-root identity → Count=1 (one SnsDestination).
//   - checkSESS3 with valid sesv2types.IdentityInfo RawStruct → Count=0
//     (SES v1 API unavailable; valid RawStruct means unknown/0, not -1).
//   - checkSESLambda with valid sesv2types.IdentityInfo RawStruct → Count=0
//     (SES v1 API unavailable).
//   - Non-graph-root identity (no config set) → Count=0 for eb-rule/kinesis/sns.
//   - Empty identity ID → Count=0 (all config-set-dependent checkers).
//   - nil clients (wrong type) → Count=-1 for checkers that need API access.
package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sesCheckerByTarget returns the RelatedChecker for the given target type registered
// under "ses". It fails the test immediately if the checker is not found.
func sesCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ses") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ses related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ses related checker for %s not found", target)
	return nil
}

// sesFixtureSrcIdentity builds a resource.Resource from the canonical SES fixture
// for the given identity name. Sets RawStruct to sesv2types.IdentityInfo so that
// checkers that call assertStruct receive the correct type.
func sesFixtureSrcIdentity(identityName string) resource.Resource {
	return resource.Resource{
		ID:   identityName,
		Name: identityName,
		Fields: map[string]string{
			"identity_name": identityName,
			"identity_type": "DOMAIN",
		},
		RawStruct: sesv2types.IdentityInfo{
			IdentityName: aws.String(identityName),
			IdentityType: sesv2types.IdentityTypeDomain,
		},
	}
}

// sesFixtureClients returns a *ServiceClients whose SESv2 fake is wired with the
// canonical fixture event destinations for the graph-root identity.
func sesFixtureClients() *awsclient.ServiceClients {
	f := fixtures.NewSESFixtures()
	return &awsclient.ServiceClients{
		SESv2: newFakeSESv2FromFixture(f),
	}
}

// newFakeSESv2FromFixture builds a fakeSESv2Batch5 that mirrors the fixture data:
//   - GetEmailIdentity for SESGraphRootIdentity → ConfigurationSetName = SESConfigSetName
//   - GetConfigurationSetEventDestinations for SESConfigSetName → fixture destinations
//   - All other identities return empty config set name
func newFakeSESv2FromFixture(f *fixtures.SESFixtures) *fakeSESv2Batch5 {
	return newFakeSESv2WithEventDestinations(
		fixtures.SESGraphRootIdentity,
		fixtures.SESConfigSetName,
		f.EventDestinationsByConfigSet[fixtures.SESConfigSetName].EventDestinations,
	)
}

// ---------------------------------------------------------------------------
// checkSESEbRule — fixture-based
// ---------------------------------------------------------------------------

// TestRelated_SES_EbRule_FixtureGraphRootMatchesOne verifies that the graph-root
// identity (acme-corp.com) wired to SESConfigSetName produces Count=1 for eb-rule
// (the fixture has one EventBridgeDestination pointing to SESEventBusARN).
func TestRelated_SES_EbRule_FixtureGraphRootMatchesOne(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one EventBridgeDestination in fixture)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_EbRule_FixtureGraphRootContainsEventBusARN verifies that the
// returned resource ID contains the fixture's SESEventBusARN.
func TestRelated_SES_EbRule_FixtureGraphRootContainsEventBusARN(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count < 1 {
		t.Fatalf("Count = %d, want >= 1", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == fixtures.SESEventBusARN {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain SESEventBusARN=%q", result.ResourceIDs, fixtures.SESEventBusARN)
	}
}

// TestRelated_SES_EbRule_NonGraphRootIdentityReturnsZero verifies that an identity
// without a config set (not the graph-root) returns Count=0 for eb-rule.
func TestRelated_SES_EbRule_NonGraphRootIdentityReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	// Use a non-graph-root identity — the fake will return empty config set name.
	src := sesFixtureSrcIdentity("noreply@acme-corp.com")

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no config set for non-graph-root identity)", result.Count)
	}
}

// TestRelated_SES_EbRule_EmptyIDReturnsZero verifies that an empty identity ID
// short-circuits to Count=0.
func TestRelated_SES_EbRule_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_SES_EbRule_NilClientsReturnsNegOne verifies that nil clients
// (wrong type assertion) returns Count=-1.
func TestRelated_SES_EbRule_NilClientsReturnsNegOne(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSESKinesis — fixture-based
// ---------------------------------------------------------------------------

// TestRelated_SES_Kinesis_FixtureGraphRootMatchesOne verifies that the graph-root
// identity produces Count=1 for kinesis (the fixture has one Firehose destination).
func TestRelated_SES_Kinesis_FixtureGraphRootMatchesOne(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one Firehose destination in fixture)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_Kinesis_FixtureContainsFirehoseStreamARN verifies that the
// returned resource ID is SESFirehoseStreamARN.
func TestRelated_SES_Kinesis_FixtureContainsFirehoseStreamARN(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count < 1 {
		t.Fatalf("Count = %d, want >= 1", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == fixtures.SESFirehoseStreamARN {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain SESFirehoseStreamARN=%q", result.ResourceIDs, fixtures.SESFirehoseStreamARN)
	}
}

// TestRelated_SES_Kinesis_NonGraphRootIdentityReturnsZero verifies Count=0 for
// an identity without a config set.
func TestRelated_SES_Kinesis_NonGraphRootIdentityReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity("alerts@acme-corp.com")

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no config set for non-graph-root identity)", result.Count)
	}
}

// TestRelated_SES_Kinesis_EmptyIDReturnsZero verifies Count=0 for an empty identity ID.
func TestRelated_SES_Kinesis_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSESSns — fixture-based
// ---------------------------------------------------------------------------

// TestRelated_SES_Sns_FixtureGraphRootMatchesOne verifies that the graph-root
// identity produces Count=1 for sns (the fixture has one SnsDestination).
func TestRelated_SES_Sns_FixtureGraphRootMatchesOne(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one SnsDestination in fixture)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_Sns_NonGraphRootIdentityReturnsZero verifies Count=0 for
// an identity without a config set.
func TestRelated_SES_Sns_NonGraphRootIdentityReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity("suppressed@acme-corp.com")

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no config set for non-graph-root identity)", result.Count)
	}
}

// TestRelated_SES_Sns_EmptyIDReturnsZero verifies Count=0 for an empty identity ID.
func TestRelated_SES_Sns_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_SES_Sns_NilClientsReturnsNegOne verifies that nil clients returns
// Count=-1 for checkSESSns.
func TestRelated_SES_Sns_NilClientsReturnsNegOne(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkSESS3 — valid clients with no SES v1 configured returns 0
// ---------------------------------------------------------------------------

// TestRelated_SES_S3_NoSESv1ClientReturnsZero verifies that checkSESS3 returns
// Count=0 when the ServiceClients has no SES v1 client (c.SES == nil).
// This is the operator-honest case: pure outbound SES account — no receipt rule set.
func TestRelated_SES_S3_NoSESv1ClientReturnsZero(t *testing.T) {
	// sesFixtureClients() only has SESv2 wired; c.SES == nil.
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (c.SES == nil → no rule set → operator-honest 0)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_SES_S3_NilClientsReturnsNegOne verifies that nil clients (wrong type
// assertion) returns Count=-1 — distinguishable from operator-honest 0.
func TestRelated_SES_S3_NilClientsReturnsNegOne(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → type assertion failed)", result.Count)
	}
}

// TestRelated_SES_S3_FixtureAllIdentitiesWithNoSESv1ReturnZero verifies that
// checkSESS3 returns Count=0 (not -1) for every fixture identity when c.SES == nil.
func TestRelated_SES_S3_FixtureAllIdentitiesWithNoSESv1ReturnZero(t *testing.T) {
	f := fixtures.NewSESFixtures()
	// sesFixtureClients() has SESv2 but no SES v1 — simulates pure outbound account.
	clients := sesFixtureClients()
	checker := sesCheckerByTarget(t, "s3")

	for _, identity := range f.Identities {
		identityName := ""
		if identity.IdentityName != nil {
			identityName = *identity.IdentityName
		}
		src := resource.Resource{
			ID:        identityName,
			Name:      identityName,
			RawStruct: identity,
		}
		result := checker(context.Background(), clients, src, resource.ResourceCache{})
		if result.Count != 0 {
			t.Errorf("identity %q: Count = %d, want 0 (c.SES == nil → no rule set)", identityName, result.Count)
		}
	}
}

// ---------------------------------------------------------------------------
// checkSESLambda — valid RawStruct returns 0 (SES v1 API unavailable)
// ---------------------------------------------------------------------------

// TestRelated_SES_Lambda_ValidRawStructReturnsZero verifies that checkSESLambda
// returns Count=0 for a valid sesv2types.IdentityInfo RawStruct (not -1).
// The SES v1 receipt-rule LambdaAction path is unavailable in SESv2 SDK.
func TestRelated_SES_Lambda_ValidRawStructReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (SES v1 API unavailable, valid RawStruct)", result.Count)
	}
}

// TestRelated_SES_Lambda_EmptyIDReturnsZero verifies Count=0 for empty identity ID.
func TestRelated_SES_Lambda_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// R53 — fixture-based supplementary tests using SESGraphRootIdentity constants
// ---------------------------------------------------------------------------

// TestRelated_SES_R53_FixtureGraphRootMatchesAcmeCorp verifies that the graph-root
// domain identity "acme-corp.com" resolves against the R53 zone "acme-corp.com."
// using the fixture constant SESGraphRootIdentity.
func TestRelated_SES_R53_FixtureGraphRootMatchesAcmeCorp(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/ZFIXTURE",
		Name: fixtures.SESGraphRootIdentity + ".",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (graph-root domain matches fixture R53 zone)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/hostedzone/ZFIXTURE" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/ZFIXTURE]", result.ResourceIDs)
	}
}

// TestRelated_SES_R53_EmailIdentityExtractsDomain verifies that an EMAIL_ADDRESS
// identity in the format "user@acme-corp.com" resolves to the parent domain zone
// "acme-corp.com." — domain is extracted after "@".
func TestRelated_SES_R53_EmailIdentityExtractsDomain(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/ZFIXTURE",
		Name: fixtures.SESGraphRootIdentity + ".",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}
	src := resource.Resource{
		ID:   "noreply@" + fixtures.SESGraphRootIdentity,
		Name: "noreply@" + fixtures.SESGraphRootIdentity,
		RawStruct: sesv2types.IdentityInfo{
			IdentityName: aws.String("noreply@" + fixtures.SESGraphRootIdentity),
			IdentityType: sesv2types.IdentityTypeEmailAddress,
		},
	}

	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (email address: domain extracted after '@')", result.Count)
	}
}

// ---------------------------------------------------------------------------
// SES v1 mock — used by Target #2 and Target #3 tests below.
// Implements SESV1API (single method: DescribeActiveReceiptRuleSet).
// ---------------------------------------------------------------------------

// fakeSESV1 implements awsclient.SESV1API for receipt-rule-set tests.
type fakeSESV1 struct {
	calls  int
	// responses is a slice of (output, error) pairs returned in order.
	// After exhausting responses the last entry is repeated.
	responses []sesV1Response
}

type sesV1Response struct {
	output *ses.DescribeActiveReceiptRuleSetOutput
	err    error
}

func (f *fakeSESV1) DescribeActiveReceiptRuleSet(
	_ context.Context,
	_ *ses.DescribeActiveReceiptRuleSetInput,
	_ ...func(*ses.Options),
) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	f.calls++
	idx := f.calls - 1
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	return f.responses[idx].output, f.responses[idx].err
}

// Compile-time check: fakeSESV1 satisfies SESV1API.
var _ awsclient.SESV1API = (*fakeSESV1)(nil)

// sesV1Clients returns a *awsclient.ServiceClients with the given SESV1API wired.
// Each call returns a fresh pointer so the sesRuleSetCaches key is distinct.
func sesV1Clients(v1 awsclient.SESV1API) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{SES: v1}
}

// sesLambdaARN returns a plausible Lambda ARN string for test data.
func sesLambdaARN(name string) string {
	return "arn:aws:lambda:us-east-1:123456789012:function:" + name
}

// sesBucketName returns a plausible S3 bucket name for test data.
func sesBucketName(name string) string {
	return "ses-inbound-" + name
}

// buildReceiptRule builds a sestypes.ReceiptRule with optional recipients and Lambda action.
func buildLambdaReceiptRule(name string, recipients []string, lambdaARN string) sestypes.ReceiptRule {
	return sestypes.ReceiptRule{
		Name:       aws.String(name),
		Recipients: recipients,
		Actions: []sestypes.ReceiptAction{
			{LambdaAction: &sestypes.LambdaAction{FunctionArn: aws.String(lambdaARN)}},
		},
	}
}

// buildS3ReceiptRule builds a sestypes.ReceiptRule with optional recipients and S3 action.
func buildS3ReceiptRule(name string, recipients []string, bucketName string) sestypes.ReceiptRule {
	return sestypes.ReceiptRule{
		Name:       aws.String(name),
		Recipients: recipients,
		Actions: []sestypes.ReceiptAction{
			{S3Action: &sestypes.S3Action{BucketName: aws.String(bucketName)}},
		},
	}
}

// ---------------------------------------------------------------------------
// Target #2 — checkSESLambda must scope results by Recipients
//
// Problem: current code ignores the resource argument — returns the union of
// Lambda ARNs from all rules regardless of Recipients field. AWS semantics:
//   - Empty Recipients → applies to all (global catch-all).
//   - Non-empty Recipients → only matched identities receive those actions.
//   - "example.com" matches any @example.com address AND the domain itself.
//   - Domain identity matches subdomain rules (left-extensible per AWS docs).
//
// These tests will FAIL until the coder implements recipient-scoped filtering.
// ---------------------------------------------------------------------------

// TestCheckSESLambda_ScopesByRecipient verifies that checkSESLambda returns only
// the Lambda ARNs whose recipient filter includes the queried identity.
// Rule A (global): empty Recipients → always matches.
// Rule B: Recipients=["support@acme.com"] → matches support@acme.com.
// Rule C: Recipients=["sales.acme.com"] → matches sales.acme.com domain.
func TestCheckSESLambda_ScopesByRecipient(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			buildLambdaReceiptRule("global-catch-all", nil, sesLambdaARN("global-router")),
			buildLambdaReceiptRule("support-rule", []string{"support@acme.com"}, sesLambdaARN("support-router")),
			buildLambdaReceiptRule("sales-rule", []string{"sales.acme.com"}, sesLambdaARN("sales-router")),
		},
	}

	checker := sesCheckerByTarget(t, "lambda")

	subtests := []struct {
		name        string
		resource    resource.Resource
		wantARNs    []string
		unwantedARN string
	}{
		{
			name: "support@acme.com → global + support-router only",
			resource: resource.Resource{
				ID:     "support@acme.com",
				Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
			},
			wantARNs:    []string{sesLambdaARN("global-router"), sesLambdaARN("support-router")},
			unwantedARN: sesLambdaARN("sales-router"),
		},
		{
			name: "billing@acme.com → global only (no specific rule matches)",
			resource: resource.Resource{
				ID:     "billing@acme.com",
				Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
			},
			wantARNs:    []string{sesLambdaARN("global-router")},
			unwantedARN: sesLambdaARN("support-router"),
		},
		{
			// DOMAIN identity matches both domain-level recipient rules AND
			// email-address recipients under that domain.
			name: "acme.com (DOMAIN) → global + support + sales (domain owns all subdomains/addresses)",
			resource: resource.Resource{
				ID:     "acme.com",
				Fields: map[string]string{"identity_type": "DOMAIN"},
			},
			wantARNs:    []string{sesLambdaARN("global-router"), sesLambdaARN("support-router"), sesLambdaARN("sales-router")},
			unwantedARN: "",
		},
		{
			name: "sales.acme.com (DOMAIN subdomain) → global + sales-router only",
			resource: resource.Resource{
				ID:     "sales.acme.com",
				Fields: map[string]string{"identity_type": "DOMAIN"},
			},
			wantARNs:    []string{sesLambdaARN("global-router"), sesLambdaARN("sales-router")},
			unwantedARN: sesLambdaARN("support-router"),
		},
	}

	for _, st := range subtests {
		st := st
		t.Run(st.name, func(t *testing.T) {
			// Fresh pointer per subtest: sesRuleSetCaches uses pointer as key,
			// so a new pointer starts with no cached entry.
			clients := sesV1Clients(&fakeSESV1{
				responses: []sesV1Response{{output: ruleSetOutput, err: nil}},
			})
			result := checker(context.Background(), clients, st.resource, resource.ResourceCache{})
			if result.Err != nil {
				t.Fatalf("unexpected error: %v", result.Err)
			}
			// Verify all expected ARNs are present.
			for _, wantARN := range st.wantARNs {
				found := false
				for _, id := range result.ResourceIDs {
					if id == wantARN {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs, wantARN)
				}
			}
			// Verify unwanted ARN is absent.
			if st.unwantedARN != "" {
				for _, id := range result.ResourceIDs {
					if id == st.unwantedARN {
						t.Errorf("ResourceIDs = %v, must NOT contain %q (wrong recipient scope)", result.ResourceIDs, st.unwantedARN)
					}
				}
			}
			// Count must equal len(wantARNs).
			if result.Count != len(st.wantARNs) {
				t.Errorf("Count = %d, want %d", result.Count, len(st.wantARNs))
			}
		})
	}
}

// TestCheckSESS3_ScopesByRecipient is the mirror of TestCheckSESLambda_ScopesByRecipient
// using S3Action.BucketName instead of LambdaAction.FunctionArn.
func TestCheckSESS3_ScopesByRecipient(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			buildS3ReceiptRule("global-catch-all", nil, sesBucketName("global")),
			buildS3ReceiptRule("support-rule", []string{"support@acme.com"}, sesBucketName("support")),
			buildS3ReceiptRule("sales-rule", []string{"sales.acme.com"}, sesBucketName("sales")),
		},
	}

	checker := sesCheckerByTarget(t, "s3")

	subtests := []struct {
		name          string
		resource      resource.Resource
		wantBuckets   []string
		unwantedBucket string
	}{
		{
			name: "support@acme.com → global + support bucket only",
			resource: resource.Resource{
				ID:     "support@acme.com",
				Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
			},
			wantBuckets:    []string{sesBucketName("global"), sesBucketName("support")},
			unwantedBucket: sesBucketName("sales"),
		},
		{
			name: "billing@acme.com → global bucket only",
			resource: resource.Resource{
				ID:     "billing@acme.com",
				Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
			},
			wantBuckets:    []string{sesBucketName("global")},
			unwantedBucket: sesBucketName("support"),
		},
		{
			name: "acme.com (DOMAIN) → all buckets",
			resource: resource.Resource{
				ID:     "acme.com",
				Fields: map[string]string{"identity_type": "DOMAIN"},
			},
			wantBuckets:    []string{sesBucketName("global"), sesBucketName("support"), sesBucketName("sales")},
			unwantedBucket: "",
		},
		{
			name: "sales.acme.com (DOMAIN) → global + sales bucket only",
			resource: resource.Resource{
				ID:     "sales.acme.com",
				Fields: map[string]string{"identity_type": "DOMAIN"},
			},
			wantBuckets:    []string{sesBucketName("global"), sesBucketName("sales")},
			unwantedBucket: sesBucketName("support"),
		},
	}

	for _, st := range subtests {
		st := st
		t.Run(st.name, func(t *testing.T) {
			// Fresh pointer per subtest avoids sesRuleSetCaches leaking between subtests.
			clients := sesV1Clients(&fakeSESV1{
				responses: []sesV1Response{{output: ruleSetOutput, err: nil}},
			})
			result := checker(context.Background(), clients, st.resource, resource.ResourceCache{})
			if result.Err != nil {
				t.Fatalf("unexpected error: %v", result.Err)
			}
			for _, wantBucket := range st.wantBuckets {
				found := false
				for _, id := range result.ResourceIDs {
					if id == wantBucket {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs, wantBucket)
				}
			}
			if st.unwantedBucket != "" {
				for _, id := range result.ResourceIDs {
					if id == st.unwantedBucket {
						t.Errorf("ResourceIDs = %v, must NOT contain %q (wrong recipient scope)", result.ResourceIDs, st.unwantedBucket)
					}
				}
			}
			if result.Count != len(st.wantBuckets) {
				t.Errorf("Count = %d, want %d", result.Count, len(st.wantBuckets))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Target #3 — sesActiveReceiptRuleSet sync.Once must not cache errors
//
// Problem: sync.Once freezes both success and error. A transient error on the
// first call prevents all subsequent calls from ever succeeding, even after
// the upstream API recovers. This test will FAIL until the coder replaces the
// sync.Once with a guard that only seals on success.
// ---------------------------------------------------------------------------

// TestSESActiveReceiptRuleSet_RetriesAfterTransientError verifies that:
//   - First checkSESLambda call with an erroring SES v1 API → Count=-1.
//   - Second call on the SAME *ServiceClients → Count=1 (error retried, rule fetched).
//   - Third call → Count=1, but mock call counter is 2 (success sealed, no 3rd API call).
func TestSESActiveReceiptRuleSet_RetriesAfterTransientError(t *testing.T) {
	// One Lambda rule in the rule set.
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			buildLambdaReceiptRule("inbound", nil, sesLambdaARN("inbound-handler")),
		},
	}

	// fakeSESV1 with two responses: first is a transient error, second is success.
	v1Mock := &fakeSESV1{
		responses: []sesV1Response{
			{output: nil, err: errors.New("ses: temporary connection error")},
			{output: ruleSetOutput, err: nil},
		},
	}
	// Use a fixed *ServiceClients pointer for all three calls — must be the same
	// pointer for the sesRuleSetCaches key to be consistent.
	clients := sesV1Clients(v1Mock)

	src := resource.Resource{
		ID:     "support@acme.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}

	checker := sesCheckerByTarget(t, "lambda")

	// Call 1: expect error (Count=-1) — transient API failure.
	result1 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result1.Count != -1 {
		t.Errorf("call 1: Count = %d, want -1 (transient API error)", result1.Count)
	}

	// Call 2: expect success (Count=1) — error must NOT be cached by sync.Once.
	// This is the regression pin: current code freezes the error so Count stays -1.
	result2 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result2.Count != 1 {
		t.Errorf("call 2: Count = %d, want 1 (error should not be cached — must retry after transient failure)", result2.Count)
	}
	if result2.Err != nil {
		t.Errorf("call 2: unexpected error: %v", result2.Err)
	}

	// Call 3: success is memoized — no additional API call.
	result3 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result3.Count != 1 {
		t.Errorf("call 3: Count = %d, want 1 (success cached from call 2)", result3.Count)
	}
	// The mock was called exactly twice: once for the error, once for the success.
	// A third API call would indicate the success is NOT being cached.
	if v1Mock.calls != 2 {
		t.Errorf("mock.calls = %d, want 2 (success must be memoized — call 3 must not hit the API again)", v1Mock.calls)
	}
}
