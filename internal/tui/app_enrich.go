// app_enrich.go handles on-demand resource enrichment for detail views.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// handleEnrichDetail dispatches the enricher for the given resource type.
// Stamps the current enrichGen so stale results are discarded.
func (m Model) handleEnrichDetail(msg messages.EnrichDetailMsg) (tea.Model, tea.Cmd) {
	enricher := resource.GetDetailEnricher(msg.ResourceType)
	if enricher == nil {
		return m, nil
	}

	gen := m.enrichGen
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.appCtx, 10*time.Second)
		defer cancel()

		enriched, err := enricher(ctx, m.clients, msg.Resource)
		return messages.EnrichDetailResultMsg{
			ResourceType: msg.ResourceType,
			ResourceID:   msg.Resource.ID,
			EnrichedRes:  enriched,
			Err:          err,
			Generation:   gen,
		}
	}
}
