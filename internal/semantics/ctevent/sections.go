package ctevent

import (
	"strings"
	"time"
)

// BuildSections builds the ordered list of detail sections for the given parsed event.
// Section order: ACTOR → ACTION → TARGET → CONTEXT → ERROR (if present) → REQUEST → RESPONSE.
//
// See specs/013-ct-event-detail-v2/contracts/ctdetail-api.md for the full contract,
// and specs/013-ct-event-detail-v2/data-model.md for the Section and Row type definitions.
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
		targetRows, cleanedParams = ExtractTarget(event.EventName, event.EventSource, event.RecipientAccountID, event.Resources, event.RequestParameters)
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
	// Strip boring-default keys before summarizing (drop-boring-defaults rule §1.2).
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
	// Only navigable when arnTargetType resolves to a known type (e.g. role, iam-user).
	// Root ARNs (arn:*:root) return "" from arnTargetType and must stay display-only.
	principalTargetType := arnTargetType(ui.ARN)
	principalRow := Row{
		Key:         "Principal",
		Value:       ui.ARN,
		IsNavigable: principalTargetType != "",
		TargetType:  principalTargetType,
		NavID:       arnNavID(ui.ARN),
	}
	rows = append(rows, principalRow)

	// As: row — when SourceIdentity is non-empty (SSO/federated opaque ARNs).
	if ui.SessionContext != nil && ui.SessionContext.SourceIdentity != "" {
		rows = append(rows, Row{Key: "As", Value: ui.SessionContext.SourceIdentity})
	}

	// Federation: row — when WebIDFederationData is present.
	if ui.SessionContext != nil && ui.SessionContext.WebIDFederationData != nil {
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

// arnTargetType derives the navigable resource type from an ARN.
// Returns "role" for assumed-role ARNs, "iam-user" for :user/ ARNs, "" otherwise.
func arnTargetType(arn string) string {
	if strings.Contains(arn, ":assumed-role/") {
		return "role"
	}
	if strings.Contains(arn, ":user/") {
		return "iam-user"
	}
	return ""
}

// arnNavID extracts the bare navigable name from an ARN for use as NavID.
// The display Value remains the full ARN; NavID is used only for navigation dispatch.
//
//   - arn:aws:sts::*:assumed-role/<role>/<session> → <role>
//   - arn:aws:iam::*:role/<name>                   → <name>
//   - arn:aws:iam::*:user/<name>                   → <name>
//   - arn:aws:iam::*:root                          → "" (not navigable by name)
//   - anything else                                → "" (falls back to Value)
func arnNavID(arn string) string {
	if _, after, ok := strings.Cut(arn, ":assumed-role/"); ok {
		rest := after
		// rest is "<role>/<session>" — take only the role part
		if before, _, ok := strings.Cut(rest, "/"); ok {
			return before
		}
		return rest
	}
	if _, after, ok := strings.Cut(arn, ":role/"); ok {
		return after
	}
	if _, after, ok := strings.Cut(arn, ":user/"); ok {
		return after
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

	// Event: row — always present; carries Severity (FR-002 single-cell exception).
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
// REQUEST section (drop-boring-defaults rule §1.2).
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
