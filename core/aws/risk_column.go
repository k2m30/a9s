// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// risk_column.go owns the vocabulary of the "risk" Field, which every type
// that declares it renders as its Status column. The values are read by an
// operator, never by code, so they are lowercase operator words rather than
// the enum-shaped tokens they replaced.
package aws

const (
	riskNoMFA            = "no MFA"
	riskKeyTooOld        = "key too old"
	riskConsoleNeverUsed = "console never used"
	riskConsoleDormant   = "console dormant"
	riskKeyUnused        = "key unused"
	riskTwoActiveKeys    = "two active keys"
	riskAdminPolicy      = "admin policy"
	riskPrivEsc          = "privilege escalation"
	riskUnattached       = "unattached"
	riskStaleValue       = "stale"
	riskPlaintextValue   = "plaintext"
)
