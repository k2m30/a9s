package tui

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestMain installs the AWS catalog before any internal/tui test runs.
// See tests/unit/testmain_test.go for the rationale.
func TestMain(m *testing.M) {
	aws.Install()
	resource.WireProjection()
	os.Exit(m.Run())
}
