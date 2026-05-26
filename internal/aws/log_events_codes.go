package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// CloudWatch Logs message-class findings emitted by FetchLogEvents and
// FetchLambdaInvocationLogs. ERROR-class messages classify as broken; WARN as
// warn. REPORT/META/uncategorized messages emit no finding (healthy).
const (
	CodeCWLogError domain.FindingCode = "log-event.broken.error"
	CodeCWLogWarn  domain.FindingCode = "log-event.warn.warning"
)
