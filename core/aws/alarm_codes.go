// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeAlarmStateAlarm        domain.FindingCode = "alarm.state.alarm"
	CodeAlarmStateInsufficient domain.FindingCode = "alarm.state.insufficient_data"
	CodeAlarmNoActions         domain.FindingCode = "alarm.no_actions"

	// CodeAlarmActionsDisabled — ActionsEnabled==false. The alarm still
	// evaluates but fires nothing. Complements CodeAlarmNoActions: an alarm
	// can have actions configured and still have them switched off.
	CodeAlarmActionsDisabled domain.FindingCode = "alarm.actions-disabled"
)

// alarmActionsDisabledDetail is the S5 sentence for CodeAlarmActionsDisabled.
const alarmActionsDisabledDetail = "The alarm still changes state but runs none of its actions, so nobody is notified when it triggers. Switch actions back on for this alarm."
