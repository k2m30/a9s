// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbc_codes.go — canonical FindingCode constants for the dbc resource type
// (DocumentDB cluster + Aurora DB cluster — they share the dbc short-name).
package aws

import "github.com/k2m30/a9s/v3/core/domain"

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

// Security-posture findings (Prowler gap closure). The same conditions the
// dbi instance fetcher evaluates, on the cluster shapes — emitted by both
// computeDBCFindings (DocumentDB) and computeRDSDBClusterFindings (Aurora /
// Multi-AZ) through the shared predicates in rds_posture.go.
const (
	CodeDBCSingleAZ          domain.FindingCode = "dbc.single-az"
	CodeDBCMinorUpgradeOff   domain.FindingCode = "dbc.minor-upgrade-off"
	CodeDBCIAMAuthOff        domain.FindingCode = "dbc.iam-auth-off"
	CodeDBCDefaultMasterUser domain.FindingCode = "dbc.default-master-user"
)

// dbcPostureCodes binds the shared RDS posture predicates to the dbc codes.
var dbcPostureCodes = rdsPostureCodes{
	singleAZ:          CodeDBCSingleAZ,
	minorUpgrade:      CodeDBCMinorUpgradeOff,
	iamAuth:           CodeDBCIAMAuthOff,
	defaultMasterUser: CodeDBCDefaultMasterUser,
}

// CodeDBCNotInBackupPlan — no backup plan selection matches this cluster
// (Aurora or DocumentDB). Severity: SevWarn.
const CodeDBCNotInBackupPlan domain.FindingCode = "dbc.not-in-backup-plan"
