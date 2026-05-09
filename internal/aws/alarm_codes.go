package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeAlarmStateAlarm         domain.FindingCode = "alarm.state.alarm"
	CodeAlarmStateInsufficient  domain.FindingCode = "alarm.state.insufficient_data"
	CodeAlarmNoActions          domain.FindingCode = "alarm.no_actions"
)
