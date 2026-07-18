// SPDX-License-Identifier: GPL-3.0-or-later

// probe_adapter.go — Bubble Tea adapter over runtime.Core probe methods. Each
// method captures ctx and clients, delegates to the corresponding Core method,
// and converts the result to TUI message types.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// loadAvailabilityCache returns a tea.Cmd that reads the per-type disk cache
// (profile/region set on m.core) and converts the result to
// messages.AvailabilityCacheLoaded via the shared runtime.CacheStoreToEvent
// conversion — the same one the headless/web executor path uses, so both
// renderers seed identically.
func (m *Model) loadAvailabilityCache() tea.Cmd {
	return func() tea.Msg {
		store := m.core.LoadAvailabilityCache()
		if store == nil {
			return messages.AvailabilityCacheLoaded{
				Entries: make(map[string]int),
			}
		}
		return runtime.CacheStoreToEvent(store)
	}
}
