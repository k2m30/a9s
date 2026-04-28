package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeNATStatePending  domain.FindingCode = "nat.state.pending"
	CodeNATStateDeleting domain.FindingCode = "nat.state.deleting"
	CodeNATStateFailed   domain.FindingCode = "nat.state.failed"
)
