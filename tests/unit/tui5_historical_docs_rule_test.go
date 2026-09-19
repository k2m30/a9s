// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Docs/historical/ pages describe the
// world before a refactor; editing them to match today destroys the record
// they exist for, so a symbol sweep of the docs leaves them alone.
package unit_test

import (
	"os"
	"strings"
	"testing"
)

// TestDevelopmentProcessStatesTheHistoricalDocsRule asserts the process doc
// names docs/historical/ and says what a symbol sweep does with it.
func TestDevelopmentProcessStatesTheHistoricalDocsRule(t *testing.T) {
	data, err := os.ReadFile("../../docs/development-process.md")
	if err != nil {
		t.Fatalf("reading development-process.md: %v", err)
	}

	// One line, so a rule spelled out somewhere else in a long numbered list
	// that happens to use all three words does not count as having said it.
	var stated bool
	for _, line := range strings.Split(string(data), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(line, "docs/historical/") &&
			strings.Contains(lower, "sweep") &&
			strings.Contains(lower, "exempt") {
			stated = true
		}
	}
	if !stated {
		t.Error("docs/development-process.md does not say whether docs/historical/ is exempt " +
			"from the deleted-symbol sweep, so every sweep decides again")
	}
}
