// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// rdsUnrecognisedStatusDetail is the detail of a DB cluster or DB instance
// status that appears in neither AWS status table below.
const rdsUnrecognisedStatusDetail = "AWS reports a status a9s does not recognise, so a9s cannot tell whether the database is serving or whether the state clears on its own. Check the database's recent events in the AWS console before relying on it."

// rdsLifecycle binds one resource type's finding codes to the status table
// AWS documents for it. DB clusters and DB instances read their status
// through the same finding method, so `stopped` and a status in neither
// table render the same on both.
type rdsLifecycle struct {
	broken           map[string]domain.FindingCode
	transitional     map[string]struct{}
	transitionalCode domain.FindingCode
	stopped          domain.FindingCode
	unrecognised     domain.FindingCode
}

// finding returns the lifecycle finding for status; ok is false for
// `available` and for a resource AWS reported no status for. The
// transitional finding is filled with transitionalPhrase.
func (l rdsLifecycle) finding(status, transitionalPhrase string) (domain.Finding, bool) {
	if status == "" || status == "available" {
		return domain.Finding{}, false
	}
	if status == "stopped" {
		return wave1Finding(l.stopped), true
	}
	if code, ok := l.broken[status]; ok {
		return wave1Finding(code), true
	}
	if _, ok := l.transitional[status]; ok {
		return wave1Finding(l.transitionalCode, transitionalPhrase), true
	}
	return wave1Finding(l.unrecognised, status), true
}

// dbcLifecycle maps the DB cluster status table in the Amazon Aurora User
// Guide, "Viewing DB cluster status"
// (https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/accessing-monitoring.html#Aurora.Status).
// The DocumentDB cluster status table
// (https://docs.aws.amazon.com/documentdb/latest/developerguide/monitoring_docdb-cluster_status.html)
// is a subset of it for instance-based clusters.
var dbcLifecycle = rdsLifecycle{
	broken: map[string]domain.FindingCode{
		"failed":                              CodeDBCFailed,
		"inaccessible-encryption-credentials": CodeDBCEncryptionKeyUnreachable,
		"inaccessible-encryption-credentials-recoverable": CodeDBCEncryptionKeyRecoverable,
		"incompatible-parameters":                         CodeDBCIncompatibleParameters,
		"cloning-failed":                                  CodeDBCCloningFailed,
		"migration-failed":                                CodeDBCMigrationFailed,
		"upgrade-failed":                                  CodeDBCUpgradeFailed,
	},
	transitional: map[string]struct{}{
		"backing-up": {}, "backtracking": {}, "creating": {}, "deleting": {},
		"failing-over": {}, "maintenance": {}, "migrating": {}, "modifying": {},
		"promoting": {}, "preparing-data-migration": {}, "renaming": {},
		"resetting-master-credentials": {}, "starting": {}, "stopping": {},
		"storage-optimization": {}, "update-iam-db-auth": {}, "upgrading": {},
	},
	transitionalCode: CodeDBCTransitional,
	stopped:          CodeDBCStopped,
	unrecognised:     CodeDBCUnrecognisedStatus,
}

// dbiLifecycle maps the DB instance status tables in the Amazon RDS User
// Guide, "Viewing Amazon RDS DB instance status"
// (https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html),
// and the Amazon Aurora User Guide, "Viewing DB instance status in an Aurora
// cluster"
// (https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/accessing-monitoring.html#Overview.DBInstance.Status),
// which adds `backtracking`.
var dbiLifecycle = rdsLifecycle{
	broken: map[string]domain.FindingCode{
		"failed":                                          CodeDBIFailed,
		"storage-full":                                    CodeDBIStorageFull,
		"incompatible-create":                             CodeDBIIncompatibleCreate,
		"incompatible-network":                            CodeDBIIncompatibleNetwork,
		"incompatible-option-group":                       CodeDBIIncompatibleOptionGroup,
		"incompatible-parameters":                         CodeDBIIncompatibleParameters,
		"incompatible-restore":                            CodeDBIIncompatibleRestore,
		"insufficient-capacity":                           CodeDBIInsufficientCapacity,
		"restore-error":                                   CodeDBIRestoreError,
		"inaccessible-encryption-credentials":             CodeDBIEncryptionKeyUnavailable,
		"inaccessible-encryption-credentials-recoverable": CodeDBIEncryptionKeyRecoverable,
		"upgrade-failed":                                  CodeDBIUpgradeFailed,
	},
	transitional: map[string]struct{}{
		"backing-up": {}, "backtracking": {}, "configuring-enhanced-monitoring": {},
		"configuring-iam-database-auth": {}, "configuring-log-exports": {},
		"converting-to-vpc": {}, "creating": {}, "delete-precheck": {}, "deleting": {},
		"maintenance": {}, "modifying": {}, "moving-to-vpc": {}, "rebooting": {},
		"renaming": {}, "resetting-master-credentials": {}, "starting": {},
		"stopping": {}, "storage-config-upgrade": {}, "storage-initialization": {},
		"storage-optimization": {}, "upgrading": {},
	},
	transitionalCode: CodeDBITransitional,
	stopped:          CodeDBIStopped,
	unrecognised:     CodeDBIUnrecognisedStatus,
}
