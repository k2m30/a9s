// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// athenaCodeWorkgroupDisabled — workgroup State is DISABLED, an
	// admin-initiated action that blocks all query execution against it.
	athenaCodeWorkgroupDisabled domain.FindingCode = "athena.workgroup-disabled"

	athenaCodeUnrecognisedState domain.FindingCode = "athena.state.unrecognised"
)
