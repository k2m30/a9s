// SPDX-License-Identifier: GPL-3.0-or-later

// app_accessors.go — exported Model accessors for tests in tests/unit/
// (package unit_test), which can't reach unexported fields from outside the
// package. Production code does not call them; test callers reach session state
// via m.Core().Session() (Core() returns the platform-agnostic *runtime.Core,
// keeping the session package out of internal/tui's import surface).
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// Core returns the runtime-owned *runtime.Core handle. Test-only accessor
// — production code uses m.core directly. Replaces the prior Session()
// accessor whose session-typed return value forced the tui package to
// import the core/session package.
func (m Model) Core() *runtime.Core {
	return m.core
}

// ListInstanceForTest returns the identity of the list screen on top, which a
// real dispatch stamps onto the page it produces. Test-only accessor: a test
// that hand-builds a page is standing in for the fetch command and has to
// stamp what the command would (tuitest.StampPage).
func (m Model) ListInstanceForTest() domain.Gen {
	return m.ctrl.GetListInstance()
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

// ActiveDetailResource is an exported test-only accessor for the resource shown
// in the active detail screen. Returns ok=false when the active screen is not a detail.
func (m Model) ActiveDetailResource() (resource.Resource, bool) {
	rs := m.activeRS()
	if rs.kind != rsKindDetail {
		return resource.Resource{}, false
	}
	body := m.ctrl.Snapshot().Body
	if body.Kind != app.BodyKindDetail || body.Detail == nil {
		return resource.Resource{}, false
	}
	return m.ctrl.GetDetailResource(), true
}

// ActiveListResources is an exported test-only accessor for the resource slice
// in the active list screen. Returns nil if the active screen is not a list.
func (m Model) ActiveListResources() []resource.Resource {
	rs := m.activeRS()
	if rs.kind != rsKindList {
		return nil
	}
	return m.ctrl.GetListAllResources()
}

// FetchResourcesCmdForTest returns a tea.Cmd produced by fetchResources for
// the given resourceType and gen. Test-only: lets tests execute the cmd
// synchronously and assert that the Gen field was captured at dispatch time.
func (m Model) FetchResourcesCmdForTest(resourceType string, gen domain.Gen) tea.Cmd {
	return m.fetchResources(resourceType, gen, messages.FetchProvenanceCanonicalList)
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
