// app_accessors.go — exported Model accessors for tests and external callers.
//
// These methods live outside _test.go so that tests in tests/unit/ (package
// unit_test) can reach them without being in the same package. Production code
// does not call them.
package tui

import (
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// EnrichmentGen returns the current session-wide enrichment generation counter.
// This accessor is used by tests to capture the pre-switch gen value and verify
// that post-switch messages carrying the old gen are correctly discarded.
//
// Note: the method name shadows the promoted Session.EnrichmentGen field.
// All write sites MUST use m.Session.EnrichmentGen explicitly.
func (m Model) EnrichmentGen() domain.Gen {
	return m.Session.EnrichmentGen
}

// FlashGen returns the current tui-adapter flash generation counter.
// Test-only accessor for verifying handleClientsReady's `hasFlashWork` gate:
// non-flash success paths and stale-gen paths must NOT advance this counter,
// otherwise an in-flight ClearFlashMsg for the current flash gets silently
// invalidated. (CXR/Architect Stage 5 R3+R4 finding regression coverage.)
func (m Model) FlashGen() domain.Gen {
	return m.flash.gen
}

// ActiveDetailResource is an exported test-only accessor for the top-of-stack
// DetailModel resource — production code does not call it.
// Returns ok=false when the active view is not a DetailModel.
func (m Model) ActiveDetailResource() (resource.Resource, bool) {
	if d, ok := m.activeView().(*views.DetailModel); ok {
		return d.SourceResource(), true
	}
	return resource.Resource{}, false
}

// ActiveListResources is an exported test-only accessor for the top-of-stack
// ResourceListModel's resource slice — production code does not call it.
// Returns nil if the active view is not a ResourceListModel.
func (m Model) ActiveListResources() []resource.Resource {
	if rl, ok := m.activeView().(*views.ResourceListModel); ok {
		return rl.AllResources()
	}
	return nil
}
