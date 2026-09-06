// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbi_codes.go — canonical FindingCode constants for the dbi resource type
// (RDS DB instance). The fetcher writes Findings using
// these codes; the dbi Color func reads wave1 Findings (Source == "wave1")
// to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeDBIFailed                   domain.FindingCode = "dbi.broken.failed"
	CodeDBIStorageFull              domain.FindingCode = "dbi.broken.storage_full"
	CodeDBIIncompatibleNetwork      domain.FindingCode = "dbi.broken.incompatible_network"
	CodeDBIIncompatibleOptionGroup  domain.FindingCode = "dbi.broken.incompatible_option_group"
	CodeDBIIncompatibleParameters   domain.FindingCode = "dbi.broken.incompatible_parameters"
	CodeDBIIncompatibleRestore      domain.FindingCode = "dbi.broken.incompatible_restore"
	CodeDBIRestoreError             domain.FindingCode = "dbi.broken.restore_error"
	CodeDBIEncryptionKeyUnavailable domain.FindingCode = "dbi.broken.encryption_key_unavailable"
	CodeDBIStopped                  domain.FindingCode = "dbi.broken.stopped"

	CodeDBITransitional          domain.FindingCode = "dbi.warn.transitional"
	CodeDBINoAutomatedBackups    domain.FindingCode = "dbi.warn.no_automated_backups"
	CodeDBIPubliclyAccessible    domain.FindingCode = "dbi.warn.publicly_accessible"
	CodeDBIUnencryptedStorage    domain.FindingCode = "dbi.warn.unencrypted_storage"
	CodeDBIDeletionProtectionOff domain.FindingCode = "dbi.warn.deletion_protection_off"
)

// Security-posture findings (Prowler gap closure). Each is evaluated
// independently by rdsPostureFindings / rdsCACertFinding in rds_posture.go.
const (
	CodeDBISingleAZ          domain.FindingCode = "dbi.single-az"
	CodeDBIMinorUpgradeOff   domain.FindingCode = "dbi.minor-upgrade-off"
	CodeDBIIAMAuthOff        domain.FindingCode = "dbi.iam-auth-off"
	CodeDBIDefaultMasterUser domain.FindingCode = "dbi.default-master-user"
	CodeDBICACertExpiring    domain.FindingCode = "dbi.ca-cert-expiring"
)

// dbiPostureCodes binds the shared RDS posture predicates to the dbi codes.
var dbiPostureCodes = rdsPostureCodes{
	singleAZ:          CodeDBISingleAZ,
	minorUpgrade:      CodeDBIMinorUpgradeOff,
	iamAuth:           CodeDBIIAMAuthOff,
	defaultMasterUser: CodeDBIDefaultMasterUser,
}

// CodeDBINotInBackupPlan — no backup plan selection matches this instance.
// Automated backups are a separate setting and do not satisfy it.
// Severity: SevWarn.
const CodeDBINotInBackupPlan domain.FindingCode = "dbi.not-in-backup-plan"
