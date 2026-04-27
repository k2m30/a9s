// aws_ses_related_fixture_test.go — Fixture-based related-panel checker tests for SES.
//
// These tests use NewSESFixtures() canonical constants to verify that the SES
// related-panel checkers produce correct counts when driven by the demo graph-root
// identity "acme-corp.com" and its wired event destinations.
//
// Contract assertions:
//   - checkSESEbRule with graph-root identity (config set = SESConfigSetName) →
//     Count>0 (rules on the "default" bus from EventBridge fixture).
//     Returned IDs are rule NAMES, not ARNs. No ID starts with "arn:".
//   - checkSESSns with graph-root identity → Count=1 (one SnsDestination).
//   - checkSESS3 with valid sesv2types.IdentityInfo RawStruct → Count=0
//     (SES v1 API unavailable; valid RawStruct means unknown/0, not -1).
//   - checkSESLambda: returned IDs are function NAMES, not full ARNs.
//   - Non-graph-root identity (no config set) → Count=0 for eb-rule/sns.
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
	"github.com/k2m30/a9s/v3/internal/session"
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

// ebRuleCache builds a ResourceCache with "eb-rule" entries from the EventBridge
// fixture (rules on the "default" bus).
func ebRuleCache() resource.ResourceCache {
	f := fixtures.NewEventBridgeFixtures()
	var resources []resource.Resource
	for _, rule := range f.Rules {
		name := ""
		if rule.Name != nil {
			name = *rule.Name
		}
		bus := ""
		if rule.EventBusName != nil {
			bus = *rule.EventBusName
		}
		resources = append(resources, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"event_bus": bus,
			},
		})
	}
	return resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{Resources: resources},
	}
}

// ebRuleNamesOnDefaultBus returns the set of rule names in the EventBridge fixture
// that are on the "default" bus. Used to validate eb-rule checker output.
func ebRuleNamesOnDefaultBus() map[string]struct{} {
	f := fixtures.NewEventBridgeFixtures()
	names := make(map[string]struct{})
	for _, rule := range f.Rules {
		if rule.EventBusName != nil && *rule.EventBusName == "default" && rule.Name != nil {
			names[*rule.Name] = struct{}{}
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// checkSESEbRule — fixture-based (new semantic: bus-name → rule names)
// ---------------------------------------------------------------------------

// TestRelated_SES_EbRule_FixtureGraphRootReturnsRuleNames verifies that the
// graph-root identity wired to SESConfigSetName returns rule NAMEs (not bus ARNs)
// for the "eb-rule" pivot. The fixture EventBridge bus is "default"; all rules on
// that bus should be returned.
func TestRelated_SES_EbRule_FixtureGraphRootReturnsRuleNames(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)
	cache := ebRuleCache()

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, cache)

	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if result.Count <= 0 {
		t.Errorf("Count = %d, want > 0 (at least one rule on the default bus)", result.Count)
	}

	knownNames := ebRuleNamesOnDefaultBus()
	for _, id := range result.ResourceIDs {
		// Guard: no returned ID may be an ARN — that is the regression we're fixing.
		if len(id) >= 4 && id[:4] == "arn:" {
			t.Errorf("ResourceID %q starts with 'arn:' — checker must return rule names, not ARNs", id)
		}
		// Every returned ID must be a recognised rule name from the fixture.
		if _, ok := knownNames[id]; !ok {
			t.Errorf("ResourceID %q is not a known EventBridge rule name from the fixture (known: %v)", id, knownNames)
		}
	}
}

// TestRelated_SES_EbRule_ScopeLimitedToBusName verifies that only rules whose
// event_bus matches the bus name extracted from the SES EventBridgeDestination ARN
// are returned. Rules on a different bus must be excluded.
func TestRelated_SES_EbRule_ScopeLimitedToBusName(t *testing.T) {
	// SES fixture ships to "default" bus.
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	// Build a synthetic cache: rules on "default" bus AND rules on "custom-bus".
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "rule-on-default", Name: "rule-on-default", Fields: map[string]string{"event_bus": "default"}},
				{ID: "rule-on-custom", Name: "rule-on-custom", Fields: map[string]string{"event_bus": "custom-bus"}},
				{ID: "another-default-rule", Name: "another-default-rule", Fields: map[string]string{"event_bus": "default"}},
			},
		},
	}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, cache)

	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}

	// "rule-on-custom" must NOT appear — it is on a different bus.
	for _, id := range result.ResourceIDs {
		if id == "rule-on-custom" {
			t.Errorf("ResourceIDs = %v, must NOT contain 'rule-on-custom' (wrong bus)", result.ResourceIDs)
		}
	}

	// "rule-on-default" and "another-default-rule" MUST appear.
	wantIDs := []string{"rule-on-default", "another-default-rule"}
	for _, want := range wantIDs {
		found := false
		for _, id := range result.ResourceIDs {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ResourceIDs = %v, want to contain %q (rule is on the default bus)", result.ResourceIDs, want)
		}
	}
}

// TestRelated_SES_EbRule_NonGraphRootIdentityReturnsZero verifies that an identity
// without a config set (not the graph-root) returns Count=0 for eb-rule.
func TestRelated_SES_EbRule_NonGraphRootIdentityReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	// Use a non-graph-root identity — the fake will return empty config set name.
	src := sesFixtureSrcIdentity("noreply@acme-corp.com")

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, ebRuleCache())

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
	result := checker(context.Background(), clients, src, ebRuleCache())

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// TestRelated_SES_EbRule_NilClientsReturnsZero verifies that nil clients returns
// Count=0 for the eb-rule checker. Unlike some other checkers, eb-rule cannot
// return -1 for nil clients: without SESv2 there are no bus names to look up,
// so the result is definitively empty rather than an error.
func TestRelated_SES_EbRule_NilClientsReturnsZero(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, ebRuleCache())

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil clients → no bus names resolvable → honest 0)", result.Count)
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
// checkSESLambda — returned IDs must be function NAMES, not full ARNs
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
	calls int
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

// sesV1Clients returns a *awsclient.ServiceClients with the given SESV1API
// wired plus a fresh per-test session.RuleSetStore. Post-PR-02d the SES
// rule-set cache lives on c.RuleSets (per-Session) rather than a process-wide
// map keyed by *ServiceClients pointer, so each test gets an isolated store
// without needing fresh pointers.
func sesV1Clients(v1 awsclient.SESV1API) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{
		SES:      v1,
		RuleSets: session.NewRuleSetStore(),
	}
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
// Target #2 — checkSESLambda must return function NAMES, not ARNs
//
// Problem: old code returned full Lambda ARNs from LambdaAction.FunctionArn.
// New code extracts the function name (last segment after "function:") so IDs
// match the lambda fetcher's resource IDs. Recipient scoping is also applied.
// ---------------------------------------------------------------------------

// TestCheckSESLambda_ScopesByRecipient verifies that checkSESLambda returns only
// the Lambda function NAMEs (not ARNs) whose recipient filter includes the queried
// identity.
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
		name          string
		resource      resource.Resource
		wantNames     []string
		unwantedName  string
	}{
		{
			name: "support@acme.com → global + support-router only",
			resource: resource.Resource{
				ID:     "support@acme.com",
				Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
			},
			wantNames:    []string{"global-router", "support-router"},
			unwantedName: "sales-router",
		},
		{
			name: "billing@acme.com → global only (no specific rule matches)",
			resource: resource.Resource{
				ID:     "billing@acme.com",
				Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
			},
			wantNames:    []string{"global-router"},
			unwantedName: "support-router",
		},
		{
			// DOMAIN identity matches both domain-level recipient rules AND
			// email-address recipients under that domain.
			name: "acme.com (DOMAIN) → global + support + sales (domain owns all subdomains/addresses)",
			resource: resource.Resource{
				ID:     "acme.com",
				Fields: map[string]string{"identity_type": "DOMAIN"},
			},
			wantNames:    []string{"global-router", "support-router", "sales-router"},
			unwantedName: "",
		},
		{
			name: "sales.acme.com (DOMAIN subdomain) → global + sales-router only",
			resource: resource.Resource{
				ID:     "sales.acme.com",
				Fields: map[string]string{"identity_type": "DOMAIN"},
			},
			wantNames:    []string{"global-router", "sales-router"},
			unwantedName: "support-router",
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
			// Verify all expected function NAMES are present (not ARNs).
			for _, wantName := range st.wantNames {
				found := false
				for _, id := range result.ResourceIDs {
					if id == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ResourceIDs = %v, want to contain function name %q (not ARN)", result.ResourceIDs, wantName)
				}
			}
			// Verify unwanted name is absent.
			if st.unwantedName != "" {
				for _, id := range result.ResourceIDs {
					if id == st.unwantedName {
						t.Errorf("ResourceIDs = %v, must NOT contain %q (wrong recipient scope)", result.ResourceIDs, st.unwantedName)
					}
				}
			}
			// No returned ID may be an ARN — guards against regression.
			for _, id := range result.ResourceIDs {
				if len(id) >= 4 && id[:4] == "arn:" {
					t.Errorf("ResourceIDs contains ARN %q — checker must return function names only", id)
				}
			}
			// Count must equal len(wantNames).
			if result.Count != len(st.wantNames) {
				t.Errorf("Count = %d, want %d", result.Count, len(st.wantNames))
			}
		})
	}
}

// TestCheckSESLambda_ExtractsFunctionNameFromARN verifies that a single LambdaAction
// with a full ARN is returned as the bare function name (last segment after "function:").
func TestCheckSESLambda_ExtractsFunctionNameFromARN(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("billing-rule"),
				Recipients: nil, // global — applies to any identity
				Actions: []sestypes.ReceiptAction{
					{LambdaAction: &sestypes.LambdaAction{
						FunctionArn: aws.String("arn:aws:lambda:us-west-2:111222333444:function:billing-webhook"),
					}},
				},
			},
		},
	}

	clients := sesV1Clients(&fakeSESV1{
		responses: []sesV1Response{{output: ruleSetOutput, err: nil}},
	})
	src := resource.Resource{
		ID:     "billing@example.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "billing-webhook" {
		t.Errorf("ResourceIDs = %v, want [\"billing-webhook\"] (bare function name, not ARN)", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// TestCheckSESS3_ScopesByRecipient is the mirror of TestCheckSESLambda_ScopesByRecipient
// using S3Action.BucketName instead of LambdaAction.FunctionArn.
// ---------------------------------------------------------------------------

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
		name           string
		resource       resource.Resource
		wantBuckets    []string
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

// ---------------------------------------------------------------------------
// Pin 1 (SES mirror) — Truncated cache-scan sets Approximate=true with matches
//
// Mirrors the DDB pin for SES checkers that do cache-scanning with truncation
// tracking: checkSESR53 and checkSESEbRule.
// Pre-fix: truncatedResultSES was NOT called on truncated+matches — relatedResult
// was used instead, yielding Approximate=false.
// Post-fix: truncated+matches → Approximate=true.
// ---------------------------------------------------------------------------

// TestCheckSESR53_TruncatedCacheWithMatches_ReturnsApproximate pins the
// truncated+matches path of checkSESR53. The r53 cache has IsTruncated=true
// and a zone whose name matches the identity domain.
// Pre-fix: result.Approximate==false.
// Post-fix: result.Approximate==true AND Count==1.
func TestCheckSESR53_TruncatedCacheWithMatches_ReturnsApproximate(t *testing.T) {
	// DOMAIN identity: domain is used as-is.
	src := resource.Resource{
		ID:   "acme-corp.com",
		Name: "acme-corp.com",
		Fields: map[string]string{
			"identity_type": "DOMAIN",
		},
	}
	zoneRes := resource.Resource{
		ID:   "/hostedzone/ZTRUNC001",
		Name: "acme-corp.com.", // trailing dot is stripped by the checker
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{zoneRes},
			IsTruncated: true, // later pages may contain additional zones
		},
	}

	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one matching zone in truncated r53 cache)", result.Count)
	}
	// Truncated+matches → must be Approximate=true so UI renders "(1+)" not "(1)".
	if !result.Approximate {
		t.Errorf("Approximate = false, want true — truncated r53 cache with matches must be approximate")
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == "/hostedzone/ZTRUNC001" {
			found = true
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain \"/hostedzone/ZTRUNC001\"", result.ResourceIDs)
	}
}

// TestCheckSESR53_TruncatedCacheNoMatches_ReturnsApproximateZero pins the
// truncated+no-matches path of checkSESR53. No zone matches the domain; the
// result must be Count==0 AND Approximate==true.
func TestCheckSESR53_TruncatedCacheNoMatches_ReturnsApproximateZero(t *testing.T) {
	src := resource.Resource{
		ID:   "acme-corp.com",
		Name: "acme-corp.com",
		Fields: map[string]string{
			"identity_type": "DOMAIN",
		},
	}
	nonMatchingZone := resource.Resource{
		ID:   "/hostedzone/ZOTHER",
		Name: "other-company.com.",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{nonMatchingZone},
			IsTruncated: true,
		},
	}

	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no zone matches in visible page)", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate = false, want true — truncated cache with zero visible matches must be approximate")
	}
}

// TestCheckSESEbRule_TruncatedCacheWithMatches_ReturnsApproximate pins the
// truncated+matches path of checkSESEbRule. The eb-rule cache has IsTruncated=true
// and a rule on the expected bus name.
// Pre-fix: result.Approximate==false.
// Post-fix: result.Approximate==true AND Count==1.
func TestCheckSESEbRule_TruncatedCacheWithMatches_ReturnsApproximate(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	// One rule on the "default" bus — but the cache is declared truncated.
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "rule-on-default", Name: "rule-on-default", Fields: map[string]string{"event_bus": "default"}},
			},
			IsTruncated: true,
		},
	}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one matching eb-rule in truncated cache)", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate = false, want true — truncated eb-rule cache with matches must be approximate")
	}
}

// TestCheckSESEbRule_TruncatedCacheNoMatches_ReturnsApproximateZero pins the
// truncated+no-matches path of checkSESEbRule.
func TestCheckSESEbRule_TruncatedCacheNoMatches_ReturnsApproximateZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	// A rule on the wrong bus — cannot match the SES identity's EventBridge bus.
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "rule-wrong-bus", Name: "rule-wrong-bus", Fields: map[string]string{"event_bus": "completely-different-bus"}},
			},
			IsTruncated: true,
		},
	}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no eb-rule on the expected bus in visible page)", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate = false, want true — truncated eb-rule cache with zero visible matches must be approximate")
	}
}
