package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeOpenSearchDeleting             domain.FindingCode = "opensearch.dim.deleting"
	CodeOpenSearchIsolated             domain.FindingCode = "opensearch.broken.isolated"
	CodeOpenSearchProcessing           domain.FindingCode = "opensearch.warn.processing"
	CodeOpenSearchSoftwareUpdateForced domain.FindingCode = "opensearch.warn.software_update_forced"
	CodeOpenSearchEncryptionAtRestOff  domain.FindingCode = "opensearch.warn.encryption_at_rest_off"
)
