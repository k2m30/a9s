// probe_adapter.go — Bubble Tea adapter over runtime.Core probe methods.
//
// PR-05a-h5 (AS-151) moves probe logic to internal/runtime/probes.go.
// This file bridges the Core methods to the tea.Cmd factories that the TUI
// Update loop and handler files expect. Each method captures ctx and clients,
// delegates to the corresponding Core method, and converts the result to TUI
// message types.
package tui

import (
	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// loadAvailabilityCache returns a tea.Cmd that reads the availability cache
// from disk and converts the result to AvailabilityCacheLoadedMsg.
func (m *Model) loadAvailabilityCache() tea.Cmd {
	profile := m.profile
	region := m.region
	return func() tea.Msg {
		cf, err := m.core.LoadAvailabilityCache(profile, region)
		if err != nil || cf == nil {
			return messages.AvailabilityCacheLoadedMsg{
				Entries: make(map[string]int),
				Expired: true,
			}
		}
		entries := make(map[string]int, len(cf.Resources))
		truncated := make(map[string]bool)
		issueCounts := make(map[string]int)
		issueTruncated := make(map[string]bool)
		issueKnown := make(map[string]bool)
		for name, entry := range cf.Resources {
			if entry.Error == "" {
				entries[name] = entry.Count
				if entry.Truncated {
					truncated[name] = true
				}
				if entry.IssuesKnown {
					issueCounts[name] = entry.Issues
					issueKnown[name] = true
					if entry.IssuesTruncated {
						issueTruncated[name] = true
					}
				}
			}
		}
		return messages.AvailabilityCacheLoadedMsg{
			Entries:        entries,
			Truncated:      truncated,
			Expired:        cf.IsExpired(cache.DefaultTTL),
			IssueCounts:    issueCounts,
			IssueTruncated: issueTruncated,
			IssueKnown:     issueKnown,
		}
	}
}

// probeResourceAvailability returns a tea.Cmd that runs a Wave-1 availability
// probe for shortName and converts the result to AvailabilityCheckedMsg.
func (m *Model) probeResourceAvailability(shortName string, gen int) tea.Cmd {
	ctx, clients := m.appCtx, m.clients
	return func() tea.Msg {
		r := m.core.ProbeResourceAvailability(ctx, clients, shortName)
		return messages.AvailabilityCheckedMsg{
			ResourceType: shortName,
			HasResources: r.HasResources,
			Count:        r.Count,
			Truncated:    r.Truncated,
			Issues:       r.Issues,
			Resources:    r.Resources,
			Err:          r.Err,
			Gen:          gen,
		}
	}
}

// saveAvailabilityCache returns a tea.Cmd that persists the current
// availability state to disk. No-op when caching is disabled (noCache=true).
func (m *Model) saveAvailabilityCache() tea.Cmd {
	if m.noCache {
		return nil
	}
	profile := m.profile
	region := m.region

	// Collect availability, truncation, and issue counts from main menu.
	var entries map[string]int
	var truncatedMap map[string]bool
	var issueCounts map[string]int
	var issueTruncated map[string]bool
	var issueKnown map[string]bool
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		entries = menu.GetAvailability()
		truncatedMap = menu.GetTruncated()
		issueCounts = menu.GetIssueCounts()
		issueTruncated = menu.GetIssueTruncated()
		issueKnown = menu.GetIssueKnown()
	}
	if entries == nil {
		return nil
	}

	return func() tea.Msg {
		// Best-effort save — ignore cache write failures.
		_ = m.core.SaveAvailabilityCache(profile, region, entries, truncatedMap, issueCounts, issueTruncated, issueKnown)
		return nil
	}
}

// demoPrefetchCounts returns a tea.Cmd that synchronously calls all registered
// paginated fetchers and converts the result to AvailabilityPrefetchedMsg.
// Used when pre-supplied clients are present and no-cache is active so the
// main menu shows counts immediately without the async probe pipeline.
func (m *Model) demoPrefetchCounts() tea.Cmd {
	ctx, clients := m.appCtx, m.clients
	gen := m.AvailabilityGen
	return func() tea.Msg {
		r := m.core.DemoPrefetchCounts(ctx, clients)
		return messages.AvailabilityPrefetchedMsg{
			Entries:        r.Entries,
			Truncated:      r.Truncated,
			IssueCounts:    r.IssueCounts,
			IssueTruncated: r.IssueTruncated,
			Resources:      r.Resources,
			Pagination:     r.Pagination,
			Gen:            gen,
			PrefetchErr:    r.PrefetchErr,
		}
	}
}

// refreshResourceListWithEnrichmentRerun wraps the ordinary refresh fetch for
// a top-level list so that the ResourcesLoadedMsg it produces carries an
// enrichment-rerun token. The token is captured at Ctrl+R dispatch time and
// stamped into the message; the ResourcesLoadedMsg handler in app.go checks
// TypeGen in its tail branch to decide whether to seed probeResources and
// dispatch probeEnrichment. APIErrorMsg and any other message pass through
// unchanged.
func (m *Model) refreshResourceListWithEnrichmentRerun(
	rl views.ResourceListModel, tok int,
) tea.Cmd {
	inner := m.refreshResourceList(rl)
	return func() tea.Msg {
		msg := inner()
		if loaded, ok := msg.(messages.ResourcesLoadedMsg); ok {
			loaded.TypeGen = tok
			return loaded
		}
		return msg
	}
}

// buildEnrichQueue delegates to Core.buildEnrichQueue. Called by
// app_handlers_availability.go during Wave-2 enrichment dispatch setup.
func (m *Model) buildEnrichQueue() []string {
	return m.core.BuildEnrichQueue()
}

// probeEnrichment returns a tea.Cmd that runs the registered Wave-2 enricher
// for shortName and converts the result to EnrichmentCheckedMsg.
func (m *Model) probeEnrichment(shortName string, gen int) tea.Cmd {
	ctx, clients := m.appCtx, m.clients
	typeGen := m.EnrichmentTypeGen[shortName]
	if awsclient.IssueEnricherRegistry[shortName].Fn == nil {
		return nil
	}
	return func() tea.Msg {
		r := m.core.ProbeEnrichment(ctx, clients, shortName)
		return messages.EnrichmentCheckedMsg{
			ResourceType: shortName,
			Issues:       r.Issues,
			Truncated:    r.Truncated,
			Findings:     r.Findings,
			FieldUpdates: r.FieldUpdates,
			TruncatedIDs: r.TruncatedIDs,
			Gen:          gen,
			TypeGen:      typeGen,
			Err:          r.Err,
		}
	}
}
