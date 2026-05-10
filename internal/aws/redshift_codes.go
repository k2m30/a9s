package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeRedshiftIncompatibleHSM        domain.FindingCode = "redshift.broken.incompatible_hsm"
	CodeRedshiftIncompatibleNetwork    domain.FindingCode = "redshift.broken.incompatible_network"
	CodeRedshiftIncompatibleParameters domain.FindingCode = "redshift.broken.incompatible_parameters"
	CodeRedshiftIncompatibleRestore    domain.FindingCode = "redshift.broken.incompatible_restore"
	CodeRedshiftHardwareFailure        domain.FindingCode = "redshift.broken.hardware_failure"
	CodeRedshiftStorageFull            domain.FindingCode = "redshift.broken.storage_full"
	CodeRedshiftUnavailable            domain.FindingCode = "redshift.broken.unavailable"
	CodeRedshiftFailed                 domain.FindingCode = "redshift.broken.failed"
	CodeRedshiftCreating               domain.FindingCode = "redshift.warn.creating"
	CodeRedshiftModifying              domain.FindingCode = "redshift.warn.modifying"
	CodeRedshiftResizing               domain.FindingCode = "redshift.warn.resizing"
	CodeRedshiftRebooting              domain.FindingCode = "redshift.warn.rebooting"
	CodeRedshiftRenaming               domain.FindingCode = "redshift.warn.renaming"
	CodeRedshiftDeleting               domain.FindingCode = "redshift.warn.deleting"
	CodeRedshiftMaintenance            domain.FindingCode = "redshift.warn.maintenance"
	CodeRedshiftAvailabilityModifying  domain.FindingCode = "redshift.warn.availability_modifying"
	CodeRedshiftPendingChange          domain.FindingCode = "redshift.warn.pending_change"
	CodeRedshiftMaintenanceDeferred    domain.FindingCode = "redshift.warn.maintenance_deferred"
	CodeRedshiftPubliclyAccessible     domain.FindingCode = "redshift.warn.publicly_accessible"
	CodeRedshiftUnencryptedAtRest      domain.FindingCode = "redshift.warn.unencrypted_at_rest"
)
