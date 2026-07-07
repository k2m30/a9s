package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// SFN execution-history event-status finding emitted by
// FetchSFNExecutionHistory. Any event whose type carries the *Failed/
// *TimedOut/ExecutionAborted suffix family (see ClassifyEventStatus)
// classifies as broken; all other event types emit no finding.
const (
	CodeSFNHistoryEventFailed domain.FindingCode = "sfn-execution-history.broken.event_failed"
)
