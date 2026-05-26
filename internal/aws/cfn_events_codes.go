package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// CloudFormation stack-event findings emitted by FetchCfnEvents. The
// *_FAILED event statuses classify as broken; *_IN_PROGRESS as warn;
// DELETE_COMPLETE as dim. Steady-state *_COMPLETE rows emit no finding and
// render healthy. Mirrors cfn_resources_codes.go because StackEvent and
// StackResourceSummary share the same ResourceStatus enum.
const (
	CodeCfnEventFailed     domain.FindingCode = "cfn-event.broken.failed"
	CodeCfnEventInProgress domain.FindingCode = "cfn-event.warn.in_progress"
	CodeCfnEventDeleted    domain.FindingCode = "cfn-event.dim.deleted"
)
