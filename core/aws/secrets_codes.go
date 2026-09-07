// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	CodeSecretStaleValue domain.FindingCode = "secrets.value.stale"
)
