// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// athena_codes.go — canonical FindingCode constants for the athena resource
// type. The fetcher writes Findings using these codes; the athena Color func
// reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// athenaCodeWorkgroupDisabled — workgroup State is DISABLED, an
	// admin-initiated action that blocks all query execution against it.
	athenaCodeWorkgroupDisabled domain.FindingCode = "athena.workgroup-disabled"
)
