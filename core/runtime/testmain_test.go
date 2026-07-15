// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestMain installs the AWS catalog before any runtime test runs.
// See tests/unit/testmain_test.go for the rationale, including the
// hermetic A9S_CONFIG_FOLDER default applied below.
func TestMain(m *testing.M) {
	var cleanupDir string
	if os.Getenv("A9S_CONFIG_FOLDER") == "" {
		if dir, err := os.MkdirTemp("", "a9s-testmain-config-*"); err == nil {
			os.Setenv("A9S_CONFIG_FOLDER", dir) //nolint:errcheck // best-effort hermetic default, not test-critical
			cleanupDir = dir
		}
	}
	aws.Install()
	resource.WireProjection()
	code := m.Run()
	if cleanupDir != "" {
		os.RemoveAll(cleanupDir) //nolint:errcheck // best-effort cleanup, process is exiting regardless
	}
	os.Exit(code)
}
