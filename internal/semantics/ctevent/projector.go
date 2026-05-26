package ctevent

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// yamlIndentSpaces is the indent width emitted by jsonToYAMLLines and matched
// by the leading-space arithmetic in buildRawJSONSection. yaml.Marshal's default
// is 4 — we override to 2 via yaml.NewEncoder.SetIndent so the level math here
// (level := leading / yamlIndentSpaces) stays correct and so RAW EVENT renders
// match the 2-space convention used elsewhere in the projection layer.
const yamlIndentSpaces = 2

// Project implements domain.DetailProjector for CloudTrail event resources.
// It parses the raw event JSON from r.RawStruct (expected to be a *Event or
// cloudtrailtypes.Event), builds the ctevent sections, and converts them to
// []domain.Section.
//
// When the source is a cloudtrailtypes.Event (live/test path), Project appends:
//   - ACTOR, ACTION, TARGET, CONTEXT (and optional ERROR/REQUEST/RESPONSE) sections
//     from the parsed CloudTrailEvent JSON (when CloudTrailEvent is non-nil).
//   - An ENVELOPE section (appended last) with the SDK envelope fields: EventId,
//     EventName, EventSource, EventTime, Username, ReadOnly, AccessKeyId.
//   - A RAW EVENT section (appended last) with the CloudTrailEvent JSON expanded
//     as structured YAML sub-fields (when CloudTrailEvent is non-nil).
//
// The ENVELOPE and RAW EVENT sections are appended after the rich ct-events sections
// so that cursor-navigation tests whose expected indices are anchored at ACTOR[0]
// are not affected.
//
// When the RawStruct contains a non-empty CloudTrailEvent blob that cannot be
// parsed, Project surfaces an explicit error section rather than silently
// degrading — a parse failure is a contract violation (the fetcher guarantees
// valid JSON for non-stub events).
//
// If RawStruct is nil and r.Fields["raw"] is empty, Project returns nil so the
// caller falls back to the config-driven rendering path in renderContent.
func Project(r domain.Resource) []domain.Section {
	// Fast-path: cloudtrailtypes.Event (live/test path).
	if r.RawStruct != nil {
		if sdkEv, ok := r.RawStruct.(cloudtrailtypes.Event); ok {
			return projectSDKEvent(sdkEv, r)
		}
	}

	event, parseErr := parseResourceWithErr(r)
	if parseErr != nil {
		return errorSection(parseErr)
	}
	if event == nil {
		return nil
	}
	return convertSections(BuildSections(event))
}

// projectSDKEvent handles the cloudtrailtypes.Event source path.
// When CloudTrailEvent is nil: returns just the envelope section (no rich sections).
// When CloudTrailEvent is set: returns rich sections + envelope + raw JSON expansion.
func projectSDKEvent(sdkEv cloudtrailtypes.Event, r domain.Resource) []domain.Section {
	var sections []domain.Section

	if sdkEv.CloudTrailEvent != nil {
		// Parse the embedded JSON blob.
		ev, err := Parse(*sdkEv.CloudTrailEvent)
		if err != nil {
			return errorSection(err)
		}
		ev.Status = ctEventStatus(r)
		// Rich ct-events sections (ACTOR, ACTION, TARGET, CONTEXT, etc.)
		sections = append(sections, convertSections(BuildSections(ev))...)
	}

	// ENVELOPE section — always appended last so it does not shift cursor indices
	// for tests that anchor navigation at ACTOR[0].
	if env := buildEnvelopeSection(sdkEv); len(env.Items) > 0 {
		sections = append(sections, env)
	}

	// RAW EVENT section — expanded CloudTrailEvent JSON as structured YAML sub-fields.
	// Appended after ENVELOPE so the raw JSON blob never appears as a single value.
	if sdkEv.CloudTrailEvent != nil {
		if rawSec := buildRawJSONSection(*sdkEv.CloudTrailEvent); len(rawSec.Items) > 0 {
			sections = append(sections, rawSec)
		}
	}

	return sections
}

// errorSection returns a single section containing an explicit error message.
func errorSection(err error) []domain.Section {
	return []domain.Section{{
		Title: "ERROR",
		Items: []domain.Item{{
			Kind:  domain.ItemField,
			Label: "Error",
			Value: "unable to parse CloudTrail event JSON: " + err.Error(),
		}},
	}}
}

// buildEnvelopeSection constructs an "ENVELOPE" section with the top-level
// cloudtrailtypes.Event struct fields (EventId, EventName, EventSource, EventTime,
// Username, ReadOnly, AccessKeyId). Only fields with non-nil/non-empty values
// are included.
func buildEnvelopeSection(sdkEv cloudtrailtypes.Event) domain.Section {
	sec := domain.Section{Title: "ENVELOPE"}

	addField := func(label, value string) {
		if value == "" {
			return
		}
		sec.Items = append(sec.Items, domain.Item{
			Kind:  domain.ItemField,
			Label: label,
			Value: value,
		})
	}

	if sdkEv.EventId != nil {
		addField("EventId", *sdkEv.EventId)
	}
	if sdkEv.EventName != nil {
		addField("EventName", *sdkEv.EventName)
	}
	if sdkEv.EventSource != nil {
		addField("EventSource", *sdkEv.EventSource)
	}
	if sdkEv.EventTime != nil {
		addField("EventTime", sdkEv.EventTime.Format(time.RFC3339))
	}
	if sdkEv.Username != nil {
		addField("Username", *sdkEv.Username)
	}
	if sdkEv.ReadOnly != nil {
		addField("ReadOnly", *sdkEv.ReadOnly)
	}
	if sdkEv.AccessKeyId != nil {
		addField("AccessKeyId", *sdkEv.AccessKeyId)
	}

	return sec
}

// buildRawJSONSection constructs a "RAW EVENT" section from the CloudTrailEvent
// JSON blob, expanding the JSON into structured YAML sub-fields rather than
// surfacing it as a raw single-line value.
//
// Each top-level JSON key becomes either:
//   - A direct field (ItemField) for scalar values.
//   - A header (ItemHeader) + sub-fields (ItemSubfield) for nested objects/arrays.
func buildRawJSONSection(rawJSON string) domain.Section {
	lines := jsonToYAMLLines(rawJSON)
	if len(lines) == 0 {
		return domain.Section{}
	}
	sec := domain.Section{Title: "RAW EVENT"}
	for _, line := range lines {
		if line == "" {
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " "))
		level := leading / yamlIndentSpaces

		trimmed := strings.TrimSpace(line)
		_, _, hasSep := strings.Cut(trimmed, ":")

		if level == 0 && hasSep {
			// Top-level key: value or key: (nested)
			key, val, _ := strings.Cut(trimmed, ":")
			val = strings.TrimSpace(val)
			if val == "" {
				// Nested header
				sec.Items = append(sec.Items, domain.Item{
					Kind:  domain.ItemHeader,
					Label: strings.TrimSuffix(key, ":"),
				})
			} else {
				sec.Items = append(sec.Items, domain.Item{
					Kind:  domain.ItemField,
					Label: strings.TrimSuffix(key, ":"),
					Value: val,
				})
			}
		} else {
			// Sub-field at level > 0 or list item
			sec.Items = append(sec.Items, domain.Item{
				Kind:        domain.ItemSubfield,
				Value:       line,
				IndentLevel: level + 1,
			})
		}
	}
	return sec
}

// jsonToYAMLLines converts a JSON string to YAML lines. Returns nil when the
// input is empty, invalid JSON, or represents an empty object/array.
func jsonToYAMLLines(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil
	}
	switch v := parsed.(type) {
	case map[string]any:
		if len(v) == 0 {
			return nil
		}
	case []any:
		if len(v) == 0 {
			return nil
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndentSpaces)
	if err := enc.Encode(parsed); err != nil {
		return nil
	}
	if err := enc.Close(); err != nil {
		return nil
	}
	raw := strings.TrimRight(buf.String(), "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

// parseResourceWithErr extracts a *Event from the domain.Resource and returns
// any parse error encountered on a non-empty source.
//
// It handles three cases:
//  1. r.RawStruct is already a *Event (demo fixtures).
//  2. r.RawStruct is a cloudtrailtypes.Event (live/test path) — extracts
//     the embedded CloudTrailEvent JSON and parses it. A non-nil
//     CloudTrailEvent that fails to parse is returned as an error.
//  3. r.Fields["raw"] contains the raw CloudTrail JSON string (fallback).
//
// Returns (nil, nil) when there is no parseable event source — callers can
// then fall back to generic/flat rendering.
func parseResourceWithErr(r domain.Resource) (*Event, error) {
	if r.RawStruct != nil {
		if ev, ok := r.RawStruct.(*Event); ok {
			ev.Status = ctEventStatus(r)
			return ev, nil
		}
		// AWS SDK type from live fetcher or test fixtures.
		if sdkEv, ok := r.RawStruct.(cloudtrailtypes.Event); ok {
			if sdkEv.CloudTrailEvent != nil {
				ev, err := Parse(*sdkEv.CloudTrailEvent)
				if err != nil {
					return nil, err
				}
				ev.Status = ctEventStatus(r)
				return ev, nil
			}
		}
	}
	raw := r.Fields["raw"]
	if raw == "" {
		return nil, nil
	}
	ev, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	ev.Status = ctEventStatus(r)
	return ev, nil
}

// convertSections adapts []Section (ctevent-local) to []domain.Section.
// Adapter note: ctevent.Row.Severity is a tier string ("ct-info"|"ct-attention"|
// "ct-danger"|""). domain.Item uses Tier for that string and Severity for the
// enum. We map the tier string to Tier and derive Severity from it.
func convertSections(sections []Section) []domain.Section {
	out := make([]domain.Section, 0, len(sections))
	for _, s := range sections {
		ds := domain.Section{
			Title: s.Name,
			Items: make([]domain.Item, 0, len(s.Rows)),
		}
		for _, row := range s.Rows {
			ds.Items = append(ds.Items, convertRow(row))
		}
		out = append(out, ds)
	}
	return out
}

// convertRow maps a ctevent Row to a domain Item.
func convertRow(r Row) domain.Item {
	return domain.Item{
		Kind:       domain.ItemField,
		Label:      r.Key,
		Value:      r.Value,
		Tier:       r.Severity, // ctevent Severity string → domain Tier string
		Severity:   tierToSeverity(r.Severity),
		Navigable:  r.IsNavigable,
		TargetType: r.TargetType,
		NavID:      r.NavID,
	}
}

// ctEventStatus returns the ct-events status tier from a Resource.
// The fetcher stores it in Fields["status"].
func ctEventStatus(r domain.Resource) string {
	return r.Fields["status"]
}

// tierToSeverity maps the ctevent tier string to a domain.Severity value.
// "ct-danger" → SevBroken, "ct-attention" → SevWarn, everything else → SevOK.
func tierToSeverity(tier string) domain.Severity {
	switch tier {
	case "ct-danger":
		return domain.SevBroken
	case "ct-attention":
		return domain.SevWarn
	default:
		return domain.SevOK
	}
}

