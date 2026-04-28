package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeVPCEStatePendingAcceptance domain.FindingCode = "vpce.state.pending_acceptance"
	CodeVPCEStatePending           domain.FindingCode = "vpce.state.pending"
	CodeVPCEStateDeleting          domain.FindingCode = "vpce.state.deleting"
	CodeVPCEStateFailed            domain.FindingCode = "vpce.state.failed"
	CodeVPCEStateRejected          domain.FindingCode = "vpce.state.rejected"
	CodeVPCEStateExpired           domain.FindingCode = "vpce.state.expired"
	CodeVPCEStatePartial           domain.FindingCode = "vpce.state.partial"
)
