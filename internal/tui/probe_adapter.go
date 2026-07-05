// probe_adapter.go — Bubble Tea adapter over runtime.Core probe methods. Each
// method captures ctx and clients, delegates to the corresponding Core method,
// and converts the result to TUI message types.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
				Expired: true,
			}
		}
		return runtime.CacheStoreToEvent(store)
	}
}

// probeEnrichment returns a tea.Cmd that runs the registered Wave-2 enricher
// for shortName and converts the result to EnrichmentCheckedMsg.
//
// Demo clients are real *awsclient.ServiceClients backed by typed fakes
// (internal/demo.NewServiceClients), so Wave-2 enrichers run against them the
// same way they run against live AWS clients — no demo-mode skip here.
func (m *Model) probeEnrichment(shortName string, gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	typeGen := m.core.EnrichmentTypeGen(shortName)
	if !m.core.HasIssueEnricher(shortName) {
		return nil
	}
	return func() tea.Msg {
		r := m.core.ProbeEnrichment(ctx, clients, shortName)
		return messages.EnrichmentChecked{
			ResourceType:     shortName,
			Issues:           r.Issues,
			Truncated:        r.Truncated,
			Findings:         r.Findings,
			AttentionDetails: r.AttentionDetails,
			FieldUpdates:     r.FieldUpdates,
			TruncatedIDs:     r.TruncatedIDs,
			Gen:              gen,
			TypeGen:          typeGen,
			Err:              r.Err,
		}
	}
}
