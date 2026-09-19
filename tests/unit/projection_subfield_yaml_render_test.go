package unit_test

// An ItemSubfield with an empty
// Label and a YAML continuation value like "  keyId: arn:..." (emitted by
// ctevent.Project or any projector using buildRawJSONSection) renders as a
// plain line.
//
// domainItemToFieldItemDetail (core/app/detail_body.go) copies Value into
// Key for such an item (Key == Value), so the renderer takes the plain-line
// branch. With Key "" the renderer's `if item.Key != item.Value` branch
// would produce:
//
//	indent + "" + ": " + "  keyId: arn:..."  →  ":   keyId: arn:..."
//
// This test constructs a cloudtrailtypes.Event with a nested
// requestParameters object, calls ctevent.Project() to get []domain.Section,
// then drives it through a real app.Controller (buildDetailFieldItems ->
// domainItemToFieldItemDetail -> RenderDetail) and asserts that NO rendered
// line in the RAW EVENT section starts with ": " (stray colon prefix).

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ctEventWithNestedParams builds a cloudtrailtypes.Event whose
// CloudTrailEvent JSON contains a nested requestParameters object.
// This guarantees that buildRawJSONSection emits at least one
// ItemSubfield{Label:"", Value:"  <key>: <val>"} line.
func ctEventWithNestedParams() cloudtrailtypes.Event {
	// Minimal CloudTrail JSON with a nested requestParameters object so that
	// buildRawJSONSection produces sub-field lines (Label=="", Value=="  keyId: ...").
	rawJSON := `{
		"eventVersion": "1.08",
		"userIdentity": {"type": "AssumedRole"},
		"eventSource": "kms.amazonaws.com",
		"eventName": "RotateKey",
		"requestParameters": {
			"keyId": "arn:aws:kms:us-east-1:123456789012:key/test-key-id",
			"bucketName": "my-test-bucket"
		},
		"responseElements": null,
		"eventID": "e-d4e5f6a7-test",
		"eventType": "AwsApiCall",
		"recipientAccountId": "123456789012"
	}`
	return cloudtrailtypes.Event{
		EventId:         aws.String("e-d4e5f6a7-test"),
		EventName:       aws.String("RotateKey"),
		EventSource:     aws.String("kms.amazonaws.com"),
		CloudTrailEvent: aws.String(rawJSON),
	}
}

// renderCTEventDetailViaController drives a ct-events resource through a
// real app.Controller (EnsureDetailState -> buildDetailFieldItems ->
// buildDetailBody) and renders it via the live NewTransientDetail+
// RenderDetail seam, mirroring detail_render_parity_test.go's pattern.
func renderCTEventDetailViaController(t *testing.T, sdkEv cloudtrailtypes.Event) string {
	t.Helper()

	res := resource.Resource{
		ID:        "e-d4e5f6a7-test",
		Name:      "e-d4e5f6a7-test",
		RawStruct: sdkEv,
	}

	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	c.EnsureDetailState(res, "ct-events")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil for a ct-events resource")
	}

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(40))
	m := views.NewTransientDetail(120, 40, vp)
	return m.RenderDetail(*body)
}

// TestCTEventRawYAMLRender_NoStrayColonPrefix verifies that a CloudTrail event
// with nested requestParameters does NOT produce lines starting with ": " in the
// rendered detail output: every sub-field line in RAW EVENT renders as plain
// YAML.
func TestCTEventRawYAMLRender_NoStrayColonPrefix(t *testing.T) {
	sdkEv := ctEventWithNestedParams()
	r := domain.Resource{
		ID:        "e-d4e5f6a7-test",
		Type:      "ct-events",
		RawStruct: sdkEv,
	}

	sections := ctevent.Project(r)
	if len(sections) == 0 {
		t.Fatal("ctevent.Project returned no sections for event with CloudTrailEvent JSON")
	}

	// Find the RAW EVENT section — it must exist for an event with embedded JSON.
	rawEventFound := false
	for _, sec := range sections {
		if sec.Title != "RAW EVENT" {
			continue
		}
		rawEventFound = true
		// At least one ItemSubfield with empty Label exists, so the fixture
		// exercises the empty-Label path.
		hasEmptyLabelSubfield := false
		for _, it := range sec.Items {
			if it.Kind == domain.ItemSubfield && it.Label == "" {
				hasEmptyLabelSubfield = true
				break
			}
		}
		if !hasEmptyLabelSubfield {
			t.Fatal("RAW EVENT section contains no ItemSubfield with empty Label — fixture does not exercise the bug path")
		}
	}
	if !rawEventFound {
		t.Fatal("ctevent.Project output has no 'RAW EVENT' section; cannot test rendering")
	}

	// Now render through the live controller-backed detail path to exercise
	// domainItemToFieldItemDetail.
	plain := stripAnsi(renderCTEventDetailViaController(t, sdkEv))

	// Check every line: none may start with ": " (the stray-colon symptom).
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, ": ") {
			t.Errorf("raw YAML subfield rendered with stray ': ' prefix — got line: %q", line)
		}
	}
}

// TestCTEventRawYAMLRender_NestedParamsExpanded verifies that the nested
// requestParameters keys ("keyId", "bucketName") appear in the rendered output
// as recognizable plain-text content, not mangled by a stray colon prefix.
func TestCTEventRawYAMLRender_NestedParamsExpanded(t *testing.T) {
	sdkEv := ctEventWithNestedParams()

	plain := stripAnsi(renderCTEventDetailViaController(t, sdkEv))
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, ": ") {
			t.Errorf("nested param rendered with stray ': ' prefix — got line: %q", line)
		}
	}
}
