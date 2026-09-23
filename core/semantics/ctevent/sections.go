// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package ctevent

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// BuildSections builds the ordered list of detail sections for the given parsed event.
// Section order: ACTOR → ACTION → TARGET → CONTEXT → ERROR (if present) → REQUEST → RESPONSE.
//
// Guarantees:
//   - Returns a non-nil slice (possibly empty in degenerate cases).
//   - Empty sections (len(Rows) == 0) are omitted from the result.
//   - ERROR section is included if and only if event.ErrorCode != "".
//   - Pure function: same input → same output, no I/O, no global state writes.
//
// Panics if event is nil.
func BuildSections(event *Event) []Section {
	sections := []Section{}

	// ACTOR — omitted entirely for Insight events.
	if event.EventCategory != "Insight" {
		actorRows := buildActorRows(event)
		sections = appendIfNonEmpty(sections, SectionActor, actorRows)
	}

	// ACTION — always present.
	actionRows := buildActionRows(event)
	sections = appendIfNonEmpty(sections, SectionAction, actionRows)

	// TARGET — from ExtractTarget; also returns cleanedParams for REQUEST.
	// Only call ExtractTarget when there is something to extract from: either
	// resources[] or requestParameters must be non-nil/non-empty. When both are
	// absent, skip the extraction to avoid synthetic "(all)" rows.
	var targetRows []Row
	var cleanedParams map[string]any
	if len(event.Resources) > 0 || event.RequestParameters != nil {
		targetRows, cleanedParams = ExtractTargetInRegion(event.EventName, event.EventSource, event.RecipientAccountID, event.AWSRegion, event.Resources, event.RequestParameters)
	} else {
		cleanedParams = map[string]any{}
	}
	sections = appendIfNonEmpty(sections, SectionTarget, targetRows)

	// CONTEXT
	contextRows := buildContextRows(event)
	sections = appendIfNonEmpty(sections, SectionContext, contextRows)

	// ERROR — hoisted after CONTEXT, before REQUEST.
	if event.ErrorCode != "" {
		errorRows := []Row{
			{Key: "errorCode", Value: event.ErrorCode},
			{Key: "errorMessage", Value: event.ErrorMessage},
		}
		sections = appendIfNonEmpty(sections, SectionError, errorRows)
	}

	// REQUEST — uses cleanedParams (TARGET fields already removed).
	// Strip boring-default keys before summarizing.
	requestParams := dropBoringKeys(cleanedParams)
	var requestRows []Row
	if summarizer, ok := summarizerByService[event.EventSource]; ok {
		requestRows = summarizer(event.EventName, requestParams)
	} else {
		requestRows = SummarizeGeneric(event.EventName, requestParams)
	}
	sections = appendIfNonEmpty(sections, SectionRequest, requestRows)

	// RESPONSE
	responseRows := SummarizeGeneric(event.EventName, event.ResponseElements)
	sections = appendIfNonEmpty(sections, SectionResponse, responseRows)

	return sections
}

// appendIfNonEmpty appends a Section only when rows is non-empty.
func appendIfNonEmpty(sections []Section, name string, rows []Row) []Section {
	if len(rows) == 0 {
		return sections
	}
	return append(sections, Section{Name: name, Rows: rows})
}

// buildActorRows constructs the ACTOR section rows for non-Insight events.
func buildActorRows(event *Event) []Row {
	ui := event.UserIdentity

	// AwsServiceEvent: emit Service row only, no Principal.
	isServiceEvent := event.EventType == "AwsServiceEvent" ||
		ui.Type == "AWSService" ||
		ui.ARN == ""

	if isServiceEvent {
		svc := event.EventSource
		if svc == "" {
			svc = ui.InvokedBy
		}
		if svc == "" {
			return nil
		}
		return []Row{{Key: "Service", Value: svc}}
	}

	var rows []Row

	// Principal row — always present for non-service events with an ARN.
	// Only navigable when arnTargetType resolves to a known type (e.g. role,
	// iam-user) and the type's resolver reads it in the account the event was
	// recorded in. Root ARNs (arn:*:root) return "" from arnTargetType and
	// stay display-only.
	principalRow := Row{Key: "Principal", Value: ui.ARN}
	if target := arnTargetType(ui.ARN); target != "" {
		if _, ok := navLink(target, ui.ARN, event.RecipientAccountID); ok {
			principalRow.IsNavigable, principalRow.TargetType = true, target
		}
	}
	rows = append(rows, principalRow)

	// As: row — when SourceIdentity is non-empty (SSO/federated opaque ARNs).
	if ui.SessionContext != nil && ui.SessionContext.SourceIdentity != "" {
		rows = append(rows, Row{Key: "As", Value: ui.SessionContext.SourceIdentity})
	}

	// Federation: row — when a provider is named. CloudTrail emits an empty
	// webIdFederationData object on most assumed-role events, which names
	// none.
	if ui.SessionContext != nil && ui.SessionContext.WebIDFederationData != nil &&
		ui.SessionContext.WebIDFederationData.FederatedProvider != "" {
		rows = append(rows, Row{Key: "Federation", Value: ui.SessionContext.WebIDFederationData.FederatedProvider})
	}

	// MFA: row — ONLY when true.
	if ui.SessionContext != nil && ui.SessionContext.Attributes.MFAAuthenticated {
		rows = append(rows, Row{Key: "MFA", Value: "yes"})
	}

	// Access key: row — when non-empty.
	if ui.AccessKeyID != "" {
		rows = append(rows, Row{Key: "Access key", Value: ui.AccessKeyID})
	}

	// User agent: row — when non-empty.
	if event.UserAgent != "" {
		rows = append(rows, Row{Key: "User agent", Value: event.UserAgent})
	}

	return rows
}

// arnTargetType derives the navigable resource type from a principal ARN:
// "role" for a role or assumed-role ARN, "iam-user" for a user ARN, "" for
// anything else. An ARN that ends at its type marker ("…:role/") names a
// type and nobody, and a row that opens nothing must not offer to open.
func arnTargetType(ref string) string {
	a, err := arn.Parse(ref)
	if err != nil {
		return ""
	}
	typ, name, _ := strings.Cut(a.Resource, "/")
	if name == "" || strings.HasSuffix(name, "/") {
		return ""
	}
	switch typ {
	case "role", "assumed-role":
		return "role"
	case "user":
		return "iam-user"
	}
	return ""
}

// serviceFromSource strips ".amazonaws.com" from an event source to get the short service name.
// e.g. "s3.amazonaws.com" → "s3", "kms.amazonaws.com" → "kms".
func serviceFromSource(eventSource string) string {
	return strings.TrimSuffix(eventSource, ".amazonaws.com")
}

// buildActionRows constructs the ACTION section rows.
func buildActionRows(event *Event) []Row {
	var rows []Row

	// Event: row — always present; the only row that carries Severity.
	eventValue := serviceFromSource(event.EventSource) + ":" + event.EventName
	rows = append(rows, Row{
		Key:      "Event",
		Value:    eventValue,
		Severity: event.Status,
	})

	// Category: row — omit when Management/AwsApiCall (boring default).
	if event.EventCategory != "Management" || event.EventType != "AwsApiCall" {
		catValue := event.EventCategory + " / " + event.EventType
		rows = append(rows, Row{Key: "Category", Value: catValue})
	}

	// Insight type: row — only when InsightDetails present.
	if event.InsightDetails != nil {
		rows = append(rows, Row{Key: "Insight type", Value: event.InsightDetails.InsightType})
		rows = append(rows, Row{Key: "State", Value: event.InsightDetails.State})
	}

	return rows
}

// boringRequestKeys are top-level requestParameters keys that duplicate information
// already present in other sections, or are meta-fields that add no value in the
// REQUEST section.
var boringRequestKeys = map[string]struct{}{
	"Verb":          {},
	"Read only":     {},
	"Identity type": {},
	"Source":        {},
}

// dropBoringKeys returns a copy of params with boring default keys removed.
// Returns a new map; does not mutate params.
func dropBoringKeys(params map[string]any) map[string]any {
	if len(params) == 0 {
		return params
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		if _, boring := boringRequestKeys[k]; !boring {
			out[k] = v
		}
	}
	return out
}

// buildContextRows constructs the CONTEXT section rows.
func buildContextRows(event *Event) []Row {
	var rows []Row

	if event.AWSRegion != "" {
		rows = append(rows, Row{Key: "Region", Value: event.AWSRegion})
	}

	if event.SourceIPAddress != "" {
		rows = append(rows, Row{Key: "Source IP", Value: event.SourceIPAddress})
	}

	// Recipient: row — only when cross-account.
	if event.RecipientAccountID != "" && event.AccountID != event.RecipientAccountID {
		rows = append(rows, Row{Key: "Recipient", Value: event.RecipientAccountID + " (cross-account)"})
	}

	// Time: row — when non-zero.
	if !event.EventTime.IsZero() {
		rows = append(rows, Row{Key: "Time", Value: event.EventTime.Format(time.RFC3339)})
	}

	return rows
}
