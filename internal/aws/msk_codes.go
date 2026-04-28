package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeMSKCreating        domain.FindingCode = "msk.warn.creating"
	CodeMSKUpdating        domain.FindingCode = "msk.warn.updating"
	CodeMSKMaintenance     domain.FindingCode = "msk.warn.maintenance"
	CodeMSKRebootingBroker domain.FindingCode = "msk.warn.rebooting_broker"
	CodeMSKHealing         domain.FindingCode = "msk.warn.healing"
	CodeMSKFailed          domain.FindingCode = "msk.broken.failed"
)
