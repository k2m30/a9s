package aws

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestMain installs the AWS catalog before any internal aws-package test runs.
// Internal tests reach into resource.GetPaginatedFetcher (and other catalog-
// backed accessors), which panic until SetTypes has been called.
//
// We don't import internal/aws here — we're already in it. Install is local.
// WireProjection replaces the legacy internal/resource init() per AS-731.
func TestMain(m *testing.M) {
	Install()
	resource.WireProjection()
	os.Exit(m.Run())
}
