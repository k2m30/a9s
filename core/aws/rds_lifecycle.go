// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

// rdsUnrecognisedStatusDetail is the detail of a DB cluster or DB instance
// status that appears in neither AWS status table below.
const rdsUnrecognisedStatusDetail = "AWS reports a status a9s does not recognise, so a9s cannot tell whether the database is serving or whether the state clears on its own. Check the database's recent events in the AWS console before relying on it."

// dbcStatuses is the DB cluster status table in the Amazon Aurora User Guide,
// "Viewing DB cluster status"
// (https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/accessing-monitoring.html#Aurora.Status).
// The DocumentDB cluster status table
// (https://docs.aws.amazon.com/documentdb/latest/developerguide/monitoring_docdb-cluster_status.html)
// is a subset of it for instance-based clusters.
var dbcStatuses = []string{ //nolint:gochecknoglobals // static AWS status domain
	"available", "backing-up", "backtracking", "cloning-failed", "creating",
	"deleting", "failed", "failing-over", "inaccessible-encryption-credentials",
	"inaccessible-encryption-credentials-recoverable", "incompatible-parameters",
	"maintenance", "migrating", "migration-failed", "modifying", "preparing-data-migration",
	"promoting", "renaming", "resetting-master-credentials", "starting", "stopped",
	"stopping", "storage-optimization", "update-iam-db-auth", "upgrade-failed", "upgrading",
}

// dbiStatuses is the DB instance status tables in the Amazon RDS User Guide,
// "Viewing Amazon RDS DB instance status"
// (https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html),
// and the Amazon Aurora User Guide, "Viewing DB instance status in an Aurora
// cluster"
// (https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/accessing-monitoring.html#Overview.DBInstance.Status),
// which adds `backtracking`.
var dbiStatuses = []string{ //nolint:gochecknoglobals // static AWS status domain
	"available", "stopped",
	"backing-up", "backtracking", "configuring-enhanced-monitoring",
	"configuring-iam-database-auth", "configuring-log-exports",
	"converting-to-vpc", "creating", "delete-precheck", "deleting",
	"maintenance", "modifying", "moving-to-vpc", "rebooting",
	"renaming", "resetting-master-credentials", "starting",
	"stopping", "storage-config-upgrade", "storage-initialization",
	"storage-optimization", "upgrading",
	"failed", "storage-full", "incompatible-network", "incompatible-option-group",
	"incompatible-parameters", "incompatible-restore", "restore-error",
	"inaccessible-encryption-credentials", "inaccessible-encryption-credentials-recoverable",
	"incompatible-create", "insufficient-capacity", "upgrade-failed",
}
