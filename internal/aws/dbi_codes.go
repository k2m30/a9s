// dbi_codes.go — canonical FindingCode constants for the dbi resource type
// (RDS DB instance). Phase 03 PR-03e. The fetcher writes Findings using
// these codes; the dbi Color func reads wave1 Findings (Source == "wave1")
// to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeDBIFailed                   domain.FindingCode = "dbi.broken.failed"
	CodeDBIStorageFull              domain.FindingCode = "dbi.broken.storage_full"
	CodeDBIIncompatibleNetwork      domain.FindingCode = "dbi.broken.incompatible_network"
	CodeDBIIncompatibleOptionGroup  domain.FindingCode = "dbi.broken.incompatible_option_group"
	CodeDBIIncompatibleParameters   domain.FindingCode = "dbi.broken.incompatible_parameters"
	CodeDBIIncompatibleRestore      domain.FindingCode = "dbi.broken.incompatible_restore"
	CodeDBIRestoreError             domain.FindingCode = "dbi.broken.restore_error"
	CodeDBIEncryptionKeyUnavailable domain.FindingCode = "dbi.broken.encryption_key_unavailable"

	CodeDBITransitional          domain.FindingCode = "dbi.warn.transitional"
	CodeDBINoAutomatedBackups    domain.FindingCode = "dbi.warn.no_automated_backups"
	CodeDBIPubliclyAccessible    domain.FindingCode = "dbi.warn.publicly_accessible"
	CodeDBIUnencryptedStorage    domain.FindingCode = "dbi.warn.unencrypted_storage"
	CodeDBIDeletionProtectionOff domain.FindingCode = "dbi.warn.deletion_protection_off"
)
