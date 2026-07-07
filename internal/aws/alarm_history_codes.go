package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// CloudWatch alarm-history state-transition findings emitted by
// FetchAlarmHistory. A StateUpdate item's HistoryData JSON is parsed for the
// transitioned-to state; ALARM classifies as broken, INSUFFICIENT_DATA as
// warn — mirroring colorAlarm's classification of the live alarm resource
// (internal/aws/catalog_monitoring.go). Transitions to OK, and
// ConfigurationUpdate/Action item types, emit no finding.
const (
	CodeAlarmHistoryStateAlarm            domain.FindingCode = "alarm-history.broken.alarm"
	CodeAlarmHistoryStateInsufficientData domain.FindingCode = "alarm-history.warn.insufficient_data"
)
