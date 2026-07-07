package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeCFNStackFailed     domain.FindingCode = "cfn.stack.failed"
	CodeCFNStackRollback   domain.FindingCode = "cfn.stack.rollback"
	CodeCFNStackInProgress domain.FindingCode = "cfn.stack.in_progress"
	// CodeCFNStackDeleted — stack is DELETE_COMPLETE. DescribeStacks still
	// reports it for a retention window after deletion.
	CodeCFNStackDeleted domain.FindingCode = "cfn.stack.deleted"
)
