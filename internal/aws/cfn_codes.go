package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeCFNStackFailed     domain.FindingCode = "cfn.stack.failed"
	CodeCFNStackRollback   domain.FindingCode = "cfn.stack.rollback"
	CodeCFNStackInProgress domain.FindingCode = "cfn.stack.in_progress"
)
