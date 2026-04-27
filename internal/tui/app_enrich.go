// app_enrich.go handles on-demand resource enrichment for detail views.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// handleEnrichDetail dispatches the enricher for the given resource type.
// Stamps the current enrichGen so stale results are discarded.
//
// Detail enrichers receive an *awsclient.DetailEnrichmentCtx — a composite of
// the AWS transport clients plus the session-scoped caches owned by
// sessionRuntime (e.g. policyDocCache). This keeps feature-specific state off
// *ServiceClients.
func (m Model) handleEnrichDetail(msg messages.EnrichDetailMsg) (tea.Model, tea.Cmd) {
	enricher := resource.GetDetailEnricher(msg.ResourceType)
	if enricher == nil {
		return m, nil
	}

	gen := m.EnrichGen
	dctx := &awsclient.DetailEnrichmentCtx{
		Clients:    m.clients,
		PolicyDocs: m.PolicyDocCache,
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.appCtx, 10*time.Second)
		defer cancel()

		enriched, err := enricher(ctx, dctx, msg.Resource)
		return messages.EnrichDetailResultMsg{
			ResourceType: msg.ResourceType,
			ResourceID:   msg.Resource.ID,
			EnrichedRes:  enriched,
			Err:          err,
			Generation:   gen,
		}
	}
}
