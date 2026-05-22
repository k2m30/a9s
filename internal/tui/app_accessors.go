// app_accessors.go — exported Model accessors for tests and external callers.
//
// These methods live outside _test.go so that tests in tests/unit/ (package
// unit_test) can reach them without being in the same package. Production code
// does not call them.
//
// PR-05a-h4-c (AS-963) deleted the Session() accessor (its return type
// was the last production-side leak of the session package into the tui
// package). Callers in tests must go through m.Core().Session() instead.
// Core() returns the platform-agnostic *runtime.Core so the boundary
// check (`go list .Imports` against internal/tui) no longer reports the
// session package.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// Core returns the runtime-owned *runtime.Core handle. Test-only accessor
// — production code uses m.core directly. Replaces the prior Session()
// accessor whose session-typed return value forced the tui package to
// import the internal/session package.
func (m Model) Core() *runtime.Core {
	return m.core
}

// EnrichmentGen returns the current session-wide enrichment generation counter.
// Test-only accessor.
func (m Model) EnrichmentGen() domain.Gen {
	return m.core.EnrichmentGen()
}

// FlashGen returns the current tui-adapter flash generation counter.
// Test-only accessor.
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

// FetchResourcesCmdForTest returns a tea.Cmd produced by fetchResources for
// the given resourceType and gen. Test-only: lets tests execute the cmd
// synchronously and assert that the Gen field was captured at dispatch time.
func (m Model) FetchResourcesCmdForTest(resourceType string, gen domain.Gen) tea.Cmd {
	return m.fetchResources(resourceType, gen)
}

// FetchIdentityCmdForTest returns a tea.Cmd produced by fetchIdentity for
// the given gen. Test-only: lets tests execute the cmd synchronously and
// assert that the Gen field was captured at dispatch time.
func (m Model) FetchIdentityCmdForTest(gen domain.Gen) tea.Cmd {
	return m.fetchIdentity(gen)
}

// FetchRevealValueCmdForTest returns a tea.Cmd produced by fetchRevealValue
// for the given resourceType, resourceID, and gen. Test-only: lets tests
// execute the cmd synchronously and assert that the Gen field was captured at
// dispatch time.
func (m Model) FetchRevealValueCmdForTest(resourceType, resourceID string, gen domain.Gen) tea.Cmd {
	return m.fetchRevealValue(resourceType, resourceID, gen)
}
