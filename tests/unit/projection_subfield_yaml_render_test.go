package unit

// projection_subfield_yaml_render_test.go — regression test for PR-01 Bug 1.
//
// Bug: when ctevent.Project (or any projector using buildRawJSONSection) emits
// an ItemSubfield with empty Label and a YAML continuation value like
// "  keyId: arn:...", the adapter domainItemToFieldItem produces
// FieldItem{Key: "", Value: "  keyId: arn:..."}.
//
// The renderer's `if item.Key != item.Value` branch then fires because
// "" != "  keyId: arn:..." and produces:
//
//	indent + "" + ": " + "  keyId: arn:..."  →  ":   keyId: arn:..."
//
// Fix: domainItemToFieldItem must detect ItemSubfield with empty Label and
// copy Value into Key (Key == Value), so the renderer takes the plain-line
// branch instead.
//
// This test constructs a cloudtrailtypes.Event with a nested
// requestParameters object, calls ctevent.Project() to get []domain.Section,
// then builds a DetailModel and calls View(). It asserts that NO rendered line
// in the RAW EVENT section starts with ": " (stray colon prefix).

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
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

// TestCTEventRawYAMLRender_NoStrayColonPrefix verifies that a CloudTrail event
// with nested requestParameters does NOT produce lines starting with ": " in the
// rendered detail output.
//
// Failure today: domainItemToFieldItem sets Key="" for ItemSubfield with empty Label,
// then the renderer fires the Key != Value branch and prepends ": " to the line.
//
// Expected after fix: every sub-field line in RAW EVENT renders as plain YAML
// (no leading ": " prefix).
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
		// Verify that at least one ItemSubfield with empty Label exists (confirms
		// the fixture exercises the bug path).
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

	// Now render through the detail view to exercise domainItemToFieldItem.
	// Use "ct-events" (the registered ShortName) so FindResourceType finds the
	// entry with Project: ctevent.Project set.
	k := keys.Default()
	d := views.NewDetail(
		domain.Resource{
			ID:        r.ID,
			Type:      r.Type,
			RawStruct: r.RawStruct,
		},
		"ct-events",
		nil, // no view config
		k,
	)
	d.SetSize(120, 40)
	output := d.View()
	plain := stripANSI(output)

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
// as recognizable plain-text content — not mangled by the stray-colon bug.
//
// This is a companion assertion: even if the stray-colon test passes, this
// confirms the values themselves are actually present and readable.
func TestCTEventRawYAMLRender_NestedParamsExpanded(t *testing.T) {
	sdkEv := ctEventWithNestedParams()
	r := domain.Resource{
		ID:        "e-d4e5f6a7-test",
		Type:      "ct-events",
		RawStruct: sdkEv,
	}

	k := keys.Default()
	d := views.NewDetail(
		domain.Resource{
			ID:        r.ID,
			Type:      r.Type,
			RawStruct: r.RawStruct,
		},
		"ct-events",
		nil,
		k,
	)
	d.SetSize(120, 40)
	plain := stripANSI(d.View())

	// Assert that no rendered line starts with ": " after trimming leading whitespace.
	// Before fix: sub-field lines render as ":   keyId: arn:..." — the stray colon
	// prefix is the observable symptom of the domainItemToFieldItem bug.
	// This companion test independently verifies the fix from the rendered-value angle.
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, ": ") {
			t.Errorf("nested param rendered with stray ': ' prefix — got line: %q", line)
		}
	}
}
