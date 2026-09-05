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

// S5 operator sentences for the dbc posture findings.
const (
	dbcSingleAZDetail          = "The cluster has no instance in a second Availability Zone, so an AZ failure takes it down until you restore it. Add a replica in another AZ."
	dbcMinorUpgradeDetail      = "Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself."
	dbcIAMAuthDetail           = "Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities."
	dbcDefaultMasterUserDetail = "The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one."
)

// dbcPostureCodes binds the shared RDS posture predicates to the dbc codes.
var dbcPostureCodes = rdsPostureCodes{
	singleAZ:                CodeDBCSingleAZ,
	singleAZDetail:          dbcSingleAZDetail,
	minorUpgrade:            CodeDBCMinorUpgradeOff,
	minorUpgradeDetail:      dbcMinorUpgradeDetail,
	iamAuth:                 CodeDBCIAMAuthOff,
	iamAuthDetail:           dbcIAMAuthDetail,
	defaultMasterUser:       CodeDBCDefaultMasterUser,
	defaultMasterUserDetail: dbcDefaultMasterUserDetail,
}
