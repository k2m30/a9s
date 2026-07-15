package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeKMSStatePendingDeletion domain.FindingCode = "kms.state.pending_deletion"
	CodeKMSStateDisabled        domain.FindingCode = "kms.state.disabled"
	CodeKMSStateUnavailable     domain.FindingCode = "kms.state.unavailable"
	CodeKMSAccessDenied         domain.FindingCode = "kms.access-denied"
)
