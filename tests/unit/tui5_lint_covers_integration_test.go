// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_lint_covers_integration_test.go — tests/integration/ is behind the
// `integration` build tag, and golangci-lint sets no build tags, so every file
// in that directory was invisible to `make lint`. The gate below pins the
// second, tagged pass that makes it visible.
package unit_test

import (
	"os"
	"strings"
	"testing"
)

// TestLintTargetCoversTaggedIntegrationTests reads the Makefile's lint recipe
// and asserts it runs a pass with the integration build tag. Without it a
// whole test directory is unlinted, and nothing else in the tree would say so.
func TestLintTargetCoversTaggedIntegrationTests(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("reading Makefile: %v", err)
	}

	var recipe []string
	inTarget := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "lint:") {
			inTarget = true
			continue
		}
		if inTarget {
			if !strings.HasPrefix(line, "\t") {
				break
			}
			recipe = append(recipe, strings.TrimSpace(line))
		}
	}
	if len(recipe) == 0 {
		t.Fatal("no lint target found in the Makefile")
	}

	tagged := false
	for _, cmd := range recipe {
		if strings.Contains(cmd, "build-tags") && strings.Contains(cmd, "integration") {
			tagged = true
		}
	}
	if !tagged {
		t.Errorf("the lint target runs no integration-tagged pass, so tests/integration/ "+
			"is never linted:\n  %s", strings.Join(recipe, "\n  "))
	}
}
