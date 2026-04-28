package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeTGWStatePending   domain.FindingCode = "tgw.state.pending"
	CodeTGWStateModifying domain.FindingCode = "tgw.state.modifying"
	CodeTGWStateDeleting  domain.FindingCode = "tgw.state.deleting"
	CodeTGWStateFailed    domain.FindingCode = "tgw.state.failed"
)
