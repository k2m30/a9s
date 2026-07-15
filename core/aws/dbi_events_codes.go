package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeDBIEventFailure    domain.FindingCode = "dbi_events.broken.failure"
	CodeDBIEventLowStorage domain.FindingCode = "dbi_events.broken.low_storage"
	CodeDBIEventFailover   domain.FindingCode = "dbi_events.warn.failover"
	CodeDBIEventRecovery   domain.FindingCode = "dbi_events.warn.recovery"
)
