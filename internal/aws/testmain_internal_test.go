package aws

import (
	"os"
	"testing"
)

// TestMain installs the AWS catalog before any internal aws-package test runs.
// Internal tests reach into resource.GetPaginatedFetcher (and other catalog-
// backed accessors), which panic until SetTypes has been called.
//
// We don't import internal/aws here — we're already in it. Install is local.
func TestMain(m *testing.M) {
	Install()
	os.Exit(m.Run())
}
