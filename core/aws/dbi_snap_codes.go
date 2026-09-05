// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbi_snap_codes.go — canonical FindingCode constants for the dbi-snap
// resource type (RDS DB instance snapshot).
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeDBISnapFailed       domain.FindingCode = "dbi-snap.broken.failed"
	CodeDBISnapIncompatible domain.FindingCode = "dbi-snap.broken.incompatible"

	CodeDBISnapCreating domain.FindingCode = "dbi-snap.warn.creating"

	// CodeDBISnapTransitional — any other non-terminal snapshot state AWS reports
	// (copying, pending, …). Severity: SevWarn.
	CodeDBISnapTransitional domain.FindingCode = "dbi-snap.warn.transitional"
	CodeDBISnapUnencrypted  domain.FindingCode = "dbi-snap.warn.unencrypted"
)
