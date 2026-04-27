// Package ctevent implements the CloudTrail event detail view data model.
// See specs/013-ct-event-detail-v2/data-model.md for the full type specification
// and specs/013-ct-event-detail-v2/contracts/ctdetail-api.md for the public API contracts.
package ctevent

import "time"

// Section name constants — used as Section.Name values in the ordered output of BuildSections.
const (
	SectionActor    = "ACTOR"
	SectionAction   = "ACTION"
	SectionTarget   = "TARGET"
	SectionContext  = "CONTEXT"
	SectionError    = "ERROR"
	SectionRequest  = "REQUEST"
	SectionResponse = "RESPONSE"
)

// Event is the parsed in-memory representation of a single CloudTrail event.
// One Event is created per detail-view open via Parse. It is immutable after Parse returns.
type Event struct {
	// Envelope (always present)
	EventID            string
	EventTime          time.Time // RFC3339 from JSON
	EventSource        string    // e.g. "iam.amazonaws.com"
	EventName          string    // e.g. "DeleteRole"
	EventCategory      string    // "Management" | "Insight" | "NetworkActivity" | "Data"
	EventType          string    // "AwsApiCall" | "AwsServiceEvent" | "AwsConsoleSignIn" | ...
	AWSRegion          string
	SourceIPAddress    string
	UserAgent          string
	AccountID          string // userIdentity.accountId
	RecipientAccountID string // top-level recipientAccountId
	RequestID          string
	EventVersion       string

	// Identity (always present, but inner shape varies)
	UserIdentity UserIdentity

	// Read-only flag
	ReadOnly bool

	// Error (only when present)
	ErrorCode    string
	ErrorMessage string

	// Payload (raw maps; summarizers walk these)
	RequestParameters map[string]any // nil when JSON omits
	ResponseElements  map[string]any // nil when JSON omits
	Resources         []ResourceRef  // SDK envelope resources[]

	// Insight-specific (only when EventCategory == "Insight")
	InsightDetails *InsightDetails

	// Verb classification (computed once during Parse via existing
	// internal/aws/ct_events_severity.go ClassifyCTVerb)
	Verb string // "R" | "W" | "D" | "S" | "I" | "N" | "?"

	// Status is the severity tier computed from the event (from resource.Resource.Status).
	// One of "ct-info" | "ct-attention" | "ct-danger".
	Status string
}

// UserIdentity holds the parsed userIdentity block. Discriminated by Type.
// All 12 variants from taxonomy §4 are representable.
type UserIdentity struct {
	Type        string // "IAMUser" | "AssumedRole" | "Root" | "AWSService" | ...
	PrincipalID string
	ARN         string
	AccountID   string
	UserName    string // present for IAMUser, Root, FederatedUser
	InvokedBy   string // present for AWSService delegation chains
	AccessKeyID string

	// Session context (present for AssumedRole, FederatedUser, SAMLUser, WebIdentityUser, IdentityCenterUser)
	SessionContext *SessionContext
}

// SessionContext holds the sessionContext block within userIdentity.
type SessionContext struct {
	Attributes          SessionAttributes
	SessionIssuer       *SessionIssuer       // present for AssumedRole — the underlying role
	WebIDFederationData *WebIDFederationData // present for IRSA/OIDC
	SourceIdentity      string
}

// SessionAttributes holds the attributes sub-block inside sessionContext.
type SessionAttributes struct {
	MFAAuthenticated bool
	CreationDate     time.Time
}

// SessionIssuer holds the sessionIssuer sub-block inside sessionContext.
type SessionIssuer struct {
	Type        string // typically "Role" or "User"
	PrincipalID string
	ARN         string
	AccountID   string
	UserName    string // role name or user name
}

// WebIDFederationData holds the webIdFederationData sub-block inside sessionContext.
type WebIDFederationData struct {
	FederatedProvider string
	Audience          string
}

// ResourceRef represents a single entry in the CloudTrail event resources[] array.
type ResourceRef struct {
	ARN       string
	AccountID string
	Type      string // e.g. "AWS::IAM::Role"
}

// InsightDetails holds the insightDetails block — populated only when EventCategory == "Insight".
type InsightDetails struct {
	State          string // "Start" | "End"
	EventSource    string
	EventName      string
	InsightType    string // e.g. "ApiCallRateInsight"
	InsightContext map[string]any
}

// Section is the output unit of BuildSections. Each Section corresponds to one
// labelled group of rows in the detail view.
type Section struct {
	Name string // one of the SectionXxx constants
	Rows []Row
}

// Row is a single labeled entry inside a Section.
type Row struct {
	Key         string // e.g. "Principal", "Event", "Bucket", "errorCode"
	Value       string // pre-rendered display string (no ANSI)
	IsNavigable bool   // true when Value is a navigable resource reference
	TargetType  string // resource type to navigate to (e.g. "role", "ec2", "s3")
	FieldPath   string // synthetic path for cursor identity (e.g. "ACTOR.Principal")
	Severity    string // OPTIONAL: severity tier for value coloring ("ct-info"|"ct-attention"|"ct-danger")
	// Set ONLY on the Event row in ACTION (FR-002 single-cell exception).
	// All other rows leave this empty and render through neutral ColDetailVal.
	NavID string // Optional navigation identifier. When non-empty, navigation dispatch uses
	// this instead of Value. Used when the display Value (e.g. full ARN) differs
	// from the navigable ID (e.g. bare role name).
}
