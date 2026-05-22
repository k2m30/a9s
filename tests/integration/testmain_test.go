//go:build !integration

package integration

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestMain installs the AWS catalog before any non-integration test runs in
// this package. The `integration`-tagged TestMain in cli_test.go also calls
// aws.Install at startup so both modes have a populated catalog.
func TestMain(m *testing.M) {
	aws.Install()
	resource.WireProjection()
	os.Exit(m.Run())
}
