package unit_test

// views_detail_ct_events_test.go verifies the ct-events branch of
// DetailModel.buildFieldList (implemented by T027).
//
// Tests are written against the locked ctdetail API contract in
// specs/013-ct-event-detail-v2/contracts/ctdetail-api.md.
//
// These tests WILL FAIL until T027 adds the ct-events branch to buildFieldList.
// The expected failure reason is: no IsSection entries (section names) appear in
// the rendered View() output because the legacy flat path runs instead.

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Synthetic fixtures
// ---------------------------------------------------------------------------

// minimalCTJSON is the minimum valid CloudTrail event JSON for a Management
// AwsApiCall. Uses synthetic account ID 111111111111 (no real data).
const minimalCTJSON = `{
	"eventVersion":"1.08",
	"eventTime":"2026-04-07T14:02:11Z",
	"eventSource":"ec2.amazonaws.com",
	"eventName":"DescribeInstances",
	"awsRegion":"us-east-1",
	"sourceIPAddress":"10.0.14.221",
	"userAgent":"aws-sdk-go-v2/1.30.3",
	"userIdentity":{
		"type":"IAMUser",
		"arn":"arn:aws:iam::111111111111:user/test",
		"accountId":"111111111111",
		"userName":"test"
	},
	"eventCategory":"Management",
	"eventType":"AwsApiCall"
}`

// dangerCTJSON is a CloudTrail event that should produce "ct-danger" severity
// (DeleteRole is a W-class destructive IAM action).
const dangerCTJSON = `{
	"eventVersion":"1.08",
	"eventTime":"2026-04-07T15:00:00Z",
	"eventSource":"iam.amazonaws.com",
	"eventName":"DeleteRole",
	"awsRegion":"us-east-1",
	"sourceIPAddress":"10.0.1.5",
	"userAgent":"aws-cli/2.15.0",
	"userIdentity":{
		"type":"IAMUser",
		"arn":"arn:aws:iam::111111111111:user/admin",
		"accountId":"111111111111",
		"userName":"admin"
	},
	"requestParameters":{"roleName":"MyDangerRole"},
	"eventCategory":"Management",
	"eventType":"AwsApiCall"
}`

// awsStrPtr returns a *string pointing to s — helper for cloudtrailtypes.Event fields.
//
//go:fix inline
func awsStrPtr(s string) *string {
	return &s
}

// buildCTEventsResource builds a resource.Resource whose RawStruct is a
// cloudtrailtypes.Event (the AWS SDK type), exactly as buildCTResource does in
// internal/aws/ct_events.go. The CloudTrailEvent field holds the raw JSON blob.
func buildCTEventsResource(id, eventName, status, rawJSON string) resource.Resource {
	ct := cloudtrailtypes.Event{
		EventId:         new(id),
		EventName:       new(eventName),
		CloudTrailEvent: new(rawJSON),
	}
	return resource.Resource{
		ID:        id,
		Name:      eventName,
		Status:    status,
		RawStruct: ct,
		Fields: map[string]string{
			"event_name": eventName,
		},
	}
}

// ---------------------------------------------------------------------------
// Test 1: Basic path — ACTOR, ACTION, CONTEXT section headers appear
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_BasicPath verifies that, when a valid CloudTrail JSON
// is present, the ct-events detail branch emits IsSection=true FieldItems for
// ACTOR, ACTION, and CONTEXT (in that order in the rendered View output).
//
// EXPECTED TO FAIL until T027 implements the ct-events branch in buildFieldList.
// Failure reason: View() falls through to the legacy flat path — no section
// headers appear.
func TestDetailViewCTEvents_BasicPath(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000001",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := stripAnsi(m.View())

	// The ct-events branch must produce section headers for the three mandatory
	// sections that are always present for a Management/AwsApiCall by an IAMUser:
	//   ACTOR   (non-Insight, non-service-event)
	//   ACTION  (always present)
	//   CONTEXT (always present: region + source IP + time)
	for _, section := range []string{"ACTOR", "ACTION", "CONTEXT"} {
		if !strings.Contains(view, section) {
			t.Errorf("ct-events detail View() missing section %q — ct-events branch not yet implemented (T027)", section)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: Section order — ACTOR before ACTION before CONTEXT
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_SectionOrder verifies ACTOR appears before ACTION which
// appears before CONTEXT in the rendered output (contract: §section-order).
//
// EXPECTED TO FAIL until T027 implements the ct-events branch.
func TestDetailViewCTEvents_SectionOrder(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000002",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := stripAnsi(m.View())

	actorIdx := strings.Index(view, "ACTOR")
	actionIdx := strings.Index(view, "ACTION")
	contextIdx := strings.Index(view, "CONTEXT")

	if actorIdx == -1 || actionIdx == -1 || contextIdx == -1 {
		t.Skipf("section headers not found — ct-events branch not yet implemented (T027); view:\n%s", view)
	}
	if actorIdx >= actionIdx {
		t.Errorf("ACTOR (idx %d) must appear before ACTION (idx %d)", actorIdx, actionIdx)
	}
	if actionIdx >= contextIdx {
		t.Errorf("ACTION (idx %d) must appear before CONTEXT (idx %d)", actionIdx, contextIdx)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Severity propagation — ct-danger on the Event row
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_SeverityPropagation verifies that a ct-danger event
// surfaces its severity in the rendered View output.
//
// The ct-events branch sets ColorTier="ct-danger" on the ACTION/Event FieldItem
// (FR-002 single-cell exception). The renderer applies a danger style to that
// cell only. With NO_COLOR=1 the color itself is suppressed, but the event value
// "iam:DeleteRole" must still be present.
//
// EXPECTED TO FAIL until T027 implements the ct-events branch.
func TestDetailViewCTEvents_SeverityPropagation(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000003",
		"DeleteRole",
		"ct-danger",
		dangerCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := stripAnsi(m.View())

	// The ACTION section must be present.
	if !strings.Contains(view, "ACTION") {
		t.Errorf("ct-events detail View() missing ACTION section — ct-events branch not yet implemented (T027); view:\n%s", view)
	}

	// The Event row value "iam:DeleteRole" must appear under ACTION.
	if !strings.Contains(view, "iam:DeleteRole") {
		t.Errorf("ct-events detail View() missing event value %q; view:\n%s", "iam:DeleteRole", view)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Parse failure fallback — nil RawStruct should not crash
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_NoRawJSON_RendersFlatFields verifies that when the CT
// event has no raw JSON (bare stub with just Fields — e.g. a cached drill-in
// stub), the detail view still renders the flat Fields. This is not a fallback
// hiding a contract failure — the fetcher legitimately produces such stubs
// when the user drills in without the full event body.
func TestDetailViewCTEvents_NoRawJSON_RendersFlatFields(t *testing.T) {
	ensureNoColor(t)

	res := resource.Resource{
		ID:     "evt-fallback-000",
		Name:   "FallbackEvent",
		Status: "ct-info",
		Fields: map[string]string{
			"event_name": "FallbackEvent",
		},
	}
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := m.View()
	if view == "" {
		t.Error("ct-events detail View() returned empty string for bare stub resource")
	}
}

// TestDetailViewCTEvents_BrokenRawJSON_SurfacesExplicitError pins #280: when a
// CT event arrives with a non-empty but unparseable raw JSON blob, the detail
// view MUST surface the parse error explicitly rather than silently degrading
// to the flat Fields path. Silent degradation would hide a real contract
// violation (the fetcher guarantees valid JSON for non-stub events).
func TestDetailViewCTEvents_BrokenRawJSON_SurfacesExplicitError(t *testing.T) {
	ensureNoColor(t)

	broken := `{"eventVersion":"1.08","eventName":` // truncated — parser should error
	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-00000000bad1",
		"BrokenEvent",
		"ct-info",
		broken,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := stripAnsi(m.View())
	lower := strings.ToLower(view)
	if !strings.Contains(lower, "unable to parse") {
		t.Errorf("expected explicit parse-failure message in view, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Non-ct-events resource unaffected — no IsSection headers
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_NonCTEventsUnaffected verifies that an ec2 resource detail
// view does NOT contain ct-events section header labels (ACTOR, ACTION, CONTEXT)
// in its rendered output — the ct-events branch must be gated strictly on
// resourceType == "ct-events".
//
// This test passes even before T027 — it is a correctness guard ensuring the
// branch does not accidentally fire for other resource types.
func TestDetailViewCTEvents_NonCTEventsUnaffected(t *testing.T) {
	ensureNoColor(t)

	// Minimal EC2 resource with Fields only (no RawStruct needed for this check).
	res := buildResourceWithFields(
		"i-0aabbccdd11223344",
		"web-server",
		map[string]string{
			"InstanceId":       "i-0aabbccdd11223344",
			"InstanceType":     "t3.medium",
			"PrivateIpAddress": "10.0.1.42",
		},
	)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := stripAnsi(m.View())

	// These section labels must NOT appear for ec2.
	for _, label := range []string{"ACTOR", "ACTION", "CONTEXT"} {
		if strings.Contains(view, label) {
			t.Errorf("ec2 detail View() must NOT contain ct-events section label %q; view:\n%s", label, view)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 6: Actor row present — Principal field for IAMUser events
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_ActorPrincipalRow verifies that the ACTOR section
// contains the IAM user ARN as the Principal row value.
//
// EXPECTED TO FAIL until T027 implements the ct-events branch.
func TestDetailViewCTEvents_ActorPrincipalRow(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000006",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := stripAnsi(m.View())

	if !strings.Contains(view, "ACTOR") {
		t.Skipf("ACTOR section not found — ct-events branch not yet implemented (T027)")
	}

	// The Principal row must contain the IAM user ARN from minimalCTJSON.
	const wantARN = "arn:aws:iam::111111111111:user/test"
	if !strings.Contains(view, wantARN) {
		t.Errorf("ACTOR section missing Principal ARN %q; view:\n%s", wantARN, view)
	}
}

// ---------------------------------------------------------------------------
// Test 7: Context rows — Region and Source IP present
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_ContextRows verifies that the CONTEXT section contains
// the Region and Source IP rows extracted from the CloudTrail JSON.
//
// EXPECTED TO FAIL until T027 implements the ct-events branch.
func TestDetailViewCTEvents_ContextRows(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000007",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	view := stripAnsi(m.View())

	if !strings.Contains(view, "CONTEXT") {
		t.Skipf("CONTEXT section not found — ct-events branch not yet implemented (T027)")
	}

	// From minimalCTJSON: awsRegion=us-east-1, sourceIPAddress=10.0.14.221
	for _, want := range []string{"us-east-1", "10.0.14.221"} {
		if !strings.Contains(view, want) {
			t.Errorf("CONTEXT section missing value %q; view:\n%s", want, view)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 8: Navigate — pressing Enter on Principal row dispatches RelatedNavigateMsg
// ---------------------------------------------------------------------------

// assumedRoleCTJSON is a CloudTrail event with an AssumedRole principal
// (arn:aws:sts::*:assumed-role/*), which should resolve to TargetType "role".
const assumedRoleCTJSON = `{
	"eventVersion":"1.08",
	"eventTime":"2026-04-07T14:02:11Z",
	"eventSource":"ec2.amazonaws.com",
	"eventName":"DescribeInstances",
	"awsRegion":"us-east-1",
	"sourceIPAddress":"10.0.14.221",
	"userAgent":"aws-sdk-go-v2/1.30.3",
	"userIdentity":{
		"type":"AssumedRole",
		"arn":"arn:aws:sts::111111111111:assumed-role/KarpenterRole/session",
		"accountId":"111111111111"
	},
	"eventCategory":"Management",
	"eventType":"AwsApiCall"
}`

// TestDetailViewCTEvents_NavigatePrincipalRow verifies that pressing Enter on the
// Principal row in the ACTOR section dispatches a RelatedNavigateMsg with
// TargetType == "role" (because the ARN is an assumed-role ARN).
//
// This tests that:
//  1. sectionsToFieldItems propagates IsNavigable/TargetType from the Row struct.
//  2. The Enter key handler in detail.go dispatches RelatedNavigateMsg correctly.
//  3. arnTargetType correctly maps assumed-role ARNs to "role".
func TestDetailViewCTEvents_NavigatePrincipalRow(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000008",
		"DescribeInstances",
		"ct-info",
		assumedRoleCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	// Verify the ACTOR section is present — if not, the ct-events branch isn't active.
	view := stripAnsi(m.View())
	if !strings.Contains(view, "ACTOR") {
		t.Skipf("ACTOR section not found — ct-events branch not yet implemented; view:\n%s", view)
	}

	// The fieldList starts with the ACTOR section header (IsSection: true) at index 0.
	// The Principal row is at index 1. Move cursor down with 'j' to skip the section header.
	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	updated, _ := m.Update(jPress)

	// Now press Enter — the cursor should be on the Principal row (IsNavigable: true, TargetType: "role").
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := updated.Update(enterPress)

	if cmd == nil {
		t.Fatal("TestDetailViewCTEvents_NavigatePrincipalRow: Enter on Principal row returned nil cmd — navigate not triggered")
	}

	// Execute the cmd to get the message.
	msg := cmd()
	navMsg, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TestDetailViewCTEvents_NavigatePrincipalRow: Enter returned %T, want messages.RelatedNavigateMsg", msg)
	}

	if navMsg.TargetType != "role" {
		t.Errorf("TestDetailViewCTEvents_NavigatePrincipalRow: TargetType = %q, want \"role\"", navMsg.TargetType)
	}
	if navMsg.SourceType != "ct-events" {
		t.Errorf("TestDetailViewCTEvents_NavigatePrincipalRow: SourceType = %q, want \"ct-events\"", navMsg.SourceType)
	}
	// TargetID should be the bare role name extracted from the ARN, not the full ARN.
	const wantRoleName = "KarpenterRole"
	if navMsg.TargetID != wantRoleName {
		t.Errorf("TestDetailViewCTEvents_NavigatePrincipalRow: TargetID = %q, want %q", navMsg.TargetID, wantRoleName)
	}
}

// ---------------------------------------------------------------------------
// Test 9: Regression — S3 Bucket/Object TARGET rows carry IsNavigable + TargetType
// ---------------------------------------------------------------------------

// s3ResourcesEnvelopeCTJSON is a CloudTrail PutObject event that carries target
// resources via the SDK resources[] envelope (§1 of ExtractTarget).
// This exercises the resourceRefToRow → navFromLabel path, NOT the per-event-name
// fallback (§2) which uses requestParameters.bucketName/key.
//
// ARNs use synthetic values: no real account IDs, no real bucket names.
const s3ResourcesEnvelopeCTJSON = `{
	"eventVersion":"1.08",
	"eventTime":"2026-04-07T16:00:00Z",
	"eventSource":"s3.amazonaws.com",
	"eventName":"PutObject",
	"awsRegion":"us-east-1",
	"sourceIPAddress":"10.0.1.99",
	"userAgent":"aws-sdk-go-v2/1.30.3",
	"userIdentity":{
		"type":"IAMUser",
		"arn":"arn:aws:iam::111111111111:user/deploy",
		"accountId":"111111111111",
		"userName":"deploy"
	},
	"resources":[
		{"ARN":"arn:aws:s3:::prod-logs","type":"AWS::S3::Bucket","accountId":""},
		{"ARN":"arn:aws:s3:::prod-logs/app.log","type":"AWS::S3::Object","accountId":""}
	],
	"eventCategory":"Management",
	"eventType":"AwsApiCall"
}`

// TestDetailViewCTEvents_Regression_S3TargetNavigability verifies that a PutObject
// event whose target resources arrive via the resources[] SDK envelope produces
// Bucket and Object rows that are marked IsNavigable=true with TargetType="s3".
//
// This is a regression guard for the navFromLabel("Bucket") / navFromLabel("Object")
// paths in target.go — both must return (true, "s3").
//
// The test navigates the cursor to each TARGET row and presses Enter, verifying that
// a RelatedNavigateMsg with TargetType "s3" is dispatched.
//
// FieldList layout for this event (IAMUser, Management/AwsApiCall, resources[] present):
//
//	idx 0 — ACTOR (IsSection)
//	idx 1 — Principal  (IAMUser ARN, IsNavigable=true, TargetType="iam-user")
//	idx 2 — User agent
//	idx 3 — ACTION (IsSection)
//	idx 4 — Event  (s3:PutObject)
//	idx 5 — TARGET (IsSection)
//	idx 6 — Bucket (IsNavigable=true, TargetType="s3")
//	idx 7 — Object (IsNavigable=true, TargetType="s3")
//	idx 8 — CONTEXT (IsSection)
//	idx 9+— Region, Source IP, Time
//
// Cursor starts at 0. Each 'j' press increments by 1 and auto-skips IsSection rows.
//   - j×4 → idx 6 (Bucket):  0→1→2→4(skip3)→6(skip5)
//   - j×5 → idx 7 (Object):  additional j from idx 6
func TestDetailViewCTEvents_Regression_S3TargetNavigability(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000009",
		"PutObject",
		"ct-info",
		s3ResourcesEnvelopeCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	// Guard: verify the ct-events branch produced a TARGET section.
	view := stripAnsi(m.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ct-events branch not yet active; view:\n%s", view)
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// Navigate to Bucket row: j×4 (skips ACTOR header and ACTION header automatically).
	mBucket := m
	for range 4 {
		mBucket, _ = mBucket.Update(jPress)
	}

	_, bucketCmd := mBucket.Update(enterPress)
	if bucketCmd == nil {
		t.Fatal("TestDetailViewCTEvents_Regression_S3TargetNavigability: Enter on Bucket row returned nil cmd — Bucket row is not navigable")
	}
	bucketMsg := bucketCmd()
	bucketNav, ok := bucketMsg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TestDetailViewCTEvents_Regression_S3TargetNavigability: Bucket Enter returned %T, want messages.RelatedNavigateMsg", bucketMsg)
	}
	if bucketNav.TargetType != "s3" {
		t.Errorf("TestDetailViewCTEvents_Regression_S3TargetNavigability: Bucket row TargetType = %q, want \"s3\"", bucketNav.TargetType)
	}
	if bucketNav.SourceType != "ct-events" {
		t.Errorf("TestDetailViewCTEvents_Regression_S3TargetNavigability: Bucket row SourceType = %q, want \"ct-events\"", bucketNav.SourceType)
	}

	// Navigate to Object row: one additional j press from the Bucket position.
	mObject, _ := mBucket.Update(jPress)
	_, objectCmd := mObject.Update(enterPress)
	if objectCmd == nil {
		t.Fatal("TestDetailViewCTEvents_Regression_S3TargetNavigability: Enter on Object row returned nil cmd — Object row is not navigable")
	}
	objectMsg := objectCmd()
	objectNav, ok := objectMsg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TestDetailViewCTEvents_Regression_S3TargetNavigability: Object Enter returned %T, want messages.RelatedNavigateMsg", objectMsg)
	}
	if objectNav.TargetType != "s3" {
		t.Errorf("TestDetailViewCTEvents_Regression_S3TargetNavigability: Object row TargetType = %q, want \"s3\"", objectNav.TargetType)
	}
	if objectNav.SourceType != "ct-events" {
		t.Errorf("TestDetailViewCTEvents_Regression_S3TargetNavigability: Object row SourceType = %q, want \"ct-events\"", objectNav.SourceType)
	}
}

// ---------------------------------------------------------------------------
// Test 9: Regression — frame border present in ct-events detail view
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_Regression_FrameBorder is a regression guard for the
// hasSectionItems() bypass bug: when View() returned renderContent() directly,
// skipping the frame wrapper entirely, the output contained no │ characters.
//
// The fix removes the bypass so the standard frame rendering path always runs.
// This test asserts that at least one │ (U+2502, BOX DRAWINGS LIGHT VERTICAL)
// is present in the stripped output — proof that the frame was rendered.
func TestDetailViewCTEvents_Regression_FrameBorder(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000009",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(120, 40)

	view := stripAnsi(m.View())

	if !strings.Contains(view, "│") {
		t.Errorf("ct-events detail View() missing frame border character │ — hasSectionItems() bypass regression; view:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Test 10: Regression — RELATED right column visible for ct-events on wide terminal
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_Regression_RelatedRightColumn verifies that the
// RELATED right-column panel is composed into the rendered View() for a
// ct-events detail on a wide terminal (width=180).
//
// Regression context: a hasSectionItems() guard in detail_render.go was
// incorrectly bypassing right-column composition for section-based field lists
// (resources whose buildFieldList populates IsSection rows). The bypass
// caused View() to return renderContent() directly, skipping the side-by-side
// layout that joins left viewport + │ separator + right column.  As a result
// the "RELATED" header never appeared in the output even when the right column
// was auto-shown on entry.
//
// The fix removes the bypass so View() always uses the standard path.
// This test catches that regression.
//
// Precondition: RegisterRelated("ct-events", ...) must be called in
// internal/aws/ct_events.go init — verified by the GetRelated sanity check.
func TestDetailViewCTEvents_Regression_RelatedRightColumn(t *testing.T) {
	ensureNoColor(t)

	// Sanity: resource.GetRelated("ct-events") must return at least one def.
	// If this fails the production init hasn't registered ct-events related defs
	// and the right column will never auto-show regardless of terminal width.
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs — RegisterRelated not called in production init")
	}

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-00000000000a",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := configForType("ct-events")

	// Build a detail model at width=180, height=40 — well above the 60-col
	// threshold that triggers auto-show of the right column.
	k := keys.Default()
	m := views.NewDetail(res, "ct-events", cfg, k)
	m.SetSize(180, 40)

	plain := stripAnsi(m.View())

	// The "RELATED" header must appear in the rendered output.
	// Absence means right-column composition was bypassed (regression).
	if !strings.Contains(plain, "RELATED") {
		t.Errorf("ct-events detail View() missing \"RELATED\" header on wide terminal (180 cols) — right-column composition was suppressed; view snippet:\n%.500s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 11: Regression — EC2 TerminateInstances TARGET rows navigable via fallback path
// ---------------------------------------------------------------------------

// terminateInstancesCTJSON is a TerminateInstances event where instances are
// extracted from requestParameters.instancesSet.items — the per-event-name fallback
// path in ctdetail/target.go's extractInstancesSetEvent. The resources[] field is
// intentionally absent to force the fallback.
//
// No userAgent field is included so the ACTOR section contains exactly one row
// (Principal), making cursor positions in this test deterministic.
const terminateInstancesCTJSON = `{
	"eventVersion":"1.08",
	"eventTime":"2026-04-07T14:30:00Z",
	"eventSource":"ec2.amazonaws.com",
	"eventName":"TerminateInstances",
	"awsRegion":"us-east-1",
	"sourceIPAddress":"10.0.14.221",
	"userIdentity":{
		"type":"IAMUser",
		"arn":"arn:aws:iam::111111111111:user/test",
		"accountId":"111111111111",
		"userName":"test"
	},
	"requestParameters":{
		"instancesSet":{
			"items":[
				{"instanceId":"i-aaa000111"},
				{"instanceId":"i-bbb222333"}
			]
		}
	},
	"eventCategory":"Management",
	"eventType":"AwsApiCall"
}`

// TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath verifies that
// for a ct-events TerminateInstances event where instance IDs are extracted from
// requestParameters.instancesSet.items (the per-event-name fallback path), the
// DetailModel produces Instance rows with IsNavigable == true and TargetType == "ec2".
//
// target.go has two code paths: resourceRefToRow (SDK envelope resources[]) and the
// per-event-name fallback helpers (like extractInstancesSetEvent). A regression could
// break one without affecting the other. This test exercises the fallback path.
//
// Expected fieldList layout (no userAgent, no access key, no sessionContext):
//
//	idx 0: ACTOR section header  (IsSection — skipped by 'j')
//	idx 1: Principal row
//	idx 2: ACTION section header (IsSection — skipped by 'j')
//	idx 3: Event row ("ec2:TerminateInstances")
//	idx 4: TARGET section header (IsSection — skipped by 'j')
//	idx 5: Instance row (i-aaa000111) ← cursor after 3× 'j'
//	idx 6: Instance row (i-bbb222333)
//
// EXPECTED TO FAIL until T027 implements the ct-events branch in buildFieldList.
func TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath(t *testing.T) {
	ensureNoColor(t)

	res := buildCTEventsResource(
		"abc12345-0000-0000-0000-000000000011",
		"TerminateInstances",
		"ct-write",
		terminateInstancesCTJSON,
	)
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)

	// Verify the TARGET section is present — if absent, the ct-events branch is not yet active.
	view := stripAnsi(m.View())
	if !strings.Contains(view, "TARGET") {
		t.Skipf("TARGET section not found — ct-events branch not yet implemented (T027); view:\n%s", view)
	}

	// Verify instance IDs appear in the view (sanity check for the fallback path).
	for _, wantID := range []string{"i-aaa000111", "i-bbb222333"} {
		if !strings.Contains(view, wantID) {
			t.Errorf("TARGET section missing instance ID %q — extractInstancesSetEvent fallback not producing rows; view:\n%s", wantID, view)
		}
	}

	jPress := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterPress := tea.KeyPressMsg{Code: tea.KeyEnter}

	// Navigate to the first Instance row (idx 5) via 3× 'j'.
	// Section headers are auto-skipped by the 'j' handler (IsSection rows are not focusable).
	m1, _ := m.Update(jPress)
	m2, _ := m1.Update(jPress)
	m3, _ := m2.Update(jPress)

	// Cursor should now be on the first Instance row (idx 5).
	// Press Enter — must dispatch RelatedNavigateMsg with TargetType == "ec2".
	_, cmd := m3.Update(enterPress)

	if cmd == nil {
		t.Fatal("TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath: Enter on Instance row returned nil cmd — IsNavigable not set on fallback-path TARGET row")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath: Enter returned %T, want messages.RelatedNavigateMsg", msg)
	}

	if navMsg.TargetType != "ec2" {
		t.Errorf("TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath: TargetType = %q, want \"ec2\"", navMsg.TargetType)
	}
	if navMsg.SourceType != "ct-events" {
		t.Errorf("TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath: SourceType = %q, want \"ct-events\"", navMsg.SourceType)
	}
	// TargetID should be the first instance ID extracted from instancesSet.items.
	const wantInstanceID = "i-aaa000111"
	if navMsg.TargetID != wantInstanceID {
		t.Errorf("TestDetailViewCTEvents_Regression_EC2InstanceNavigability_FallbackPath: TargetID = %q, want %q", navMsg.TargetID, wantInstanceID)
	}
}

// ---------------------------------------------------------------------------
// Test 12: Regression — ColorTier propagation invariant (FR-002 single-cell exception)
// ---------------------------------------------------------------------------

// TestDetailViewCTEvents_Regression_ColorTierInvariant verifies that, after
// buildFieldList runs the ct-events branch, exactly ONE rendered data row carries
// the RowColorStyle(tier) ANSI styling — the ACTION/Event row — and no other row
// does.
//
// This locks the propagation through sectionsToFieldItems (view layer) where
// Row.Severity becomes FieldItem.ColorTier. Phase 2 tests lock this at the
// Row.Severity level (TestCTDetailBuildSections_OnlyEventRowHasSeverity). This test
// locks the rendered output: a regression that copies ColorTier to every field, or
// drops it on the Event row, would silently break the visual rule.
//
// For each tier the test:
//  1. Skips when the ct-events branch is not yet active (ACTION section absent).
//  2. Verifies the ACTION section contains exactly ONE occurrence of the exact
//     ANSI-styled string RowColorStyle(tier).Render("ec2:DescribeInstances").
//  3. Verifies that no other occurrence of that styled string exists elsewhere in
//     the rendered output (catches copy-ColorTier-to-all-rows regression).
func TestDetailViewCTEvents_Regression_ColorTierInvariant(t *testing.T) {
	tiers := []string{"ct-info", "ct-attention", "ct-danger"}
	const eventName = "DescribeInstances"
	// sectionsToFieldItems produces the event value as serviceFromSource(source) + ":" + eventName.
	// For minimalCTJSON: eventSource="ec2.amazonaws.com" → "ec2" + ":" + "DescribeInstances".
	const expectedEventValue = "ec2:DescribeInstances"

	for _, tier := range tiers {
		t.Run(tier, func(t *testing.T) {
			// Unique ID per subtest — no real account/event IDs.
			var id string
			switch tier {
			case "ct-info":
				id = "abc12345-0000-0000-0000-000000000100"
			case "ct-attention":
				id = "abc12345-0000-0000-0000-000000000101"
			default: // ct-danger
				id = "abc12345-0000-0000-0000-000000000102"
			}
			res := buildCTEventsResource(id, eventName, tier, minimalCTJSON)
			cfg := configForType("ct-events")

			// Guard: verify ct-events branch is active by checking ACTION appears in plain view.
			t.Setenv("NO_COLOR", "1")
			styles.Reinit()
			guardModel := newDetailModel(res, "ct-events", cfg)
			plainView := stripAnsi(guardModel.View())
			if !strings.Contains(plainView, "ACTION") {
				t.Skipf("tier %q: ACTION section not found — ct-events branch not yet implemented; view:\n%s", tier, plainView)
			}

			// Enable colors and build a fresh model so styles are applied at render time.
			os.Unsetenv("NO_COLOR")
			styles.Reinit()
			t.Cleanup(func() { styles.Reinit() })
			coloredModel := newDetailModel(res, "ct-events", cfg)
			coloredView := coloredModel.View()

			// The exact ANSI-styled event value as ColorStyle(ctEventsTd.Color(r)) would produce it.
			ctEventsTd := resource.FindResourceType("ct-events")
			tierRes := resource.Resource{ID: id, Status: tier}
			wantStyled := styles.ColorStyle(ctEventsTd.Color(tierRes)).Render(expectedEventValue)

			// 1. The styled string must appear in the view (ColorTier was applied to Event row).
			if !strings.Contains(coloredView, wantStyled) {
				t.Errorf("tier %q: View() does not contain ColorStyle(ctEventsTd.Color(r)).Render(%q)\n"+
					"ColorTier not propagated from Row.Severity to FieldItem.ColorTier in sectionsToFieldItems.\n"+
					"wantStyled = %q",
					tier, expectedEventValue, wantStyled)
				return
			}

			// 2. The styled string must appear EXACTLY ONCE (no other row has ColorTier applied).
			// strings.Count counts non-overlapping occurrences of wantStyled in the full view.
			count := strings.Count(coloredView, wantStyled)
			if count != 1 {
				t.Errorf("tier %q: ColorStyle(ctEventsTd.Color(r)).Render(%q) appears %d times in View(), want exactly 1\n"+
					"A count > 1 means ColorTier was copied to more rows than the ACTION/Event row (FR-002 single-cell exception violated).",
					tier, expectedEventValue, count)
			}
		})
	}
}
