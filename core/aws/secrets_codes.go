package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeSecretStateDeleted         domain.FindingCode = "secrets.state.deleted"
	CodeSecretStateRotationOverdue domain.FindingCode = "secrets.state.rotation_overdue"
	CodeSecretStateDormant         domain.FindingCode = "secrets.state.dormant"

	// CodeSecretRotationDisabled — secret has no automatic rotation
	// configured. Severity: SevWarn.
	CodeSecretRotationDisabled domain.FindingCode = "secrets.rotation.disabled"

	// CodeSecretStaleValue — secret's value has not changed in over 365
	// days. Severity: SevWarn.
	//
	// Note: colorSecrets also has a "last_accessed > 180d" Warning branch,
	// but that condition always produces secretStateFindings' DORMANT status
	// first (same threshold, same source field) — so it can never reach this
	// structural fallback and has no corresponding FindingCode here.
	CodeSecretStaleValue domain.FindingCode = "secrets.value.stale"
)
