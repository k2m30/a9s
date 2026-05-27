package tui

// derive_helper.go — retained no-op shims for the 7 PR-03a-shim entry points.
//
// Post-W1.4a, fetchers write wave1 Findings directly (see AS-1393/W1.1) so the
// derive shim that these helpers wrapped has been deleted. The function names
// are kept as no-ops to avoid churning every caller in handlers_*.go; W1.4b
// (AS-1428) will remove the calls once the surrounding Status/Issues migration
// lands.

import (
	"github.com/k2m30/a9s/v3/internal/resource"
)

// deriveFindingsForType is a no-op after W1.4a; fetchers populate wave1
// Findings directly. The function is retained so handlers_*.go callers do not
// need to change.
func (m *Model) deriveFindingsForType(short string, rows []resource.Resource) {
	// W1.4a: fetchers write wave1 Findings directly; no derivation needed.
}

// deriveFindingsForResource is a no-op after W1.4a; fetchers populate wave1
// Findings directly. The function is retained so single-resource callers do
// not need to change.
func (m *Model) deriveFindingsForResource(short string, r *resource.Resource) {
	// W1.4a: fetchers write wave1 Findings directly; no derivation needed.
}
