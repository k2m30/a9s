package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

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

func sesFixtureSrcIdentity(identityName string) resource.Resource {
	return resource.Resource{
		ID:   identityName,
		Name: identityName,
		Fields: map[string]string{
			"identity_name": identityName,
			"identity_type": "domain",
		},
		RawStruct: sesv2types.IdentityInfo{
			IdentityName: aws.String(identityName),
			IdentityType: sesv2types.IdentityTypeDomain,
		},
	}
}

func sesFixtureClients() *awsclient.ServiceClients {
	f := fixtures.NewSESFixtures()
	return &awsclient.ServiceClients{
		SESv2: newFakeSESv2FromFixture(f),
	}
}

// newFakeSESv2FromFixture builds a fakeSESv2Checker that mirrors the fixture data:
//   - GetEmailIdentity for SESGraphRootIdentity → ConfigurationSetName = SESConfigSetName
//   - GetConfigurationSetEventDestinations for SESConfigSetName → fixture destinations
//   - All other identities return empty config set name
func newFakeSESv2FromFixture(f *fixtures.SESFixtures) *fakeSESv2Checker {
	return newFakeSESv2WithEventDestinations(
		fixtures.SESGraphRootIdentity,
		fixtures.SESConfigSetName,
		f.EventDestinationsByConfigSet[fixtures.SESConfigSetName].EventDestinations,
	)
}

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

func TestRelated_SES_EbRule_FixtureGraphRootReturnsRuleNames(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)
	cache := ebRuleCache()

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, cache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() <= 0 {
		t.Errorf("Count = %d, want > 0 (at least one rule on the default bus)", result.Count())
	}

	knownNames := ebRuleNamesOnDefaultBus()
	for _, id := range result.ResourceIDs() {
		if len(id) >= 4 && id[:4] == "arn:" {
			t.Errorf("ResourceID %q starts with 'arn:' — checker must return rule names, not ARNs", id)
		}
		if _, ok := knownNames[id]; !ok {
			t.Errorf("ResourceID %q is not a known EventBridge rule name from the fixture (known: %v)", id, knownNames)
		}
	}
}

func TestRelated_SES_EbRule_ScopeLimitedToBusName(t *testing.T) {
	// SES fixture ships to "default" bus.
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

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

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}

	for _, id := range result.ResourceIDs() {
		if id == "rule-on-custom" {
			t.Errorf("ResourceIDs = %v, must NOT contain 'rule-on-custom' (wrong bus)", result.ResourceIDs())
		}
	}

	wantIDs := []string{"rule-on-default", "another-default-rule"}
	for _, want := range wantIDs {
		found := false
		for _, id := range result.ResourceIDs() {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ResourceIDs = %v, want to contain %q (rule is on the default bus)", result.ResourceIDs(), want)
		}
	}
}

func TestRelated_SES_EbRule_NonGraphRootIdentityReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity("noreply@acme-corp.com")

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, ebRuleCache())

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no config set for non-graph-root identity)", result.Count())
	}
}

func TestRelated_SES_EbRule_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, ebRuleCache())

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}

// Without SESv2 there is no bus name to look up, so eb-rule answers an empty
// list rather than unknown.
func TestRelated_SES_EbRule_NilClientsReturnsZero(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, ebRuleCache())

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil clients → no bus names resolvable → honest 0)", result.Count())
	}
}

func TestRelated_SES_Sns_FixtureGraphRootMatchesOne(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one SnsDestination in fixture)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_SES_Sns_NonGraphRootIdentityReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity("suppressed@acme-corp.com")

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no config set for non-graph-root identity)", result.Count())
	}
}

func TestRelated_SES_Sns_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}

func TestRelated_SES_Sns_NilClientsReturnsNegOne(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

// No SES v1 client means a pure outbound SES account with no receipt rule
// set, which answers zero.
func TestRelated_SES_S3_NoSESv1ClientReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (c.SES == nil → no rule set → operator-honest 0)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_SES_S3_NilClientsReturnsNegOne(t *testing.T) {
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → type assertion failed)", result.Count())
	}
}

func TestRelated_SES_S3_FixtureAllIdentitiesWithNoSESv1ReturnZero(t *testing.T) {
	f := fixtures.NewSESFixtures()
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
		if result.Count() != 0 {
			t.Errorf("identity %q: Count = %d, want 0 (c.SES == nil → no rule set)", identityName, result.Count())
		}
	}
}

// Receipt-rule LambdaActions exist only in the SES v1 API, not SESv2.
func TestRelated_SES_Lambda_ValidRawStructReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (SES v1 API unavailable, valid RawStruct)", result.Count())
	}
}

func TestRelated_SES_Lambda_EmptyIDReturnsZero(t *testing.T) {
	clients := sesFixtureClients()
	src := resource.Resource{ID: ""}

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (graph-root domain matches fixture R53 zone)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "/hostedzone/ZFIXTURE" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/ZFIXTURE]", result.ResourceIDs())
	}
}

// An EMAIL_ADDRESS identity's zone is the domain after "@".
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (email address: domain extracted after '@')", result.Count())
	}
}

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

var _ awsclient.SESV1API = (*fakeSESV1)(nil)

// sesV1Clients wires v1 with a fresh session.RuleSetStore, so each test's
// rule-set cache is isolated.
func sesV1Clients(v1 awsclient.SESV1API) *awsclient.ServiceClients {
	c := &awsclient.ServiceClients{SES: v1}
	c.SetRuleSets(session.NewRuleSetStore())
	return c
}

func sesLambdaARN(name string) string {
	return "arn:aws:lambda:us-east-1:123456789012:function:" + name
}

func sesBucketName(name string) string {
	return "ses-inbound-" + name
}

func buildLambdaReceiptRule(name string, recipients []string, lambdaARN string) sestypes.ReceiptRule {
	return sestypes.ReceiptRule{
		Name:       aws.String(name),
		Recipients: recipients,
		Actions: []sestypes.ReceiptAction{
			{LambdaAction: &sestypes.LambdaAction{FunctionArn: aws.String(lambdaARN)}},
		},
	}
}

func buildS3ReceiptRule(name string, recipients []string, bucketName string) sestypes.ReceiptRule {
	return sestypes.ReceiptRule{
		Name:       aws.String(name),
		Recipients: recipients,
		Actions: []sestypes.ReceiptAction{
			{S3Action: &sestypes.S3Action{BucketName: aws.String(bucketName)}},
		},
	}
}

// Lambda IDs are function names (the segment after "function:"), matching
// the lambda fetcher's row IDs.

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
		name         string
		resource     resource.Resource
		wantNames    []string
		unwantedName string
	}{
		{
			name: "support@acme.com → global + support-router only",
			resource: resource.Resource{
				ID:     "support@acme.com",
				Fields: map[string]string{"identity_type": "email address"},
			},
			wantNames:    []string{"global-router", "support-router"},
			unwantedName: "sales-router",
		},
		{
			name: "billing@acme.com → global only (no specific rule matches)",
			resource: resource.Resource{
				ID:     "billing@acme.com",
				Fields: map[string]string{"identity_type": "email address"},
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
				Fields: map[string]string{"identity_type": "domain"},
			},
			wantNames:    []string{"global-router", "support-router", "sales-router"},
			unwantedName: "",
		},
		{
			name: "sales.acme.com (DOMAIN subdomain) → global + sales-router only",
			resource: resource.Resource{
				ID:     "sales.acme.com",
				Fields: map[string]string{"identity_type": "domain"},
			},
			wantNames:    []string{"global-router", "sales-router"},
			unwantedName: "support-router",
		},
	}

	for _, st := range subtests {
		st := st
		t.Run(st.name, func(t *testing.T) {
			// Each ServiceClients carries its own rule-set cache, so a fresh one starts
			// empty.
			clients := sesV1Clients(&fakeSESV1{
				responses: []sesV1Response{{output: ruleSetOutput, err: nil}},
			})
			result := checker(context.Background(), clients, st.resource, resource.ResourceCache{})
			if result.Err() != nil {
				t.Fatalf("unexpected error: %v", result.Err())
			}
			for _, wantName := range st.wantNames {
				found := false
				for _, id := range result.ResourceIDs() {
					if id == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ResourceIDs = %v, want to contain function name %q (not ARN)", result.ResourceIDs(), wantName)
				}
			}
			if st.unwantedName != "" {
				for _, id := range result.ResourceIDs() {
					if id == st.unwantedName {
						t.Errorf("ResourceIDs = %v, must NOT contain %q (wrong recipient scope)", result.ResourceIDs(), st.unwantedName)
					}
				}
			}
			for _, id := range result.ResourceIDs() {
				if len(id) >= 4 && id[:4] == "arn:" {
					t.Errorf("ResourceIDs contains ARN %q — checker must return function names only", id)
				}
			}
			if result.Count() != len(st.wantNames) {
				t.Errorf("Count = %d, want %d", result.Count(), len(st.wantNames))
			}
		})
	}
}

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
		Fields: map[string]string{"identity_type": "email address"},
	}

	checker := sesCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Fatalf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "billing-webhook" {
		t.Errorf("ResourceIDs = %v, want [\"billing-webhook\"] (bare function name, not ARN)", result.ResourceIDs())
	}
}

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
				Fields: map[string]string{"identity_type": "email address"},
			},
			wantBuckets:    []string{sesBucketName("global"), sesBucketName("support")},
			unwantedBucket: sesBucketName("sales"),
		},
		{
			name: "billing@acme.com → global bucket only",
			resource: resource.Resource{
				ID:     "billing@acme.com",
				Fields: map[string]string{"identity_type": "email address"},
			},
			wantBuckets:    []string{sesBucketName("global")},
			unwantedBucket: sesBucketName("support"),
		},
		{
			name: "acme.com (DOMAIN) → all buckets",
			resource: resource.Resource{
				ID:     "acme.com",
				Fields: map[string]string{"identity_type": "domain"},
			},
			wantBuckets:    []string{sesBucketName("global"), sesBucketName("support"), sesBucketName("sales")},
			unwantedBucket: "",
		},
		{
			name: "sales.acme.com (DOMAIN) → global + sales bucket only",
			resource: resource.Resource{
				ID:     "sales.acme.com",
				Fields: map[string]string{"identity_type": "domain"},
			},
			wantBuckets:    []string{sesBucketName("global"), sesBucketName("sales")},
			unwantedBucket: sesBucketName("support"),
		},
	}

	for _, st := range subtests {
		st := st
		t.Run(st.name, func(t *testing.T) {
			// Each ServiceClients carries its own rule-set cache, so a fresh one starts
			// empty.
			clients := sesV1Clients(&fakeSESV1{
				responses: []sesV1Response{{output: ruleSetOutput, err: nil}},
			})
			result := checker(context.Background(), clients, st.resource, resource.ResourceCache{})
			if result.Err() != nil {
				t.Fatalf("unexpected error: %v", result.Err())
			}
			for _, wantBucket := range st.wantBuckets {
				found := false
				for _, id := range result.ResourceIDs() {
					if id == wantBucket {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), wantBucket)
				}
			}
			if st.unwantedBucket != "" {
				for _, id := range result.ResourceIDs() {
					if id == st.unwantedBucket {
						t.Errorf("ResourceIDs = %v, must NOT contain %q (wrong recipient scope)", result.ResourceIDs(), st.unwantedBucket)
					}
				}
			}
			if result.Count() != len(st.wantBuckets) {
				t.Errorf("Count = %d, want %d", result.Count(), len(st.wantBuckets))
			}
		})
	}
}

// The active receipt rule set is sealed only on success: a transient error on
// the first call must not block later calls once the API recovers.

func TestSESActiveReceiptRuleSet_RetriesAfterTransientError(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			buildLambdaReceiptRule("inbound", nil, sesLambdaARN("inbound-handler")),
		},
	}

	v1Mock := &fakeSESV1{
		responses: []sesV1Response{
			{output: nil, err: errors.New("ses: temporary connection error")},
			{output: ruleSetOutput, err: nil},
		},
	}
	// The rule-set cache is keyed by the ServiceClients, so all three calls
	// share one.
	clients := sesV1Clients(v1Mock)

	src := resource.Resource{
		ID:     "support@acme.com",
		Fields: map[string]string{"identity_type": "email address"},
	}

	checker := sesCheckerByTarget(t, "lambda")

	result1 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result1.State() != domain.RelatedError {
		t.Errorf("call 1: State = %v, want RelatedError (transient API error)", result1.State())
	}

	result2 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result2.Count() != 1 {
		t.Errorf("call 2: Count = %d, want 1 (error should not be cached — must retry after transient failure)", result2.Count())
	}
	if result2.Err() != nil {
		t.Errorf("call 2: unexpected error: %v", result2.Err())
	}

	result3 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result3.Count() != 1 {
		t.Errorf("call 3: Count = %d, want 1 (success cached from call 2)", result3.Count())
	}
	if v1Mock.calls != 2 {
		t.Errorf("mock.calls = %d, want 2 (success must be memoized — call 3 must not hit the API again)", v1Mock.calls)
	}
}

func TestCheckSESR53_TruncatedCacheWithMatches_ReturnsTruncated(t *testing.T) {
	// DOMAIN identity: domain is used as-is.
	src := resource.Resource{
		ID:   "acme-corp.com",
		Name: "acme-corp.com",
		Fields: map[string]string{
			"identity_type": "domain",
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one matching zone in truncated r53 cache)", result.Count())
	}
	// Truncated with matches renders "(1+)", not "(1)".
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true — truncated r53 cache with matches must be truncated")
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "/hostedzone/ZTRUNC001" {
			found = true
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain \"/hostedzone/ZTRUNC001\"", result.ResourceIDs())
	}
}

func TestCheckSESR53_TruncatedCacheNoMatches_ReturnsTruncatedResult(t *testing.T) {
	src := resource.Resource{
		ID:   "acme-corp.com",
		Name: "acme-corp.com",
		Fields: map[string]string{
			"identity_type": "domain",
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no zone matches in visible page)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true — truncated cache with zero visible matches must be truncated")
	}
}

func TestCheckSESEbRule_TruncatedCacheWithMatches_ReturnsTruncated(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one matching eb-rule in truncated cache)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true — truncated eb-rule cache with matches must be truncated")
	}
}

func TestCheckSESEbRule_TruncatedCacheNoMatches_ReturnsTruncatedResult(t *testing.T) {
	clients := sesFixtureClients()
	src := sesFixtureSrcIdentity(fixtures.SESGraphRootIdentity)

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no eb-rule on the expected bus in visible page)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true — truncated eb-rule cache with zero visible matches must be truncated")
	}
}
