package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeSecretStateDeleted          domain.FindingCode = "secrets.state.deleted"
	CodeSecretStateRotationOverdue  domain.FindingCode = "secrets.state.rotation_overdue"
	CodeSecretStateDormant          domain.FindingCode = "secrets.state.dormant"
)
