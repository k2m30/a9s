// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// CloudWatch alarm-history state-transition findings emitted by
// FetchAlarmHistory. A StateUpdate item's HistoryData JSON is parsed for the
// transitioned-to state; ALARM classifies as broken, INSUFFICIENT_DATA as
// warn — the classes the alarm state table gives the live alarm resource
// (core/aws/catalog_monitoring.go). Transitions to OK, and
// ConfigurationUpdate/Action item types, emit no finding.
const (
	CodeAlarmHistoryStateAlarm            domain.FindingCode = "alarm-history.broken.alarm"
	CodeAlarmHistoryStateInsufficientData domain.FindingCode = "alarm-history.warn.insufficient_data"
)
