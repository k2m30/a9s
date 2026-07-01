// dbc_snap_codes.go — canonical FindingCode constants for the dbc-snap
// resource type (DocumentDB + Aurora DB cluster snapshot).
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeDBCSnapFailed       domain.FindingCode = "dbc-snap.broken.failed"
	CodeDBCSnapIncompatible domain.FindingCode = "dbc-snap.broken.incompatible"

	CodeDBCSnapCreating     domain.FindingCode = "dbc-snap.warn.creating"
	CodeDBCSnapManualUnused domain.FindingCode = "dbc-snap.warn.manual_unused"
)
