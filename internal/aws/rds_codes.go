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
	CodeDBITransitional             domain.FindingCode = "dbi.warn.transitional"
	CodeDBINoAutomatedBackups       domain.FindingCode = "dbi.warn.no_automated_backups"
	CodeDBIPubliclyAccessible       domain.FindingCode = "dbi.warn.publicly_accessible"
	CodeDBIUnencryptedStorage       domain.FindingCode = "dbi.warn.unencrypted_storage"
	CodeDBIDeletionProtectionOff    domain.FindingCode = "dbi.warn.deletion_protection_off"

	CodeDBISnapFailed       domain.FindingCode = "dbi-snap.broken.failed"
	CodeDBISnapIncompatible domain.FindingCode = "dbi-snap.broken.incompatible"
	CodeDBISnapCreating     domain.FindingCode = "dbi-snap.warn.creating"
	CodeDBISnapUnencrypted  domain.FindingCode = "dbi-snap.warn.unencrypted"
)
