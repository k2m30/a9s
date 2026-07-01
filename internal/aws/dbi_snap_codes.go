// dbi_snap_codes.go — canonical FindingCode constants for the dbi-snap
// resource type (RDS DB instance snapshot).
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeDBISnapFailed       domain.FindingCode = "dbi-snap.broken.failed"
	CodeDBISnapIncompatible domain.FindingCode = "dbi-snap.broken.incompatible"

	CodeDBISnapCreating    domain.FindingCode = "dbi-snap.warn.creating"
	CodeDBISnapUnencrypted domain.FindingCode = "dbi-snap.warn.unencrypted"
)
