// dbc_codes.go — canonical FindingCode constants for the dbc resource type
// (DocumentDB cluster + Aurora DB cluster — they share the dbc short-name).
// Phase 03 PR-03e.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeDBCFailed                   domain.FindingCode = "dbc.broken.failed"
	CodeDBCEncryptionKeyUnreachable domain.FindingCode = "dbc.broken.encryption_key_unreachable"
	CodeDBCIncompatibleParameters   domain.FindingCode = "dbc.broken.incompatible_parameters"
	CodeDBCNoWriter                 domain.FindingCode = "dbc.broken.no_writer"

	CodeDBCTransitional          domain.FindingCode = "dbc.warn.transitional"
	CodeDBCDeletionProtectionOff domain.FindingCode = "dbc.warn.deletion_protection_off"
	CodeDBCNotEncryptedAtRest    domain.FindingCode = "dbc.warn.not_encrypted_at_rest"
	CodeDBCNoAutomatedBackups    domain.FindingCode = "dbc.warn.no_automated_backups"
)
