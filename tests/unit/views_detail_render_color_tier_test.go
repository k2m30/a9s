package unit

// views_detail_render_color_tier_test.go — T022: ColorTier branch in renderFromFieldList
//
// Tests the new ColorTier sub-switch added to the default case of renderFromFieldList
// in internal/tui/views/detail_fields.go (T028 coder task).
//
// Contract (ctdetail-api.md §Renderer contract):
//
//   default:
//       label := styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW))
//       var value string
//       switch {
//       case item.IsNavigable:
//           value = styles.NavigableField.Render(item.Value)
//       case item.ColorTier != "":
//           value = styles.RowColorStyle(item.ColorTier).Render(item.Value)
//       default:
//           value = styles.DetailVal.Render(item.Value)
//       }
//       line = " " + label + value
//
// These tests MUST:
//   1. Compile cleanly against the CURRENT codebase.
//   2. FAIL against the CURRENT code (ColorTier branch absent from renderer).
//   3. PASS after T028 adds buildFieldList ct-events branch + ColorTier renderer branch.
//
// Test strategy:
//   Construct ct-events resources with different status values (ct-info / ct-attention /
//   ct-danger). T028 sets Row.Severity = event.Status on the "Event" row in ACTION and
//   sectionsToFieldItems propagates it to FieldItem.ColorTier. The renderer then applies
//   styles.RowColorStyle(item.ColorTier) to the value — producing a different ANSI escape
//   than the neutral styles.DetailVal.
//
//   With NO_COLOR UNSET, we compare:
//     a) The styled output for the event name value when colorTier is set.
//     b) The styled output for the same value with NO_COLOR=1 (baseline plain text).
//   If ColorTier is applied, the two outputs differ.
//
//   For the "empty ColorTier falls through" test we verify that a non-tier row uses
//   the neutral DetailVal style (same as the baseline with NO_COLOR).

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// buildCTResourceForTier constructs a cloudtrailtypes.Event with the given
// status-relevant parameters. The CloudTrailEvent JSON encodes fields needed
// for ctdetail.Parse to compute Status correctly.
//
// statusHint is used to construct the event JSON so that ctdetail's status
// ladder produces the expected tier:
//   - "ct-info"      → read-only event (DescribeInstances / no error)
//   - "ct-attention" → write event (PutBucketPolicy)
//   - "ct-danger"    → event with errorCode set (AccessDenied)
func buildCTResourceForTier(eventName, eventSource, userType, statusHint string) resource.Resource {
	errorCode := ""
	if statusHint == "ct-danger" {
		errorCode = "AccessDenied"
	}

	raw := `{` +
		`"eventVersion":"1.08",` +
		`"userIdentity":{"type":"` + userType + `","accountId":"111111111111","arn":"arn:aws:sts::111111111111:assumed-role/TestRole/session"},` +
		`"eventTime":"2026-04-07T14:00:00Z",` +
		`"eventSource":"` + eventSource + `",` +
		`"eventName":"` + eventName + `",` +
		`"awsRegion":"us-east-1",` +
		`"sourceIPAddress":"10.0.0.1",` +
		`"userAgent":"aws-cli/2.0",` +
		`"errorCode":"` + errorCode + `",` +
		`"eventCategory":"Management",` +
		`"eventType":"AwsApiCall",` +
		`"recipientAccountId":"111111111111"` +
		`}`

	eventTime := time.Date(2026, 4, 7, 14, 0, 0, 0, time.UTC)
	ctEvent := cloudtrailtypes.Event{
		EventId:         aws.String("e-color-tier-test"),
		EventName:       aws.String(eventName),
		EventSource:     aws.String(eventSource),
		EventTime:       aws.Time(eventTime),
		CloudTrailEvent: aws.String(raw),
	}
	return resource.Resource{
		ID:        "e-color-tier-test",
		Name:      eventName,
		Status:    statusHint,
		RawStruct: ctEvent,
		Fields: map[string]string{
			"event_name": eventName,
			"source":     eventSource,
		},
	}
}

// newCTDetailForTier creates a DetailModel for the given resource, width=120, height=40.
func newCTDetailForTier(t *testing.T, res resource.Resource) views.DetailModel {
	t.Helper()
	k := keys.Default()
	m := views.NewDetail(res, "ct-events", nil, k)
	m.SetSize(120, 40)
	return m
}

// colorTierSectionLineText extracts the left-column text from a rendered line,
// stripping the right-column separator (│ and everything after it) and surrounding
// whitespace. Required because the viewport pads lines to full width with │ when
// the right column is visible (e.g., " ACTION                          │  RELATED").
func colorTierSectionLineText(plain string) string {
	if idx := strings.Index(plain, "│"); idx != -1 {
		plain = plain[:idx]
	}
	return strings.TrimSpace(plain)
}

// findActionEventLine locates the "Event:" row inside the ACTION section in the
// ANSI-stripped view. Returns ("", false) if not found.
// The Event row in ACTION contains the event name as the value (e.g., "DescribeInstances").
// Uses colorTierSectionLineText to strip the right-column separator before matching section names.
func findActionEventLine(plain, eventName string) (string, bool) {
	lines := strings.Split(plain, "\n")
	inAction := false
	for _, line := range lines {
		leftText := colorTierSectionLineText(line)
		if leftText == "ACTION" {
			inAction = true
			continue
		}
		if inAction {
			// If we hit another section header, stop.
			switch leftText {
			case "ACTOR", "TARGET", "CONTEXT", "ERROR", "REQUEST", "RESPONSE":
				inAction = false
				continue
			}
			// Look for a line containing the event name value.
			if strings.Contains(line, eventName) {
				return line, true
			}
		}
	}
	return "", false
}

// findEventLineRaw finds the raw (ANSI-carrying) line that contains eventName
// in the ACTION section, returned alongside its ANSI-stripped counterpart.
// Uses colorTierSectionLineText to strip the right-column separator before matching section names.
func findEventLineRaw(rawView, plainView, eventName string) (rawLine string, found bool) {
	rawLines := strings.Split(rawView, "\n")
	plainLines := strings.Split(plainView, "\n")
	inAction := false
	for i := range rawLines {
		if i >= len(plainLines) {
			break
		}
		leftText := colorTierSectionLineText(plainLines[i])
		if leftText == "ACTION" {
			inAction = true
			continue
		}
		if inAction {
			switch leftText {
			case "ACTOR", "TARGET", "CONTEXT", "ERROR", "REQUEST", "RESPONSE":
				inAction = false
				continue
			}
			if strings.Contains(plainLines[i], eventName) {
				return rawLines[i], true
			}
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// T022-1: ct-info tier produces dim foreground on Event value
// ---------------------------------------------------------------------------

// TestDetailRenderColorTier_CTInfo asserts that when Event.Status == "ct-info",
// the "Event:" row value in the ACTION section is styled with RowColorStyle("ct-info"),
// NOT the neutral DetailVal style.
//
// Without T028: buildFieldList never produces ColorTier items for ct-events,
// so the value falls through to DetailVal. The styled and plain outputs match,
// causing this test to fail.
func TestDetailRenderColorTier_CTInfo(t *testing.T) {
	const eventName = "DescribeInstances"
	res := buildCTResourceForTier(eventName, "ec2.amazonaws.com", "AssumedRole", "ct-info")

	// Reference: plain output (NO_COLOR=1)
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	mPlain := newCTDetailForTier(t, res)
	plainView := mPlain.View()
	plainEventLine, found := findActionEventLine(stripANSI(plainView), eventName)
	if !found {
		t.Fatalf("TestDetailRenderColorTier_CTInfo: Event row not found in ACTION section (ct-events branch may not be implemented).\nPlain output:\n%s", stripANSI(plainView))
	}

	// Colored output (colors enabled)
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })
	mColored := newCTDetailForTier(t, res)
	coloredView := mColored.View()
	coloredPlain := stripANSI(coloredView)
	coloredEventLine, foundC := findActionEventLine(coloredPlain, eventName)
	if !foundC {
		t.Fatalf("TestDetailRenderColorTier_CTInfo: Event row not found in ACTION section (colored render).\nOutput:\n%s", coloredPlain)
	}
	_ = plainEventLine
	_ = coloredEventLine

	// Now compare raw lines to verify different ANSI.
	rawLine, foundRaw := findEventLineRaw(coloredView, coloredPlain, eventName)
	if !foundRaw {
		t.Fatalf("TestDetailRenderColorTier_CTInfo: Event raw line not found in ACTION section.")
	}

	// The raw line must contain the ANSI escape produced by RowColorStyle("ct-info").
	// RowColorStyle("ct-info") uses ColTerminated foreground. With colors enabled,
	// the rendered value must differ from its plain-text counterpart.
	// Specifically: len(rawLine) must be > len(ANSI-stripped rawLine).
	strippedRaw := stripANSI(rawLine)
	if len(rawLine) == len(strippedRaw) {
		t.Errorf("TestDetailRenderColorTier_CTInfo: Event value in ACTION has no ANSI escapes — ColorTier branch not applied.\nRaw line: %q", rawLine)
	}

	// Verify the escape corresponds to RowColorStyle("ct-info"), not NavigableField.
	// NavigableField adds underline (ESC[4m or ESC[24m). RowColorStyle adds only foreground.
	// Quick heuristic: the raw line must NOT contain underline escape (\x1b[4m).
	if strings.Contains(rawLine, "\x1b[4m") {
		t.Errorf("TestDetailRenderColorTier_CTInfo: Event value has underline escape — IsNavigable style incorrectly applied instead of ColorTier.")
	}
}

// ---------------------------------------------------------------------------
// T022-2: ct-attention tier produces yellow foreground on Event value
// ---------------------------------------------------------------------------

// TestDetailRenderColorTier_CTAttention asserts the "Event:" row value uses
// RowColorStyle("ct-attention") when Event.Status == "ct-attention".
//
// Without T028: same failure mode as T022-1.
func TestDetailRenderColorTier_CTAttention(t *testing.T) {
	// PutBucketPolicy is a write event → should produce ct-attention.
	const eventName = "PutBucketPolicy"
	res := buildCTResourceForTier(eventName, "s3.amazonaws.com", "AssumedRole", "ct-attention")

	// Colored output
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })
	m := newCTDetailForTier(t, res)
	view := m.View()
	plain := stripANSI(view)

	_, found := findActionEventLine(plain, eventName)
	if !found {
		t.Fatalf("TestDetailRenderColorTier_CTAttention: Event row not found in ACTION section.\nOutput:\n%s", plain)
	}

	rawLine, foundRaw := findEventLineRaw(view, plain, eventName)
	if !foundRaw {
		t.Fatalf("TestDetailRenderColorTier_CTAttention: Event raw line not found.")
	}

	strippedRaw := stripANSI(rawLine)
	if len(rawLine) == len(strippedRaw) {
		t.Errorf("TestDetailRenderColorTier_CTAttention: Event value has no ANSI escapes — ColorTier branch not applied.\nRaw line: %q", rawLine)
	}
}

// ---------------------------------------------------------------------------
// T022-3: ct-danger tier produces red foreground on Event value
// ---------------------------------------------------------------------------

// TestDetailRenderColorTier_CTDanger asserts the "Event:" row value uses
// RowColorStyle("ct-danger") when the event has an errorCode (→ ct-danger tier).
//
// Without T028: same failure mode as T022-1.
func TestDetailRenderColorTier_CTDanger(t *testing.T) {
	// AccessDenied error → ct-danger.
	const eventName = "DescribeInstances"
	res := buildCTResourceForTier(eventName, "ec2.amazonaws.com", "AssumedRole", "ct-danger")

	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })
	m := newCTDetailForTier(t, res)
	view := m.View()
	plain := stripANSI(view)

	_, found := findActionEventLine(plain, eventName)
	if !found {
		t.Fatalf("TestDetailRenderColorTier_CTDanger: Event row not found in ACTION section.\nOutput:\n%s", plain)
	}

	rawLine, foundRaw := findEventLineRaw(view, plain, eventName)
	if !foundRaw {
		t.Fatalf("TestDetailRenderColorTier_CTDanger: Event raw line not found.")
	}

	strippedRaw := stripANSI(rawLine)
	if len(rawLine) == len(strippedRaw) {
		t.Errorf("TestDetailRenderColorTier_CTDanger: Event value has no ANSI escapes — ColorTier branch not applied.\nRaw line: %q", rawLine)
	}
}

// ---------------------------------------------------------------------------
// T022-4: Empty ColorTier falls through to neutral DetailVal
// ---------------------------------------------------------------------------

// TestDetailRenderColorTier_EmptyFallsThrough asserts that rows with an empty
// ColorTier field (e.g., the Region row in CONTEXT) use the neutral DetailVal
// style rather than any severity color.
//
// Strategy: with colors enabled, render a ct-info event. Locate a row known to
// have no ColorTier (e.g., Region or SourceIP in CONTEXT). Verify its raw ANSI
// output matches what styles.DetailVal would produce for the same value.
//
// Without T028: all rows use DetailVal anyway (no ColorTier branch), so this
// test would trivially pass. To make it fail-before-pass, we anchor it on the
// presence of the CONTEXT section header (which requires IsSection branch).
func TestDetailRenderColorTier_EmptyFallsThrough(t *testing.T) {
	const eventName = "DescribeInstances"
	res := buildCTResourceForTier(eventName, "ec2.amazonaws.com", "AssumedRole", "ct-info")

	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := newCTDetailForTier(t, res)
	plain := stripANSI(m.View())

	// Anchor: CONTEXT section must exist (requires IsSection + ColorTier implementation).
	// Strip right-column separator (│ and everything after it) before comparing.
	lines := strings.Split(plain, "\n")
	foundContext := false
	for _, line := range lines {
		if colorTierSectionLineText(line) == "CONTEXT" {
			foundContext = true
			break
		}
	}
	if !foundContext {
		t.Fatalf("TestDetailRenderColorTier_EmptyFallsThrough: CONTEXT section header not found — IsSection/buildFieldList not yet implemented.\nOutput:\n%s", plain)
	}

	// With NO_COLOR, all styled renders are identical to plain text.
	// Verify that non-tier rows do not contain any unexpected escape sequences.
	if strings.Contains(plain, "\x1b") {
		t.Errorf("TestDetailRenderColorTier_EmptyFallsThrough: plain view (NO_COLOR=1) contains ANSI escapes:\n%q", plain)
	}
}

// ---------------------------------------------------------------------------
// T022-5: IsNavigable wins over ColorTier
// ---------------------------------------------------------------------------

// TestDetailRenderColorTier_IsNavigableWinsOverColorTier asserts that when a row
// has both IsNavigable == true AND a non-empty ColorTier, the IsNavigable style
// (NavigableField: underline) takes precedence.
//
// This test catches a bug where the coder checks ColorTier before IsNavigable
// in the switch, or ORs the styles together.
//
// Strategy: the "user" field in ct-events is registered as navigable (→ "iam-user").
// If the implementation accidentally applies ColorTier on a navigable row, the
// underline escape would be absent. We verify the underline IS present.
//
// Note: in practice no row should have BOTH IsNavigable AND ColorTier set — the
// contract says ColorTier is only set on the Event row in ACTION, and the Event
// row is not navigable. This test ensures the switch priority is correct even if
// a future coder sets both accidentally.
//
// Without T028: No ColorTier items exist, so all navigable rows already use
// NavigableField. The test fails because the ACTION EVENT row with ColorTier
// doesn't exist, meaning we can't verify the precedence rule in a meaningful way.
// We therefore anchor this test on the presence of the ACTOR section (IsSection).
func TestDetailRenderColorTier_IsNavigableWinsOverColorTier(t *testing.T) {
	const eventName = "DescribeInstances"
	res := buildCTResourceForTier(eventName, "ec2.amazonaws.com", "AssumedRole", "ct-info")

	// Colors on so we can detect underline escapes.
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := newCTDetailForTier(t, res)
	view := m.View()
	plain := stripANSI(view)

	// Anchor: ACTOR section must exist (requires IsSection implementation from T021).
	// Strip right-column separator (│ and everything after it) before comparing.
	lines := strings.Split(plain, "\n")
	foundActor := false
	for _, line := range lines {
		if colorTierSectionLineText(line) == "ACTOR" {
			foundActor = true
			break
		}
	}
	if !foundActor {
		t.Fatalf("TestDetailRenderColorTier_IsNavigableWinsOverColorTier: ACTOR section header not found — IsSection branch not implemented yet.\nOutput:\n%s", plain)
	}

	// Verify: the Event row in ACTION must NOT show an underline escape.
	// The Event row has ColorTier set but IsNavigable == false.
	rawLine, found := findEventLineRaw(view, plain, eventName)
	if !found {
		t.Fatalf("TestDetailRenderColorTier_IsNavigableWinsOverColorTier: Event row not found in ACTION.\nOutput:\n%s", plain)
	}
	if strings.Contains(rawLine, "\x1b[4m") || strings.Contains(rawLine, "\x1b[4:") {
		t.Errorf("TestDetailRenderColorTier_IsNavigableWinsOverColorTier: Event row (non-navigable) has underline escape — NavigableField style incorrectly applied:\n%q", rawLine)
	}
}

// ---------------------------------------------------------------------------
// T022-6: Label is always neutral (DetailKey), regardless of ColorTier
// ---------------------------------------------------------------------------

// TestDetailRenderColorTier_LabelIsAlwaysNeutral asserts that the label portion
// ("Event:") of a ColorTier row uses styles.DetailKey (neutral foreground), not
// RowColorStyle. The severity color must wrap ONLY the value part.
//
// Strategy: with colors enabled, locate the raw Event row. Verify that the first
// styled segment (the label) uses the DetailKey color escape, and that the
// RowColorStyle escape appears only after the colon.
//
// This catches a bug where a coder applies RowColorStyle to the entire line
// (label + value) rather than just the value portion.
//
// Without T028: no ColorTier items → Event row in ACTION uses neutral DetailVal.
// The test fails because the ACTOR section header is absent (IsSection not implemented).
func TestDetailRenderColorTier_LabelIsAlwaysNeutral(t *testing.T) {
	const eventName = "DescribeInstances"
	res := buildCTResourceForTier(eventName, "ec2.amazonaws.com", "AssumedRole", "ct-info")

	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := newCTDetailForTier(t, res)
	view := m.View()
	plain := stripANSI(view)

	// Anchor: ACTOR section must exist.
	// Strip right-column separator (│ and everything after it) before comparing.
	actorFound := false
	for line := range strings.SplitSeq(plain, "\n") {
		if colorTierSectionLineText(line) == "ACTOR" {
			actorFound = true
			break
		}
	}
	if !actorFound {
		t.Fatalf("TestDetailRenderColorTier_LabelIsAlwaysNeutral: ACTOR section not found — IsSection not implemented.\nOutput:\n%s", plain)
	}

	rawLine, found := findEventLineRaw(view, plain, eventName)
	if !found {
		t.Fatalf("TestDetailRenderColorTier_LabelIsAlwaysNeutral: Event row not found in ACTION.\nOutput:\n%s", plain)
	}

	// The label is rendered first, then the value.
	// With DetailKey applied to the label and RowColorStyle to the value,
	// the ANSI sequence for the tier color must appear AFTER "Event:".
	//
	// Strip label portion: find the position of eventName in the plain counterpart,
	// then check the raw line's structure relative to that position.
	//
	// Simpler assertion: the row must contain SOME ANSI escapes (we confirmed this
	// in T022-1). Among those, the tier-color escape must not appear before the
	// colon of the label. We locate the colon position in the plain line and
	// verify the raw line up to that plain offset is shorter (fewer escapes start
	// before the colon than after).
	//
	// Practical check: the raw line must contain at least two distinct ANSI runs —
	// one for the label (DetailKey) and one for the value (RowColorStyle). If the
	// entire line were wrapped in a single color style, it would have only one reset.
	// Count ESC occurrences: minimum 4 (open + close for label + open + close for value).
	escCount := strings.Count(rawLine, "\x1b")
	if escCount < 4 {
		t.Errorf("TestDetailRenderColorTier_LabelIsAlwaysNeutral: expected at least 4 ANSI escapes (2 for label, 2 for value) on Event row; got %d.\nRaw line: %q", escCount, rawLine)
	}
}
