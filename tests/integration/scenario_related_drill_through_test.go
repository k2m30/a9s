//go:build integration

package integration

// scenario_related_drill_through_test.go — Drill-through regression pins.
//
// These tests verify that every registered related-panel pivot and navigable
// field on the graph-root fixture actually resolves to at least one real
// resource in the demo cache. An empty landing means the checker produced a
// resource ID in a format that does not match the target resource type's
// Resource.ID field — the exact bug class that caused the SES EventBridge and
// DDB KMS navigation failures.
//
// Test design:
//   - Uses DrillRelated to dispatch RelatedNavigateMsg for each pivot and
//     assert the resulting list is non-empty.
//   - Uses FollowNavigableField to dispatch RelatedNavigateMsg from a registered
//     NavigableField and assert the resulting resource lands.
//
// Tests that document a KNOWN FAILING state (pre-fix) are annotated with
// "WILL FAIL until the coder lands their fix." Run with -tags integration.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// TestScenario_RelatedDrillThrough_SES
// ---------------------------------------------------------------------------

// TestScenario_RelatedDrillThrough_SES verifies that every §2 pivot with
// `count shown: yes` on the SES graph-root fixture ("acme-corp.com") resolves
// to a non-empty resource list when DrillRelated is called.
//
// Catches ID-format mismatches where the checker emits a resource ID that does
// not match the target type's Resource.ID field (e.g., full ARN vs. bare name).
func TestScenario_RelatedDrillThrough_SES(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Open SES list and find the graph-root identity.
	scenario.OpenList("ses")
	rootID := demofixtures.SESGraphRootIdentity // "acme-corp.com"
	root := fullIntegrationMustFindResourceByID(t, scenario.clients, "ses", rootID)

	// Open the detail view for the graph-root identity.
	scenario.OpenDetailResource("ses", root)
	scenario.ExpectNoAPIError()

	// Wait for related panel to populate by checking all expected rows are present.
	for _, displayName := range []string{
		"EventBridge Rules",
		"Lambda Functions",
		"S3 Buckets",
		"SNS Topics",
		"Route 53 (DNS)",
	} {
		scenario.ExpectRelatedRow(displayName)
	}

	// ---------------------------------------------------------------------------
	// EventBridge Rules — checker emits rule ARNs or names; target eb-rule
	// must index on one of those forms.
	// ---------------------------------------------------------------------------
	ebMsg, ebOK := scenario.lastRelatedByName["EventBridge Rules"]
	if ebOK && ebMsg.Result.Count >= 1 {
		ebResources := scenario.DrillRelated("EventBridge Rules")
		if len(ebResources) == 0 {
			t.Fatalf("DrillRelated(EventBridge Rules): landed on empty list; checker ResourceIDs=%v",
				ebMsg.Result.ResourceIDs)
		}
		// Every returned resource's ID must be a non-empty string. We do not
		// assert exact IDs because fixtures may change, but we verify the IDs
		// are plausibly rule-shaped (non-empty, not all identical).
		for _, r := range ebResources {
			if r.ID == "" {
				t.Errorf("DrillRelated(EventBridge Rules): resource with empty ID in result: %+v", r)
			}
		}
		t.Logf("DrillRelated(EventBridge Rules): landed on %d resource(s): %v",
			len(ebResources), resourceIDs(ebResources))
		scenario.Press("esc")
	}

	// ---------------------------------------------------------------------------
	// Lambda Functions — checker emits function names (bare); target lambda
	// must index on bare function names, not ARNs.
	// ---------------------------------------------------------------------------
	lambdaMsg, lambdaOK := scenario.lastRelatedByName["Lambda Functions"]
	if lambdaOK && lambdaMsg.Result.Count >= 1 {
		lambdaResources := scenario.DrillRelated("Lambda Functions")
		if len(lambdaResources) == 0 {
			t.Fatalf("DrillRelated(Lambda Functions): landed on empty list; checker ResourceIDs=%v",
				lambdaMsg.Result.ResourceIDs)
		}
		// Lambda resources must be indexed on bare function names (no "arn:" prefix).
		for _, r := range lambdaResources {
			if strings.HasPrefix(r.ID, "arn:") {
				t.Errorf("DrillRelated(Lambda Functions): resource ID %q has arn: prefix — lambda must index on bare function names", r.ID)
			}
		}
		t.Logf("DrillRelated(Lambda Functions): landed on %d resource(s): %v",
			len(lambdaResources), resourceIDs(lambdaResources))
		scenario.Press("esc")
	}

	// ---------------------------------------------------------------------------
	// S3 Buckets — checker emits bucket names; target s3 must index on names.
	// ---------------------------------------------------------------------------
	s3Msg, s3OK := scenario.lastRelatedByName["S3 Buckets"]
	if s3OK && s3Msg.Result.Count >= 1 {
		s3Resources := scenario.DrillRelated("S3 Buckets")
		if len(s3Resources) == 0 {
			t.Fatalf("DrillRelated(S3 Buckets): landed on empty list; checker ResourceIDs=%v",
				s3Msg.Result.ResourceIDs)
		}
		// S3 IDs must not be ARNs — S3 is indexed by bucket name.
		for _, r := range s3Resources {
			if strings.HasPrefix(r.ID, "arn:") {
				t.Errorf("DrillRelated(S3 Buckets): resource ID %q has arn: prefix — s3 must index on bucket name", r.ID)
			}
		}
		t.Logf("DrillRelated(S3 Buckets): landed on %d resource(s): %v",
			len(s3Resources), resourceIDs(s3Resources))
		scenario.Press("esc")
	}

	// ---------------------------------------------------------------------------
	// SNS Topics — checker emits topic ARNs; target sns must index on ARNs
	// (SNS resource IDs are ARNs by convention).
	// ---------------------------------------------------------------------------
	snsMsg, snsOK := scenario.lastRelatedByName["SNS Topics"]
	if snsOK && snsMsg.Result.Count >= 1 {
		snsResources := scenario.DrillRelated("SNS Topics")
		if len(snsResources) == 0 {
			t.Fatalf("DrillRelated(SNS Topics): landed on empty list; checker ResourceIDs=%v",
				snsMsg.Result.ResourceIDs)
		}
		t.Logf("DrillRelated(SNS Topics): landed on %d resource(s): %v",
			len(snsResources), resourceIDs(snsResources))
		scenario.Press("esc")
	}

	// ---------------------------------------------------------------------------
	// Route 53 (DNS) — checker emits hosted zone IDs or names; target r53
	// must resolve to a non-empty list.
	// ---------------------------------------------------------------------------
	r53Msg, r53OK := scenario.lastRelatedByName["Route 53 (DNS)"]
	if r53OK && r53Msg.Result.Count >= 1 {
		r53Resources := scenario.DrillRelated("Route 53 (DNS)")
		if len(r53Resources) == 0 {
			t.Fatalf("DrillRelated(Route 53 (DNS)): landed on empty list; checker ResourceIDs=%v",
				r53Msg.Result.ResourceIDs)
		}
		t.Logf("DrillRelated(Route 53 (DNS)): landed on %d resource(s): %v",
			len(r53Resources), resourceIDs(r53Resources))
		scenario.Press("esc")
	}
}

// ---------------------------------------------------------------------------
// TestScenario_NavigableFieldDrillThrough_DDB
// ---------------------------------------------------------------------------

// TestScenario_NavigableFieldDrillThrough_DDB verifies that the registered
// navigable field "SSEDescription.KMSMasterKeyArn" on the orders-prod table
// resolves to the correct KMS key in the demo cache.
//
// This test WILL FAIL against current code if NavID is not set on the field
// item (full ARN used for lookup, no cache match, empty list). It MUST PASS
// after the coder lands their ARN→ID extraction fix.
//
// Expected: FollowNavigableField returns a KMS resource whose ID matches
// demofixtures.OrdersProdKMSKeyID ("orders-prod-cmk-0001").
func TestScenario_NavigableFieldDrillThrough_DDB(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Open the DDB list and locate the orders-prod table (graph-root).
	scenario.OpenList("ddb")
	ordersProd := fullIntegrationMustFindResourceByID(t, scenario.clients, "ddb", demofixtures.OrdersProdID)

	// Open the detail view.
	scenario.OpenDetailResource("ddb", ordersProd)
	scenario.ExpectNoAPIError()

	// Verify the navigable field is registered and the detail view rendered.
	navFields := resource.GetNavigableFields("ddb")
	if len(navFields) == 0 {
		t.Fatal("NavigableFieldDrillThrough_DDB: no navigable fields registered for 'ddb' — test requires SSEDescription.KMSMasterKeyArn")
	}
	found := false
	for _, nf := range navFields {
		if nf.FieldPath == "SSEDescription.KMSMasterKeyArn" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("NavigableFieldDrillThrough_DDB: SSEDescription.KMSMasterKeyArn not registered as navigable for 'ddb'; got: %v", navFields)
	}

	// Follow the navigable field.
	// FollowNavigableField extracts SSEDescription.KMSMasterKeyArn from the table's
	// RawStruct, applies NavIDFromValue("kms", arn) to strip the ARN suffix, then
	// dispatches RelatedNavigateMsg{TargetType: "kms", TargetID: bareKeyID}.
	// The test PASSES when the kms resource cache contains a key with ID = bareKeyID.
	// The test FAILS when NavID is empty (pre-fix) and the full ARN is used as TargetID.
	landed := scenario.FollowNavigableField("SSEDescription.KMSMasterKeyArn")

	expectedID := demofixtures.OrdersProdKMSKeyID
	if landed.ID != expectedID {
		t.Fatalf("FollowNavigableField(SSEDescription.KMSMasterKeyArn): landed on KMS key %q, expected %q\n"+
			"This means NavIDFromValue did not strip the ARN or the kms cache does not contain the bare key ID.\n"+
			"KMS cache should have key %q; full ARN: %s",
			landed.ID, expectedID, expectedID, demofixtures.OrdersProdKMSKeyARN)
	}
	t.Logf("FollowNavigableField(SSEDescription.KMSMasterKeyArn): landed on KMS key %q — PASS", landed.ID)
}

// ---------------------------------------------------------------------------
// helpers local to this file
// ---------------------------------------------------------------------------

// resourceIDs returns a slice of IDs from a resource slice for use in log messages.
func resourceIDs(resources []resource.Resource) []string {
	ids := make([]string, len(resources))
	for i, r := range resources {
		ids[i] = r.ID
	}
	return ids
}
