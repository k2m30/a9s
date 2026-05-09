package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeKMSStatePendingDeletion domain.FindingCode = "kms.state.pending_deletion"
	CodeKMSStateDisabled        domain.FindingCode = "kms.state.disabled"
	CodeKMSStateUnavailable     domain.FindingCode = "kms.state.unavailable"
)
