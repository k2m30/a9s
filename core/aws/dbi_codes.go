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

// S5 operator sentences for the dbi posture findings.
const (
	dbiSingleAZDetail          = "The instance runs in one Availability Zone, so an AZ failure takes the database down until you restore it. Enable Multi-AZ to keep a synchronous standby in a second AZ."
	dbiMinorUpgradeDetail      = "Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself."
	dbiIAMAuthDetail           = "Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities."
	dbiDefaultMasterUserDetail = "The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one."
	dbiCACertExpiringDetail    = "The server certificate expires soon; clients that verify the connection will refuse to talk to it once it does. Rotate the instance onto the current certificate authority during a maintenance window."
)

// dbiPostureCodes binds the shared RDS posture predicates to the dbi codes.
var dbiPostureCodes = rdsPostureCodes{
	singleAZ:                CodeDBISingleAZ,
	singleAZDetail:          dbiSingleAZDetail,
	minorUpgrade:            CodeDBIMinorUpgradeOff,
	minorUpgradeDetail:      dbiMinorUpgradeDetail,
	iamAuth:                 CodeDBIIAMAuthOff,
	iamAuthDetail:           dbiIAMAuthDetail,
	defaultMasterUser:       CodeDBIDefaultMasterUser,
	defaultMasterUserDetail: dbiDefaultMasterUserDetail,
}

// CodeDBINotInBackupPlan — no backup plan selection matches this instance.
// Automated backups are a separate setting and do not satisfy it.
// Severity: SevWarn.
const CodeDBINotInBackupPlan domain.FindingCode = "dbi.not-in-backup-plan"

// dbiNotInBackupPlanDetail is the S5 operator sentence for it.
const dbiNotInBackupPlanDetail = "No backup plan selects this database, so its retention is whatever the instance's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on."
