package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// CloudFormation stack-resource findings emitted by FetchCfnResources. The
// *_FAILED resource statuses classify as broken; *_IN_PROGRESS as warn;
// DELETE_COMPLETE as dim. Steady-state *_COMPLETE rows emit no finding and
// render healthy.
const (
	CodeCfnResourceFailed     domain.FindingCode = "cfn-resource.broken.failed"
	CodeCfnResourceInProgress domain.FindingCode = "cfn-resource.warn.in_progress"
	CodeCfnResourceDeleted    domain.FindingCode = "cfn-resource.dim.deleted"
)
