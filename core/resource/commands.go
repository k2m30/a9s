// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import "slices"

// CostsCommandNames are the -c/--command and :command aliases that open the
// Cost Explorer screen instead of a resource list — the single resolution
// cmd/a9s's CLI flag validation, the TUI's colon-command dispatch, and the
// runtime's one-shot startup navigation all share (no dual truth).
var CostsCommandNames = []string{"costs", "ce"}

// IsCostsCommand reports whether cmd names the Cost Explorer pseudo-command.
func IsCostsCommand(cmd string) bool {
	return slices.Contains(CostsCommandNames, cmd)
}
