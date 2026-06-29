//go:build integration

// Package webintegration drives the real web server (internal/web) over HTTP
// in demo mode and asserts on GET /state JSON. All tests are deterministic —
// demo fetchers are synchronous, DrainSync runs inline, no sleeps needed.
package webintegration

import (
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestMain(m *testing.M) {
	// Mirror cmd/a9s/main.go startup sequence — catalog must be populated
	// before any Server is constructed, or resource.AllResourceTypes() panics.
	aws.Install()
	resource.WireProjection()
	resource.BootstrapActiveNavFields()
	os.Exit(m.Run())
}
