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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
