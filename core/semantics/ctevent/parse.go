package ctevent

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Parse parses a raw CloudTrail event JSON blob into an Event struct.
// The input is the value of cloudtrailtypes.Event.CloudTrailEvent from
// github.com/aws/aws-sdk-go-v2/service/cloudtrail/types.
//
// See specs/013-ct-event-detail-v2/contracts/ctevent-api.md for the full contract,
// and specs/013-ct-event-detail-v2/data-model.md for the Event type definition.
//
// Guarantees:
//   - Returns a non-nil *Event and nil error for any well-formed CloudTrail JSON.
//   - Returns (nil, error) for empty input or malformed JSON.
//   - Tolerates missing optional fields (responseElements, errorCode, etc.).
//   - Pure and idempotent: same input → same output, no I/O, no global state.
func Parse(rawJSON string) (*Event, error) {
	if rawJSON == "" {
		return nil, errors.New("ctdetail: empty input")
	}

	var raw rawEvent
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return nil, fmt.Errorf("ctdetail: parse failed: %w", err)
	}

	ev := &Event{
		EventID:            raw.EventID,
		EventSource:        raw.EventSource,
		EventName:          raw.EventName,
		EventCategory:      raw.EventCategory,
		EventType:          raw.EventType,
		AWSRegion:          raw.AWSRegion,
		SourceIPAddress:    raw.SourceIPAddress,
		UserAgent:          raw.UserAgent,
		RecipientAccountID: raw.RecipientAccountID,
		RequestID:          raw.RequestID,
		EventVersion:       raw.EventVersion,
		ReadOnly:           raw.ReadOnly,
		ErrorCode:          raw.ErrorCode,
		ErrorMessage:       raw.ErrorMessage,
	}

	// Parse EventTime (RFC3339); missing → zero time.Time{}
	if raw.EventTime != "" {
		if t, err := time.Parse(time.RFC3339, raw.EventTime); err == nil {
			ev.EventTime = t
		}
	}

	// Parse UserIdentity
	ev.UserIdentity = parseUserIdentity(raw.UserIdentity)

	// AccountID comes from userIdentity.accountId per data-model.md
	ev.AccountID = ev.UserIdentity.AccountID

	// Parse RequestParameters and ResponseElements
	if raw.RequestParameters != nil {
		var m map[string]any
		if err := json.Unmarshal(raw.RequestParameters, &m); err == nil {
			ev.RequestParameters = m
		}
	}
	if raw.ResponseElements != nil {
		var m map[string]any
		if err := json.Unmarshal(raw.ResponseElements, &m); err == nil {
			ev.ResponseElements = m
		}
	}

	// Parse resources[]
	for _, r := range raw.Resources {
		ev.Resources = append(ev.Resources, ResourceRef(r))
	}

	// Parse InsightDetails only when eventCategory == "Insight"
	if ev.EventCategory == "Insight" && raw.InsightDetails != nil {
		ev.InsightDetails = parseInsightDetails(raw.InsightDetails)
	}

	// Classify verb via existing function
	ev.Verb = ClassifyCTVerb(ev.EventName, ev.EventCategory, ev.EventType)

	return ev, nil
}

// ---------------------------------------------------------------------------
// raw JSON shapes
// ---------------------------------------------------------------------------

type rawEvent struct {
	EventVersion       string             `json:"eventVersion"`
	EventID            string             `json:"eventID"`
	EventTime          string             `json:"eventTime"`
	EventSource        string             `json:"eventSource"`
	EventName          string             `json:"eventName"`
	EventCategory      string             `json:"eventCategory"`
	EventType          string             `json:"eventType"`
	AWSRegion          string             `json:"awsRegion"`
	SourceIPAddress    string             `json:"sourceIPAddress"`
	UserAgent          string             `json:"userAgent"`
	RecipientAccountID string             `json:"recipientAccountId"`
	RequestID          string             `json:"requestID"`
	ReadOnly           bool               `json:"readOnly"`
	ErrorCode          string             `json:"errorCode"`
	ErrorMessage       string             `json:"errorMessage"`
	UserIdentity       rawUserIdentity    `json:"userIdentity"`
	RequestParameters  json.RawMessage    `json:"requestParameters"`
	ResponseElements   json.RawMessage    `json:"responseElements"`
	Resources          []rawResourceRef   `json:"resources"`
	InsightDetails     *rawInsightDetails `json:"insightDetails"`
}

type rawUserIdentity struct {
	Type           string             `json:"type"`
	PrincipalID    string             `json:"principalId"`
	ARN            string             `json:"arn"`
	AccountID      string             `json:"accountId"`
	UserName       string             `json:"userName"`
	InvokedBy      string             `json:"invokedBy"`
	AccessKeyID    string             `json:"accessKeyId"`
	SessionContext *rawSessionContext `json:"sessionContext"`
}

type rawSessionContext struct {
	Attributes          rawSessionAttributes `json:"attributes"`
	SessionIssuer       *rawSessionIssuer    `json:"sessionIssuer"`
	WebIDFederationData *rawWebIDFedData     `json:"webIdFederationData"`
	SourceIdentity      string               `json:"sourceIdentity"`
}

type rawSessionAttributes struct {
	MFAAuthenticated string `json:"mfaAuthenticated"`
	CreationDate     string `json:"creationDate"`
}

type rawSessionIssuer struct {
	Type        string `json:"type"`
	PrincipalID string `json:"principalId"`
	ARN         string `json:"arn"`
	AccountID   string `json:"accountId"`
	UserName    string `json:"userName"`
}

type rawWebIDFedData struct {
	FederatedProvider string `json:"federatedProvider"`
	Audience          string `json:"audience"`
}

type rawResourceRef struct {
	ARN       string `json:"ARN"`
	AccountID string `json:"accountId"`
	Type      string `json:"type"`
}

type rawInsightDetails struct {
	State          string          `json:"state"`
	EventSource    string          `json:"eventSource"`
	EventName      string          `json:"eventName"`
	InsightType    string          `json:"insightType"`
	InsightContext json.RawMessage `json:"insightContext"`
}

// ---------------------------------------------------------------------------
// parsers
// ---------------------------------------------------------------------------

func parseUserIdentity(r rawUserIdentity) UserIdentity {
	ui := UserIdentity{
		Type:        r.Type,
		PrincipalID: r.PrincipalID,
		ARN:         r.ARN,
		AccountID:   r.AccountID,
		UserName:    r.UserName,
		InvokedBy:   r.InvokedBy,
		AccessKeyID: r.AccessKeyID,
	}
	if r.SessionContext != nil {
		ui.SessionContext = parseSessionContext(r.SessionContext)
	}
	return ui
}

func parseSessionContext(r *rawSessionContext) *SessionContext {
	sc := &SessionContext{
		SourceIdentity: r.SourceIdentity,
	}

	// Attributes
	sc.Attributes.MFAAuthenticated = r.Attributes.MFAAuthenticated == "true"
	if r.Attributes.CreationDate != "" {
		if t, err := time.Parse(time.RFC3339, r.Attributes.CreationDate); err == nil {
			sc.Attributes.CreationDate = t
		}
	}

	// SessionIssuer
	if r.SessionIssuer != nil {
		sc.SessionIssuer = &SessionIssuer{
			Type:        r.SessionIssuer.Type,
			PrincipalID: r.SessionIssuer.PrincipalID,
			ARN:         r.SessionIssuer.ARN,
			AccountID:   r.SessionIssuer.AccountID,
			UserName:    r.SessionIssuer.UserName,
		}
	}

	// WebIDFederationData
	if r.WebIDFederationData != nil {
		sc.WebIDFederationData = &WebIDFederationData{
			FederatedProvider: r.WebIDFederationData.FederatedProvider,
			Audience:          r.WebIDFederationData.Audience,
		}
	}

	return sc
}

func parseInsightDetails(r *rawInsightDetails) *InsightDetails {
	id := &InsightDetails{
		State:       r.State,
		EventSource: r.EventSource,
		EventName:   r.EventName,
		InsightType: r.InsightType,
	}
	if r.InsightContext != nil {
		var m map[string]any
		if err := json.Unmarshal(r.InsightContext, &m); err == nil {
			id.InsightContext = m
		}
	}
	return id
}
